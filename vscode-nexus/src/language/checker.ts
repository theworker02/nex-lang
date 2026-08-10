import {
  AsyncExpression,
  AwaitExpression,
  BlockStatement,
  CallExpression,
  ChanExpression,
  ConstructorExpression,
  EnumDeclaration,
  Expression,
  ExpressionStatement,
  FunctionLiteral,
  Identifier,
  IfExpression,
  InfixExpression,
  LetStatement,
  MatchArm,
  MatchExpression,
  MoveExpression,
  Node,
  Pattern,
  PrefixExpression,
  Program,
  RefExpression,
  ReturnStatement,
  SpawnExpression,
  Statement,
} from "./parser";
import { Token } from "./lexer";

export type CheckSeverity = "error" | "warning";

export interface CheckDiagnostic {
  message: string;
  line: number;
  column: number;
  severity: CheckSeverity;
  code:
    | "use-after-move"
    | "borrow-conflict"
    | "invalid-borrow"
    | "non-exhaustive-match"
    | "unknown-variant"
    | "ownership"
    | "other";
}

type OwnershipState =
  | { kind: "owned" }
  | { kind: "moved"; atLine: number; atColumn: number }
  | { kind: "borrowed"; shared: number; mutable: boolean };

interface Binding {
  name: string;
  mutable: boolean;
  copyable: boolean;
  state: OwnershipState;
  token: Token;
}

interface EnumInfo {
  name: string;
  variants: Map<string, number>; // variant -> field count
}

interface Scope {
  bindings: Map<string, Binding>;
  parent: Scope | null;
}

/**
 * Affine ownership / borrow checker plus match exhaustiveness analysis.
 */
export class OwnershipChecker {
  private readonly diagnostics: CheckDiagnostic[] = [];
  private readonly enums = new Map<string, EnumInfo>();
  private readonly variantToEnum = new Map<string, string>();
  private scope: Scope = { bindings: new Map(), parent: null };

  check(program: Program): CheckDiagnostic[] {
    this.diagnostics.length = 0;
    this.enums.clear();
    this.variantToEnum.clear();
    this.scope = { bindings: new Map(), parent: null };

    // First pass: collect enum declarations.
    for (const stmt of program.statements) {
      if (stmt.type === "EnumDeclaration") {
        this.registerEnum(stmt as EnumDeclaration);
      }
    }

    for (const stmt of program.statements) {
      this.checkStatement(stmt);
    }

    return [...this.diagnostics];
  }

  private registerEnum(decl: EnumDeclaration): void {
    const variants = new Map<string, number>();
    for (const v of decl.variants) {
      variants.set(v.name.value, v.fields.length);
      this.variantToEnum.set(v.name.value, decl.name.value);
    }
    this.enums.set(decl.name.value, {
      name: decl.name.value,
      variants,
    });
  }

  private pushScope(): void {
    this.scope = { bindings: new Map(), parent: this.scope };
  }

  private popScope(): void {
    if (this.scope.parent) {
      this.scope = this.scope.parent;
    }
  }

  private lookup(name: string): Binding | undefined {
    let cur: Scope | null = this.scope;
    while (cur) {
      const b = cur.bindings.get(name);
      if (b) {
        return b;
      }
      cur = cur.parent;
    }
    return undefined;
  }

  private define(binding: Binding): void {
    this.scope.bindings.set(binding.name, binding);
  }

  private error(
    message: string,
    line: number,
    column: number,
    code: CheckDiagnostic["code"],
  ): void {
    this.diagnostics.push({ message, line, column, severity: "error", code });
  }

  private checkStatement(stmt: Statement): void {
    switch (stmt.type) {
      case "LetStatement":
        this.checkLet(stmt as LetStatement);
        break;
      case "ReturnStatement": {
        const rs = stmt as ReturnStatement;
        if (rs.returnValue) {
          this.checkExpression(rs.returnValue, "use");
        }
        break;
      }
      case "ExpressionStatement": {
        const es = stmt as ExpressionStatement;
        if (es.expression) {
          this.checkExpression(es.expression, "use");
        }
        break;
      }
      case "BlockStatement":
        this.checkBlock(stmt as BlockStatement);
        break;
      case "EnumDeclaration":
      case "MacroDefinition":
      case "ExternDeclaration":
        break;
      default:
        break;
    }
  }

  private checkLet(stmt: LetStatement): void {
    if (stmt.value) {
      this.checkExpression(stmt.value, "move");
    }

    const copyable = this.isCopyableExpression(stmt.value);
    this.define({
      name: stmt.name.value,
      mutable: stmt.mutable,
      copyable,
      state: { kind: "owned" },
      token: stmt.name.token,
    });
  }

  private checkBlock(block: BlockStatement): void {
    this.pushScope();
    for (const stmt of block.statements) {
      this.checkStatement(stmt);
    }
    this.popScope();
  }

  /**
   * @param mode "use" reads a binding; "move" consumes it (unless copyable).
   */
  private checkExpression(
    expr: Expression,
    mode: "use" | "move",
  ): void {
    switch (expr.type) {
      case "Identifier":
        this.checkIdent(expr as Identifier, mode);
        break;
      case "IntegerLiteral":
      case "StringLiteral":
      case "BooleanLiteral":
        break;
      case "PrefixExpression": {
        const p = expr as PrefixExpression;
        if (p.right) {
          this.checkExpression(p.right, "use");
        }
        break;
      }
      case "InfixExpression": {
        const infix = expr as InfixExpression;
        this.checkExpression(infix.left, "use");
        if (infix.right) {
          this.checkExpression(infix.right, "use");
        }
        break;
      }
      case "IfExpression": {
        const iff = expr as IfExpression;
        if (iff.condition) {
          this.checkExpression(iff.condition, "use");
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
        for (const p of fn.parameters) {
          this.define({
            name: p.value,
            mutable: false,
            copyable: true,
            state: { kind: "owned" },
            token: p.token,
          });
        }
        this.checkBlock(fn.body);
        this.popScope();
        break;
      }
      case "CallExpression": {
        const call = expr as CallExpression;
        this.checkExpression(call.function, "use");
        for (const arg of call.arguments) {
          this.checkExpression(arg, "move");
        }
        break;
      }
      case "ConstructorExpression": {
        const ctor = expr as ConstructorExpression;
        this.validateConstructor(ctor);
        for (const arg of ctor.arguments) {
          this.checkExpression(arg, "move");
        }
        break;
      }
      case "MatchExpression":
        this.checkMatch(expr as MatchExpression);
        break;
      case "RefExpression":
        this.checkRef(expr as RefExpression);
        break;
      case "MoveExpression": {
        const mv = expr as MoveExpression;
        if (mv.value) {
          this.checkExpression(mv.value, "move");
        }
        break;
      }
      case "AsyncExpression": {
        const a = expr as AsyncExpression;
        if (a.body.type === "BlockStatement") {
          this.checkBlock(a.body as BlockStatement);
        } else {
          this.checkExpression(a.body as Expression, "use");
        }
        break;
      }
      case "AwaitExpression": {
        const a = expr as AwaitExpression;
        if (a.argument) {
          this.checkExpression(a.argument, "use");
        }
        break;
      }
      case "SpawnExpression": {
        const s = expr as SpawnExpression;
        if (s.argument) {
          this.checkExpression(s.argument, "use");
        }
        break;
      }
      case "ChanExpression": {
        const c = expr as ChanExpression;
        if (c.capacity) {
          this.checkExpression(c.capacity, "use");
        }
        break;
      }
      case "MacroInvocation":
        // Macros should be expanded before checking; treat args cautiously.
        break;
      default:
        break;
    }
  }

  private checkIdent(ident: Identifier, mode: "use" | "move"): void {
    const binding = this.lookup(ident.value);
    if (!binding) {
      return; // unresolved names are runtime / other errors
    }

    if (binding.state.kind === "moved") {
      this.error(
        `use of moved value \`${ident.value}\` at line ${ident.token.line}, column ${ident.token.column} (moved at line ${binding.state.atLine}, column ${binding.state.atColumn})`,
        ident.token.line,
        ident.token.column,
        "use-after-move",
      );
      return;
    }

    if (mode === "move" && !binding.copyable) {
      if (binding.state.kind === "borrowed") {
        this.error(
          `cannot move \`${ident.value}\` while borrowed at line ${ident.token.line}, column ${ident.token.column}`,
          ident.token.line,
          ident.token.column,
          "borrow-conflict",
        );
        return;
      }
      binding.state = {
        kind: "moved",
        atLine: ident.token.line,
        atColumn: ident.token.column,
      };
    }

    if (mode === "use" && binding.state.kind === "borrowed" && binding.state.mutable) {
      // shared use while mutably borrowed is ok for the owner reading through? disallow
      // Owner can still be named when only shared-borrowed; mutable borrow exclusive.
    }
  }

  private checkRef(ref: RefExpression): void {
    if (!ref.value) {
      this.error(
        `borrow missing operand at line ${ref.token.line}, column ${ref.token.column}`,
        ref.token.line,
        ref.token.column,
        "invalid-borrow",
      );
      return;
    }

    if (ref.value.type !== "Identifier") {
      this.checkExpression(ref.value, "use");
      return;
    }

    const ident = ref.value as Identifier;
    const binding = this.lookup(ident.value);
    if (!binding) {
      return;
    }

    if (binding.state.kind === "moved") {
      this.error(
        `cannot borrow moved value \`${ident.value}\` at line ${ident.token.line}, column ${ident.token.column}`,
        ident.token.line,
        ident.token.column,
        "use-after-move",
      );
      return;
    }

    if (ref.mutable) {
      if (!binding.mutable) {
        this.error(
          `cannot mutably borrow immutable binding \`${ident.value}\` at line ${ident.token.line}, column ${ident.token.column}`,
          ident.token.line,
          ident.token.column,
          "invalid-borrow",
        );
        return;
      }
      if (binding.state.kind === "borrowed") {
        this.error(
          `cannot mutably borrow \`${ident.value}\` while already borrowed at line ${ident.token.line}, column ${ident.token.column}`,
          ident.token.line,
          ident.token.column,
          "borrow-conflict",
        );
        return;
      }
      binding.state = { kind: "borrowed", shared: 0, mutable: true };
    } else {
      if (binding.state.kind === "borrowed" && binding.state.mutable) {
        this.error(
          `cannot shared-borrow \`${ident.value}\` while mutably borrowed at line ${ident.token.line}, column ${ident.token.column}`,
          ident.token.line,
          ident.token.column,
          "borrow-conflict",
        );
        return;
      }
      if (binding.state.kind === "owned") {
        binding.state = { kind: "borrowed", shared: 1, mutable: false };
      } else if (binding.state.kind === "borrowed") {
        binding.state.shared += 1;
      }
    }
  }

  private checkMatch(matchExpr: MatchExpression): void {
    if (matchExpr.scrutinee) {
      this.checkExpression(matchExpr.scrutinee, "use");
    }

    for (const arm of matchExpr.arms) {
      this.pushScope();
      this.bindPattern(arm.pattern);
      this.checkExpression(arm.body, "use");
      this.popScope();
    }

    this.checkExhaustiveness(matchExpr);
  }

  private bindPattern(pattern: Pattern): void {
    switch (pattern.kind) {
      case "wildcard":
        break;
      case "ident":
        this.define({
          name: pattern.name.value,
          mutable: false,
          copyable: true,
          state: { kind: "owned" },
          token: pattern.name.token,
        });
        break;
      case "literal":
        break;
      case "variant":
        for (const field of pattern.fields) {
          this.bindPattern(field);
        }
        break;
    }
  }

  private checkExhaustiveness(matchExpr: MatchExpression): void {
    const enumName = this.inferMatchEnum(matchExpr);
    if (!enumName) {
      // If any arm is wildcard or ident catch-all, OK; otherwise skip.
      const hasCatchAll = matchExpr.arms.some(
        (a) => a.pattern.kind === "wildcard" || a.pattern.kind === "ident",
      );
      if (!hasCatchAll && matchExpr.arms.length > 0) {
        // Only enforce when we know it's an enum.
      }
      return;
    }

    const info = this.enums.get(enumName);
    if (!info) {
      return;
    }

    const covered = new Set<string>();
    let catchAll = false;

    for (const arm of matchExpr.arms) {
      if (arm.pattern.kind === "wildcard" || arm.pattern.kind === "ident") {
        catchAll = true;
        continue;
      }
      if (arm.pattern.kind === "variant") {
        covered.add(arm.pattern.variant.value);
      }
    }

    if (catchAll) {
      return;
    }

    const missing: string[] = [];
    for (const variant of info.variants.keys()) {
      if (!covered.has(variant)) {
        missing.push(variant);
      }
    }

    if (missing.length > 0) {
      this.error(
        `non-exhaustive match over enum \`${enumName}\`: missing variant(s) ${missing.join(", ")} at line ${matchExpr.token.line}, column ${matchExpr.token.column}`,
        matchExpr.token.line,
        matchExpr.token.column,
        "non-exhaustive-match",
      );
    }
  }

  private inferMatchEnum(matchExpr: MatchExpression): string | null {
    for (const arm of matchExpr.arms) {
      if (arm.pattern.kind === "variant") {
        if (arm.pattern.enumName) {
          return arm.pattern.enumName;
        }
        const mapped = this.variantToEnum.get(arm.pattern.variant.value);
        if (mapped) {
          return mapped;
        }
      }
    }

    if (matchExpr.scrutinee?.type === "ConstructorExpression") {
      const ctor = matchExpr.scrutinee as ConstructorExpression;
      if (ctor.enumName) {
        return ctor.enumName;
      }
      return this.variantToEnum.get(ctor.variant.value) ?? null;
    }

    return null;
  }

  private validateConstructor(ctor: ConstructorExpression): void {
    const enumName =
      ctor.enumName ?? this.variantToEnum.get(ctor.variant.value) ?? null;
    if (!enumName) {
      return;
    }
    const info = this.enums.get(enumName);
    if (!info) {
      return;
    }
    if (!info.variants.has(ctor.variant.value)) {
      this.error(
        `unknown variant \`${ctor.variant.value}\` for enum \`${enumName}\` at line ${ctor.token.line}, column ${ctor.token.column}`,
        ctor.token.line,
        ctor.token.column,
        "unknown-variant",
      );
      return;
    }
    const want = info.variants.get(ctor.variant.value)!;
    if (want !== ctor.arguments.length) {
      this.error(
        `variant \`${ctor.variant.value}\` expects ${want} field(s), got ${ctor.arguments.length} at line ${ctor.token.line}, column ${ctor.token.column}`,
        ctor.token.line,
        ctor.token.column,
        "unknown-variant",
      );
    }
  }

  private isCopyableExpression(expr: Expression | null): boolean {
    if (!expr) {
      return true;
    }
    switch (expr.type) {
      case "IntegerLiteral":
      case "StringLiteral":
      case "BooleanLiteral":
      case "RefExpression":
      case "FunctionLiteral":
      case "ChanExpression":
        return true;
      case "Identifier": {
        const b = this.lookup((expr as Identifier).value);
        return b?.copyable ?? true;
      }
      case "ConstructorExpression":
        return false; // ADT values are affine by default
      case "MoveExpression":
        return false;
      case "CallExpression":
        return false;
      default:
        return true;
    }
  }
}

export function formatCheckDiagnostic(d: CheckDiagnostic): string {
  return `${d.message}`;
}

/** Walk helper used by diagnostics to attach node positions. */
export function walkProgram(program: Program, visit: (node: Node) => void): void {
  visit(program);
  for (const stmt of program.statements) {
    walkStatement(stmt, visit);
  }
}

function walkStatement(stmt: Statement, visit: (node: Node) => void): void {
  visit(stmt);
  switch (stmt.type) {
    case "LetStatement": {
      const ls = stmt as LetStatement;
      visit(ls.name);
      if (ls.value) {
        walkExpression(ls.value, visit);
      }
      break;
    }
    case "ReturnStatement": {
      const rs = stmt as ReturnStatement;
      if (rs.returnValue) {
        walkExpression(rs.returnValue, visit);
      }
      break;
    }
    case "ExpressionStatement": {
      const es = stmt as ExpressionStatement;
      if (es.expression) {
        walkExpression(es.expression, visit);
      }
      break;
    }
    case "BlockStatement":
      for (const s of (stmt as BlockStatement).statements) {
        walkStatement(s, visit);
      }
      break;
    default:
      break;
  }
}

function walkExpression(expr: Expression, visit: (node: Node) => void): void {
  visit(expr);
  switch (expr.type) {
    case "PrefixExpression": {
      const p = expr as PrefixExpression;
      if (p.right) {
        walkExpression(p.right, visit);
      }
      break;
    }
    case "InfixExpression": {
      const i = expr as InfixExpression;
      walkExpression(i.left, visit);
      if (i.right) {
        walkExpression(i.right, visit);
      }
      break;
    }
    case "IfExpression": {
      const iff = expr as IfExpression;
      if (iff.condition) {
        walkExpression(iff.condition, visit);
      }
      walkStatement(iff.consequence, visit);
      if (iff.alternative) {
        walkStatement(iff.alternative, visit);
      }
      break;
    }
    case "CallExpression": {
      const c = expr as CallExpression;
      walkExpression(c.function, visit);
      for (const a of c.arguments) {
        walkExpression(a, visit);
      }
      break;
    }
    case "MatchExpression": {
      const m = expr as MatchExpression;
      if (m.scrutinee) {
        walkExpression(m.scrutinee, visit);
      }
      for (const arm of m.arms) {
        walkMatchArm(arm, visit);
      }
      break;
    }
    case "FunctionLiteral":
      walkStatement((expr as FunctionLiteral).body, visit);
      break;
    case "RefExpression": {
      const r = expr as RefExpression;
      if (r.value) {
        walkExpression(r.value, visit);
      }
      break;
    }
    case "MoveExpression": {
      const m = expr as MoveExpression;
      if (m.value) {
        walkExpression(m.value, visit);
      }
      break;
    }
    case "ConstructorExpression": {
      const c = expr as ConstructorExpression;
      for (const a of c.arguments) {
        walkExpression(a, visit);
      }
      break;
    }
    case "AsyncExpression": {
      const a = expr as AsyncExpression;
      if (a.body.type === "BlockStatement") {
        walkStatement(a.body as BlockStatement, visit);
      } else {
        walkExpression(a.body as Expression, visit);
      }
      break;
    }
    case "AwaitExpression": {
      const a = expr as AwaitExpression;
      if (a.argument) {
        walkExpression(a.argument, visit);
      }
      break;
    }
    case "SpawnExpression": {
      const s = expr as SpawnExpression;
      if (s.argument) {
        walkExpression(s.argument, visit);
      }
      break;
    }
    default:
      break;
  }
}

function walkMatchArm(arm: MatchArm, visit: (node: Node) => void): void {
  walkExpression(arm.body, visit);
}
