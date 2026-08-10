/**
 * Convert between Nexus runtime objects and plain JS values.
 */

import {
  ArrayObj,
  BooleanObj,
  ErrorObj,
  FALSE_OBJ,
  HashObj,
  IntegerObj,
  NexusObject,
  NULL_OBJ,
  StringObj,
  TRUE_OBJ,
} from "../language/values";

export function fromJs(value: unknown): NexusObject {
  if (value === null || value === undefined) {
    return NULL_OBJ;
  }
  if (value instanceof Error) {
    return new ErrorObj(value.message);
  }
  if (typeof value === "boolean") {
    return value ? TRUE_OBJ : FALSE_OBJ;
  }
  if (typeof value === "number") {
    return new IntegerObj(Math.trunc(value));
  }
  if (typeof value === "string") {
    return new StringObj(value);
  }
  if (Array.isArray(value)) {
    return new ArrayObj(value.map((el) => fromJs(el)));
  }
  if (typeof value === "object") {
    const h = new HashObj();
    for (const [k, v] of Object.entries(value as Record<string, unknown>)) {
      h.setString(k, fromJs(v));
    }
    return h;
  }
  return new StringObj(String(value));
}

export function toJs(obj: NexusObject): unknown {
  if (obj.type === "NULL") {
    return null;
  }
  if (obj instanceof BooleanObj) {
    return obj.value;
  }
  if (obj instanceof IntegerObj) {
    return obj.value;
  }
  if (obj instanceof StringObj) {
    return obj.value;
  }
  if (obj instanceof ArrayObj) {
    return obj.elements.map((el) => toJs(el));
  }
  if (obj instanceof HashObj) {
    const out: Record<string, unknown> = {};
    for (const { key, value } of obj.pairs.values()) {
      const k =
        key instanceof StringObj
          ? key.value
          : key instanceof IntegerObj
            ? String(key.value)
            : key.inspect();
      out[k] = toJs(value);
    }
    return out;
  }
  if (obj instanceof ErrorObj) {
    return { error: obj.message };
  }
  return obj.inspect();
}

export function asString(obj: NexusObject | undefined): string | null {
  if (!obj) {
    return null;
  }
  if (obj instanceof StringObj) {
    return obj.value;
  }
  return null;
}

export function asInt(obj: NexusObject | undefined): number | null {
  if (!obj) {
    return null;
  }
  if (obj instanceof IntegerObj) {
    return obj.value;
  }
  if (obj instanceof StringObj) {
    const n = Number.parseInt(obj.value, 10);
    return Number.isNaN(n) ? null : n;
  }
  return null;
}

export function asBool(obj: NexusObject | undefined): boolean {
  if (!obj || obj.type === "NULL") {
    return false;
  }
  if (obj instanceof BooleanObj) {
    return obj.value;
  }
  if (obj instanceof IntegerObj) {
    return obj.value !== 0;
  }
  if (obj instanceof StringObj) {
    const s = obj.value.toLowerCase();
    return s === "true" || s === "1" || s === "yes";
  }
  return true;
}

export function hashGetString(hash: HashObj, key: string): string {
  const v = hash.getString(key);
  if (v instanceof StringObj) {
    return v.value;
  }
  if (v.type === "NULL") {
    return "";
  }
  return v.inspect();
}

export function expectArgs(
  name: string,
  n: number,
  args: NexusObject[],
): ErrorObj | null {
  if (args.length !== n) {
    return new ErrorObj(`${name}: wrong number of arguments. got=${args.length}, want=${n}`);
  }
  return null;
}

export function expectMinArgs(
  name: string,
  n: number,
  args: NexusObject[],
): ErrorObj | null {
  if (args.length < n) {
    return new ErrorObj(
      `${name}: wrong number of arguments. got=${args.length}, want>=${n}`,
    );
  }
  return null;
}
