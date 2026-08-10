/**
 * Multi-tier compiler engine: tree-walk eval, bytecode VM, WASM / LLVM codegen.
 */

import * as fs from "fs";
import * as path from "path";
import { Lexer } from "../language/lexer";
import { Parser, Program } from "../language/parser";
import { MacroExpander } from "../language/macro";
import { OwnershipChecker, CheckDiagnostic } from "../language/checker";
import {
  runSourceAsync,
  EvalResult,
  Environment,
} from "../language/evaluator";
import { lowerSyntax } from "../language/syntax";
import { expandReflection } from "../language/reflection";
import { inferRegions, RegionInfo } from "../language/regions";
import { checkEffects } from "../language/effects";
import {
  CodeGenerator,
  CodegenResult,
  CodegenTarget,
  generateLlvm,
  generateWasm,
} from "./codegen";
import { resetMemoryEngine, getMemoryEngine } from "../runtime/memory";
import { installStdlib } from "../std";
import { findStdlibDir, MODULES_DIRNAME } from "../runtime/modules";
import { compileProgram, runBytecode } from "../vm";
import { ErrorObj, NexusObject } from "../language/values";
import type { WebHost } from "../host";

export type EngineTier = "eval" | "vm" | "wasm" | "llvm" | "native";

export interface CompileOptions {
  tier?: EngineTier;
  checkOwnership?: boolean;
  enableReflection?: boolean;
  enableRegions?: boolean;
  enableEffects?: boolean;
  outputDir?: string;
  fileName?: string;
  rootDir?: string;
  modulesDir?: string;
  stdlibDir?: string;
  filePath?: string;
  /** Extra top-level bindings (e.g. `__argv__` for selfhost). */
  bindings?: Record<string, NexusObject>;
  /** Optional HTTP/registry host (keeps process alive when routes register). */
  webHost?: WebHost;
}

export interface PipelineDiagnostic {
  message: string;
  line?: number;
  column?: number;
  phase:
    | "lex"
    | "parse"
    | "macro"
    | "syntax"
    | "reflection"
    | "region"
    | "ownership"
    | "effects"
    | "codegen"
    | "eval"
    | "vm";
}

export interface CompileArtifact {
  program: Program;
  diagnostics: PipelineDiagnostic[];
  regions: RegionInfo[];
  codegen?: CodegenResult;
  outputPath?: string;
}

/**
 * Multi-tier high-performance compiler engine.
 * Fast path: tree-walking evaluator. Second tier: bytecode VM.
 * Slow path: WASM / LLVM codegen.
 */
export class CompilerEngine {
  compile(source: string, options: CompileOptions = {}): CompileArtifact {
    const diagnostics: PipelineDiagnostic[] = [];
    const checkOwnership = options.checkOwnership !== false;

    const lowered = lowerSyntax(source);

    const lexer = new Lexer(lowered);
    const parser = new Parser(lexer);
    let program = parser.parseProgram();
    for (const err of parser.getErrors()) {
      diagnostics.push({ message: err, phase: "parse" });
    }

    const macro = new MacroExpander().expand(program);
    program = macro.program;
    for (const err of macro.errors) {
      diagnostics.push({ message: err, phase: "macro" });
    }

    if (options.enableReflection !== false) {
      const reflected = expandReflection(program);
      program = reflected.program;
      for (const err of reflected.errors) {
        diagnostics.push({ message: err, phase: "reflection" });
      }
    }

    let regions: RegionInfo[] = [];
    if (options.enableRegions !== false) {
      const regionResult = inferRegions(program);
      regions = regionResult.regions;
      program = regionResult.program;
      for (const err of regionResult.errors) {
        diagnostics.push({ message: err, phase: "region" });
      }
    }

    if (checkOwnership) {
      const checker = new OwnershipChecker();
      const ownershipErrors = checker.check(program);
      for (const d of ownershipErrors) {
        diagnostics.push(checkToPipeline(d, "ownership"));
      }
    }

    if (options.enableEffects !== false) {
      const effectDiags = checkEffects(program);
      for (const d of effectDiags) {
        diagnostics.push({
          message: d.message,
          line: d.line,
          column: d.column,
          phase: "effects",
        });
      }
    }

    const artifact: CompileArtifact = {
      program,
      diagnostics,
      regions,
    };

    const tier = options.tier ?? "eval";
    if (tier === "wasm" || tier === "native") {
      artifact.codegen = generateWasm(program);
      if (options.outputDir) {
        artifact.outputPath = writeCodegen(
          artifact.codegen,
          options.outputDir,
          options.fileName ?? "main",
          "wat",
        );
      }
    } else if (tier === "llvm") {
      artifact.codegen = generateLlvm(program);
      if (options.outputDir) {
        artifact.outputPath = writeCodegen(
          artifact.codegen,
          options.outputDir,
          options.fileName ?? "main",
          "ll",
        );
      }
    }

    return artifact;
  }

  async evaluate(
    source: string,
    options: CompileOptions = {},
  ): Promise<EvalResult & { diagnostics: PipelineDiagnostic[] }> {
    resetMemoryEngine();
    const tier = options.tier ?? "eval";
    const artifact = this.compile(source, {
      ...options,
      tier: tier === "vm" ? "eval" : tier,
    });
    const hardErrors = artifact.diagnostics.filter(
      (d) =>
        d.phase === "parse" ||
        d.phase === "macro" ||
        d.phase === "ownership" ||
        d.phase === "effects",
    );
    if (hardErrors.some((d) => d.phase === "parse" || d.phase === "ownership")) {
      return {
        value: new ErrorObj(hardErrors.map((d) => d.message).join("\n")),
        output: [],
        diagnostics: artifact.diagnostics,
      };
    }

    const rootDir = options.rootDir ?? process.cwd();
    const modulesDir =
      options.modulesDir ?? path.join(rootDir, MODULES_DIRNAME);
    const stdlibDir =
      options.stdlibDir !== undefined
        ? options.stdlibDir
        : findStdlibDir(rootDir);

    if (tier === "vm") {
      const compiled = compileProgram(artifact.program);
      if (compiled.error || !compiled.bytecode) {
        return {
          value: new ErrorObj(compiled.error ?? "vm compile failed"),
          output: [],
          diagnostics: [
            ...artifact.diagnostics,
            {
              message: compiled.error ?? "vm compile failed",
              phase: "vm",
            },
          ],
        };
      }
      const result = runBytecode(compiled.bytecode);
      return {
        value: result.value,
        output: result.output,
        diagnostics: result.error
          ? [
              ...artifact.diagnostics,
              { message: result.error, phase: "vm" },
            ]
          : artifact.diagnostics,
      };
    }

    const env = new Environment();
    installStdlib(env);
    if (options.bindings) {
      for (const [name, value] of Object.entries(options.bindings)) {
        env.set(name, value);
      }
    }
    void getMemoryEngine();
    const result = await runSourceAsync(source, env, artifact.program, {
      rootDir,
      modulesDir,
      stdlibDir,
      filePath: options.filePath,
      onReady: options.webHost
        ? ({ env: e, evaluator }) => {
            options.webHost!.install(e, (fn, args) =>
              evaluator.applyFunction(fn, args),
            );
          }
        : undefined,
    });
    return { ...result, diagnostics: artifact.diagnostics };
  }

  compileToWasm(source: string, outputDir?: string, fileName = "main"): CompileArtifact {
    return this.compile(source, { tier: "wasm", outputDir, fileName });
  }

  compileToLlvm(source: string, outputDir?: string, fileName = "main"): CompileArtifact {
    return this.compile(source, { tier: "llvm", outputDir, fileName });
  }

  compileNative(source: string, outputDir?: string, fileName = "main"): CompileArtifact {
    const artifact = this.compile(source, { tier: "native", outputDir, fileName });
    if (artifact.codegen && outputDir) {
      const note = path.join(outputDir, "NATIVE.md");
      fs.writeFileSync(
        note,
        `# Nexus native compile path

Generated WebAssembly text at: ${artifact.outputPath ?? "(in-memory)"}

To produce a binary module:
  wat2wasm main.wat -o main.wasm

To go via LLVM (alternative artifact):
  Use the Nexus: Compile to LLVM command, then:
  llc main.ll -o main.s
  clang main.s -o main
`,
        "utf8",
      );
    }
    return artifact;
  }
}

function checkToPipeline(
  d: CheckDiagnostic,
  phase: PipelineDiagnostic["phase"],
): PipelineDiagnostic {
  return {
    message: d.message,
    line: d.line,
    column: d.column,
    phase,
  };
}

function writeCodegen(
  codegen: CodegenResult,
  outputDir: string,
  fileName: string,
  ext: string,
): string {
  fs.mkdirSync(outputDir, { recursive: true });
  const outPath = path.join(outputDir, `${fileName}.${ext}`);
  fs.writeFileSync(outPath, codegen.source, "utf8");
  return outPath;
}

export const defaultEngine = new CompilerEngine();

export function compile(
  source: string,
  options?: CompileOptions,
): CompileArtifact {
  return defaultEngine.compile(source, options);
}

export async function evaluate(
  source: string,
  options?: CompileOptions,
): Promise<EvalResult & { diagnostics: PipelineDiagnostic[] }> {
  return defaultEngine.evaluate(source, options);
}

export function compileToWasm(
  source: string,
  outputDir?: string,
): CompileArtifact {
  return defaultEngine.compileToWasm(source, outputDir);
}

export function compileToLlvm(
  source: string,
  outputDir?: string,
): CompileArtifact {
  return defaultEngine.compileToLlvm(source, outputDir);
}

export function compileToNative(
  source: string,
  outputDir?: string,
): CompileArtifact {
  return defaultEngine.compileNative(source, outputDir);
}

export { CodeGenerator, generateWasm, generateLlvm };
export type { CodegenResult, CodegenTarget };
