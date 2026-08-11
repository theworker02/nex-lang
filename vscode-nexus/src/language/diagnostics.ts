import * as vscode from "vscode";
import { Lexer } from "./lexer";
import { Parser } from "./parser";
import { MacroExpander } from "./macro";
import { OwnershipChecker } from "./checker";
import { checkEffects } from "./effects";
import { checkProgram } from "./static_check";
import { expandReflection } from "./reflection";
import { lowerSyntax } from "./syntax";

const DIAGNOSTIC_SOURCE = "nexus";

/**
 * Parses Nexus source and publishes diagnostics to the Problems panel.
 */
export class NexusDiagnostics {
  private readonly collection: vscode.DiagnosticCollection;
  private readonly timers = new Map<string, NodeJS.Timeout>();
  private readonly debounceMs = 250;

  constructor() {
    this.collection = vscode.languages.createDiagnosticCollection("nexus");
  }

  get disposable(): vscode.Disposable {
    return this.collection;
  }

  dispose(): void {
    for (const timer of this.timers.values()) {
      clearTimeout(timer);
    }
    this.timers.clear();
    this.collection.dispose();
  }

  register(context: vscode.ExtensionContext): void {
    context.subscriptions.push(this.collection);

    for (const doc of vscode.workspace.textDocuments) {
      this.schedule(doc);
    }

    context.subscriptions.push(
      vscode.workspace.onDidOpenTextDocument((doc) => this.schedule(doc)),
      vscode.workspace.onDidChangeTextDocument((e) =>
        this.schedule(e.document),
      ),
      vscode.workspace.onDidSaveTextDocument((doc) => this.schedule(doc, true)),
      vscode.workspace.onDidCloseTextDocument((doc) => {
        this.clearTimer(doc.uri);
        this.collection.delete(doc.uri);
      }),
      vscode.workspace.onDidChangeConfiguration((e) => {
        if (e.affectsConfiguration("nexus.diagnostics")) {
          for (const doc of vscode.workspace.textDocuments) {
            this.schedule(doc, true);
          }
        }
      }),
    );
  }

  private schedule(document: vscode.TextDocument, immediate = false): void {
    if (document.languageId !== "nexus") {
      return;
    }

    const key = document.uri.toString();
    this.clearTimer(document.uri);

    if (immediate) {
      this.refresh(document);
      return;
    }

    const timer = setTimeout(() => {
      this.timers.delete(key);
      this.refresh(document);
    }, this.debounceMs);
    this.timers.set(key, timer);
  }

  private clearTimer(uri: vscode.Uri): void {
    const key = uri.toString();
    const existing = this.timers.get(key);
    if (existing) {
      clearTimeout(existing);
      this.timers.delete(key);
    }
  }

  private refresh(document: vscode.TextDocument): void {
    const enabled = vscode.workspace
      .getConfiguration("nexus")
      .get<boolean>("diagnostics.enable", true);

    if (!enabled) {
      this.collection.delete(document.uri);
      return;
    }

    const diagnostics = collectParseDiagnostics(document);
    this.collection.set(document.uri, diagnostics);
  }
}

export function collectParseDiagnostics(
  document: vscode.TextDocument,
): vscode.Diagnostic[] {
  const lowered = lowerSyntax(document.getText());
  const lexer = new Lexer(lowered);
  const parser = new Parser(lexer);
  let program = parser.parseProgram();

  const diagnostics: vscode.Diagnostic[] = [];
  for (const message of parser.getErrors()) {
    diagnostics.push(errorToDiagnostic(document, message));
  }

  const macro = new MacroExpander().expand(program);
  program = macro.program;
  for (const message of macro.errors) {
    diagnostics.push(errorToDiagnostic(document, message));
  }

  program = expandReflection(program).program;

  const ownership = new OwnershipChecker().check(program);
  for (const d of ownership) {
    diagnostics.push(
      errorToDiagnostic(document, d.message, vscode.DiagnosticSeverity.Error),
    );
  }

  for (const d of checkEffects(program)) {
    diagnostics.push(
      errorToDiagnostic(document, d.message, vscode.DiagnosticSeverity.Error),
    );
  }

  for (const d of checkProgram(program)) {
    const severity =
      d.severity === "warning"
        ? vscode.DiagnosticSeverity.Warning
        : vscode.DiagnosticSeverity.Error;
    diagnostics.push(errorToDiagnostic(document, d.message, severity));
  }

  return diagnostics;
}

function errorToDiagnostic(
  document: vscode.TextDocument,
  message: string,
  severity: vscode.DiagnosticSeverity = vscode.DiagnosticSeverity.Error,
): vscode.Diagnostic {
  const match = /line (\d+), column (\d+)/i.exec(message);
  let range: vscode.Range;

  if (match) {
    const line = Math.max(0, Number.parseInt(match[1]!, 10) - 1);
    const column = Math.max(0, Number.parseInt(match[2]!, 10) - 1);
    const safeLine = Math.min(line, Math.max(0, document.lineCount - 1));
    const lineText = document.lineAt(safeLine).text;
    const start = Math.min(column, lineText.length);
    const end = Math.min(start + 1, Math.max(lineText.length, start + 1));
    range = new vscode.Range(safeLine, start, safeLine, end);
  } else {
    range =
      document.lineCount > 0
        ? document.lineAt(0).range
        : new vscode.Range(0, 0, 0, 0);
  }

  const diagnostic = new vscode.Diagnostic(range, message, severity);
  diagnostic.source = DIAGNOSTIC_SOURCE;
  return diagnostic;
}
