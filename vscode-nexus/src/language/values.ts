/**
 * Shared runtime value types for the Nexus interpreter.
 */

export type ObjectType =
  | "INTEGER"
  | "STRING"
  | "BOOLEAN"
  | "NULL"
  | "RETURN_VALUE"
  | "BREAK"
  | "CONTINUE"
  | "ERROR"
  | "FUNCTION"
  | "BUILTIN"
  | "CHANNEL"
  | "PROMISE"
  | "REF"
  | "ENUM_VALUE"
  | "EXTERN_FN"
  | "ASYNC_FN"
  | "ARRAY"
  | "HASH"
  | "COMPILED_FUNCTION"
  | "CLOSURE";

export interface NexusObject {
  type: ObjectType | string;
  inspect(): string;
}

export class IntegerObj implements NexusObject {
  readonly type = "INTEGER" as const;
  constructor(readonly value: number) {}
  inspect(): string {
    return String(this.value);
  }
}

export class StringObj implements NexusObject {
  readonly type = "STRING" as const;
  constructor(readonly value: string) {}
  inspect(): string {
    return this.value;
  }
}

export class BooleanObj implements NexusObject {
  readonly type = "BOOLEAN" as const;
  constructor(readonly value: boolean) {}
  inspect(): string {
    return String(this.value);
  }
}

export class NullObj implements NexusObject {
  readonly type = "NULL" as const;
  inspect(): string {
    return "null";
  }
}

export class ReturnValue implements NexusObject {
  readonly type = "RETURN_VALUE" as const;
  constructor(readonly value: NexusObject) {}
  inspect(): string {
    return this.value.inspect();
  }
}

export class BreakSignal implements NexusObject {
  readonly type = "BREAK" as const;
  inspect(): string {
    return "break";
  }
}

export class ContinueSignal implements NexusObject {
  readonly type = "CONTINUE" as const;
  inspect(): string {
    return "continue";
  }
}

export const BREAK_SIGNAL = new BreakSignal();
export const CONTINUE_SIGNAL = new ContinueSignal();

export class ErrorObj implements NexusObject {
  readonly type = "ERROR" as const;
  constructor(readonly message: string) {}
  inspect(): string {
    return `ERROR: ${this.message}`;
  }
}

export class ArrayObj implements NexusObject {
  readonly type = "ARRAY" as const;
  constructor(readonly elements: NexusObject[]) {}
  inspect(): string {
    return `[${this.elements.map((e) => e.inspect()).join(", ")}]`;
  }
}

export interface HashPair {
  key: NexusObject;
  value: NexusObject;
}

export class HashObj implements NexusObject {
  readonly type = "HASH" as const;
  readonly pairs = new Map<string, HashPair>();

  set(key: NexusObject, value: NexusObject): void {
    this.pairs.set(hashKey(key), { key, value });
  }

  get(key: NexusObject): NexusObject {
    const pair = this.pairs.get(hashKey(key));
    return pair ? pair.value : NULL_OBJ;
  }

  getString(field: string): NexusObject {
    return this.get(new StringObj(field));
  }

  setString(field: string, value: NexusObject): void {
    this.set(new StringObj(field), value);
  }

  inspect(): string {
    const parts: string[] = [];
    for (const { key, value } of this.pairs.values()) {
      parts.push(`${key.inspect()}: ${value.inspect()}`);
    }
    return `{${parts.join(", ")}}`;
  }
}

export class CompiledFunctionObj implements NexusObject {
  readonly type = "COMPILED_FUNCTION" as const;
  constructor(
    readonly instructions: Uint8Array,
    readonly numLocals: number,
    readonly numParameters: number,
  ) {}
  inspect(): string {
    return "compiled function";
  }
}

export class ClosureObj implements NexusObject {
  readonly type = "CLOSURE" as const;
  constructor(
    readonly fn: CompiledFunctionObj,
    readonly free: NexusObject[] = [],
  ) {}
  inspect(): string {
    return "closure";
  }
}

export const NULL_OBJ = new NullObj();
export const TRUE_OBJ = new BooleanObj(true);
export const FALSE_OBJ = new BooleanObj(false);

export function hashKey(key: NexusObject): string {
  if (key instanceof IntegerObj) {
    return `i:${key.value}`;
  }
  if (key instanceof StringObj) {
    return `s:${key.value}`;
  }
  if (key instanceof BooleanObj) {
    return `b:${key.value}`;
  }
  return `o:${key.type}:${key.inspect()}`;
}

export function nativeBool(value: boolean): BooleanObj {
  return value ? TRUE_OBJ : FALSE_OBJ;
}

export function newError(message: string): ErrorObj {
  return new ErrorObj(message);
}

export function isError(obj: NexusObject): obj is ErrorObj {
  return obj instanceof ErrorObj || obj.type === "ERROR";
}

export function isTruthy(obj: NexusObject): boolean {
  if (obj === NULL_OBJ || obj instanceof NullObj) {
    return false;
  }
  if (obj === FALSE_OBJ) {
    return false;
  }
  if (obj instanceof BooleanObj && !obj.value) {
    return false;
  }
  return true;
}
