import {
  BlockStatement,
  EffectDeclaration,
  Expression,
  ExpressionStatement,
  HandleExpression,
  PerformExpression,
  Program,
  Statement,
} from "./parser";

export interface EffectDiagnostic {
  message: string;
  line: number;
  column: number;
}

export interface EffectOpInfo {
  effect: string;
  name: string;
  arity: number;
}

/**
 * Algebraic effect handlers — perform / handle with delimited continuations.
 */
export class EffectRegistry {
  private readonly effects = new Map<string, Map<string, EffectOpInfo>>();

  register(decl: EffectDeclaration): void {
    const ops = new Map<string, EffectOpInfo>();
    for (const op of decl.operations) {
      ops.set(op.name.value, {
        effect: decl.name.value,
        name: op.name.value,
        arity: op.parameters.length,
      });
    }
    this.effects.set(decl.name.value, ops);
  }

  lookup(effect: string | null, op: string): EffectOpInfo | undefined {
    if (effect) {
      return this.effects.get(effect)?.get(op);
    }
    for (const ops of this.effects.values()) {
      const found = ops.get(op);
      if (found) {
        return found;
      }
    }
    return undefined;
  }

  has(effect: string): boolean {
    return this.effects.has(effect);
  }
}

export function checkEffects(program: Program): EffectDiagnostic[] {
  const diags: EffectDiagnostic[] = [];
  const registry = new EffectRegistry();

  for (const stmt of program.statements) {
    if (stmt.type === "EffectDeclaration") {
      registry.register(stmt as EffectDeclaration);
    }
  }

  const walkStmt = (stmt: Statement): void => {
    switch (stmt.type) {
      case "ExpressionStatement": {
        const es = stmt as ExpressionStatement;
        if (es.expression) {
          walkExpr(es.expression);
        }
        break;
      }
      case "BlockStatement":
        for (const s of (stmt as BlockStatement).statements) {
          walkStmt(s);
        }
        break;
      case "LetStatement":
      case "ReturnStatement":
        break;
      default:
        break;
    }
  };

  const walkExpr = (expr: Expression): void => {
    if (expr.type === "PerformExpression") {
      const p = expr as PerformExpression;
      const info = registry.lookup(p.effectName, p.operation.value);
      if (!info) {
        diags.push({
          message: `unknown effect operation \`${p.effectName ? p.effectName + "::" : ""}${p.operation.value}\` at line ${p.token.line}, column ${p.token.column}`,
          line: p.token.line,
          column: p.token.column,
        });
      } else if (info.arity !== p.arguments.length) {
        diags.push({
          message: `effect \`${info.name}\` expects ${info.arity} arg(s), got ${p.arguments.length} at line ${p.token.line}, column ${p.token.column}`,
          line: p.token.line,
          column: p.token.column,
        });
      }
    } else if (expr.type === "HandleExpression") {
      const h = expr as HandleExpression;
      if (!registry.has(h.effectName.value)) {
        diags.push({
          message: `unknown effect \`${h.effectName.value}\` in handle at line ${h.token.line}, column ${h.token.column}`,
          line: h.token.line,
          column: h.token.column,
        });
      }
      walkStmt(h.body);
      for (const arm of h.handlers) {
        walkExpr(arm.body);
      }
    }
  };

  for (const stmt of program.statements) {
    walkStmt(stmt);
  }

  return diags;
}

export type Continuations<T> = {
  resume: (value: T) => T;
  abort: (value: T) => T;
};

export interface EffectFrame {
  effectName: string;
  handlers: Map<
    string,
    {
      parameters: string[];
      invoke: (args: unknown[], cont: Continuations<unknown>) => unknown;
    }
  >;
}

/**
 * Runtime delimited-continuation stack for algebraic effects.
 */
export class EffectRuntime {
  private readonly stack: EffectFrame[] = [];

  push(frame: EffectFrame): void {
    this.stack.push(frame);
  }

  pop(): void {
    this.stack.pop();
  }

  perform(effectName: string | null, op: string, args: unknown[]): unknown {
    for (let i = this.stack.length - 1; i >= 0; i--) {
      const frame = this.stack[i]!;
      if (effectName && frame.effectName !== effectName) {
        continue;
      }
      const handler = frame.handlers.get(op);
      if (!handler) {
        continue;
      }
      let resumed: unknown = undefined;
      let didResume = false;
      const cont: Continuations<unknown> = {
        resume: (value) => {
          didResume = true;
          resumed = value;
          return value;
        },
        abort: (value) => {
          didResume = true;
          resumed = value;
          return value;
        },
      };
      const result = handler.invoke(args, cont);
      return didResume ? resumed : result;
    }
    throw new Error(
      `uncaught effect ${effectName ? effectName + "::" : ""}${op}`,
    );
  }
}
