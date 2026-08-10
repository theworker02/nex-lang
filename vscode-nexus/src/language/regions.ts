import {
  BlockStatement,
  Expression,
  ExpressionStatement,
  LetStatement,
  Program,
  RegionExpression,
  Statement,
} from "./parser";

export interface RegionInfo {
  id: string;
  name: string | null;
  line: number;
  column: number;
  allocations: string[];
  parentId: string | null;
}

export interface RegionInferResult {
  program: Program;
  regions: RegionInfo[];
  errors: string[];
}

/**
 * Region-based memory management: infer lexical arenas and annotate
 * allocations so they are freed deterministically when the region exits.
 */
export class RegionInferencer {
  private regions: RegionInfo[] = [];
  private counter = 0;
  private stack: string[] = [];
  private errors: string[] = [];

  infer(program: Program): RegionInferResult {
    this.regions = [];
    this.counter = 0;
    this.stack = [];
    this.errors = [];

    // Implicit root region
    const root = this.pushRegion(null, 1, 1);
    for (const stmt of program.statements) {
      this.walkStatement(stmt);
    }
    this.popRegion();

    // Annotate: nothing structural required — regions list drives runtime.
    void root;

    return {
      program,
      regions: [...this.regions],
      errors: [...this.errors],
    };
  }

  private pushRegion(
    name: string | null,
    line: number,
    column: number,
  ): RegionInfo {
    const id = `r${this.counter++}`;
    const info: RegionInfo = {
      id,
      name,
      line,
      column,
      allocations: [],
      parentId: this.stack.length ? this.stack[this.stack.length - 1]! : null,
    };
    this.regions.push(info);
    this.stack.push(id);
    return info;
  }

  private popRegion(): void {
    this.stack.pop();
  }

  private current(): RegionInfo | undefined {
    const id = this.stack[this.stack.length - 1];
    return this.regions.find((r) => r.id === id);
  }

  private walkStatement(stmt: Statement): void {
    switch (stmt.type) {
      case "LetStatement": {
        const ls = stmt as LetStatement;
        if (ls.value) {
          this.walkExpression(ls.value);
          const cur = this.current();
          if (cur && this.isAllocating(ls.value)) {
            cur.allocations.push(ls.name.value);
          }
        }
        break;
      }
      case "ExpressionStatement": {
        const es = stmt as ExpressionStatement;
        if (es.expression) {
          this.walkExpression(es.expression);
        }
        break;
      }
      case "BlockStatement":
        for (const s of (stmt as BlockStatement).statements) {
          this.walkStatement(s);
        }
        break;
      case "ReturnStatement":
        break;
      default:
        break;
    }
  }

  private walkExpression(expr: Expression): void {
    if (expr.type === "RegionExpression") {
      const re = expr as RegionExpression;
      this.pushRegion(
        re.name?.value ?? null,
        re.token.line,
        re.token.column,
      );
      this.walkStatement(re.body);
      this.popRegion();
      return;
    }

    if (expr.type === "CallExpression") {
      // handled via isAllocating on parent let
    }

    // Recurse lightly into common nodes
    const anyExpr = expr as unknown as Record<string, unknown>;
    for (const key of ["right", "left", "condition", "argument", "value", "scrutinee"]) {
      const child = anyExpr[key];
      if (child && typeof child === "object" && "type" in (child as object)) {
        this.walkExpression(child as Expression);
      }
    }
    if (Array.isArray(anyExpr.arguments)) {
      for (const a of anyExpr.arguments as Expression[]) {
        this.walkExpression(a);
      }
    }
    if (anyExpr.body && typeof anyExpr.body === "object") {
      const body = anyExpr.body as { type?: string };
      if (body.type === "BlockStatement") {
        this.walkStatement(anyExpr.body as BlockStatement);
      } else if (body.type) {
        this.walkExpression(anyExpr.body as Expression);
      }
    }
  }

  private isAllocating(expr: Expression): boolean {
    if (expr.type === "ConstructorExpression") {
      return true;
    }
    if (expr.type === "CallExpression") {
      return true;
    }
    if (expr.type === "ChanExpression") {
      return true;
    }
    if (expr.type === "RefExpression") {
      return true;
    }
    return false;
  }
}

export function inferRegions(program: Program): RegionInferResult {
  return new RegionInferencer().infer(program);
}

/**
 * Runtime region arena — bump allocations freed as a group on exit.
 */
export class RegionArena {
  private readonly slots: Array<{ name: string; drop: () => void }> = [];
  private closed = false;

  alloc(name: string, drop: () => void): void {
    if (this.closed) {
      throw new Error("alloc in closed region");
    }
    this.slots.push({ name, drop });
  }

  exit(): void {
    if (this.closed) {
      return;
    }
    this.closed = true;
    for (let i = this.slots.length - 1; i >= 0; i--) {
      try {
        this.slots[i]!.drop();
      } catch {
        // best-effort deterministic teardown
      }
    }
    this.slots.length = 0;
  }
}

export class RegionStack {
  private readonly stack: RegionArena[] = [];

  enter(): RegionArena {
    const arena = new RegionArena();
    this.stack.push(arena);
    return arena;
  }

  exit(): void {
    const arena = this.stack.pop();
    arena?.exit();
  }

  current(): RegionArena | undefined {
    return this.stack[this.stack.length - 1];
  }
}
