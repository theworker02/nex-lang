import { Lexer, Token, TokenType } from "./lexer";

export type NodeType =
  | "Program"
  | "LetStatement"
  | "ReturnStatement"
  | "ImportStatement"
  | "WhileStatement"
  | "BreakStatement"
  | "ContinueStatement"
  | "ExpressionStatement"
  | "BlockStatement"
  | "EnumDeclaration"
  | "MacroDefinition"
  | "ExternDeclaration"
  | "Identifier"
  | "IntegerLiteral"
  | "StringLiteral"
  | "BooleanLiteral"
  | "NullLiteral"
  | "PrefixExpression"
  | "InfixExpression"
  | "IfExpression"
  | "FunctionLiteral"
  | "CallExpression"
  | "ArrayLiteral"
  | "HashLiteral"
  | "IndexExpression"
  | "MemberExpression"
  | "AssignExpression"
  | "PipeExpression"
  | "MatchExpression"
  | "AsyncExpression"
  | "AwaitExpression"
  | "SpawnExpression"
  | "ChanExpression"
  | "RefExpression"
  | "MoveExpression"
  | "ConstructorExpression"
  | "MacroInvocation"
  | "TypeAnnotation"
  | "EffectDeclaration"
  | "PerformExpression"
  | "HandleExpression"
  | "StructDeclaration"
  | "RegionExpression"
  | "ReflectExpression"
  | "Attribute"
  | "DeriveAttribute";

export interface Node {
  type: NodeType;
  tokenLiteral(): string;
}

export interface Statement extends Node {
  kind: "statement";
}

export interface Expression extends Node {
  kind: "expression";
}

export interface Program extends Node {
  type: "Program";
  statements: Statement[];
}

export interface Identifier extends Expression {
  type: "Identifier";
  value: string;
  token: Token;
}

export interface TypeAnnotation {
  type: "TypeAnnotation";
  name: string;
  mutable: boolean;
  reference: boolean;
  token: Token;
}

export interface LetStatement extends Statement {
  type: "LetStatement";
  token: Token;
  name: Identifier;
  mutable: boolean;
  typeAnnotation: TypeAnnotation | null;
  value: Expression | null;
}

export interface ReturnStatement extends Statement {
  type: "ReturnStatement";
  token: Token;
  returnValue: Expression | null;
}

export interface ImportStatement extends Statement {
  type: "ImportStatement";
  token: Token;
  path: string;
}

export interface WhileStatement extends Statement {
  type: "WhileStatement";
  token: Token;
  condition: Expression | null;
  body: BlockStatement;
}

export interface BreakStatement extends Statement {
  type: "BreakStatement";
  token: Token;
}

export interface ContinueStatement extends Statement {
  type: "ContinueStatement";
  token: Token;
}

export interface ExpressionStatement extends Statement {
  type: "ExpressionStatement";
  token: Token;
  expression: Expression | null;
}

export interface BlockStatement extends Statement {
  type: "BlockStatement";
  token: Token;
  statements: Statement[];
}

export interface EnumVariant {
  name: Identifier;
  fields: Identifier[];
}

export interface EnumDeclaration extends Statement {
  type: "EnumDeclaration";
  token: Token;
  name: Identifier;
  variants: EnumVariant[];
}

export type MacroFragmentKind = "expr" | "ident" | "tt" | "literal";

export interface MacroFragment {
  kind: MacroFragmentKind;
  name?: string;
  token?: Token;
  literal?: string;
}

export interface MacroRule {
  pattern: MacroFragment[];
  body: Expression;
}

export interface MacroDefinition extends Statement {
  type: "MacroDefinition";
  token: Token;
  name: Identifier;
  rules: MacroRule[];
}

export interface ExternFnDecl {
  name: Identifier;
  parameters: Identifier[];
  returnType: string;
}

export interface ExternDeclaration extends Statement {
  type: "ExternDeclaration";
  token: Token;
  library: string;
  functions: ExternFnDecl[];
}

export interface IntegerLiteral extends Expression {
  type: "IntegerLiteral";
  token: Token;
  value: number;
}

export interface StringLiteral extends Expression {
  type: "StringLiteral";
  token: Token;
  value: string;
}

export interface BooleanLiteral extends Expression {
  type: "BooleanLiteral";
  token: Token;
  value: boolean;
}

export interface NullLiteral extends Expression {
  type: "NullLiteral";
  token: Token;
}

export interface ArrayLiteral extends Expression {
  type: "ArrayLiteral";
  token: Token;
  elements: Expression[];
}

export interface HashLiteral extends Expression {
  type: "HashLiteral";
  token: Token;
  pairs: Array<{ key: Expression; value: Expression }>;
}

export interface IndexExpression extends Expression {
  type: "IndexExpression";
  token: Token;
  left: Expression;
  index: Expression | null;
}

export interface MemberExpression extends Expression {
  type: "MemberExpression";
  token: Token;
  object: Expression;
  property: Identifier;
}

export interface AssignExpression extends Expression {
  type: "AssignExpression";
  token: Token;
  name: Expression;
  value: Expression | null;
}

export interface PipeExpression extends Expression {
  type: "PipeExpression";
  token: Token;
  left: Expression;
  right: Expression | null;
}

export interface PrefixExpression extends Expression {
  type: "PrefixExpression";
  token: Token;
  operator: string;
  right: Expression | null;
}

export interface InfixExpression extends Expression {
  type: "InfixExpression";
  token: Token;
  left: Expression;
  operator: string;
  right: Expression | null;
}

export interface IfExpression extends Expression {
  type: "IfExpression";
  token: Token;
  condition: Expression | null;
  consequence: BlockStatement;
  alternative: BlockStatement | null;
}

export interface FunctionLiteral extends Expression {
  type: "FunctionLiteral";
  token: Token;
  parameters: Identifier[];
  body: BlockStatement;
  isAsync: boolean;
}

export interface CallExpression extends Expression {
  type: "CallExpression";
  token: Token;
  function: Expression;
  arguments: Expression[];
}

export type Pattern =
  | WildcardPattern
  | IdentPattern
  | LiteralPattern
  | VariantPattern;

export interface WildcardPattern {
  kind: "wildcard";
  token: Token;
}

export interface IdentPattern {
  kind: "ident";
  name: Identifier;
}

export interface LiteralPattern {
  kind: "literal";
  value: IntegerLiteral | StringLiteral | BooleanLiteral;
}

export interface VariantPattern {
  kind: "variant";
  enumName: string | null;
  variant: Identifier;
  fields: Pattern[];
  token: Token;
}

export interface MatchArm {
  pattern: Pattern;
  body: Expression;
  token: Token;
}

export interface MatchExpression extends Expression {
  type: "MatchExpression";
  token: Token;
  scrutinee: Expression | null;
  arms: MatchArm[];
}

export interface AsyncExpression extends Expression {
  type: "AsyncExpression";
  token: Token;
  body: BlockStatement | Expression;
}

export interface AwaitExpression extends Expression {
  type: "AwaitExpression";
  token: Token;
  argument: Expression | null;
}

export interface SpawnExpression extends Expression {
  type: "SpawnExpression";
  token: Token;
  argument: Expression | null;
}

export interface ChanExpression extends Expression {
  type: "ChanExpression";
  token: Token;
  capacity: Expression | null;
}

export interface RefExpression extends Expression {
  type: "RefExpression";
  token: Token;
  mutable: boolean;
  value: Expression | null;
}

export interface MoveExpression extends Expression {
  type: "MoveExpression";
  token: Token;
  value: Expression | null;
}

export interface ConstructorExpression extends Expression {
  type: "ConstructorExpression";
  token: Token;
  enumName: string | null;
  variant: Identifier;
  arguments: Expression[];
}

export interface MacroInvocation extends Expression {
  type: "MacroInvocation";
  token: Token;
  name: Identifier;
  arguments: Expression[];
  rawTokens: Token[];
}

export interface Attribute {
  type: "Attribute";
  name: string;
  args: string[];
  token: Token;
}

export interface StructField {
  name: Identifier;
  typeName: string;
}

export interface StructDeclaration extends Statement {
  type: "StructDeclaration";
  token: Token;
  name: Identifier;
  fields: StructField[];
  attributes: Attribute[];
}

export interface EffectOp {
  name: Identifier;
  parameters: Identifier[];
}

export interface EffectDeclaration extends Statement {
  type: "EffectDeclaration";
  token: Token;
  name: Identifier;
  operations: EffectOp[];
}

export interface PerformExpression extends Expression {
  type: "PerformExpression";
  token: Token;
  effectName: string | null;
  operation: Identifier;
  arguments: Expression[];
}

export interface EffectHandlerArm {
  operation: Identifier;
  parameters: Identifier[];
  body: Expression;
  token: Token;
}

export interface HandleExpression extends Expression {
  type: "HandleExpression";
  token: Token;
  body: BlockStatement;
  effectName: Identifier;
  handlers: EffectHandlerArm[];
}

export interface RegionExpression extends Expression {
  type: "RegionExpression";
  token: Token;
  name: Identifier | null;
  body: BlockStatement;
}

export interface ReflectExpression extends Expression {
  type: "ReflectExpression";
  token: Token;
  target: Identifier;
}

const enum Precedence {
  Lowest = 1,
  Assign,
  Pipe,
  Or,
  And,
  Equals,
  LessGreater,
  Sum,
  Product,
  Prefix,
  Call,
  Index,
}

const PRECEDENCES: Partial<Record<TokenType, Precedence>> = {
  "=": Precedence.Assign,
  "|>": Precedence.Pipe,
  "||": Precedence.Or,
  "&&": Precedence.And,
  OR: Precedence.Or,
  AND: Precedence.And,
  "==": Precedence.Equals,
  "!=": Precedence.Equals,
  "<": Precedence.LessGreater,
  ">": Precedence.LessGreater,
  "<=": Precedence.LessGreater,
  ">=": Precedence.LessGreater,
  "+": Precedence.Sum,
  "-": Precedence.Sum,
  "/": Precedence.Product,
  "*": Precedence.Product,
  "%": Precedence.Product,
  "(": Precedence.Call,
  "!": Precedence.Call,
  "::": Precedence.Call,
  "[": Precedence.Index,
  ".": Precedence.Index,
};

type PrefixParseFn = () => Expression | null;
type InfixParseFn = (left: Expression) => Expression | null;

/**
 * Recursive-descent / Pratt parser for Nexus source.
 */
export class Parser {
  private readonly lexer: Lexer;
  private curToken!: Token;
  private peekToken!: Token;
  private readonly errors: string[] = [];
  private readonly prefixParseFns = new Map<TokenType, PrefixParseFn>();
  private readonly infixParseFns = new Map<TokenType, InfixParseFn>();

  constructor(lexer: Lexer) {
    this.lexer = lexer;

    this.registerPrefix("IDENT", () => this.parseIdentifierOrConstructor());
    this.registerPrefix("TYPE", () => this.parseKeywordAsIdent("type"));
    this.registerPrefix("INT", () => this.parseIntegerLiteral());
    this.registerPrefix("STRING", () => this.parseStringLiteral());
    this.registerPrefix("!", () => this.parsePrefixExpression());
    this.registerPrefix("-", () => this.parsePrefixExpression());
    this.registerPrefix("&", () => this.parseRefExpression());
    this.registerPrefix("TRUE", () => this.parseBoolean());
    this.registerPrefix("FALSE", () => this.parseBoolean());
    this.registerPrefix("NULL", () => this.parseNullLiteral());
    this.registerPrefix("(", () => this.parseGroupedExpression());
    this.registerPrefix("IF", () => this.parseIfExpression());
    this.registerPrefix("FN", () => this.parseFunctionLiteral(false));
    this.registerPrefix("ASYNC", () => this.parseAsyncExpression());
    this.registerPrefix("AWAIT", () => this.parseAwaitExpression());
    this.registerPrefix("SPAWN", () => this.parseSpawnExpression());
    this.registerPrefix("CHAN", () => this.parseChanExpression());
    this.registerPrefix("MATCH", () => this.parseMatchExpression());
    this.registerPrefix("MOVE", () => this.parseMoveExpression());
    this.registerPrefix("PERFORM", () => this.parsePerformExpression());
    this.registerPrefix("HANDLE", () => this.parseHandleExpression());
    this.registerPrefix("REGION", () => this.parseRegionExpression());
    this.registerPrefix("REFLECT", () => this.parseReflectExpression());
    this.registerPrefix("[", () => this.parseArrayLiteral());
    this.registerPrefix("{", () => this.parseHashOrBlock());

    this.registerInfix("+", (left) => this.parseInfixExpression(left));
    this.registerInfix("-", (left) => this.parseInfixExpression(left));
    this.registerInfix("/", (left) => this.parseInfixExpression(left));
    this.registerInfix("*", (left) => this.parseInfixExpression(left));
    this.registerInfix("%", (left) => this.parseInfixExpression(left));
    this.registerInfix("==", (left) => this.parseInfixExpression(left));
    this.registerInfix("!=", (left) => this.parseInfixExpression(left));
    this.registerInfix("<", (left) => this.parseInfixExpression(left));
    this.registerInfix(">", (left) => this.parseInfixExpression(left));
    this.registerInfix("<=", (left) => this.parseInfixExpression(left));
    this.registerInfix(">=", (left) => this.parseInfixExpression(left));
    this.registerInfix("&&", (left) => this.parseInfixExpression(left));
    this.registerInfix("||", (left) => this.parseInfixExpression(left));
    this.registerInfix("AND", (left) => this.parseInfixExpression(left));
    this.registerInfix("OR", (left) => this.parseInfixExpression(left));
    this.registerInfix("(", (left) => this.parseCallExpression(left));
    this.registerInfix("!", (left) => this.parseMacroInvocation(left));
    this.registerInfix("::", (left) => this.parsePathConstructor(left));
    this.registerInfix("[", (left) => this.parseIndexExpression(left));
    this.registerInfix(".", (left) => this.parseMemberExpression(left));
    this.registerInfix("=", (left) => this.parseAssignExpression(left));
    this.registerInfix("|>", (left) => this.parsePipeExpression(left));

    this.nextToken();
    this.nextToken();
  }

  getErrors(): readonly string[] {
    return this.errors;
  }

  parseProgram(): Program {
    const program: Program = {
      type: "Program",
      statements: [],
      tokenLiteral: () =>
        program.statements.length > 0
          ? program.statements[0]!.tokenLiteral()
          : "",
    };

    while (this.curToken.type !== "EOF") {
      const stmt = this.parseStatement();
      if (stmt) {
        program.statements.push(stmt);
      }
      this.nextToken();
    }

    for (const err of this.lexer.getErrors()) {
      this.errors.push(err);
    }

    return program;
  }

  private registerPrefix(type: TokenType, fn: PrefixParseFn): void {
    this.prefixParseFns.set(type, fn);
  }

  private registerInfix(type: TokenType, fn: InfixParseFn): void {
    this.infixParseFns.set(type, fn);
  }

  private nextToken(): void {
    this.curToken = this.peekToken;
    this.peekToken = this.lexer.nextToken();
  }

  private parseStatement(): Statement | null {
    switch (this.curToken.type) {
      case "LET":
        return this.parseLetStatement();
      case "RETURN":
        return this.parseReturnStatement();
      case "IMPORT":
        return this.parseImportStatement();
      case "WHILE":
        return this.parseWhileStatement();
      case "BREAK":
        return this.parseBreakStatement();
      case "CONTINUE":
        return this.parseContinueStatement();
      case "ENUM":
        return this.parseEnumDeclaration();
      case "MACRO":
        return this.parseMacroDefinition();
      case "EXTERN":
        return this.parseExternDeclaration();
      case "EFFECT":
        return this.parseEffectDeclaration();
      case "STRUCT":
        return this.parseStructDeclaration();
      case ";":
        return null;
      default:
        return this.parseExpressionStatement();
    }
  }

  private parseLetStatement(): LetStatement | null {
    const token = this.curToken;
    let mutable = false;

    if (this.peekTokenIs("MUT")) {
      this.nextToken();
      mutable = true;
    }

    if (!this.expectPeek("IDENT")) {
      return null;
    }

    const name: Identifier = {
      kind: "expression",
      type: "Identifier",
      token: this.curToken,
      value: this.curToken.literal,
      tokenLiteral: () => name.token.literal,
    };

    let typeAnnotation: TypeAnnotation | null = null;
    if (this.peekTokenIs(":")) {
      this.nextToken();
      typeAnnotation = this.parseTypeAnnotation();
    }

    if (!this.expectPeek("=")) {
      return null;
    }

    this.nextToken();
    const value = this.parseExpression(Precedence.Lowest);

    if (this.peekTokenIs(";")) {
      this.nextToken();
    }

    const stmt: LetStatement = {
      kind: "statement",
      type: "LetStatement",
      token,
      name,
      mutable,
      typeAnnotation,
      value,
      tokenLiteral: () => token.literal,
    };
    return stmt;
  }

  private parseTypeAnnotation(): TypeAnnotation | null {
    let reference = false;
    let mutable = false;
    let token = this.peekToken;

    if (this.peekTokenIs("&")) {
      this.nextToken();
      reference = true;
      token = this.curToken;
      if (this.peekTokenIs("MUT")) {
        this.nextToken();
        mutable = true;
      }
    }

    if (!this.expectPeek("IDENT") && !this.expectPeekTypeKeyword()) {
      return null;
    }

    return {
      type: "TypeAnnotation",
      name: this.curToken.literal,
      mutable,
      reference,
      token,
    };
  }

  private expectPeekTypeKeyword(): boolean {
    const typeKeywords: TokenType[] = [
      "FN",
      "CHAN",
      "TRUE",
      "FALSE",
    ];
    // Allow common type names that are keywords? Prefer IDENT only.
    // Keep helper for future extension; currently unused path returns false.
    void typeKeywords;
    return false;
  }

  private parseReturnStatement(): ReturnStatement {
    const token = this.curToken;
    this.nextToken();
    const returnValue = this.parseExpression(Precedence.Lowest);

    if (this.peekTokenIs(";")) {
      this.nextToken();
    }

    return {
      kind: "statement",
      type: "ReturnStatement",
      token,
      returnValue,
      tokenLiteral: () => token.literal,
    };
  }

  private parseImportStatement(): ImportStatement | null {
    const token = this.curToken;
    if (!this.expectPeek("STRING")) {
      return null;
    }
    const path = this.curToken.literal;
    if (this.peekTokenIs(";")) {
      this.nextToken();
    }
    return {
      kind: "statement",
      type: "ImportStatement",
      token,
      path,
      tokenLiteral: () => token.literal,
    };
  }

  private parseWhileStatement(): WhileStatement | null {
    const token = this.curToken;
    if (!this.expectPeek("(")) {
      return null;
    }
    this.nextToken();
    const condition = this.parseExpression(Precedence.Lowest);
    if (!this.expectPeek(")")) {
      return null;
    }
    if (!this.expectPeek("{")) {
      return null;
    }
    const body = this.parseBlockStatement();
    if (this.peekTokenIs(";")) {
      this.nextToken();
    }
    return {
      kind: "statement",
      type: "WhileStatement",
      token,
      condition,
      body,
      tokenLiteral: () => token.literal,
    };
  }

  private parseBreakStatement(): BreakStatement {
    const token = this.curToken;
    if (this.peekTokenIs(";")) {
      this.nextToken();
    }
    return {
      kind: "statement",
      type: "BreakStatement",
      token,
      tokenLiteral: () => token.literal,
    };
  }

  private parseContinueStatement(): ContinueStatement {
    const token = this.curToken;
    if (this.peekTokenIs(";")) {
      this.nextToken();
    }
    return {
      kind: "statement",
      type: "ContinueStatement",
      token,
      tokenLiteral: () => token.literal,
    };
  }

  private parseExpressionStatement(): ExpressionStatement {
    const token = this.curToken;
    const expression = this.parseExpression(Precedence.Lowest);

    if (this.peekTokenIs(";")) {
      this.nextToken();
    }

    return {
      kind: "statement",
      type: "ExpressionStatement",
      token,
      expression,
      tokenLiteral: () => token.literal,
    };
  }

  private parseEnumDeclaration(): EnumDeclaration | null {
    const token = this.curToken;
    if (!this.expectPeek("IDENT")) {
      return null;
    }

    const name = this.parseIdentifier();
    if (!this.expectPeek("{")) {
      return null;
    }

    const variants: EnumVariant[] = [];
    this.nextToken();

    while (!this.curTokenIs("}") && !this.curTokenIs("EOF")) {
      if (!this.curTokenIs("IDENT")) {
        this.errors.push(
          `expected variant name, got ${this.curToken.type} at line ${this.curToken.line}, column ${this.curToken.column}`,
        );
        break;
      }

      const variantName = this.parseIdentifier();
      const fields: Identifier[] = [];

      if (this.peekTokenIs("(")) {
        this.nextToken();
        if (!this.peekTokenIs(")")) {
          this.nextToken();
          fields.push(this.parseIdentifier());
          while (this.peekTokenIs(",")) {
            this.nextToken();
            this.nextToken();
            fields.push(this.parseIdentifier());
          }
        }
        if (!this.expectPeek(")")) {
          return null;
        }
      }

      variants.push({ name: variantName, fields });

      if (this.peekTokenIs(",")) {
        this.nextToken();
        this.nextToken();
      } else {
        this.nextToken();
      }
    }

    if (!this.curTokenIs("}")) {
      this.errors.push(
        `expected '}' after enum variants at line ${this.curToken.line}, column ${this.curToken.column}`,
      );
    }

    if (this.peekTokenIs(";")) {
      this.nextToken();
    }

    return {
      kind: "statement",
      type: "EnumDeclaration",
      token,
      name,
      variants,
      tokenLiteral: () => token.literal,
    };
  }

  private parseMacroDefinition(): MacroDefinition | null {
    const token = this.curToken;
    if (!this.expectPeek("RULES")) {
      return null;
    }
    if (!this.expectPeek("!")) {
      return null;
    }
    if (!this.expectPeek("IDENT")) {
      return null;
    }

    const name = this.parseIdentifier();
    if (!this.expectPeek("{")) {
      return null;
    }

    const rules: MacroRule[] = [];
    this.nextToken();

    while (!this.curTokenIs("}") && !this.curTokenIs("EOF")) {
      if (!this.curTokenIs("(")) {
        this.errors.push(
          `expected '(' to start macro pattern at line ${this.curToken.line}, column ${this.curToken.column}`,
        );
        break;
      }

      const pattern = this.parseMacroPattern();
      if (!this.expectPeek("=>")) {
        return null;
      }
      this.nextToken();
      const body = this.parseExpression(Precedence.Lowest);
      if (!body) {
        return null;
      }

      rules.push({ pattern, body });

      if (this.peekTokenIs(";")) {
        this.nextToken();
      }
      if (this.peekTokenIs(",")) {
        this.nextToken();
      }
      this.nextToken();
    }

    if (this.peekTokenIs(";")) {
      this.nextToken();
    }

    return {
      kind: "statement",
      type: "MacroDefinition",
      token,
      name,
      rules,
      tokenLiteral: () => token.literal,
    };
  }

  private parseMacroPattern(): MacroFragment[] {
    const fragments: MacroFragment[] = [];
    this.nextToken(); // skip '('

    while (!this.curTokenIs(")") && !this.curTokenIs("EOF")) {
      if (this.curTokenIs("IDENT") && this.curToken.literal.startsWith("$")) {
        const metaName = this.curToken.literal.slice(1);
        if (this.peekTokenIs(":")) {
          this.nextToken();
          this.nextToken();
          const kindLiteral = this.curToken.literal;
          const kind: MacroFragmentKind =
            kindLiteral === "expr" ||
            kindLiteral === "ident" ||
            kindLiteral === "tt"
              ? kindLiteral
              : "expr";
          fragments.push({ kind, name: metaName });
        } else {
          fragments.push({ kind: "expr", name: metaName });
        }
      } else if (
        this.curTokenIs("IDENT") ||
        this.curTokenIs("INT") ||
        this.curTokenIs("STRING") ||
        this.curToken.type === "," ||
        this.curToken.type === "+" ||
        this.curToken.type === "-" ||
        this.curToken.type === "*" ||
        this.curToken.type === "/" ||
        this.curToken.type === "!" ||
        this.curToken.type === "=" ||
        this.curToken.type === "(" ||
        this.curToken.type === ")" ||
        this.curToken.type === "{" ||
        this.curToken.type === "}"
      ) {
        fragments.push({
          kind: "literal",
          literal: this.curToken.literal,
          token: this.curToken,
        });
      } else {
        fragments.push({
          kind: "literal",
          literal: this.curToken.literal,
          token: this.curToken,
        });
      }

      this.nextToken();
    }

    return fragments;
  }

  private parseExternDeclaration(): ExternDeclaration | null {
    const token = this.curToken;
    // extern "C" from "lib.dll" { fn name(a, b) -> int; }
    if (!this.expectPeek("STRING")) {
      // allow skipping ABI string
      if (this.peekTokenIs("FROM")) {
        // ok
      } else if (this.curTokenIs("IDENT") || this.peekTokenIs("IDENT")) {
        // optional ABI identifier like C
        if (this.peekTokenIs("IDENT") || this.peekTokenIs("STRING")) {
          this.nextToken();
        }
      }
    }

    let library = "";
    if (this.peekTokenIs("FROM")) {
      this.nextToken();
      if (!this.expectPeek("STRING")) {
        return null;
      }
      library = this.curToken.literal;
    } else if (this.curTokenIs("STRING")) {
      library = this.curToken.literal;
      if (this.peekTokenIs("FROM")) {
        this.nextToken();
        if (!this.expectPeek("STRING")) {
          return null;
        }
        library = this.curToken.literal;
      }
    } else if (this.peekTokenIs("STRING")) {
      this.nextToken();
      library = this.curToken.literal;
    }

    if (!this.expectPeek("{")) {
      return null;
    }

    const functions: ExternFnDecl[] = [];
    this.nextToken();

    while (!this.curTokenIs("}") && !this.curTokenIs("EOF")) {
      if (!this.curTokenIs("FN")) {
        this.errors.push(
          `expected fn in extern block at line ${this.curToken.line}, column ${this.curToken.column}`,
        );
        break;
      }
      if (!this.expectPeek("IDENT")) {
        return null;
      }
      const fname = this.parseIdentifier();
      if (!this.expectPeek("(")) {
        return null;
      }
      const parameters = this.parseFunctionParameters();
      if (!parameters) {
        return null;
      }

      let returnType = "void";
      if (this.peekTokenIs("-") && this.peekTokenLiteralIsBeyond(">", 1)) {
        // handle -> 
        this.nextToken(); // -
        if (this.peekTokenIs(">") || this.curToken.literal === "-") {
          // We used separate tokens; support IDENT after "->" via ">" then type
        }
      }
      // Parse "->" as "-" then we need ">" - but ">" is a token. Use "-" followed by checking...
      // Better: look for custom. We'll accept: ) IDENT for return type after optional :
      if (this.peekTokenIs(":")) {
        this.nextToken();
        this.nextToken();
        returnType = this.curToken.literal;
      } else if (this.peekTokenIs("IDENT")) {
        // allow `fn foo(x) int`
        this.nextToken();
        returnType = this.curToken.literal;
      }

      // Also support `-> type` where - and > are separate... 
      // After parameters we're at ')'. Check peek for '-'
      // Actually parseFunctionParameters already consumed ')'.
      // Re-check: if next is '-' then '>' then type
      if (this.peekTokenIs("-")) {
        this.nextToken();
        if (this.peekTokenIs(">") || this.curTokenIs("-")) {
          if (this.peekTokenIs(">")) {
            this.nextToken();
          }
          this.nextToken();
          returnType = this.curToken.literal;
        }
      }

      functions.push({ name: fname, parameters, returnType });

      if (this.peekTokenIs(";")) {
        this.nextToken();
      }
      this.nextToken();
    }

    if (this.peekTokenIs(";")) {
      this.nextToken();
    }

    return {
      kind: "statement",
      type: "ExternDeclaration",
      token,
      library,
      functions,
      tokenLiteral: () => token.literal,
    };
  }

  private peekTokenLiteralIsBeyond(lit: string, _n: number): boolean {
    const snap = this.lexer.checkpoint();
    const next = this.lexer.nextToken();
    this.lexer.restore(snap);
    return next.literal === lit || next.type === (lit as TokenType);
  }

  private parseExpression(precedence: Precedence): Expression | null {
    const prefix = this.prefixParseFns.get(this.curToken.type);
    if (!prefix) {
      this.errors.push(
        `no prefix parse function for ${this.curToken.type} at line ${this.curToken.line}, column ${this.curToken.column}`,
      );
      return null;
    }

    let left = prefix();
    if (!left) {
      return null;
    }

    while (
      !this.peekTokenIs(";") &&
      precedence < this.peekPrecedence()
    ) {
      const infix = this.infixParseFns.get(this.peekToken.type);
      if (!infix) {
        return left;
      }
      this.nextToken();
      left = infix(left);
      if (!left) {
        return null;
      }
    }

    return left;
  }

  private parseIdentifier(): Identifier {
    const ident: Identifier = {
      kind: "expression",
      type: "Identifier",
      token: this.curToken,
      value: this.curToken.literal,
      tokenLiteral: () => ident.token.literal,
    };
    return ident;
  }

  /** Allow reserved words like `type` to be used as callables/idents. */
  private parseKeywordAsIdent(name: string): Identifier {
    const ident: Identifier = {
      kind: "expression",
      type: "Identifier",
      token: this.curToken,
      value: name,
      tokenLiteral: () => name,
    };
    return ident;
  }

  private parseIdentifierOrConstructor(): Expression {
    const ident = this.parseIdentifier();
    // Bare Variant(args) is handled via call expression on Identifier.
    // Enum::Variant handled via :: infix.
    return ident;
  }

  private parseIntegerLiteral(): IntegerLiteral | null {
    const token = this.curToken;
    const value = Number.parseInt(token.literal, 10);
    if (Number.isNaN(value)) {
      this.errors.push(
        `could not parse ${JSON.stringify(token.literal)} as integer at line ${token.line}, column ${token.column}`,
      );
      return null;
    }
    return {
      kind: "expression",
      type: "IntegerLiteral",
      token,
      value,
      tokenLiteral: () => token.literal,
    };
  }

  private parseStringLiteral(): StringLiteral {
    const token = this.curToken;
    return {
      kind: "expression",
      type: "StringLiteral",
      token,
      value: token.literal,
      tokenLiteral: () => token.literal,
    };
  }

  private parseBoolean(): BooleanLiteral {
    const token = this.curToken;
    return {
      kind: "expression",
      type: "BooleanLiteral",
      token,
      value: this.curTokenIs("TRUE"),
      tokenLiteral: () => token.literal,
    };
  }

  private parseNullLiteral(): NullLiteral {
    const token = this.curToken;
    return {
      kind: "expression",
      type: "NullLiteral",
      token,
      tokenLiteral: () => token.literal,
    };
  }

  private parseArrayLiteral(): ArrayLiteral | null {
    const token = this.curToken;
    const elements = this.parseExpressionList("]");
    if (!elements) {
      return null;
    }
    return {
      kind: "expression",
      type: "ArrayLiteral",
      token,
      elements,
      tokenLiteral: () => token.literal,
    };
  }

  private parseHashOrBlock(): Expression | null {
    // Distinguish `{ "k": v }` / `{ k: v }` hashes from block expressions.
    if (this.peekTokenIs("}")) {
      return this.parseHashLiteral();
    }
    if (
      (this.peekTokenIs("STRING") || this.peekTokenIs("IDENT")) &&
      this.peekTokenLiteralIsBeyond(":", 1)
    ) {
      return this.parseHashLiteral();
    }
    return this.parseBlockExpression();
  }

  private parseHashLiteral(): HashLiteral | null {
    const token = this.curToken;
    const pairs: Array<{ key: Expression; value: Expression }> = [];
    this.nextToken();
    while (!this.curTokenIs("}") && !this.curTokenIs("EOF")) {
      const key = this.parseExpression(Precedence.Lowest);
      if (!key || !this.expectPeek(":")) {
        return null;
      }
      this.nextToken();
      const value = this.parseExpression(Precedence.Lowest);
      if (!value) {
        return null;
      }
      pairs.push({ key, value });
      if (this.peekTokenIs(",")) {
        this.nextToken();
      }
      this.nextToken();
    }
    if (!this.curTokenIs("}")) {
      this.errors.push(
        `expected '}' after hash literal at line ${this.curToken.line}, column ${this.curToken.column}`,
      );
      return null;
    }
    return {
      kind: "expression",
      type: "HashLiteral",
      token,
      pairs,
      tokenLiteral: () => token.literal,
    };
  }

  private parseIndexExpression(left: Expression): IndexExpression | null {
    const token = this.curToken;
    this.nextToken();
    const index = this.parseExpression(Precedence.Lowest);
    if (!this.expectPeek("]")) {
      return null;
    }
    return {
      kind: "expression",
      type: "IndexExpression",
      token,
      left,
      index,
      tokenLiteral: () => token.literal,
    };
  }

  private parseMemberExpression(left: Expression): MemberExpression | null {
    const token = this.curToken;
    if (!this.expectPeek("IDENT")) {
      return null;
    }
    const property = this.parseIdentifier();
    return {
      kind: "expression",
      type: "MemberExpression",
      token,
      object: left,
      property,
      tokenLiteral: () => token.literal,
    };
  }

  private parseAssignExpression(left: Expression): AssignExpression {
    const token = this.curToken;
    this.nextToken();
    const value = this.parseExpression(Precedence.Lowest);
    return {
      kind: "expression",
      type: "AssignExpression",
      token,
      name: left,
      value,
      tokenLiteral: () => token.literal,
    };
  }

  private parsePipeExpression(left: Expression): PipeExpression {
    const token = this.curToken;
    this.nextToken();
    const right = this.parseExpression(Precedence.Pipe);
    return {
      kind: "expression",
      type: "PipeExpression",
      token,
      left,
      right,
      tokenLiteral: () => token.literal,
    };
  }

  private parsePrefixExpression(): PrefixExpression {
    const token = this.curToken;
    const operator = token.literal;
    this.nextToken();
    const right = this.parseExpression(Precedence.Prefix);
    return {
      kind: "expression",
      type: "PrefixExpression",
      token,
      operator,
      right,
      tokenLiteral: () => token.literal,
    };
  }

  private parseRefExpression(): RefExpression {
    const token = this.curToken;
    let mutable = false;
    if (this.peekTokenIs("MUT")) {
      this.nextToken();
      mutable = true;
    }
    this.nextToken();
    const value = this.parseExpression(Precedence.Prefix);
    return {
      kind: "expression",
      type: "RefExpression",
      token,
      mutable,
      value,
      tokenLiteral: () => token.literal,
    };
  }

  private parseMoveExpression(): MoveExpression {
    const token = this.curToken;
    this.nextToken();
    const value = this.parseExpression(Precedence.Prefix);
    return {
      kind: "expression",
      type: "MoveExpression",
      token,
      value,
      tokenLiteral: () => token.literal,
    };
  }

  private parseInfixExpression(left: Expression): InfixExpression {
    const token = this.curToken;
    const operator = token.literal;
    const precedence = this.curPrecedence();
    this.nextToken();
    const right = this.parseExpression(precedence);
    return {
      kind: "expression",
      type: "InfixExpression",
      token,
      left,
      operator,
      right,
      tokenLiteral: () => token.literal,
    };
  }

  private parseGroupedExpression(): Expression | null {
    this.nextToken();
    const exp = this.parseExpression(Precedence.Lowest);
    if (!this.expectPeek(")")) {
      return null;
    }
    return exp;
  }

  private parseBlockExpression(): Expression {
    const block = this.parseBlockStatement();
    // Blocks are statements structurally but valid as expression bodies
    // (match arms, async bodies, etc.).
    const asExpr = block as unknown as Expression & BlockStatement;
    (asExpr as { kind: string }).kind = "expression";
    return asExpr;
  }

  private parseIfExpression(): IfExpression | null {
    const token = this.curToken;
    if (!this.expectPeek("(")) {
      return null;
    }

    this.nextToken();
    const condition = this.parseExpression(Precedence.Lowest);

    if (!this.expectPeek(")")) {
      return null;
    }
    if (!this.expectPeek("{")) {
      return null;
    }

    const consequence = this.parseBlockStatement();
    let alternative: BlockStatement | null = null;

    if (this.peekTokenIs("ELSE")) {
      this.nextToken();
      if (!this.expectPeek("{")) {
        return null;
      }
      alternative = this.parseBlockStatement();
    }

    return {
      kind: "expression",
      type: "IfExpression",
      token,
      condition,
      consequence,
      alternative,
      tokenLiteral: () => token.literal,
    };
  }

  private parseBlockStatement(): BlockStatement {
    const token = this.curToken;
    const statements: Statement[] = [];
    this.nextToken();

    while (!this.curTokenIs("}") && !this.curTokenIs("EOF")) {
      const stmt = this.parseStatement();
      if (stmt) {
        statements.push(stmt);
      }
      this.nextToken();
    }

    if (this.curTokenIs("EOF")) {
      this.errors.push("unexpected EOF while parsing block statement");
    }

    return {
      kind: "statement",
      type: "BlockStatement",
      token,
      statements,
      tokenLiteral: () => token.literal,
    };
  }

  private parseFunctionLiteral(isAsync: boolean): FunctionLiteral | null {
    const token = this.curToken;
    if (!this.expectPeek("(")) {
      return null;
    }

    const parameters = this.parseFunctionParameters();
    if (!parameters) {
      return null;
    }
    if (!this.expectPeek("{")) {
      return null;
    }

    const body = this.parseBlockStatement();
    return {
      kind: "expression",
      type: "FunctionLiteral",
      token,
      parameters,
      body,
      isAsync,
      tokenLiteral: () => token.literal,
    };
  }

  private parseAsyncExpression(): Expression | null {
    const token = this.curToken;
    if (this.peekTokenIs("FN")) {
      this.nextToken();
      return this.parseFunctionLiteral(true);
    }
    if (this.peekTokenIs("{")) {
      this.nextToken();
      const body = this.parseBlockStatement();
      const node: AsyncExpression = {
        kind: "expression",
        type: "AsyncExpression",
        token,
        body,
        tokenLiteral: () => token.literal,
      };
      return node;
    }
    this.nextToken();
    const body = this.parseExpression(Precedence.Prefix);
    if (!body) {
      return null;
    }
    const node: AsyncExpression = {
      kind: "expression",
      type: "AsyncExpression",
      token,
      body,
      tokenLiteral: () => token.literal,
    };
    return node;
  }

  private parseAwaitExpression(): AwaitExpression {
    const token = this.curToken;
    this.nextToken();
    const argument = this.parseExpression(Precedence.Prefix);
    return {
      kind: "expression",
      type: "AwaitExpression",
      token,
      argument,
      tokenLiteral: () => token.literal,
    };
  }

  private parseSpawnExpression(): SpawnExpression {
    const token = this.curToken;
    this.nextToken();
    const argument = this.parseExpression(Precedence.Prefix);
    return {
      kind: "expression",
      type: "SpawnExpression",
      token,
      argument,
      tokenLiteral: () => token.literal,
    };
  }

  private parseChanExpression(): ChanExpression | null {
    const token = this.curToken;
    if (!this.expectPeek("(")) {
      return null;
    }
    let capacity: Expression | null = null;
    if (!this.peekTokenIs(")")) {
      this.nextToken();
      capacity = this.parseExpression(Precedence.Lowest);
    }
    if (!this.expectPeek(")")) {
      return null;
    }
    return {
      kind: "expression",
      type: "ChanExpression",
      token,
      capacity,
      tokenLiteral: () => token.literal,
    };
  }

  private parseMatchExpression(): MatchExpression | null {
    const token = this.curToken;
    this.nextToken();
    const scrutinee = this.parseExpression(Precedence.Lowest);
    if (!this.expectPeek("{")) {
      return null;
    }

    const arms: MatchArm[] = [];
    this.nextToken();

    while (!this.curTokenIs("}") && !this.curTokenIs("EOF")) {
      const armToken = this.curToken;
      const pattern = this.parsePattern();
      if (!this.peekTokenIs("=>") && !this.peekTokenIs("->")) {
        this.errors.push(
          `expected => or -> after match pattern at line ${this.peekToken.line}, column ${this.peekToken.column}`,
        );
        return null;
      }
      this.nextToken();
      this.nextToken();
      const body = this.parseExpression(Precedence.Lowest);
      if (!body) {
        return null;
      }
      arms.push({ pattern, body, token: armToken });

      if (this.peekTokenIs(",")) {
        this.nextToken();
      }
      if (this.peekTokenIs(";")) {
        this.nextToken();
      }
      this.nextToken();
    }

    return {
      kind: "expression",
      type: "MatchExpression",
      token,
      scrutinee,
      arms,
      tokenLiteral: () => token.literal,
    };
  }

  private parsePattern(): Pattern {
    if (this.curTokenIs("IDENT") && this.curToken.literal === "_") {
      return { kind: "wildcard", token: this.curToken };
    }

    if (this.curTokenIs("INT")) {
      const lit = this.parseIntegerLiteral();
      return { kind: "literal", value: lit! };
    }
    if (this.curTokenIs("STRING")) {
      return { kind: "literal", value: this.parseStringLiteral() };
    }
    if (this.curTokenIs("TRUE") || this.curTokenIs("FALSE")) {
      return { kind: "literal", value: this.parseBoolean() };
    }

    if (this.curTokenIs("IDENT")) {
      const first = this.parseIdentifier();

      // Enum::Variant or Enum::Variant(...)
      if (this.peekTokenIs("::")) {
        this.nextToken();
        if (!this.expectPeek("IDENT")) {
          return { kind: "ident", name: first };
        }
        const variant = this.parseIdentifier();
        const fields: Pattern[] = [];
        if (this.peekTokenIs("(")) {
          this.nextToken();
          if (!this.peekTokenIs(")")) {
            this.nextToken();
            fields.push(this.parsePattern());
            while (this.peekTokenIs(",")) {
              this.nextToken();
              this.nextToken();
              fields.push(this.parsePattern());
            }
          }
          if (!this.expectPeek(")")) {
            // continue with what we have
          }
        }
        return {
          kind: "variant",
          enumName: first.value,
          variant,
          fields,
          token: first.token,
        };
      }

      // Variant(...)
      if (this.peekTokenIs("(")) {
        this.nextToken();
        const fields: Pattern[] = [];
        if (!this.peekTokenIs(")")) {
          this.nextToken();
          fields.push(this.parsePattern());
          while (this.peekTokenIs(",")) {
            this.nextToken();
            this.nextToken();
            fields.push(this.parsePattern());
          }
        }
        if (!this.expectPeek(")")) {
          // keep going
        }
        return {
          kind: "variant",
          enumName: null,
          variant: first,
          fields,
          token: first.token,
        };
      }

      return { kind: "ident", name: first };
    }

    this.errors.push(
      `invalid pattern at line ${this.curToken.line}, column ${this.curToken.column}`,
    );
    return {
      kind: "wildcard",
      token: this.curToken,
    };
  }

  private parseFunctionParameters(): Identifier[] | null {
    const identifiers: Identifier[] = [];

    if (this.peekTokenIs(")")) {
      this.nextToken();
      return identifiers;
    }

    this.nextToken();
    identifiers.push(this.parseIdentifier());

    while (this.peekTokenIs(",")) {
      this.nextToken();
      this.nextToken();
      identifiers.push(this.parseIdentifier());
    }

    if (!this.expectPeek(")")) {
      return null;
    }

    return identifiers;
  }

  private parseCallExpression(fn: Expression): Expression | null {
    const token = this.curToken;
    const args = this.parseExpressionList(")");
    if (!args) {
      return null;
    }

    // Promote Ident(args) that looks like a variant constructor when uppercase
    if (fn.type === "Identifier") {
      const name = (fn as Identifier).value;
      if (name.length > 0 && name[0]! >= "A" && name[0]! <= "Z") {
        const ctor: ConstructorExpression = {
          kind: "expression",
          type: "ConstructorExpression",
          token,
          enumName: null,
          variant: fn as Identifier,
          arguments: args,
          tokenLiteral: () => token.literal,
        };
        return ctor;
      }
    }

    return {
      kind: "expression",
      type: "CallExpression",
      token,
      function: fn,
      arguments: args,
      tokenLiteral: () => token.literal,
    } as CallExpression;
  }

  private parsePathConstructor(left: Expression): Expression | null {
    // left::Variant or left::Variant(args)
    if (left.type !== "Identifier") {
      this.errors.push(
        `expected identifier before :: at line ${this.curToken.line}, column ${this.curToken.column}`,
      );
      return null;
    }
    const enumName = (left as Identifier).value;
    if (!this.expectPeek("IDENT")) {
      return null;
    }
    const variant = this.parseIdentifier();
    const token = variant.token;
    let args: Expression[] = [];
    if (this.peekTokenIs("(")) {
      this.nextToken();
      const list = this.parseExpressionList(")");
      if (!list) {
        return null;
      }
      args = list;
    }
    const ctor: ConstructorExpression = {
      kind: "expression",
      type: "ConstructorExpression",
      token,
      enumName,
      variant,
      arguments: args,
      tokenLiteral: () => token.literal,
    };
    return ctor;
  }

  private parseMacroInvocation(left: Expression): Expression | null {
    // name!(args)
    if (left.type !== "Identifier") {
      this.errors.push(
        `macro invocation requires identifier at line ${this.curToken.line}, column ${this.curToken.column}`,
      );
      return null;
    }
    const name = left as Identifier;
    const token = this.curToken; // !
    if (!this.expectPeek("(")) {
      return null;
    }
    const args = this.parseExpressionList(")");
    if (!args) {
      return null;
    }
    const inv: MacroInvocation = {
      kind: "expression",
      type: "MacroInvocation",
      token,
      name,
      arguments: args,
      rawTokens: [],
      tokenLiteral: () => name.value + "!",
    };
    return inv;
  }

  private parseEffectDeclaration(): EffectDeclaration | null {
    const token = this.curToken;
    if (!this.expectPeek("IDENT")) {
      return null;
    }
    const name = this.parseIdentifier();
    if (!this.expectPeek("{")) {
      return null;
    }
    const operations: EffectOp[] = [];
    this.nextToken();
    while (!this.curTokenIs("}") && !this.curTokenIs("EOF")) {
      if (!this.curTokenIs("IDENT")) {
        this.errors.push(
          `expected effect operation name at line ${this.curToken.line}, column ${this.curToken.column}`,
        );
        break;
      }
      const opName = this.parseIdentifier();
      let parameters: Identifier[] = [];
      if (this.peekTokenIs("(")) {
        this.nextToken();
        const params = this.parseFunctionParameters();
        if (!params) {
          return null;
        }
        parameters = params;
      }
      operations.push({ name: opName, parameters });
      if (this.peekTokenIs(",")) {
        this.nextToken();
      }
      if (this.peekTokenIs(";")) {
        this.nextToken();
      }
      this.nextToken();
    }
    return {
      kind: "statement",
      type: "EffectDeclaration",
      token,
      name,
      operations,
      tokenLiteral: () => token.literal,
    };
  }

  private parseStructDeclaration(): StructDeclaration | null {
    const token = this.curToken;
    const attributes: Attribute[] = [];
    if (!this.expectPeek("IDENT")) {
      return null;
    }
    const name = this.parseIdentifier();
    if (!this.expectPeek("{")) {
      return null;
    }
    const fields: StructField[] = [];
    this.nextToken();
    while (!this.curTokenIs("}") && !this.curTokenIs("EOF")) {
      if (!this.curTokenIs("IDENT")) {
        break;
      }
      const fieldName = this.parseIdentifier();
      let typeName = "any";
      if (this.peekTokenIs(":")) {
        this.nextToken();
        this.nextToken();
        typeName = this.curToken.literal;
      }
      fields.push({ name: fieldName, typeName });
      if (this.peekTokenIs(",")) {
        this.nextToken();
      }
      if (this.peekTokenIs(";")) {
        this.nextToken();
      }
      this.nextToken();
    }
    if (this.peekTokenIs(";")) {
      this.nextToken();
    }
    return {
      kind: "statement",
      type: "StructDeclaration",
      token,
      name,
      fields,
      attributes,
      tokenLiteral: () => token.literal,
    };
  }

  private parsePerformExpression(): PerformExpression | null {
    const token = this.curToken;
    this.nextToken();
    if (!this.curTokenIs("IDENT")) {
      this.errors.push(
        `expected operation after perform at line ${this.curToken.line}, column ${this.curToken.column}`,
      );
      return null;
    }
    let effectName: string | null = null;
    let operation = this.parseIdentifier();
    if (this.peekTokenIs("::")) {
      this.nextToken();
      effectName = operation.value;
      if (!this.expectPeek("IDENT")) {
        return null;
      }
      operation = this.parseIdentifier();
    }
    let args: Expression[] = [];
    if (this.peekTokenIs("(")) {
      this.nextToken();
      const list = this.parseExpressionList(")");
      if (!list) {
        return null;
      }
      args = list;
    }
    return {
      kind: "expression",
      type: "PerformExpression",
      token,
      effectName,
      operation,
      arguments: args,
      tokenLiteral: () => token.literal,
    };
  }

  private parseHandleExpression(): HandleExpression | null {
    const token = this.curToken;
    if (!this.expectPeek("{")) {
      return null;
    }
    const body = this.parseBlockStatement();
    if (!this.expectPeek("WITH")) {
      return null;
    }
    if (!this.expectPeek("IDENT")) {
      return null;
    }
    const effectName = this.parseIdentifier();
    if (!this.expectPeek("{")) {
      return null;
    }
    const handlers: EffectHandlerArm[] = [];
    this.nextToken();
    while (!this.curTokenIs("}") && !this.curTokenIs("EOF")) {
      if (!this.curTokenIs("IDENT")) {
        break;
      }
      const armToken = this.curToken;
      const op = this.parseIdentifier();
      let parameters: Identifier[] = [];
      if (this.peekTokenIs("(")) {
        this.nextToken();
        const params = this.parseFunctionParameters();
        if (!params) {
          return null;
        }
        parameters = params;
      }
      if (!this.expectPeek("=>")) {
        return null;
      }
      this.nextToken();
      const armBody = this.parseExpression(Precedence.Lowest);
      if (!armBody) {
        return null;
      }
      handlers.push({
        operation: op,
        parameters,
        body: armBody,
        token: armToken,
      });
      if (this.peekTokenIs(",")) {
        this.nextToken();
      }
      if (this.peekTokenIs(";")) {
        this.nextToken();
      }
      this.nextToken();
    }
    return {
      kind: "expression",
      type: "HandleExpression",
      token,
      body,
      effectName,
      handlers,
      tokenLiteral: () => token.literal,
    };
  }

  private parseRegionExpression(): RegionExpression | null {
    const token = this.curToken;
    let name: Identifier | null = null;
    if (this.peekTokenIs("IDENT")) {
      this.nextToken();
      name = this.parseIdentifier();
    }
    if (!this.expectPeek("{")) {
      return null;
    }
    const body = this.parseBlockStatement();
    return {
      kind: "expression",
      type: "RegionExpression",
      token,
      name,
      body,
      tokenLiteral: () => token.literal,
    };
  }

  private parseReflectExpression(): ReflectExpression | null {
    const token = this.curToken;
    if (!this.expectPeek("(")) {
      return null;
    }
    if (!this.expectPeek("IDENT")) {
      return null;
    }
    const target = this.parseIdentifier();
    if (!this.expectPeek(")")) {
      return null;
    }
    return {
      kind: "expression",
      type: "ReflectExpression",
      token,
      target,
      tokenLiteral: () => token.literal,
    };
  }

  private parseExpressionList(end: TokenType): Expression[] | null {
    const list: Expression[] = [];

    if (this.peekTokenIs(end)) {
      this.nextToken();
      return list;
    }

    this.nextToken();
    const first = this.parseExpression(Precedence.Lowest);
    if (first) {
      list.push(first);
    }

    while (this.peekTokenIs(",")) {
      this.nextToken();
      this.nextToken();
      const exp = this.parseExpression(Precedence.Lowest);
      if (exp) {
        list.push(exp);
      }
    }

    if (!this.expectPeek(end)) {
      return null;
    }

    return list;
  }

  private curTokenIs(type: TokenType): boolean {
    return this.curToken.type === type;
  }

  private peekTokenIs(type: TokenType): boolean {
    return this.peekToken.type === type;
  }

  private expectPeek(type: TokenType): boolean {
    if (this.peekTokenIs(type)) {
      this.nextToken();
      return true;
    }
    this.errors.push(
      `expected next token to be ${type}, got ${this.peekToken.type} instead at line ${this.peekToken.line}, column ${this.peekToken.column}`,
    );
    return false;
  }

  private peekPrecedence(): Precedence {
    return PRECEDENCES[this.peekToken.type] ?? Precedence.Lowest;
  }

  private curPrecedence(): Precedence {
    return PRECEDENCES[this.curToken.type] ?? Precedence.Lowest;
  }
}
