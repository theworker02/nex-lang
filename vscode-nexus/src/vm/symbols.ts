export type SymbolScope =
  | "GLOBAL"
  | "LOCAL"
  | "BUILTIN"
  | "FREE"
  | "FUNCTION";

export interface Symbol {
  name: string;
  scope: SymbolScope;
  index: number;
}

export class SymbolTable {
  outer: SymbolTable | null;
  private readonly store = new Map<string, Symbol>();
  numDefinitions = 0;
  freeSymbols: Symbol[] = [];

  constructor(outer: SymbolTable | null = null) {
    this.outer = outer;
  }

  define(name: string): Symbol {
    const symbol: Symbol = {
      name,
      index: this.numDefinitions,
      scope: this.outer === null ? "GLOBAL" : "LOCAL",
    };
    this.store.set(name, symbol);
    this.numDefinitions++;
    return symbol;
  }

  defineBuiltin(index: number, name: string): Symbol {
    const symbol: Symbol = { name, index, scope: "BUILTIN" };
    this.store.set(name, symbol);
    return symbol;
  }

  defineFunctionName(name: string): Symbol {
    const symbol: Symbol = { name, index: 0, scope: "FUNCTION" };
    this.store.set(name, symbol);
    return symbol;
  }

  resolve(name: string): Symbol | undefined {
    const obj = this.store.get(name);
    if (obj) {
      return obj;
    }
    if (!this.outer) {
      return undefined;
    }
    const outer = this.outer.resolve(name);
    if (!outer) {
      return undefined;
    }
    if (outer.scope === "GLOBAL" || outer.scope === "BUILTIN") {
      return outer;
    }
    return this.defineFree(outer);
  }

  private defineFree(original: Symbol): Symbol {
    this.freeSymbols.push(original);
    const symbol: Symbol = {
      name: original.name,
      index: this.freeSymbols.length - 1,
      scope: "FREE",
    };
    this.store.set(original.name, symbol);
    return symbol;
  }
}
