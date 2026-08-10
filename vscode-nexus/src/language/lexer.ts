export type TokenType =
  | "ILLEGAL"
  | "EOF"
  | "IDENT"
  | "INT"
  | "STRING"
  | "="
  | "+"
  | "-"
  | "*"
  | "/"
  | "%"
  | "!"
  | "<"
  | ">"
  | "=="
  | "!="
  | "<="
  | ">="
  | ","
  | ";"
  | ":"
  | "::"
  | "=>"
  | "->"
  | "."
  | "|>"
  | "&"
  | "&&"
  | "|"
  | "||"
  | "("
  | ")"
  | "{"
  | "}"
  | "["
  | "]"
  | "LET"
  | "FN"
  | "RETURN"
  | "IF"
  | "ELSE"
  | "TRUE"
  | "FALSE"
  | "NULL"
  | "IMPORT"
  | "ENUM"
  | "MATCH"
  | "ASYNC"
  | "AWAIT"
  | "SPAWN"
  | "CHAN"
  | "MACRO"
  | "RULES"
  | "MUT"
  | "MOVE"
  | "EXTERN"
  | "FROM"
  | "TYPE"
  | "REF"
  | "EFFECT"
  | "PERFORM"
  | "HANDLE"
  | "WITH"
  | "RESUME"
  | "STRUCT"
  | "REGION"
  | "REFLECT"
  | "ALLOC"
  | "DERIVE"
  | "AND"
  | "OR"
  | "NOT"
  | "THEN"
  | "DO"
  | "END"
  | "FUN"
  | "VAR"
  | "WHEN"
  | "IS"
  | "IN"
  | "FOR"
  | "WHILE"
  | "BREAK"
  | "CONTINUE"
  | "PUB"
  | "USE"
  | "HASH";

export interface Token {
  type: TokenType;
  literal: string;
  line: number;
  column: number;
}

const KEYWORDS: Readonly<Record<string, TokenType>> = {
  let: "LET",
  fn: "FN",
  return: "RETURN",
  if: "IF",
  else: "ELSE",
  true: "TRUE",
  false: "FALSE",
  null: "NULL",
  import: "IMPORT",
  enum: "ENUM",
  match: "MATCH",
  async: "ASYNC",
  await: "AWAIT",
  spawn: "SPAWN",
  chan: "CHAN",
  macro: "MACRO",
  rules: "RULES",
  mut: "MUT",
  move: "MOVE",
  extern: "EXTERN",
  from: "FROM",
  type: "TYPE",
  ref: "REF",
  effect: "EFFECT",
  perform: "PERFORM",
  handle: "HANDLE",
  with: "WITH",
  resume: "RESUME",
  struct: "STRUCT",
  region: "REGION",
  reflect: "REFLECT",
  alloc: "ALLOC",
  derive: "DERIVE",
  and: "AND",
  or: "OR",
  not: "NOT",
  then: "THEN",
  do: "DO",
  end: "END",
  fun: "FUN",
  var: "VAR",
  when: "WHEN",
  is: "IS",
  in: "IN",
  for: "FOR",
  while: "WHILE",
  break: "BREAK",
  continue: "CONTINUE",
  pub: "PUB",
  use: "USE",
};

export function lookupIdent(ident: string): TokenType {
  return KEYWORDS[ident] ?? "IDENT";
}

/**
 * Lexer tokenizes Nexus (`.nex`) source into a stream of tokens.
 */
export class Lexer {
  private readonly input: string;
  private position = 0;
  private readPosition = 0;
  private ch = "";
  private line = 1;
  private column = 0;
  private readonly errors: string[] = [];

  constructor(input: string) {
    this.input = input;
    this.readChar();
  }

  getErrors(): readonly string[] {
    return this.errors;
  }

  /** Capture lexer cursor for speculative lookahead. */
  checkpoint(): {
    position: number;
    readPosition: number;
    ch: string;
    line: number;
    column: number;
  } {
    return {
      position: this.position,
      readPosition: this.readPosition,
      ch: this.ch,
      line: this.line,
      column: this.column,
    };
  }

  restore(state: {
    position: number;
    readPosition: number;
    ch: string;
    line: number;
    column: number;
  }): void {
    this.position = state.position;
    this.readPosition = state.readPosition;
    this.ch = state.ch;
    this.line = state.line;
    this.column = state.column;
  }

  nextToken(): Token {
    this.skipWhitespaceAndComments();

    const line = this.line;
    const column = this.column;
    let token: Token;

    switch (this.ch) {
      case "=":
        if (this.peekChar() === "=") {
          const ch = this.ch;
          this.readChar();
          token = { type: "==", literal: ch + this.ch, line, column };
        } else if (this.peekChar() === ">") {
          const ch = this.ch;
          this.readChar();
          token = { type: "=>", literal: ch + this.ch, line, column };
        } else {
          token = this.newToken("=", this.ch, line, column);
        }
        break;
      case "+":
        token = this.newToken("+", this.ch, line, column);
        break;
      case "-":
        if (this.peekChar() === ">") {
          const ch = this.ch;
          this.readChar();
          token = { type: "->", literal: ch + this.ch, line, column };
        } else {
          token = this.newToken("-", this.ch, line, column);
        }
        break;
      case "*":
        token = this.newToken("*", this.ch, line, column);
        break;
      case "/":
        token = this.newToken("/", this.ch, line, column);
        break;
      case "!":
        if (this.peekChar() === "=") {
          const ch = this.ch;
          this.readChar();
          token = { type: "!=", literal: ch + this.ch, line, column };
        } else {
          token = this.newToken("!", this.ch, line, column);
        }
        break;
      case "<":
        if (this.peekChar() === "=") {
          const ch = this.ch;
          this.readChar();
          token = { type: "<=", literal: ch + this.ch, line, column };
        } else {
          token = this.newToken("<", this.ch, line, column);
        }
        break;
      case ">":
        if (this.peekChar() === "=") {
          const ch = this.ch;
          this.readChar();
          token = { type: ">=", literal: ch + this.ch, line, column };
        } else {
          token = this.newToken(">", this.ch, line, column);
        }
        break;
      case "#":
        token = this.newToken("HASH", this.ch, line, column);
        break;
      case "&":
        if (this.peekChar() === "&") {
          const ch = this.ch;
          this.readChar();
          token = { type: "&&", literal: ch + this.ch, line, column };
        } else {
          token = this.newToken("&", this.ch, line, column);
        }
        break;
      case "|":
        if (this.peekChar() === ">") {
          const ch = this.ch;
          this.readChar();
          token = { type: "|>", literal: ch + this.ch, line, column };
        } else if (this.peekChar() === "|") {
          const ch = this.ch;
          this.readChar();
          token = { type: "||", literal: ch + this.ch, line, column };
        } else {
          token = this.newToken("|", this.ch, line, column);
        }
        break;
      case ".":
        token = this.newToken(".", this.ch, line, column);
        break;
      case "%":
        token = this.newToken("%", this.ch, line, column);
        break;
      case ":":
        if (this.peekChar() === ":") {
          const ch = this.ch;
          this.readChar();
          token = { type: "::", literal: ch + this.ch, line, column };
        } else {
          token = this.newToken(":", this.ch, line, column);
        }
        break;
      case ",":
        token = this.newToken(",", this.ch, line, column);
        break;
      case ";":
        token = this.newToken(";", this.ch, line, column);
        break;
      case "(":
        token = this.newToken("(", this.ch, line, column);
        break;
      case ")":
        token = this.newToken(")", this.ch, line, column);
        break;
      case "{":
        token = this.newToken("{", this.ch, line, column);
        break;
      case "}":
        token = this.newToken("}", this.ch, line, column);
        break;
      case "[":
        token = this.newToken("[", this.ch, line, column);
        break;
      case "]":
        token = this.newToken("]", this.ch, line, column);
        break;
      case '"':
        return {
          type: "STRING",
          literal: this.readString(line, column),
          line,
          column,
        };
      case "":
        return { type: "EOF", literal: "", line, column };
      default:
        if (isLetter(this.ch)) {
          const literal = this.readIdentifier();
          return {
            type: lookupIdent(literal),
            literal,
            line,
            column,
          };
        }
        if (isDigit(this.ch)) {
          return {
            type: "INT",
            literal: this.readNumber(),
            line,
            column,
          };
        }
        this.errors.push(
          `illegal character ${JSON.stringify(this.ch)} at line ${line}, column ${column}`,
        );
        token = this.newToken("ILLEGAL", this.ch, line, column);
        break;
    }

    this.readChar();
    return token;
  }

  /** Tokenize the entire input into an array (including trailing EOF). */
  tokenize(): Token[] {
    const tokens: Token[] = [];
    for (;;) {
      const tok = this.nextToken();
      tokens.push(tok);
      if (tok.type === "EOF") {
        break;
      }
    }
    return tokens;
  }

  private newToken(
    type: TokenType,
    ch: string,
    line: number,
    column: number,
  ): Token {
    return { type, literal: ch, line, column };
  }

  private readChar(): void {
    if (this.readPosition >= this.input.length) {
      this.ch = "";
      this.position = this.readPosition;
      return;
    }

    const ch = this.input[this.readPosition]!;
    this.ch = ch;
    this.position = this.readPosition;
    this.readPosition += 1;

    if (ch === "\n") {
      this.line += 1;
      this.column = 0;
    } else {
      this.column += 1;
    }
  }

  private peekChar(): string {
    if (this.readPosition >= this.input.length) {
      return "";
    }
    return this.input[this.readPosition]!;
  }

  private skipWhitespaceAndComments(): void {
    for (;;) {
      while (
        this.ch === " " ||
        this.ch === "\t" ||
        this.ch === "\n" ||
        this.ch === "\r"
      ) {
        this.readChar();
      }

      const ch = this.ch;
      const peek = this.peekChar();

      if (ch === "/" && peek === "/") {
        this.readChar();
        for (;;) {
          const c = this.currentChar();
          if (c === "\n" || c === "") {
            break;
          }
          this.readChar();
        }
        continue;
      }

      if (ch === "/" && peek === "*") {
        this.readChar(); // /
        this.readChar(); // *
        for (;;) {
          const c = this.currentChar();
          if (c === "") {
            break;
          }
          if (c === "*" && this.peekChar() === "/") {
            this.readChar();
            this.readChar();
            break;
          }
          this.readChar();
        }
        continue;
      }

      break;
    }
  }

  /** Returns the current character without control-flow narrowing side effects. */
  private currentChar(): string {
    return this.ch;
  }

  private readIdentifier(): string {
    const start = this.position;
    while (isLetter(this.ch) || isDigit(this.ch)) {
      this.readChar();
    }
    return this.input.slice(start, this.position);
  }

  private readNumber(): string {
    const start = this.position;
    while (isDigit(this.ch)) {
      this.readChar();
    }
    return this.input.slice(start, this.position);
  }

  private readString(startLine: number, startColumn: number): string {
    this.readChar(); // opening quote
    let value = "";

    for (;;) {
      const c = this.ch;
      if (c === '"' || c === "") {
        break;
      }

      if (c === "\\") {
        this.readChar();
        const escaped = this.ch;
        switch (escaped) {
          case "n":
            value += "\n";
            break;
          case "t":
            value += "\t";
            break;
          case "r":
            value += "\r";
            break;
          case "\\":
            value += "\\";
            break;
          case '"':
            value += '"';
            break;
          case "":
            break;
          default:
            value += escaped;
            break;
        }
        if (this.ch !== "") {
          this.readChar();
        }
        continue;
      }

      value += c;
      this.readChar();
    }

    if (this.ch === "") {
      this.errors.push(
        `unterminated string starting at line ${startLine}, column ${startColumn}`,
      );
      return value;
    }

    this.readChar(); // closing quote
    return value;
  }
}

function isLetter(ch: string): boolean {
  return (
    (ch >= "a" && ch <= "z") ||
    (ch >= "A" && ch <= "Z") ||
    ch === "_" ||
    ch === "$"
  );
}

function isDigit(ch: string): boolean {
  return ch >= "0" && ch <= "9";
}
