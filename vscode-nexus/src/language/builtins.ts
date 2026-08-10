/**
 * Core Nexus builtins shared by the tree-walk evaluator and bytecode VM.
 * Index order in BUILTIN_NAMES is stable for OpGetBuiltin.
 */

import {
  ArrayObj,
  BooleanObj,
  FALSE_OBJ,
  HashObj,
  IntegerObj,
  NULL_OBJ,
  NexusObject,
  StringObj,
  TRUE_OBJ,
  isError,
  isTruthy,
  nativeBool,
  newError,
} from "./values";

export class BuiltinObj implements NexusObject {
  readonly type = "BUILTIN" as const;
  constructor(readonly fn: (...args: NexusObject[]) => NexusObject) {}
  inspect(): string {
    return "builtin function";
  }
}

export interface EnvLike {
  set(name: string, value: NexusObject): NexusObject;
  get(name: string): NexusObject | undefined;
}

export const BUILTIN_NAMES = [
  "len",
  "puts",
  "str",
  "int",
  "type",
  "typeof",
  "push",
  "first",
  "last",
  "rest",
  "keys",
  "has",
  "get",
  "split",
  "join",
  "trim",
  "lower",
  "upper",
  "contains",
  "starts_with",
  "replace",
  "slice",
  "ok",
  "err",
  "is_ok",
  "is_err",
  "unwrap",
  "map",
  "filter",
  "assert",
  "assert_eq",
  "getenv",
  "escape_html",
  "merge",
] as const;

export type ApplyUserFn = (
  fn: NexusObject,
  args: NexusObject[],
) => NexusObject;

export type ApplyUserFnAsync = (
  fn: NexusObject,
  args: NexusObject[],
) => NexusObject | Promise<NexusObject>;

export interface BuiltinHost {
  writeOutput?: (line: string) => void;
  applyFunction?: ApplyUserFn;
}

export function objectsEqual(a: NexusObject, b: NexusObject): boolean {
  if (a === b) {
    return true;
  }
  if (a.type !== b.type) {
    return false;
  }
  if (a instanceof IntegerObj && b instanceof IntegerObj) {
    return a.value === b.value;
  }
  if (a instanceof StringObj && b instanceof StringObj) {
    return a.value === b.value;
  }
  if (a instanceof BooleanObj && b instanceof BooleanObj) {
    return a.value === b.value;
  }
  if (a instanceof ArrayObj && b instanceof ArrayObj) {
    if (a.elements.length !== b.elements.length) {
      return false;
    }
    return a.elements.every((el, i) => objectsEqual(el, b.elements[i]!));
  }
  if (a instanceof HashObj && b instanceof HashObj) {
    if (a.pairs.size !== b.pairs.size) {
      return false;
    }
    for (const [k, pair] of a.pairs) {
      const other = b.pairs.get(k);
      if (!other || !objectsEqual(pair.value, other.value)) {
        return false;
      }
    }
    return true;
  }
  return a.inspect() === b.inspect();
}

function asResultOk(value: NexusObject): HashObj {
  const h = new HashObj();
  h.setString("ok", TRUE_OBJ);
  h.setString("value", value);
  h.setString("error", NULL_OBJ);
  return h;
}

function asResultErr(error: NexusObject): HashObj {
  const h = new HashObj();
  h.setString("ok", FALSE_OBJ);
  h.setString("value", NULL_OBJ);
  h.setString("error", error);
  return h;
}

function defaultApply(fn: NexusObject, args: NexusObject[]): NexusObject {
  if (fn instanceof BuiltinObj) {
    return fn.fn(...args);
  }
  return newError(`not a function: ${fn.type}`);
}

export function createBuiltins(host: BuiltinHost = {}): Map<string, BuiltinObj> {
  const write =
    host.writeOutput ??
    ((line: string) => {
      // eslint-disable-next-line no-console
      console.log(line);
    });
  const apply = host.applyFunction ?? defaultApply;
  const map = new Map<string, BuiltinObj>();

  const set = (name: string, fn: (...args: NexusObject[]) => NexusObject) => {
    map.set(name, new BuiltinObj(fn));
  };

  set("len", (...args) => {
    if (args.length !== 1) {
      return newError(`wrong number of arguments. got=${args.length}, want=1`);
    }
    const arg = args[0]!;
    if (arg instanceof StringObj) {
      return new IntegerObj(arg.value.length);
    }
    if (arg instanceof ArrayObj) {
      return new IntegerObj(arg.elements.length);
    }
    if (arg instanceof HashObj) {
      return new IntegerObj(arg.pairs.size);
    }
    return newError(`argument to \`len\` not supported, got ${arg.type}`);
  });

  set("puts", (...args) => {
    for (const arg of args) {
      write(arg.inspect());
    }
    return NULL_OBJ;
  });

  set("str", (...args) => {
    if (args.length !== 1) {
      return newError(`wrong number of arguments. got=${args.length}, want=1`);
    }
    return new StringObj(args[0]!.inspect());
  });

  set("int", (...args) => {
    if (args.length !== 1) {
      return newError(`wrong number of arguments. got=${args.length}, want=1`);
    }
    const arg = args[0]!;
    if (arg instanceof IntegerObj) {
      return arg;
    }
    if (arg instanceof StringObj) {
      const trimmed = arg.value.trim();
      if (trimmed === "") {
        return new IntegerObj(0);
      }
      const n = Number.parseInt(trimmed, 10);
      if (Number.isNaN(n)) {
        return newError(`cannot parse int: ${arg.value}`);
      }
      return new IntegerObj(n);
    }
    return newError(`int expects string or integer, got ${arg.type}`);
  });

  set("type", (...args) => {
    if (args.length !== 1) {
      return newError(`wrong number of arguments. got=${args.length}, want=1`);
    }
    return new StringObj(args[0]!.type);
  });

  // Alias: `type` is a keyword in source, so call sites use `typeof`.
  set("typeof", (...args) => {
    if (args.length !== 1) {
      return newError(`wrong number of arguments. got=${args.length}, want=1`);
    }
    return new StringObj(args[0]!.type);
  });

  set("push", (...args) => {
    if (args.length < 2) {
      return newError(`wrong number of arguments. got=${args.length}, want>=2`);
    }
    const arr = args[0];
    if (!(arr instanceof ArrayObj)) {
      return newError("push expects array as first argument");
    }
    return new ArrayObj([...arr.elements, ...args.slice(1)]);
  });

  set("first", (...args) => {
    if (args.length !== 1 || !(args[0] instanceof ArrayObj)) {
      return newError("first expects array");
    }
    return args[0].elements[0] ?? NULL_OBJ;
  });

  set("last", (...args) => {
    if (args.length !== 1 || !(args[0] instanceof ArrayObj)) {
      return newError("last expects array");
    }
    const el = args[0].elements;
    return el[el.length - 1] ?? NULL_OBJ;
  });

  set("rest", (...args) => {
    if (args.length !== 1 || !(args[0] instanceof ArrayObj)) {
      return newError("rest expects array");
    }
    return new ArrayObj(args[0].elements.slice(1));
  });

  set("keys", (...args) => {
    if (args.length !== 1 || !(args[0] instanceof HashObj)) {
      return newError("keys expects hash");
    }
    const keys: NexusObject[] = [];
    for (const { key } of args[0].pairs.values()) {
      keys.push(key);
    }
    return new ArrayObj(keys);
  });

  set("has", (...args) => {
    if (args.length !== 2 || !(args[0] instanceof HashObj)) {
      return newError("has expects (hash, key)");
    }
    const key = args[1]!;
    for (const { key: k } of args[0].pairs.values()) {
      if (objectsEqual(k, key)) {
        return TRUE_OBJ;
      }
    }
    return FALSE_OBJ;
  });

  set("get", (...args) => {
    if (args.length !== 2 || !(args[0] instanceof HashObj)) {
      return newError("get expects (hash, key)");
    }
    return args[0].get(args[1]!);
  });

  set("split", (...args) => {
    if (args.length !== 2) {
      return newError(`wrong number of arguments. got=${args.length}, want=2`);
    }
    if (!(args[0] instanceof StringObj) || !(args[1] instanceof StringObj)) {
      return newError("split expects (string, separator)");
    }
    return new ArrayObj(
      args[0].value.split(args[1].value).map((s) => new StringObj(s)),
    );
  });

  set("join", (...args) => {
    if (args.length !== 2) {
      return newError(`wrong number of arguments. got=${args.length}, want=2`);
    }
    if (!(args[0] instanceof ArrayObj) || !(args[1] instanceof StringObj)) {
      return newError("join expects (array, separator)");
    }
    return new StringObj(
      args[0].elements.map((e) => e.inspect()).join(args[1].value),
    );
  });

  set("trim", (...args) => {
    if (args.length !== 1 || !(args[0] instanceof StringObj)) {
      return newError("trim expects string");
    }
    return new StringObj(args[0].value.trim());
  });

  set("lower", (...args) => {
    if (args.length !== 1 || !(args[0] instanceof StringObj)) {
      return newError("lower expects string");
    }
    return new StringObj(args[0].value.toLowerCase());
  });

  set("upper", (...args) => {
    if (args.length !== 1 || !(args[0] instanceof StringObj)) {
      return newError("upper expects string");
    }
    return new StringObj(args[0].value.toUpperCase());
  });

  set("contains", (...args) => {
    if (args.length !== 2) {
      return newError(`wrong number of arguments. got=${args.length}, want=2`);
    }
    if (args[0] instanceof StringObj && args[1] instanceof StringObj) {
      return nativeBool(args[0].value.includes(args[1].value));
    }
    if (args[0] instanceof ArrayObj) {
      return nativeBool(
        args[0].elements.some((e) => objectsEqual(e, args[1]!)),
      );
    }
    return newError("contains expects (string, string) or (array, value)");
  });

  set("starts_with", (...args) => {
    if (
      args.length !== 2 ||
      !(args[0] instanceof StringObj) ||
      !(args[1] instanceof StringObj)
    ) {
      return newError("starts_with expects (string, prefix)");
    }
    return nativeBool(args[0].value.startsWith(args[1].value));
  });

  set("replace", (...args) => {
    if (
      args.length !== 3 ||
      !(args[0] instanceof StringObj) ||
      !(args[1] instanceof StringObj) ||
      !(args[2] instanceof StringObj)
    ) {
      return newError("replace expects (string, from, to)");
    }
    return new StringObj(
      args[0].value.split(args[1].value).join(args[2].value),
    );
  });

  set("slice", (...args) => {
    if (args.length < 2 || args.length > 3) {
      return newError(
        `wrong number of arguments. got=${args.length}, want=2 or 3`,
      );
    }
    const startObj = args[1];
    if (!(startObj instanceof IntegerObj)) {
      return newError("slice start must be integer");
    }
    const start = startObj.value;
    if (args[0] instanceof ArrayObj) {
      let end = args[0].elements.length;
      if (args.length === 3) {
        if (!(args[2] instanceof IntegerObj)) {
          return newError("slice end must be integer");
        }
        end = args[2].value;
      }
      return new ArrayObj(args[0].elements.slice(start, end));
    }
    if (args[0] instanceof StringObj) {
      let end = args[0].value.length;
      if (args.length === 3) {
        if (!(args[2] instanceof IntegerObj)) {
          return newError("slice end must be integer");
        }
        end = args[2].value;
      }
      return new StringObj(args[0].value.slice(start, end));
    }
    return newError("slice expects array or string");
  });

  set("ok", (...args) => {
    if (args.length !== 1) {
      return newError(`wrong number of arguments. got=${args.length}, want=1`);
    }
    return asResultOk(args[0]!);
  });

  set("err", (...args) => {
    if (args.length !== 1) {
      return newError(`wrong number of arguments. got=${args.length}, want=1`);
    }
    return asResultErr(args[0]!);
  });

  set("is_ok", (...args) => {
    if (args.length !== 1) {
      return newError(`wrong number of arguments. got=${args.length}, want=1`);
    }
    if (!(args[0] instanceof HashObj)) {
      return FALSE_OBJ;
    }
    const flag = args[0].getString("ok");
    return nativeBool(flag instanceof BooleanObj && flag.value);
  });

  set("is_err", (...args) => {
    if (args.length !== 1) {
      return newError(`wrong number of arguments. got=${args.length}, want=1`);
    }
    if (!(args[0] instanceof HashObj)) {
      return FALSE_OBJ;
    }
    const flag = args[0].getString("ok");
    return nativeBool(flag instanceof BooleanObj && !flag.value);
  });

  set("unwrap", (...args) => {
    if (args.length !== 1) {
      return newError(`wrong number of arguments. got=${args.length}, want=1`);
    }
    if (!(args[0] instanceof HashObj)) {
      return newError("unwrap expects Result hash");
    }
    const flag = args[0].getString("ok");
    if (!(flag instanceof BooleanObj)) {
      return newError("unwrap expects Result with ok field");
    }
    if (flag.value) {
      return args[0].getString("value");
    }
    return newError(`unwrap on Err: ${args[0].getString("error").inspect()}`);
  });

  set("assert", (...args) => {
    if (args.length < 1 || args.length > 2) {
      return newError(
        `wrong number of arguments. got=${args.length}, want=1 or 2`,
      );
    }
    let msg = "assertion failed";
    if (args.length === 2) {
      msg =
        args[1] instanceof StringObj ? args[1].value : args[1]!.inspect();
    }
    if (!isTruthy(args[0]!)) {
      return newError(msg);
    }
    return TRUE_OBJ;
  });

  set("assert_eq", (...args) => {
    if (args.length < 2 || args.length > 3) {
      return newError(
        `wrong number of arguments. got=${args.length}, want=2 or 3`,
      );
    }
    if (objectsEqual(args[0]!, args[1]!)) {
      return TRUE_OBJ;
    }
    let msg = `assert_eq failed: got ${args[0]!.inspect()}, want ${args[1]!.inspect()}`;
    if (args.length === 3) {
      const label =
        args[2] instanceof StringObj ? args[2].value : args[2]!.inspect();
      msg = `${label}: ${msg}`;
    }
    return newError(msg);
  });

  set("getenv", (...args) => {
    if (args.length !== 1 || !(args[0] instanceof StringObj)) {
      return newError("getenv expects string");
    }
    return new StringObj(process.env[args[0].value] ?? "");
  });

  set("escape_html", (...args) => {
    if (args.length !== 1 || !(args[0] instanceof StringObj)) {
      return newError("escape_html expects string");
    }
    const s = args[0].value
      .replace(/&/g, "&amp;")
      .replace(/</g, "&lt;")
      .replace(/>/g, "&gt;")
      .replace(/"/g, "&quot;")
      .replace(/'/g, "&#39;");
    return new StringObj(s);
  });

  set("merge", (...args) => {
    if (args.length < 1) {
      return newError("merge expects one or more hashes");
    }
    const out = new HashObj();
    for (const arg of args) {
      if (!(arg instanceof HashObj)) {
        return newError("merge expects hashes");
      }
      for (const { key, value } of arg.pairs.values()) {
        out.set(key, value);
      }
    }
    return out;
  });

  set("map", (...args) => {
    if (args.length !== 2 || !(args[0] instanceof ArrayObj)) {
      return newError("map expects (array, fn)");
    }
    const out: NexusObject[] = [];
    for (const el of args[0].elements) {
      const mapped = apply(args[1]!, [el]);
      if (isError(mapped)) {
        return mapped;
      }
      out.push(mapped);
    }
    return new ArrayObj(out);
  });

  set("filter", (...args) => {
    if (args.length !== 2 || !(args[0] instanceof ArrayObj)) {
      return newError("filter expects (array, fn)");
    }
    const out: NexusObject[] = [];
    for (const el of args[0].elements) {
      const keep = apply(args[1]!, [el]);
      if (isError(keep)) {
        return keep;
      }
      if (isTruthy(keep)) {
        out.push(el);
      }
    }
    return new ArrayObj(out);
  });

  return map;
}

export function getBuiltinByIndex(
  index: number,
  host?: BuiltinHost,
): BuiltinObj | undefined {
  const name = BUILTIN_NAMES[index];
  if (!name) {
    return undefined;
  }
  return createBuiltins(host).get(name);
}

export function installCoreBuiltins(
  env: EnvLike,
  host: BuiltinHost = {},
): Map<string, BuiltinObj> {
  const builtins = createBuiltins(host);
  for (const [name, builtin] of builtins) {
    env.set(name, builtin);
  }
  return builtins;
}
