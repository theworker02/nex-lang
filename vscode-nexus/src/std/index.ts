import { Environment, BuiltinObj, BooleanObj, IntegerObj, ErrorObj, StringObj } from "../language/evaluator";
import { installFs } from "./fs";
import { installNet } from "./net";
import { installCrypto } from "./crypto";
import { installTask } from "./task";
import { NexusRuntime } from "../language/runtime";
import { getMemoryEngine } from "../runtime/memory";
import {
  ffiLoadBuiltin,
  ffiSymbolBuiltin,
  ffiCallBuiltin,
  isFfiAvailable,
} from "../language/ffi";

/**
 * Register the zero-config universal standard library into a Nexus environment.
 */
export function installStdlib(
  env: Environment,
  runtime?: NexusRuntime,
): void {
  installFs(env);
  installNet(env);
  installCrypto(env);
  installTask(env, runtime);

  // Logical helpers used by syntax sugar
  env.set(
    "__and",
    new BuiltinObj((...args) => {
      if (args.length !== 2) {
        return new ErrorObj("__and: want 2 args");
      }
      const a = args[0]!;
      const b = args[1]!;
      const aTruthy =
        !(a instanceof BooleanObj && !a.value) && a.type !== "NULL";
      if (!aTruthy) {
        return new BooleanObj(false);
      }
      const bTruthy =
        !(b instanceof BooleanObj && !b.value) && b.type !== "NULL";
      return new BooleanObj(bTruthy);
    }),
  );

  env.set(
    "__or",
    new BuiltinObj((...args) => {
      if (args.length !== 2) {
        return new ErrorObj("__or: want 2 args");
      }
      const a = args[0]!;
      const b = args[1]!;
      const aTruthy =
        !(a instanceof BooleanObj && !a.value) && a.type !== "NULL";
      if (aTruthy) {
        return new BooleanObj(true);
      }
      const bTruthy =
        !(b instanceof BooleanObj && !b.value) && b.type !== "NULL";
      return new BooleanObj(bTruthy);
    }),
  );

  // Memory engine introspection
  env.set(
    "mem_stats",
    new BuiltinObj(() => {
      const s = getMemoryEngine().stats();
      return new StringObj(
        `allocated=${s.allocated} live=${s.live} freed=${s.freed} buffered=${s.buffered}`,
      );
    }),
  );

  env.set(
    "mem_collect",
    new BuiltinObj(() => {
      const n = getMemoryEngine().collectCycles();
      return new IntegerObj(n);
    }),
  );

  // FFI
  env.set(
    "ffi_load",
    new BuiltinObj((...args) => {
      if (args.length !== 1) {
        return new ErrorObj("ffi_load: want library path string");
      }
      return ffiLoadBuiltin(args[0]!);
    }),
  );

  env.set(
    "ffi_symbol",
    new BuiltinObj((...args) => {
      if (args.length < 3) {
        return new ErrorObj(
          "ffi_symbol: want lib, name, retType, ...argTypes",
        );
      }
      return ffiSymbolBuiltin(
        args[0]!,
        args[1]!,
        args[2]!,
        ...args.slice(3),
      );
    }),
  );

  env.set(
    "ffi_call",
    new BuiltinObj((...args) => {
      if (args.length < 1) {
        return new ErrorObj("ffi_call: want fn, ...args");
      }
      return ffiCallBuiltin(args[0]!, ...args.slice(1));
    }),
  );

  env.set(
    "ffi_available",
    new BuiltinObj(() => new BooleanObj(isFfiAvailable())),
  );
}
