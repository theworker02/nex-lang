import * as vscode from "vscode";
import { Lexer, Token } from "./lexer";
import {
  Expression,
  FunctionLiteral,
  LetStatement,
  Parser,
  Program,
  Statement,
} from "./parser";

/**
 * Outline / breadcrumbs for `let` bindings and `fn` definitions.
 */
export class NexusDocumentSymbolProvider
  implements vscode.DocumentSymbolProvider
{
  provideDocumentSymbols(
    document: vscode.TextDocument,
    _token: vscode.CancellationToken,
  ): vscode.DocumentSymbol[] {
    const lexer = new Lexer(document.getText());
    const parser = new Parser(lexer);
    const program = parser.parseProgram();
    return collectSymbols(document, program);
  }
}

function collectSymbols(
  document: vscode.TextDocument,
  program: Program,
): vscode.DocumentSymbol[] {
  const symbols: vscode.DocumentSymbol[] = [];

  for (const stmt of program.statements) {
    const symbol = statementToSymbol(document, stmt);
    if (symbol) {
      symbols.push(symbol);
    }
  }

  return symbols;
}

function statementToSymbol(
  document: vscode.TextDocument,
  stmt: Statement,
): vscode.DocumentSymbol | undefined {
  if (stmt.type !== "LetStatement") {
    return undefined;
  }

  const letStmt = stmt as LetStatement;
  const name = letStmt.name.value;
  const nameRange = tokenRange(document, letStmt.name.token);
  const fullRange = expandRange(
    document,
    tokenRange(document, letStmt.token),
    expressionEndRange(document, letStmt.value),
  );

  const fn = asFunctionLiteral(letStmt.value);
  if (fn) {
    const params = fn.parameters.map((p) => p.value).join(", ");
    const symbol = new vscode.DocumentSymbol(
      name,
      `fn(${params})`,
      vscode.SymbolKind.Function,
      fullRange,
      nameRange,
    );

    for (const nested of fn.body.statements) {
      const child = statementToSymbol(document, nested);
      if (child) {
        symbol.children.push(child);
      }
    }
    return symbol;
  }

  return new vscode.DocumentSymbol(
    name,
    "let",
    vscode.SymbolKind.Variable,
    fullRange,
    nameRange,
  );
}

function asFunctionLiteral(
  expr: Expression | null,
): FunctionLiteral | undefined {
  if (expr && expr.type === "FunctionLiteral") {
    return expr as FunctionLiteral;
  }
  return undefined;
}

function expressionEndRange(
  document: vscode.TextDocument,
  expr: Expression | null,
): vscode.Range {
  if (!expr) {
    return new vscode.Range(0, 0, 0, 0);
  }

  if (expr.type === "FunctionLiteral") {
    return tokenRange(document, (expr as FunctionLiteral).body.token);
  }

  const token = getExpressionToken(expr);
  if (token) {
    return tokenRange(document, token);
  }
  return new vscode.Range(0, 0, 0, 0);
}

function getExpressionToken(expr: Expression): Token | undefined {
  if ("token" in expr) {
    return (expr as { token: Token }).token;
  }
  return undefined;
}

function tokenRange(document: vscode.TextDocument, token: Token): vscode.Range {
  const line = Math.max(0, token.line - 1);
  const character = Math.max(0, token.column - 1);
  const safeLine = Math.min(line, Math.max(0, document.lineCount - 1));
  const lineText = document.lineAt(safeLine).text;
  const start = Math.min(character, lineText.length);
  const end = Math.min(
    start + Math.max(token.literal.length, 1),
    lineText.length,
  );
  return new vscode.Range(safeLine, start, safeLine, Math.max(end, start));
}

function expandRange(
  document: vscode.TextDocument,
  start: vscode.Range,
  end: vscode.Range,
): vscode.Range {
  const a = start.start.isBefore(end.start) ? start.start : end.start;
  let b = start.end.isAfter(end.end) ? start.end : end.end;

  if (b.isEqual(a)) {
    b = document.lineAt(a.line).range.end;
  }
  return new vscode.Range(a, b);
}
