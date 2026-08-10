import * as vscode from "vscode";

interface KeywordCompletion {
  label: string;
  kind: vscode.CompletionItemKind;
  detail: string;
  insertText?: string | vscode.SnippetString;
  documentation?: string;
}

const KEYWORD_ITEMS: KeywordCompletion[] = [
  {
    label: "let",
    kind: vscode.CompletionItemKind.Keyword,
    detail: "Variable declaration",
    insertText: new vscode.SnippetString("let ${1:name} = ${2:value};"),
  },
  {
    label: "let mut",
    kind: vscode.CompletionItemKind.Keyword,
    detail: "Mutable binding",
    insertText: new vscode.SnippetString("let mut ${1:name} = ${2:value};"),
  },
  {
    label: "fn",
    kind: vscode.CompletionItemKind.Keyword,
    detail: "Function literal",
    insertText: new vscode.SnippetString(
      "fn(${1:params}) {\n\t${2:return ${3:0};}\n}",
    ),
  },
  {
    label: "enum",
    kind: vscode.CompletionItemKind.Keyword,
    detail: "Algebraic data type",
    insertText: new vscode.SnippetString(
      "enum ${1:Name} {\n\t${2:Variant},\n\t${3:Other}(${4:x})\n}",
    ),
  },
  {
    label: "match",
    kind: vscode.CompletionItemKind.Keyword,
    detail: "Exhaustive pattern match",
    insertText: new vscode.SnippetString(
      "match ${1:value} {\n\t${2:Pattern} => { $0 },\n}",
    ),
  },
  {
    label: "struct",
    kind: vscode.CompletionItemKind.Keyword,
    detail: "Struct type",
    insertText: new vscode.SnippetString(
      "struct ${1:Name} {\n\t${2:field}: ${3:int}\n}",
    ),
  },
  {
    label: "effect",
    kind: vscode.CompletionItemKind.Keyword,
    detail: "Algebraic effect",
    insertText: new vscode.SnippetString(
      "effect ${1:IO} {\n\t${2:print}(${3:msg})\n}",
    ),
  },
  {
    label: "perform",
    kind: vscode.CompletionItemKind.Keyword,
    detail: "Perform an effect operation",
    insertText: new vscode.SnippetString(
      "perform ${1:IO}::${2:print}(${3:args})",
    ),
  },
  {
    label: "handle",
    kind: vscode.CompletionItemKind.Keyword,
    detail: "Handle effect operations",
    insertText: new vscode.SnippetString(
      "handle {\n\t$1\n} with ${2:IO} {\n\t${3:print}(${4:msg}) => { resume($5); }\n}",
    ),
  },
  {
    label: "region",
    kind: vscode.CompletionItemKind.Keyword,
    detail: "Lexical memory region",
    insertText: new vscode.SnippetString("region ${1:r} {\n\t$0\n}"),
  },
  {
    label: "async",
    kind: vscode.CompletionItemKind.Keyword,
    detail: "Async block / function",
    insertText: new vscode.SnippetString("async {\n\t$0\n}"),
  },
  {
    label: "await",
    kind: vscode.CompletionItemKind.Keyword,
    detail: "Await a promise",
    insertText: new vscode.SnippetString("await ${1:expr}"),
  },
  {
    label: "spawn",
    kind: vscode.CompletionItemKind.Keyword,
    detail: "Spawn green task",
    insertText: new vscode.SnippetString("spawn ${1:expr}"),
  },
  {
    label: "chan",
    kind: vscode.CompletionItemKind.Keyword,
    detail: "Message channel",
    insertText: new vscode.SnippetString("chan(${1:0})"),
  },
  {
    label: "macro rules!",
    kind: vscode.CompletionItemKind.Snippet,
    detail: "Hygienic macro",
    insertText: new vscode.SnippetString(
      "macro rules! ${1:name} {\n\t($${2:x}:expr) => { $0 }\n}",
    ),
  },
  {
    label: "&mut",
    kind: vscode.CompletionItemKind.Keyword,
    detail: "Mutable borrow",
    insertText: new vscode.SnippetString("&mut ${1:name}"),
  },
  {
    label: "move",
    kind: vscode.CompletionItemKind.Keyword,
    detail: "Explicit move",
    insertText: new vscode.SnippetString("move ${1:name}"),
  },
  {
    label: "reflect",
    kind: vscode.CompletionItemKind.Function,
    detail: "Compile-time / runtime type reflection",
    insertText: new vscode.SnippetString("reflect(${1:Type})"),
  },
  {
    label: "return",
    kind: vscode.CompletionItemKind.Keyword,
    detail: "Return from a function",
    insertText: new vscode.SnippetString("return ${1:value};"),
  },
  {
    label: "if",
    kind: vscode.CompletionItemKind.Keyword,
    detail: "Conditional expression",
    insertText: new vscode.SnippetString("if (${1:condition}) {\n\t$0\n}"),
  },
  {
    label: "else",
    kind: vscode.CompletionItemKind.Keyword,
    detail: "Alternate branch",
    insertText: new vscode.SnippetString("else {\n\t$0\n}"),
  },
  {
    label: "true",
    kind: vscode.CompletionItemKind.Constant,
    detail: "Boolean true",
  },
  {
    label: "false",
    kind: vscode.CompletionItemKind.Constant,
    detail: "Boolean false",
  },
  {
    label: "puts",
    kind: vscode.CompletionItemKind.Function,
    detail: "Builtin: print values",
    insertText: new vscode.SnippetString("puts(${1:value})"),
  },
  {
    label: "fs_read",
    kind: vscode.CompletionItemKind.Function,
    detail: "Std: read file",
    insertText: new vscode.SnippetString("fs_read(${1:path})"),
  },
  {
    label: "sha256",
    kind: vscode.CompletionItemKind.Function,
    detail: "Std: SHA-256 hash",
    insertText: new vscode.SnippetString("sha256(${1:text})"),
  },
  {
    label: "net_fetch",
    kind: vscode.CompletionItemKind.Function,
    detail: "Std: HTTP fetch",
    insertText: new vscode.SnippetString("net_fetch(${1:url})"),
  },
  {
    label: "spawn_task",
    kind: vscode.CompletionItemKind.Function,
    detail: "Std: spawn task",
    insertText: new vscode.SnippetString("spawn_task(${1:fn})"),
  },
];

export class NexusCompletionItemProvider
  implements vscode.CompletionItemProvider
{
  provideCompletionItems(
    document: vscode.TextDocument,
    position: vscode.Position,
    _token: vscode.CancellationToken,
    _context: vscode.CompletionContext,
  ): vscode.CompletionItem[] {
    const linePrefix = document
      .lineAt(position)
      .text.slice(0, position.character);

    if (isInsideString(linePrefix)) {
      return [];
    }

    const wordRange = document.getWordRangeAtPosition(
      position,
      /[A-Za-z_][A-Za-z0-9_]*/,
    );

    return KEYWORD_ITEMS.map((item) => {
      const completion = new vscode.CompletionItem(item.label, item.kind);
      completion.detail = item.detail;
      completion.documentation = item.documentation
        ? new vscode.MarkdownString(item.documentation)
        : undefined;
      completion.insertText = item.insertText ?? item.label;
      if (wordRange) {
        completion.range = wordRange;
      }
      return completion;
    });
  }
}

function isInsideString(linePrefix: string): boolean {
  let inString = false;
  let escaped = false;
  for (const ch of linePrefix) {
    if (escaped) {
      escaped = false;
      continue;
    }
    if (ch === "\\") {
      escaped = true;
      continue;
    }
    if (ch === '"') {
      inString = !inString;
    }
  }
  return inString;
}
