import { BUILTIN_NAMES } from "./builtins";
import {
  ArrayLiteral,
  AssignExpression,
  BlockStatement,
  CallExpression,
  ConstructorExpression,
  Expression,
  ExpressionStatement,
  FunctionLiteral,
  HashLiteral,
  Identifier,
  IfExpression,
  IndexExpression,
  InfixExpression,
  LetStatement,
  MatchExpression,
  MemberExpression,
  PipeExpression,
  PrefixExpression,
  Program,
  ReturnStatement,
  Statement,
} from "./parser";

export type StaticCheckCode =
  | "undefined-name"
  | "arity-mismatch"
  | "unused-local";

export type StaticCheckSeverity = "error" | "warning";

export interface StaticCheckDiagnostic {
  message: string;
  line: number;
  column: number;
  severity: StaticCheckSeverity;
  code: StaticCheckCode;
}

type AritySpec = { min: number; max: number | null };

const BUILTIN_ARITY: Record<string, AritySpec> = {
  len: { min: 1, max: 1 },
  puts: { min: 0, max: null },
  str: { min: 1, max: 1 },
  int: { min: 1, max: 1 },
  type: { min: 1, max: 1 },
  typeof: { min: 1, max: 1 },
  push: { min: 2, max: null },
  first: { min: 1, max: 1 },
  last: { min: 1, max: 1 },
  rest: { min: 1, max: 1 },
  keys: { min: 1, max: 1 },
  has: { min: 2, max: 2 },
  get: { min: 2, max: 2 },
  split: { min: 2, max: 2 },
  join: { min: 2, max: 2 },
  trim: { min: 1, max: 1 },
  lower: { min: 1, max: 1 },
  upper: { min: 1, max: 1 },
  contains: { min: 2, max: 2 },
  starts_with: { min: 2, max: 2 },
  replace: { min: 3, max: 3 },
  slice: { min: 2, max: 3 },
  ok: { min: 1, max: 1 },
  err: { min: 1, max: 1 },
  is_ok: { min: 1, max: 1 },
  is_err: { min: 1, max: 1 },
  unwrap: { min: 1, max: 1 },
  map: { min: 2, max: 2 },
  filter: { min: 2, max: 2 },
  assert: { min: 1, max: 2 },
  assert_eq: { min: 2, max: 3 },
  getenv: { min: 1, max: 1 },
  escape_html: { min: 1, max: 1 },
  merge: { min: 2, max: 2 },
};

interface BindingInfo {
  name: string;
  arity: number | null;
  used: boolean;
  token: { line: number; column: number };
}

interface Scope {
  bindings: Map<string, BindingInfo>;
  parent: Scope | null;
}

/**
 * Lightweight static checks shared by diagnostics and `nex-ts check`.
 */
export function checkProgram(program: Program): StaticCheckDiagnostic[] {
  const checker = new StaticChecker();
  return checker.check(program);
}

class StaticChecker {
  private readonly diagnostics: StaticCheckDiagnostic[] = [];
  private scope: Scope = { bindings: new Map(), parent: null };

  check(program: Program): StaticCheckDiagnostic[] {
    this.diagnostics.length = 0;
    this.scope = { bindings: new Map(), parent: null };

    for (const stmt of program.statements) {
      this.collectBindings(stmt);
    }
    for (const stmt of program.statements) {
      this.checkStatement(stmt);
    }
    this.reportUnusedInScope(this.scope);
    return [...this.diagnostics];
  }

  private collectBindings(stmt: Statement): void {
    if (stmt.type !== "LetStatement") {
      return;
    }
    const letStmt = stmt as LetStatement;
    const arity = functionArity(letStmt.value);
    this.define(letStmt.name.value, arity, letStmt.name.token);
  }

  private checkStatement(stmt: Statement): void {
    switch (stmt.type) {
      case "LetStatement": {
        const letStmt = stmt as LetStatement;
        if (letStmt.value) {
          this.checkExpression(letStmt.value);
        }
        break;
      }
      case "ReturnStatement": {
        const rs = stmt as ReturnStatement;
        if (rs.returnValue) {
          this.checkExpression(rs.returnValue);
        }
        break;
      }
      case "ExpressionStatement": {
        const es = stmt as ExpressionStatement;
        if (es.expression) {
          this.checkExpression(es.expression);
        }
        break;
      }
      case "BlockStatement":
        this.checkBlock(stmt as BlockStatement);
        break;
      default:
        break;
    }
  }

  private checkBlock(block: BlockStatement): void {
    this.pushScope();
    for (const stmt of block.statements) {
      this.collectBindings(stmt);
    }
    for (const stmt of block.statements) {
      this.checkStatement(stmt);
    }
    this.reportUnusedInScope(this.scope);
    this.popScope();
  }

  private checkExpression(expr: Expression): void {
    switch (expr.type) {
      case "Identifier":
        this.checkIdentifier(expr as Identifier);
        break;
      case "PrefixExpression": {
        const p = expr as PrefixExpression;
        if (p.right) {
          this.checkExpression(p.right);
        }
        break;
      }
      case "InfixExpression": {
        const infix = expr as InfixExpression;
        this.checkExpression(infix.left);
        if (infix.right) {
          this.checkExpression(infix.right);
        }
        break;
      }
      case "IfExpression": {
        const iff = expr as IfExpression;
        if (iff.condition) {
          this.checkExpression(iff.condition);
        }
        this.checkBlock(iff.consequence);
        if (iff.alternative) {
          this.checkBlock(iff.alternative);
        }
        break;
      }
      case "BlockStatement":
        this.checkBlock(expr as unknown as BlockStatement);
        break;
      case "FunctionLiteral": {
        const fn = expr as FunctionLiteral;
        this.pushScope();
        for (const param of fn.parameters) {
          this.define(param.value, null, param.token);
        }
        this.checkBlock(fn.body);
        this.reportUnusedInScope(this.scope);
        this.popScope();
        break;
      }
      case "CallExpression":
        this.checkCall(expr as CallExpression);
        break;
      default:
        this.walkChildren(expr);
        break;
    }
  }

  private walkChildren(expr: Expression): void {
    switch (expr.type) {
      case "ArrayLiteral":
        for (const el of (expr as ArrayLiteral).elements) {
          this.checkExpression(el);
        }
        break;
      case "HashLiteral":
        for (const pair of (expr as HashLiteral).pairs) {
          this.checkExpression(pair.value);
        }
        break;
      case "IndexExpression": {
        const idx = expr as IndexExpression;
        this.checkExpression(idx.left);
        if (idx.index) {
          this.checkExpression(idx.index);
        }
        break;
      }
      case "MemberExpression": {
        const mem = expr as MemberExpression;
        this.checkExpression(mem.object);
        break;
      }
      case "AssignExpression": {
        const assign = expr as AssignExpression;
        if (assign.value) {
          this.checkExpression(assign.value);
        }
        break;
      }
      case "PipeExpression": {
        const pipe = expr as PipeExpression;
        this.checkExpression(pipe.left);
        if (pipe.right) {
          this.checkExpression(pipe.right);
        }
        break;
      }
      case "MatchExpression": {
        const match = expr as MatchExpression;
        if (match.scrutinee) {
          this.checkExpression(match.scrutinee);
        }
        for (const arm of match.arms) {
          this.checkExpression(arm.body);
        }
        break;
      }
      case "ConstructorExpression": {
        for (const arg of (expr as ConstructorExpression).arguments) {
          this.checkExpression(arg);
        }
        break;
      }
      case "AsyncExpression":
      case "AwaitExpression":
      case "SpawnExpression":
      case "RefExpression":
      case "MoveExpression":
        this.walkUnary(expr);
        break;
      default:
        break;
    }
  }

  private walkUnary(expr: Expression): void {
    const node = expr as { argument?: Expression | null; value?: Expression | null; body?: BlockStatement | Expression };
    if (node.argument) {
      this.checkExpression(node.argument);
    }
    if (node.value) {
      this.checkExpression(node.value);
    }
    if (node.body) {
      if ((node.body as BlockStatement).type === "BlockStatement") {
        this.checkBlock(node.body as BlockStatement);
      } else {
        this.checkExpression(node.body as Expression);
      }
    }
  }

  private checkCall(call: CallExpression): void {
    this.checkExpression(call.function);
    for (const arg of call.arguments) {
      this.checkExpression(arg);
    }

    const callee = resolveCalleeName(call.function);
    if (!callee) {
      return;
    }

    const arity = this.lookupArity(callee);
    if (arity === undefined) {
      return;
    }

    const got = call.arguments.length;
    if (!arityMatches(arity, got)) {
      const want = formatArity(arity);
      this.push(
        `function \`${callee}\` expects ${want} argument(s), got ${got} at line ${call.token.line}, column ${call.token.column}`,
        call.token.line,
        call.token.column,
        "error",
        "arity-mismatch",
      );
    }
  }

  private checkIdentifier(ident: Identifier): void {
    const binding = this.lookup(ident.value);
    if (!binding) {
      if (isBuiltin(ident.value)) {
        return;
      }
      this.push(
        `undefined name \`${ident.value}\` at line ${ident.token.line}, column ${ident.token.column}`,
        ident.token.line,
        ident.token.column,
        "error",
        "undefined-name",
      );
      return;
    }
    binding.used = true;
  }

  private lookupArity(name: string): AritySpec | number | undefined {
    if (isBuiltin(name)) {
      return BUILTIN_ARITY[name] ?? { min: 0, max: null };
    }
    const binding = this.lookup(name);
    if (!binding || binding.arity === null) {
      return undefined;
    }
    return binding.arity;
  }

  private reportUnusedInScope(scope: Scope): void {
    for (const binding of scope.bindings.values()) {
      if (!binding.used) {
        this.push(
          `unused local \`${binding.name}\` at line ${binding.token.line}, column ${binding.token.column}`,
          binding.token.line,
          binding.token.column,
          "warning",
          "unused-local",
        );
      }
    }
  }

  private push(
    message: string,
    line: number,
    column: number,
    severity: StaticCheckSeverity,
    code: StaticCheckCode,
  ): void {
    this.diagnostics.push({ message, line, column, severity, code });
  }

  private pushScope(): void {
    this.scope = { bindings: new Map(), parent: this.scope };
  }

  private popScope(): void {
    if (this.scope.parent) {
      this.scope = this.scope.parent;
    }
  }

  private define(
    name: string,
    arity: number | null,
    token: { line: number; column: number },
  ): void {
    this.scope.bindings.set(name, { name, arity, used: false, token });
  }

  private lookup(name: string): BindingInfo | undefined {
    let cur: Scope | null = this.scope;
    while (cur) {
      const binding = cur.bindings.get(name);
      if (binding) {
        return binding;
      }
      cur = cur.parent;
    }
    return undefined;
  }
}

function isBuiltin(name: string): boolean {
  return (BUILTIN_NAMES as readonly string[]).includes(name);
}

function functionArity(expr: Expression | null): number | null {
  if (!expr || expr.type !== "FunctionLiteral") {
    return null;
  }
  return (expr as FunctionLiteral).parameters.length;
}

function resolveCalleeName(expr: Expression): string | null {
  if (expr.type === "Identifier") {
    return (expr as Identifier).value;
  }
  return null;
}

function arityMatches(spec: AritySpec | number, got: number): boolean {
  if (typeof spec === "number") {
    return got === spec;
  }
  if (got < spec.min) {
    return false;
  }
  if (spec.max !== null && got > spec.max) {
    return false;
  }
  return true;
}

function formatArity(spec: AritySpec | number): string {
  if (typeof spec === "number") {
    return String(spec);
  }
  if (spec.max === null) {
    if (spec.min === 0) {
      return "any number of";
    }
    return `at least ${spec.min}`;
  }
  if (spec.min === spec.max) {
    return String(spec.min);
  }
  return `${spec.min} to ${spec.max}`;
}
