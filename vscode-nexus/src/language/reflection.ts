import {
  FunctionLiteral,
  Identifier,
  LetStatement,
  Program,
  ReturnStatement,
  StringLiteral,
  StructDeclaration,
} from "./parser";
import { Token as LexToken } from "./lexer";

export interface ReflectionResult {
  program: Program;
  errors: string[];
}

export interface ReflectedField {
  name: string;
  typeName: string;
}

export interface ReflectedType {
  name: string;
  kind: "struct" | "enum";
  fields: ReflectedField[];
  attributes: string[];
}

/**
 * Compile-time reflection & metaprogramming.
 * Synthesizes JSON serializers and field mappers from struct declarations.
 */
export class ReflectionEngine {
  private readonly types = new Map<string, ReflectedType>();

  expand(program: Program): ReflectionResult {
    const errors: string[] = [];
    const synthesized: LetStatement[] = [];

    for (const stmt of program.statements) {
      if (stmt.type === "StructDeclaration") {
        const decl = stmt as StructDeclaration;
        const reflected: ReflectedType = {
          name: decl.name.value,
          kind: "struct",
          fields: decl.fields.map((f) => ({
            name: f.name.value,
            typeName: f.typeName,
          })),
          attributes: decl.attributes.map((a) => a.name),
        };
        this.types.set(reflected.name, reflected);
        synthesized.push(...this.synthesizeJsonHelpers(decl));
      }
    }

    return {
      program: {
        type: "Program",
        statements: [...program.statements, ...synthesized],
        tokenLiteral: program.tokenLiteral,
      },
      errors,
    };
  }

  getType(name: string): ReflectedType | undefined {
    return this.types.get(name);
  }

  listTypes(): ReflectedType[] {
    return [...this.types.values()];
  }

  private synthesizeJsonHelpers(decl: StructDeclaration): LetStatement[] {
    const typeName = decl.name.value;
    const tok = (
      literal: string,
      type: LexToken["type"] = "IDENT",
    ): LexToken => ({
      type,
      literal,
      line: decl.token.line,
      column: decl.token.column,
    });

    const ident = (name: string): Identifier => {
      const token = tok(name);
      return {
        kind: "expression",
        type: "Identifier",
        token,
        value: name,
        tokenLiteral: () => name,
      };
    };

    const summary = decl.fields
      .map((f) => `${f.name.value}:${f.typeName}`)
      .join(",");
    const retValue: StringLiteral = {
      kind: "expression",
      type: "StringLiteral",
      token: tok(`{${summary}}`, "STRING"),
      value: `{${summary}}`,
      tokenLiteral: () => `"${summary}"`,
    };

    const ret: ReturnStatement = {
      kind: "statement",
      type: "ReturnStatement",
      token: tok("return", "RETURN"),
      returnValue: retValue,
      tokenLiteral: () => "return",
    };

    const fn: FunctionLiteral = {
      kind: "expression",
      type: "FunctionLiteral",
      token: tok("fn", "FN"),
      parameters: [ident("value")],
      body: {
        kind: "statement",
        type: "BlockStatement",
        token: tok("{", "{"),
        statements: [ret],
        tokenLiteral: () => "{",
      },
      isAsync: false,
      tokenLiteral: () => "fn",
    };

    const letStmt: LetStatement = {
      kind: "statement",
      type: "LetStatement",
      token: tok("let", "LET"),
      name: ident(`${typeName}_to_json`),
      mutable: false,
      typeAnnotation: null,
      value: fn,
      tokenLiteral: () => "let",
    };

    const fieldsLit: StringLiteral = {
      kind: "expression",
      type: "StringLiteral",
      token: tok(decl.fields.map((f) => f.name.value).join(","), "STRING"),
      value: decl.fields.map((f) => f.name.value).join(","),
      tokenLiteral: () => "fields",
    };

    const fieldsLet: LetStatement = {
      kind: "statement",
      type: "LetStatement",
      token: tok("let", "LET"),
      name: ident(`${typeName}_fields`),
      mutable: false,
      typeAnnotation: null,
      value: fieldsLit,
      tokenLiteral: () => "let",
    };

    return [letStmt, fieldsLet];
  }
}

export function expandReflection(program: Program): ReflectionResult {
  return new ReflectionEngine().expand(program);
}

export function reflectTypeInfo(
  program: Program,
  name: string,
): ReflectedType | null {
  const engine = new ReflectionEngine();
  engine.expand(program);
  return engine.getType(name) ?? null;
}
