import {
  AsyncExpression,
  AwaitExpression,
  BlockStatement,
  CallExpression,
  ChanExpression,
  ConstructorExpression,
  Expression,
  ExpressionStatement,
  FunctionLiteral,
  Identifier,
  IfExpression,
  InfixExpression,
  LetStatement,
  MacroDefinition,
  MacroFragment,
  MacroInvocation,
  MacroRule,
  MatchExpression,
  MoveExpression,
  PrefixExpression,
  Program,
  RefExpression,
  ReturnStatement,
  SpawnExpression,
  Statement,
} from "./parser";
import { Token } from "./lexer";

export interface MacroExpandResult {
  program: Program;
  errors: string[];
}

interface MacroEnv {
  macros: Map<string, MacroDefinition>;
  hygieneCounter: number;
}

/**
 * Hygienic macro expansion for `macro rules!` definitions.
 */
export class MacroExpander {
  expand(program: Program): MacroExpandResult {
    const errors: string[] = [];
    const env: MacroEnv = { macros: new Map(), hygieneCounter: 0 };
    const withoutDefs = this.collectAndStrip(program.statements, env, errors);
    const expandedStmts: Statement[] = [];

    for (const stmt of withoutDefs) {
      const result = this.expandStatement(stmt, env, errors);
      if (Array.isArray(result)) {
        expandedStmts.push(...result);
      } else if (result) {
        expandedStmts.push(result);
      }
    }

    return {
      program: {
        type: "Program",
        statements: expandedStmts,
        tokenLiteral: () =>
          expandedStmts.length > 0 ? expandedStmts[0]!.tokenLiteral() : "",
      },
      errors,
    };
  }

  private collectAndStrip(
    statements: Statement[],
    env: MacroEnv,
    errors: string[],
  ): Statement[] {
    const out: Statement[] = [];
    for (const stmt of statements) {
      if (stmt.type === "MacroDefinition") {
        const def = stmt as MacroDefinition;
        if (env.macros.has(def.name.value)) {
          errors.push(
            `duplicate macro definition \`${def.name.value}\` at line ${def.token.line}, column ${def.token.column}`,
          );
        }
        env.macros.set(def.name.value, def);
        continue;
      }
      out.push(stmt);
    }
    return out;
  }

  private expandStatement(
    stmt: Statement,
    env: MacroEnv,
    errors: string[],
  ): Statement | Statement[] | null {
    switch (stmt.type) {
      case "LetStatement": {
        const ls = stmt as LetStatement;
        const next: LetStatement = {
          ...ls,
          value: ls.value ? this.expandExpression(ls.value, env, errors) : null,
        };
        return next;
      }
      case "ReturnStatement": {
        const rs = stmt as ReturnStatement;
        const next: ReturnStatement = {
          ...rs,
          returnValue: rs.returnValue
            ? this.expandExpression(rs.returnValue, env, errors)
            : null,
        };
        return next;
      }
      case "ExpressionStatement": {
        const es = stmt as ExpressionStatement;
        if (!es.expression) {
          return es;
        }
        const expanded = this.expandExpression(es.expression, env, errors);
        if (expanded.type === "BlockStatement") {
          return (expanded as unknown as BlockStatement).statements;
        }
        const next: ExpressionStatement = { ...es, expression: expanded };
        return next;
      }
      case "BlockStatement": {
        const block = stmt as BlockStatement;
        const stmts: Statement[] = [];
        for (const s of block.statements) {
          const r = this.expandStatement(s, env, errors);
          if (Array.isArray(r)) {
            stmts.push(...r);
          } else if (r) {
            stmts.push(r);
          }
        }
        const next: BlockStatement = { ...block, statements: stmts };
        return next;
      }
      default:
        return stmt;
    }
  }

  private expandExpression(
    expr: Expression,
    env: MacroEnv,
    errors: string[],
  ): Expression {
    switch (expr.type) {
      case "MacroInvocation":
        return this.expandInvocation(expr as MacroInvocation, env, errors);
      case "CallExpression": {
        const c = expr as CallExpression;
        const next: CallExpression = {
          ...c,
          function: this.expandExpression(c.function, env, errors),
          arguments: c.arguments.map((a) =>
            this.expandExpression(a, env, errors),
          ),
        };
        return next;
      }
      case "PrefixExpression": {
        const p = expr as PrefixExpression;
        const next: PrefixExpression = {
          ...p,
          right: p.right ? this.expandExpression(p.right, env, errors) : null,
        };
        return next;
      }
      case "InfixExpression": {
        const i = expr as InfixExpression;
        const next: InfixExpression = {
          ...i,
          left: this.expandExpression(i.left, env, errors),
          right: i.right ? this.expandExpression(i.right, env, errors) : null,
        };
        return next;
      }
      case "IfExpression": {
        const iff = expr as IfExpression;
        const next: IfExpression = {
          ...iff,
          condition: iff.condition
            ? this.expandExpression(iff.condition, env, errors)
            : null,
          consequence: this.expandStatement(
            iff.consequence,
            env,
            errors,
          ) as BlockStatement,
          alternative: iff.alternative
            ? (this.expandStatement(
                iff.alternative,
                env,
                errors,
              ) as BlockStatement)
            : null,
        };
        return next;
      }
      case "FunctionLiteral": {
        const fn = expr as FunctionLiteral;
        const next: FunctionLiteral = {
          ...fn,
          body: this.expandStatement(fn.body, env, errors) as BlockStatement,
        };
        return next;
      }
      case "MatchExpression": {
        const m = expr as MatchExpression;
        const next: MatchExpression = {
          ...m,
          scrutinee: m.scrutinee
            ? this.expandExpression(m.scrutinee, env, errors)
            : null,
          arms: m.arms.map((arm) => ({
            ...arm,
            body: this.expandExpression(arm.body, env, errors),
          })),
        };
        return next;
      }
      case "AsyncExpression": {
        const a = expr as AsyncExpression;
        if (a.body.type === "BlockStatement") {
          const next: AsyncExpression = {
            ...a,
            body: this.expandStatement(
              a.body as BlockStatement,
              env,
              errors,
            ) as BlockStatement,
          };
          return next;
        }
        const next: AsyncExpression = {
          ...a,
          body: this.expandExpression(a.body as Expression, env, errors),
        };
        return next;
      }
      case "AwaitExpression": {
        const a = expr as AwaitExpression;
        const next: AwaitExpression = {
          ...a,
          argument: a.argument
            ? this.expandExpression(a.argument, env, errors)
            : null,
        };
        return next;
      }
      case "SpawnExpression": {
        const s = expr as SpawnExpression;
        const next: SpawnExpression = {
          ...s,
          argument: s.argument
            ? this.expandExpression(s.argument, env, errors)
            : null,
        };
        return next;
      }
      case "ChanExpression": {
        const c = expr as ChanExpression;
        const next: ChanExpression = {
          ...c,
          capacity: c.capacity
            ? this.expandExpression(c.capacity, env, errors)
            : null,
        };
        return next;
      }
      case "RefExpression": {
        const r = expr as RefExpression;
        const next: RefExpression = {
          ...r,
          value: r.value ? this.expandExpression(r.value, env, errors) : null,
        };
        return next;
      }
      case "MoveExpression": {
        const m = expr as MoveExpression;
        const next: MoveExpression = {
          ...m,
          value: m.value ? this.expandExpression(m.value, env, errors) : null,
        };
        return next;
      }
      case "ConstructorExpression": {
        const c = expr as ConstructorExpression;
        const next: ConstructorExpression = {
          ...c,
          arguments: c.arguments.map((a) =>
            this.expandExpression(a, env, errors),
          ),
        };
        return next;
      }
      default:
        return expr;
    }
  }

  private expandInvocation(
    inv: MacroInvocation,
    env: MacroEnv,
    errors: string[],
  ): Expression {
    const def = env.macros.get(inv.name.value);
    if (!def) {
      errors.push(
        `unknown macro \`${inv.name.value}\` at line ${inv.token.line}, column ${inv.token.column}`,
      );
      return inv;
    }

    for (const rule of def.rules) {
      const captures = this.matchRule(rule, inv.arguments);
      if (captures) {
        const hygieneId = ++env.hygieneCounter;
        const substituted = this.substitute(
          rule.body,
          captures,
          hygieneId,
          new Set(this.introducedBindings(rule.body)),
        );
        return this.expandExpression(substituted, env, errors);
      }
    }

    errors.push(
      `no matching macro rule for \`${inv.name.value}!\` at line ${inv.token.line}, column ${inv.token.column}`,
    );
    return inv;
  }

  private matchRule(
    rule: MacroRule,
    args: Expression[],
  ): Map<string, Expression> | null {
    const captures = new Map<string, Expression>();
    const metaFragments = rule.pattern.filter(
      (f) => f.kind === "expr" || f.kind === "ident" || f.kind === "tt",
    );

    if (metaFragments.length === 0) {
      return args.length === 0 ? captures : null;
    }
    if (metaFragments.length !== args.length) {
      return null;
    }

    for (let i = 0; i < metaFragments.length; i++) {
      const frag = metaFragments[i]!;
      const arg = args[i]!;
      if (!frag.name) {
        return null;
      }
      if (frag.kind === "ident" && arg.type !== "Identifier") {
        return null;
      }
      captures.set(frag.name, arg);
    }
    return captures;
  }

  private introducedBindings(expr: Expression): string[] {
    const names: string[] = [];
    const visitStmt = (s: Statement): void => {
      if (s.type === "LetStatement") {
        names.push((s as LetStatement).name.value);
      } else if (s.type === "BlockStatement") {
        for (const st of (s as BlockStatement).statements) {
          visitStmt(st);
        }
      } else if (s.type === "ExpressionStatement") {
        const e = (s as ExpressionStatement).expression;
        if (e) {
          visitExpr(e);
        }
      }
    };
    const visitExpr = (e: Expression): void => {
      if (e.type === "IfExpression") {
        const iff = e as IfExpression;
        visitStmt(iff.consequence);
        if (iff.alternative) {
          visitStmt(iff.alternative);
        }
      } else if (e.type === "FunctionLiteral") {
        visitStmt((e as FunctionLiteral).body);
      } else if (e.type === "AsyncExpression") {
        const a = e as AsyncExpression;
        if (a.body.type === "BlockStatement") {
          visitStmt(a.body as BlockStatement);
        }
      }
    };
    visitExpr(expr);
    return names;
  }

  private substitute(
    expr: Expression,
    captures: Map<string, Expression>,
    hygieneId: number,
    introduced: Set<string>,
  ): Expression {
    if (expr.type === "Identifier") {
      const id = expr as Identifier;
      const captured =
        captures.get(id.value) ??
        (id.value.startsWith("$")
          ? captures.get(id.value.slice(1))
          : undefined) ??
        captures.get(`$${id.value}`);
      if (captured) {
        return captured;
      }
      if (introduced.has(id.value)) {
        return this.gensymIdent(id, hygieneId);
      }
      return expr;
    }

    switch (expr.type) {
      case "PrefixExpression": {
        const p = expr as PrefixExpression;
        const next: PrefixExpression = {
          ...p,
          right: p.right
            ? this.substitute(p.right, captures, hygieneId, introduced)
            : null,
        };
        return next;
      }
      case "InfixExpression": {
        const i = expr as InfixExpression;
        const next: InfixExpression = {
          ...i,
          left: this.substitute(i.left, captures, hygieneId, introduced),
          right: i.right
            ? this.substitute(i.right, captures, hygieneId, introduced)
            : null,
        };
        return next;
      }
      case "IfExpression": {
        const iff = expr as IfExpression;
        const next: IfExpression = {
          ...iff,
          condition: iff.condition
            ? this.substitute(iff.condition, captures, hygieneId, introduced)
            : null,
          consequence: this.substituteBlock(
            iff.consequence,
            captures,
            hygieneId,
            introduced,
          ),
          alternative: iff.alternative
            ? this.substituteBlock(
                iff.alternative,
                captures,
                hygieneId,
                introduced,
              )
            : null,
        };
        return next;
      }
      case "CallExpression": {
        const c = expr as CallExpression;
        const next: CallExpression = {
          ...c,
          function: this.substitute(c.function, captures, hygieneId, introduced),
          arguments: c.arguments.map((a) =>
            this.substitute(a, captures, hygieneId, introduced),
          ),
        };
        return next;
      }
      case "FunctionLiteral": {
        const fn = expr as FunctionLiteral;
        const next: FunctionLiteral = {
          ...fn,
          body: this.substituteBlock(fn.body, captures, hygieneId, introduced),
        };
        return next;
      }
      case "MatchExpression": {
        const m = expr as MatchExpression;
        const next: MatchExpression = {
          ...m,
          scrutinee: m.scrutinee
            ? this.substitute(m.scrutinee, captures, hygieneId, introduced)
            : null,
          arms: m.arms.map((arm) => ({
            ...arm,
            body: this.substitute(arm.body, captures, hygieneId, introduced),
          })),
        };
        return next;
      }
      case "ConstructorExpression": {
        const c = expr as ConstructorExpression;
        const next: ConstructorExpression = {
          ...c,
          arguments: c.arguments.map((a) =>
            this.substitute(a, captures, hygieneId, introduced),
          ),
        };
        return next;
      }
      case "BlockStatement":
        return this.substituteBlock(
          expr as unknown as BlockStatement,
          captures,
          hygieneId,
          introduced,
        ) as unknown as Expression;
      default:
        return expr;
    }
  }

  private substituteBlock(
    block: BlockStatement,
    captures: Map<string, Expression>,
    hygieneId: number,
    introduced: Set<string>,
  ): BlockStatement {
    const statements = block.statements.map((stmt) => {
      if (stmt.type === "LetStatement") {
        const ls = stmt as LetStatement;
        const name = introduced.has(ls.name.value)
          ? this.gensymIdent(ls.name, hygieneId)
          : ls.name;
        const next: LetStatement = {
          ...ls,
          name,
          value: ls.value
            ? this.substitute(ls.value, captures, hygieneId, introduced)
            : null,
        };
        return next;
      }
      if (stmt.type === "ReturnStatement") {
        const rs = stmt as ReturnStatement;
        const next: ReturnStatement = {
          ...rs,
          returnValue: rs.returnValue
            ? this.substitute(rs.returnValue, captures, hygieneId, introduced)
            : null,
        };
        return next;
      }
      if (stmt.type === "ExpressionStatement") {
        const es = stmt as ExpressionStatement;
        const next: ExpressionStatement = {
          ...es,
          expression: es.expression
            ? this.substitute(es.expression, captures, hygieneId, introduced)
            : null,
        };
        return next;
      }
      if (stmt.type === "BlockStatement") {
        return this.substituteBlock(
          stmt as BlockStatement,
          captures,
          hygieneId,
          introduced,
        );
      }
      return stmt;
    });
    return { ...block, statements };
  }

  private gensymIdent(id: Identifier, hygieneId: number): Identifier {
    const token: Token = {
      ...id.token,
      literal: `__nx${hygieneId}_${id.value}`,
    };
    return {
      ...id,
      token,
      value: `__nx${hygieneId}_${id.value}`,
      tokenLiteral: () => token.literal,
    };
  }
}

export type { MacroFragment };
