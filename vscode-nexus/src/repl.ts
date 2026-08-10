/**
 * Interactive Nexus REPL (tree-walk or bytecode VM).
 */

import * as readline from "readline";
import * as path from "path";
import { evaluate } from "./compiler/engine";
import { findStdlibDir, MODULES_DIRNAME } from "./runtime/modules";
import { BytecodeCompiler } from "./vm/compiler";
import { VirtualMachine } from "./vm/vm";
import { BUILTIN_NAMES } from "./language/builtins";
import { SymbolTable } from "./vm/symbols";
import { NexusObject, NULL_OBJ } from "./language/values";
import { Lexer } from "./language/lexer";
import { Parser } from "./language/parser";
import { lowerSyntax } from "./language/syntax";
import { MacroExpander } from "./language/macro";

export interface ReplConfig {
  engine?: "eval" | "vm";
  rootDir?: string;
  stdlibDir?: string;
  prompt?: string;
  input?: NodeJS.ReadableStream;
  output?: NodeJS.WritableStream;
}

export async function runRepl(cfg: ReplConfig = {}): Promise<void> {
  const engine = cfg.engine ?? "eval";
  const rootDir = cfg.rootDir ?? process.cwd();
  const stdlibDir =
    cfg.stdlibDir !== undefined ? cfg.stdlibDir : findStdlibDir(rootDir);
  const prompt = cfg.prompt ?? "nex> ";
  const input = cfg.input ?? process.stdin;
  const output = cfg.output ?? process.stdout;

  output.write("Nexus REPL — :help for commands, :quit to exit\n");
  output.write(`engine=${engine}\n`);

  let pending = "";
  let currentEngine = engine;

  // VM persistent state
  let constants: NexusObject[] = [];
  let globals: (NexusObject | undefined)[] = new Array(65536);
  let symbolTable = new SymbolTable();
  for (let i = 0; i < BUILTIN_NAMES.length; i++) {
    symbolTable.defineBuiltin(i, BUILTIN_NAMES[i]!);
  }

  const rl = readline.createInterface({ input, output, terminal: true });

  const ask = (): Promise<string | null> =>
    new Promise((resolve) => {
      rl.question(pending ? "...  " : prompt, (line) => {
        resolve(line);
      });
      rl.once("close", () => resolve(null));
    });

  while (true) {
    const line = await ask();
    if (line === null) {
      output.write("\n");
      break;
    }
    const trim = line.trim();

    if (!pending && trim.startsWith(":")) {
      if (trim === ":quit" || trim === ":exit") {
        break;
      }
      if (trim === ":help") {
        output.write(
          "Commands: :help :quit :engine eval|vm :clear\n",
        );
        continue;
      }
      if (trim.startsWith(":engine")) {
        const parts = trim.split(/\s+/);
        const next = parts[1];
        if (next === "eval" || next === "vm") {
          currentEngine = next;
          output.write(`engine=${currentEngine}\n`);
        } else {
          output.write("usage: :engine eval|vm\n");
        }
        continue;
      }
      if (trim === ":clear") {
        pending = "";
        constants = [];
        globals = new Array(65536);
        symbolTable = new SymbolTable();
        for (let i = 0; i < BUILTIN_NAMES.length; i++) {
          symbolTable.defineBuiltin(i, BUILTIN_NAMES[i]!);
        }
        output.write("state cleared\n");
        continue;
      }
      output.write(`unknown command ${trim}\n`);
      continue;
    }

    pending = pending ? `${pending}\n${line}` : line;
    if (!isBalanced(pending)) {
      continue;
    }
    const source = pending;
    pending = "";

    try {
      if (currentEngine === "vm") {
        const lowered = lowerSyntax(source);
        const lexer = new Lexer(lowered);
        const parser = new Parser(lexer);
        let program = parser.parseProgram();
        if (parser.getErrors().length > 0) {
          output.write(parser.getErrors().join("\n") + "\n");
          continue;
        }
        program = new MacroExpander().expand(program).program;
        const compiler = new BytecodeCompiler(symbolTable, constants);
        const err = compiler.compile(program);
        if (err) {
          output.write(err + "\n");
          continue;
        }
        const bytecode = compiler.bytecode();
        constants = bytecode.constants;
        symbolTable = compiler.symbolTable;
        const vm = new VirtualMachine(bytecode, globals);
        const runErr = vm.run();
        globals = vm.globals;
        for (const o of vm.output) {
          output.write(o + "\n");
        }
        if (runErr) {
          output.write(`ERROR: ${runErr}\n`);
        } else {
          const v = vm.lastPoppedStackElem();
          if (v !== NULL_OBJ && v.type !== "NULL") {
            output.write(v.inspect() + "\n");
          }
        }
      } else {
        const result = await evaluate(source, {
          tier: "eval",
          rootDir,
          modulesDir: path.join(rootDir, MODULES_DIRNAME),
          stdlibDir,
          checkOwnership: false,
          enableEffects: false,
        });
        for (const o of result.output) {
          output.write(o + "\n");
        }
        if (result.value.type === "ERROR") {
          output.write(result.value.inspect() + "\n");
        } else if (result.value.type !== "NULL") {
          output.write(result.value.inspect() + "\n");
        }
      }
    } catch (err) {
      output.write(
        `ERROR: ${err instanceof Error ? err.message : String(err)}\n`,
      );
    }
  }

  rl.close();
}

function isBalanced(source: string): boolean {
  let braces = 0;
  let parens = 0;
  let brackets = 0;
  let inString = false;
  for (let i = 0; i < source.length; i++) {
    const ch = source[i]!;
    if (ch === '"' && source[i - 1] !== "\\") {
      inString = !inString;
      continue;
    }
    if (inString) {
      continue;
    }
    if (ch === "{") {
      braces++;
    } else if (ch === "}") {
      braces--;
    } else if (ch === "(") {
      parens++;
    } else if (ch === ")") {
      parens--;
    } else if (ch === "[") {
      brackets++;
    } else if (ch === "]") {
      brackets--;
    }
  }
  return braces <= 0 && parens <= 0 && brackets <= 0;
}
