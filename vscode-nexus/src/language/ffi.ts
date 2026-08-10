import * as fs from "fs";
import * as path from "path";
import {
  ErrorObj,
  IntegerObj,
  NexusObject,
  NullObj,
  StringObj,
  NULL_OBJ,
  newError,
} from "./values";
import { ExternFnObj } from "./runtime";

export type FfiPrimitiveType =
  | "void"
  | "int"
  | "int32"
  | "uint32"
  | "int64"
  | "uint64"
  | "float"
  | "double"
  | "string"
  | "pointer"
  | "bool";

export interface FfiSymbolSpec {
  name: string;
  returnType: FfiPrimitiveType;
  argTypes: FfiPrimitiveType[];
}

export interface LoadedLibrary {
  readonly path: string;
  readonly available: boolean;
  readonly backend: "koffi" | "stub";
  define(spec: FfiSymbolSpec): ExternFnObj;
}

type KoffiLib = {
  func: (sig: string) => (...args: unknown[]) => unknown;
};

type KoffiModule = {
  load: (libPath: string) => KoffiLib;
};

let koffiModule: KoffiModule | null | undefined;

function tryLoadKoffi(): KoffiModule | null {
  if (koffiModule !== undefined) {
    return koffiModule;
  }
  try {
    // Dynamic require so the extension can activate without native bindings.
    // eslint-disable-next-line @typescript-eslint/no-require-imports
    koffiModule = require("koffi") as KoffiModule;
    return koffiModule;
  } catch {
    koffiModule = null;
    return null;
  }
}

function mapType(t: FfiPrimitiveType): string {
  switch (t) {
    case "void":
      return "void";
    case "int":
    case "int32":
      return "int";
    case "uint32":
      return "uint32";
    case "int64":
      return "int64";
    case "uint64":
      return "uint64";
    case "float":
      return "float";
    case "double":
      return "double";
    case "string":
      return "str";
    case "pointer":
      return "void *";
    case "bool":
      return "bool";
    default:
      return "void";
  }
}

function nexusToNative(obj: NexusObject, typ: FfiPrimitiveType): unknown {
  switch (typ) {
    case "string":
      if (obj instanceof StringObj) {
        return obj.value;
      }
      return obj.inspect();
    case "int":
    case "int32":
    case "uint32":
    case "int64":
    case "uint64":
      if (obj instanceof IntegerObj) {
        return obj.value;
      }
      throw new Error(`expected integer for FFI arg type ${typ}`);
    case "bool":
      if (obj.type === "BOOLEAN" && "value" in obj) {
        return Boolean((obj as unknown as { value: boolean }).value);
      }
      return Boolean(obj);
    case "float":
    case "double":
      if (obj instanceof IntegerObj) {
        return obj.value;
      }
      throw new Error(`expected number for FFI arg type ${typ}`);
    case "pointer":
      return null;
    case "void":
      return undefined;
    default:
      return obj.inspect();
  }
}

function nativeToNexus(value: unknown, typ: FfiPrimitiveType): NexusObject {
  switch (typ) {
    case "void":
      return NULL_OBJ;
    case "int":
    case "int32":
    case "uint32":
    case "int64":
    case "uint64":
      return new IntegerObj(Number(value));
    case "float":
    case "double":
      return new IntegerObj(Math.trunc(Number(value)));
    case "string":
      return new StringObj(String(value ?? ""));
    case "bool":
      return value
        ? ({ type: "BOOLEAN", value: true, inspect: () => "true" } as NexusObject)
        : ({ type: "BOOLEAN", value: false, inspect: () => "false" } as NexusObject);
    case "pointer":
      return value == null
        ? NULL_OBJ
        : new StringObj(String(value));
    default:
      return new StringObj(String(value));
  }
}

class KoffiLibrary implements LoadedLibrary {
  readonly backend = "koffi" as const;
  readonly available = true;

  constructor(
    readonly path: string,
    private readonly lib: KoffiLib,
  ) {}

  define(spec: FfiSymbolSpec): ExternFnObj {
    const ret = mapType(spec.returnType);
    const args = spec.argTypes.map(mapType).join(", ");
    const sig = `${ret} ${spec.name}(${args})`;
    let nativeFn: (...args: unknown[]) => unknown;
    try {
      nativeFn = this.lib.func(sig);
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      return new ExternFnObj(this.path, spec.name, spec.returnType, () =>
        newError(`FFI symbol bind failed for ${spec.name}: ${message}`),
      );
    }

    return new ExternFnObj(
      this.path,
      spec.name,
      spec.returnType,
      (...callArgs: NexusObject[]) => {
        if (callArgs.length !== spec.argTypes.length) {
          return newError(
            `FFI ${spec.name}: wrong number of arguments got=${callArgs.length} want=${spec.argTypes.length}`,
          );
        }
        try {
          const nativeArgs = callArgs.map((a, i) =>
            nexusToNative(a, spec.argTypes[i]!),
          );
          const result = nativeFn(...nativeArgs);
          return nativeToNexus(result, spec.returnType);
        } catch (err) {
          const message = err instanceof Error ? err.message : String(err);
          return newError(`FFI call ${spec.name} failed: ${message}`);
        }
      },
    );
  }
}

class StubLibrary implements LoadedLibrary {
  readonly backend = "stub" as const;
  readonly available = false;

  constructor(
    readonly path: string,
    private readonly reason: string,
  ) {}

  define(spec: FfiSymbolSpec): ExternFnObj {
    return new ExternFnObj(
      this.path,
      spec.name,
      spec.returnType,
      () =>
        newError(
          `FFI unavailable (${this.reason}); cannot call ${spec.name} from ${this.path}`,
        ),
    );
  }
}

/**
 * Resolve a library path for the current platform, trying common variants.
 */
export function resolveLibraryPath(libPath: string): string {
  if (path.isAbsolute(libPath) && fs.existsSync(libPath)) {
    return libPath;
  }
  if (fs.existsSync(libPath)) {
    return path.resolve(libPath);
  }

  const candidates: string[] = [libPath];
  if (process.platform === "win32") {
    if (!libPath.endsWith(".dll")) {
      candidates.push(`${libPath}.dll`);
    }
  } else if (process.platform === "darwin") {
    if (!libPath.includes(".dylib") && !libPath.includes(".so")) {
      candidates.push(`lib${libPath}.dylib`, `${libPath}.dylib`);
    }
  } else {
    if (!libPath.includes(".so")) {
      candidates.push(`lib${libPath}.so`, `${libPath}.so`);
    }
  }

  for (const c of candidates) {
    if (fs.existsSync(c)) {
      return path.resolve(c);
    }
  }

  return libPath;
}

/**
 * Load a native dynamic library for FFI. Falls back to a safe stub backend
 * when koffi is unavailable or the library cannot be opened — the API remains
 * fully usable and returns clear ErrorObj values on call.
 */
export function loadLibrary(libPath: string): LoadedLibrary {
  const resolved = resolveLibraryPath(libPath);
  const koffi = tryLoadKoffi();
  if (!koffi) {
    return new StubLibrary(
      resolved,
      "koffi native module not loaded in this host",
    );
  }

  try {
    const lib = koffi.load(resolved);
    return new KoffiLibrary(resolved, lib);
  } catch (err) {
    const message = err instanceof Error ? err.message : String(err);
    return new StubLibrary(resolved, message);
  }
}

/** Parse a type name from Nexus extern declarations. */
export function parseFfiType(name: string): FfiPrimitiveType {
  const n = name.toLowerCase();
  switch (n) {
    case "void":
    case "nil":
    case "null":
      return "void";
    case "int":
    case "i32":
    case "int32":
      return "int";
    case "u32":
    case "uint32":
      return "uint32";
    case "i64":
    case "int64":
      return "int64";
    case "u64":
    case "uint64":
      return "uint64";
    case "float":
    case "f32":
      return "float";
    case "double":
    case "f64":
      return "double";
    case "string":
    case "str":
      return "string";
    case "bool":
    case "boolean":
      return "bool";
    case "ptr":
    case "pointer":
      return "pointer";
    default:
      return "int";
  }
}

export function ffiLoadBuiltin(libPath: NexusObject): NexusObject {
  if (!(libPath instanceof StringObj)) {
    return newError("ffi_load: library path must be a string");
  }
  const loaded = loadLibrary(libPath.value);
  return new FfiLibraryObj(loaded);
}

export class FfiLibraryObj implements NexusObject {
  readonly type = "FFI_LIBRARY" as const;

  constructor(readonly library: LoadedLibrary) {}

  inspect(): string {
    return `ffi_library(${JSON.stringify(this.library.path)}, backend=${this.library.backend})`;
  }
}

export function ffiSymbolBuiltin(
  libObj: NexusObject,
  nameObj: NexusObject,
  retObj: NexusObject,
  ...argTypeObjs: NexusObject[]
): NexusObject {
  if (!(libObj instanceof FfiLibraryObj)) {
    return newError("ffi_symbol: first argument must be an ffi library");
  }
  if (!(nameObj instanceof StringObj)) {
    return newError("ffi_symbol: symbol name must be a string");
  }
  if (!(retObj instanceof StringObj)) {
    return newError("ffi_symbol: return type must be a string");
  }
  const argTypes = argTypeObjs.map((a) => {
    if (!(a instanceof StringObj)) {
      throw new Error("ffi_symbol: arg types must be strings");
    }
    return parseFfiType(a.value);
  });

  try {
    return libObj.library.define({
      name: nameObj.value,
      returnType: parseFfiType(retObj.value),
      argTypes,
    });
  } catch (err) {
    const message = err instanceof Error ? err.message : String(err);
    return newError(`ffi_symbol failed: ${message}`);
  }
}

export function ffiCallBuiltin(
  fnObj: NexusObject,
  ...args: NexusObject[]
): NexusObject {
  if (!(fnObj instanceof ExternFnObj)) {
    return newError("ffi_call: first argument must be an extern fn");
  }
  return fnObj.call(...args);
}

export function isFfiAvailable(): boolean {
  return tryLoadKoffi() !== null;
}

export { ErrorObj, NullObj };
