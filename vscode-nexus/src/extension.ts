import * as vscode from "vscode";
import * as path from "path";
import * as fs from "fs";
import { NexusCompletionItemProvider } from "./language/completions";
import { NexusDiagnostics } from "./language/diagnostics";
import { NexusDocumentSymbolProvider } from "./language/symbols";
import {
  DEFAULT_REGISTRY_URL,
  RegistryClient,
} from "./registry/client";
import {
  compileToLlvm,
  compileToNative,
  compileToWasm,
  evaluate,
  EngineTier,
} from "./compiler/engine";
import { runTests } from "./nextest";
import { findStdlibDir, MODULES_DIRNAME } from "./runtime/modules";

const NEXUS_LANGUAGE = "nexus";
const SELECTOR: vscode.DocumentSelector = { language: NEXUS_LANGUAGE };

let outputChannel: vscode.OutputChannel | undefined;
let diagnostics: NexusDiagnostics | undefined;

export function activate(context: vscode.ExtensionContext): void {
  outputChannel = vscode.window.createOutputChannel("Nexus");
  context.subscriptions.push(outputChannel);

  diagnostics = new NexusDiagnostics();
  diagnostics.register(context);
  context.subscriptions.push({
    dispose: () => diagnostics?.dispose(),
  });

  context.subscriptions.push(
    vscode.languages.registerCompletionItemProvider(
      SELECTOR,
      new NexusCompletionItemProvider(),
    ),
    vscode.languages.registerDocumentSymbolProvider(
      SELECTOR,
      new NexusDocumentSymbolProvider(),
    ),
  );

  context.subscriptions.push(
    vscode.commands.registerCommand("nexus.run", () => runCommand(false)),
    vscode.commands.registerCommand("nexus.runVm", () => runCommand(true)),
    vscode.commands.registerCommand("nexus.selfhost", () => selfhostCommand()),
    vscode.commands.registerCommand("nexus.test", () => testCommand()),
    vscode.commands.registerCommand("nexus.repl", () => replCommand()),
    vscode.commands.registerCommand("nexus.compileWasm", () =>
      compileCommand("wasm"),
    ),
    vscode.commands.registerCommand("nexus.compileLlvm", () =>
      compileCommand("llvm"),
    ),
    vscode.commands.registerCommand("nexus.compileNative", () =>
      compileCommand("native"),
    ),
    vscode.commands.registerCommand("nexus.publish", () => publishCommand()),
    vscode.commands.registerCommand("nexus.install", () => installCommand()),
  );

  outputChannel.appendLine(
    "Nexus TS toolchain ready: modules, bytecode VM, selfhost, tests, REPL, stdlib.",
  );
}

export function deactivate(): void {
  diagnostics?.dispose();
  diagnostics = undefined;
  outputChannel?.dispose();
  outputChannel = undefined;
}

function executionTier(forceVm?: boolean): EngineTier {
  if (forceVm) {
    return "vm";
  }
  const config = vscode.workspace.getConfiguration("nexus");
  const engine = config.get<string>("executionEngine") ?? "eval";
  return engine === "vm" ? "vm" : "eval";
}

async function runCommand(forceVm = false): Promise<void> {
  const editor = vscode.window.activeTextEditor;
  if (!editor) {
    void vscode.window.showErrorMessage("Nexus: no active editor to run.");
    return;
  }

  const document = editor.document;
  if (
    document.languageId !== NEXUS_LANGUAGE &&
    !document.fileName.endsWith(".nex")
  ) {
    const proceed = await vscode.window.showWarningMessage(
      "The active file does not appear to be a Nexus (.nex) file. Run anyway?",
      "Run",
      "Cancel",
    );
    if (proceed !== "Run") {
      return;
    }
  }

  const source = document.getText();
  const channel = ensureOutput();
  channel.clear();
  channel.show(true);
  const tier = executionTier(forceVm);
  channel.appendLine(
    `Running ${document.fileName || "untitled"} [${tier}]…`,
  );
  channel.appendLine("─".repeat(40));

  const workspaceRoot = getWorkspaceRoot() ?? path.dirname(document.fileName || process.cwd());

  try {
    const result = await evaluate(source, {
      tier,
      rootDir: workspaceRoot,
      modulesDir: path.join(workspaceRoot, MODULES_DIRNAME),
      stdlibDir: findStdlibDir(workspaceRoot),
      filePath: document.fileName || undefined,
      checkOwnership: false,
      enableEffects: false,
    });

    for (const d of result.diagnostics) {
      if (
        d.phase === "ownership" ||
        d.phase === "parse" ||
        d.phase === "effects" ||
        d.phase === "vm"
      ) {
        channel.appendLine(`[${d.phase}] ${d.message}`);
      }
    }

    for (const line of result.output) {
      channel.appendLine(line);
    }

    if (result.value.type === "ERROR") {
      channel.appendLine(result.value.inspect());
      void vscode.window.showErrorMessage(
        `Nexus runtime error: ${result.value.inspect()}`,
      );
      return;
    }

    if (result.value.type !== "NULL") {
      channel.appendLine(result.value.inspect());
    }

    channel.appendLine("─".repeat(40));
    channel.appendLine("Finished.");
  } catch (err) {
    const message = err instanceof Error ? err.message : String(err);
    channel.appendLine(`Fatal error: ${message}`);
    void vscode.window.showErrorMessage(`Nexus: ${message}`);
  }
}

async function selfhostCommand(): Promise<void> {
  const editor = vscode.window.activeTextEditor;
  if (!editor) {
    void vscode.window.showErrorMessage("Nexus: no active editor to selfhost.");
    return;
  }
  const document = editor.document;
  if (!document.fileName.endsWith(".nex")) {
    void vscode.window.showErrorMessage("Nexus selfhost: save a .nex file first.");
    return;
  }
  await document.save();

  const workspaceRoot =
    getWorkspaceRoot() ?? path.dirname(document.fileName);
  const channel = ensureOutput();
  channel.clear();
  channel.show(true);
  channel.appendLine(`Selfhosting ${document.fileName}…`);
  channel.appendLine("─".repeat(40));

  const candidates = [
    path.join(workspaceRoot, "vscode-nexus", "out", "cli.js"),
    path.join(workspaceRoot, "out", "cli.js"),
    path.join(__dirname, "cli.js"),
  ];
  const cli = candidates.find((c) => fs.existsSync(c));
  if (!cli) {
    void vscode.window.showErrorMessage(
      "Nexus selfhost: could not find out/cli.js (run npm run compile).",
    );
    return;
  }

  const { spawn } = await import("child_process");
  const cliRoot = path.dirname(path.dirname(cli));
  await new Promise<void>((resolve) => {
    const child = spawn(
      process.execPath,
      [cli, "selfhost", document.fileName],
      { cwd: cliRoot },
    );
    child.stdout.on("data", (buf: Buffer) => {
      channel.append(buf.toString());
    });
    child.stderr.on("data", (buf: Buffer) => {
      channel.append(buf.toString());
    });
    child.on("close", (code) => {
      channel.appendLine("─".repeat(40));
      channel.appendLine(
        code === 0 ? "Selfhost finished." : `Selfhost exited ${code}`,
      );
      resolve();
    });
  });
}

async function testCommand(): Promise<void> {
  const workspaceRoot = getWorkspaceRoot();
  if (!workspaceRoot) {
    void vscode.window.showErrorMessage(
      "Nexus: open a workspace folder before running tests.",
    );
    return;
  }
  const channel = ensureOutput();
  channel.clear();
  channel.show(true);
  channel.appendLine(`Running Nexus tests in ${workspaceRoot}…`);
  channel.appendLine("─".repeat(40));

  try {
    const summary = await runTests({
      rootDir: workspaceRoot,
      stdlibDir: findStdlibDir(workspaceRoot),
      out: (line) => channel.appendLine(line),
    });
    if (summary.failed > 0) {
      void vscode.window.showErrorMessage(
        `Nexus tests: ${summary.failed} failed, ${summary.passed} passed`,
      );
    } else {
      void vscode.window.showInformationMessage(
        `Nexus tests: ${summary.passed} passed`,
      );
    }
  } catch (err) {
    const message = err instanceof Error ? err.message : String(err);
    channel.appendLine(message);
    void vscode.window.showErrorMessage(`Nexus test: ${message}`);
  }
}

async function replCommand(): Promise<void> {
  const workspaceRoot = getWorkspaceRoot() ?? process.cwd();
  const config = vscode.workspace.getConfiguration("nexus");
  const engine =
    config.get<string>("executionEngine") === "vm" ? "vm" : "eval";

  const terminal = vscode.window.createTerminal({
    name: "Nexus REPL",
    cwd: workspaceRoot,
  });
  terminal.show();

  // Prefer compiled CLI under the extension install / workspace vscode-nexus
  const candidates = [
    path.join(workspaceRoot, "vscode-nexus", "out", "cli.js"),
    path.join(workspaceRoot, "out", "cli.js"),
    path.join(__dirname, "cli.js"),
  ];
  const cli = candidates.find((c) => fs.existsSync(c));
  if (!cli) {
    void vscode.window.showErrorMessage(
      "Nexus REPL: could not find out/cli.js (run npm run compile in vscode-nexus).",
    );
    return;
  }
  const vmFlag = engine === "vm" ? " --vm" : "";
  terminal.sendText(`node "${cli}" repl${vmFlag}`);
}

async function compileCommand(
  target: "wasm" | "llvm" | "native",
): Promise<void> {
  const editor = vscode.window.activeTextEditor;
  if (!editor) {
    void vscode.window.showErrorMessage("Nexus: no active editor to compile.");
    return;
  }

  const document = editor.document;
  const source = document.getText();
  const channel = ensureOutput();
  channel.show(true);

  const config = vscode.workspace.getConfiguration("nexus");
  const configuredOut = config.get<string>("compileOutputDir");
  const workspaceRoot = getWorkspaceRoot();
  const outputDir =
    configuredOut ||
    (workspaceRoot
      ? path.join(workspaceRoot, "nex-out")
      : path.join(path.dirname(document.fileName || process.cwd()), "nex-out"));

  const base = path.basename(
    document.fileName || "main.nex",
    path.extname(document.fileName || "main.nex"),
  );

  channel.appendLine(`Compiling → ${target} into ${outputDir}…`);

  try {
    const artifact =
      target === "wasm"
        ? compileToWasm(source, outputDir)
        : target === "llvm"
          ? compileToLlvm(source, outputDir)
          : compileToNative(source, outputDir);

    for (const d of artifact.diagnostics) {
      channel.appendLine(`[${d.phase}] ${d.message}`);
    }

    if (artifact.codegen) {
      for (const w of artifact.codegen.warnings) {
        channel.appendLine(`[codegen] ${w}`);
      }
    }

    if (artifact.codegen && !artifact.outputPath) {
      const { writeFileSync, mkdirSync } = await import("fs");
      mkdirSync(outputDir, { recursive: true });
      const ext = target === "llvm" ? "ll" : "wat";
      const outPath = path.join(outputDir, `${base}.${ext}`);
      writeFileSync(outPath, artifact.codegen.source, "utf8");
      channel.appendLine(`Wrote ${outPath}`);
    } else if (artifact.outputPath) {
      channel.appendLine(`Wrote ${artifact.outputPath}`);
    }

    void vscode.window.showInformationMessage(
      `Nexus: compiled ${target} → ${artifact.outputPath ?? outputDir}`,
    );
  } catch (err) {
    const message = err instanceof Error ? err.message : String(err);
    channel.appendLine(`Compile failed: ${message}`);
    void vscode.window.showErrorMessage(`Nexus compile failed: ${message}`);
  }
}

async function publishCommand(): Promise<void> {
  const workspaceRoot = getWorkspaceRoot();
  if (!workspaceRoot) {
    void vscode.window.showErrorMessage(
      "Nexus: open a workspace folder before publishing.",
    );
    return;
  }

  const channel = ensureOutput();
  channel.show(true);
  channel.appendLine(`Publishing workspace: ${workspaceRoot}`);

  try {
    const client = createClient();
    channel.appendLine(`Registry: ${client.baseUrl}`);

    await vscode.window.withProgress(
      {
        location: vscode.ProgressLocation.Notification,
        title: "Nexus: publishing package…",
        cancellable: false,
      },
      async () => {
        const result = await client.publish(workspaceRoot);
        const msg =
          result.message ||
          `Published ${result.package.name}@${result.package.version}`;
        channel.appendLine(msg);
        channel.appendLine(`Checksum: ${result.package.checksum}`);
        void vscode.window.showInformationMessage(`Nexus: ${msg}`);
      },
    );
  } catch (err) {
    const message = err instanceof Error ? err.message : String(err);
    channel.appendLine(`Publish failed: ${message}`);
    void vscode.window.showErrorMessage(`Nexus publish failed: ${message}`);
  }
}

async function installCommand(): Promise<void> {
  const workspaceRoot = getWorkspaceRoot();
  if (!workspaceRoot) {
    void vscode.window.showErrorMessage(
      "Nexus: open a workspace folder before installing packages.",
    );
    return;
  }

  const spec = await vscode.window.showInputBox({
    title: "Nexus: Install Package",
    prompt: "Package name (optional @version)",
    placeHolder: "leftpad@1.0.0",
    ignoreFocusOut: true,
    validateInput: (value) =>
      value.trim().length === 0 ? "Package name is required" : undefined,
  });

  if (!spec) {
    return;
  }

  const channel = ensureOutput();
  channel.show(true);
  channel.appendLine(`Installing ${spec.trim()}…`);

  try {
    const client = createClient();
    await vscode.window.withProgress(
      {
        location: vscode.ProgressLocation.Notification,
        title: `Nexus: installing ${spec.trim()}…`,
        cancellable: false,
      },
      async () => {
        const result = await client.install(workspaceRoot, spec.trim());
        const msg = `Installed ${result.name}@${result.version} → ${result.destination}`;
        channel.appendLine(msg);
        void vscode.window.showInformationMessage(`Nexus: ${msg}`);
      },
    );
  } catch (err) {
    const message = err instanceof Error ? err.message : String(err);
    channel.appendLine(`Install failed: ${message}`);
    void vscode.window.showErrorMessage(`Nexus install failed: ${message}`);
  }
}

function createClient(): RegistryClient {
  const config = vscode.workspace.getConfiguration("nexus");
  const registryUrl =
    config.get<string>("registryUrl") ||
    process.env.NEX_REGISTRY_URL ||
    DEFAULT_REGISTRY_URL;
  return new RegistryClient({ registryUrl });
}

function getWorkspaceRoot(): string | undefined {
  const folders = vscode.workspace.workspaceFolders;
  if (!folders || folders.length === 0) {
    return undefined;
  }

  const editor = vscode.window.activeTextEditor;
  if (editor?.document.uri.scheme === "file") {
    const folder = vscode.workspace.getWorkspaceFolder(editor.document.uri);
    if (folder) {
      return folder.uri.fsPath;
    }
  }

  return folders[0]?.uri.fsPath;
}

function ensureOutput(): vscode.OutputChannel {
  if (!outputChannel) {
    outputChannel = vscode.window.createOutputChannel("Nexus");
  }
  return outputChannel;
}
