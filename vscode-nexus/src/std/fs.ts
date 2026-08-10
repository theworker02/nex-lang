import * as fs from "fs";
import * as path from "path";
import {
  BuiltinObj,
  Environment,
  IntegerObj,
  StringObj,
  BooleanObj,
  ErrorObj,
  NexusObject,
} from "../language/evaluator";

function asString(obj: NexusObject): string | null {
  if (obj instanceof StringObj) {
    return obj.value;
  }
  return null;
}

/**
 * Filesystem I/O builtins for Nexus.
 */
export function installFs(env: Environment): void {
  env.set(
    "fs_read",
    new BuiltinObj((...args) => {
      if (args.length !== 1) {
        return new ErrorObj("fs_read: want 1 string path");
      }
      const p = asString(args[0]!);
      if (p === null) {
        return new ErrorObj("fs_read: path must be string");
      }
      try {
        return new StringObj(fs.readFileSync(p, "utf8"));
      } catch (err) {
        return new ErrorObj(
          `fs_read failed: ${err instanceof Error ? err.message : String(err)}`,
        );
      }
    }),
  );

  env.set(
    "fs_write",
    new BuiltinObj((...args) => {
      if (args.length !== 2) {
        return new ErrorObj("fs_write: want path, contents");
      }
      const p = asString(args[0]!);
      const contents = asString(args[1]!);
      if (p === null || contents === null) {
        return new ErrorObj("fs_write: args must be strings");
      }
      try {
        fs.mkdirSync(path.dirname(p), { recursive: true });
        fs.writeFileSync(p, contents, "utf8");
        return new IntegerObj(contents.length);
      } catch (err) {
        return new ErrorObj(
          `fs_write failed: ${err instanceof Error ? err.message : String(err)}`,
        );
      }
    }),
  );

  env.set(
    "fs_exists",
    new BuiltinObj((...args) => {
      if (args.length !== 1) {
        return new ErrorObj("fs_exists: want 1 path");
      }
      const p = asString(args[0]!);
      if (p === null) {
        return new ErrorObj("fs_exists: path must be string");
      }
      return new BooleanObj(fs.existsSync(p));
    }),
  );

  env.set(
    "fs_list",
    new BuiltinObj((...args) => {
      if (args.length !== 1) {
        return new ErrorObj("fs_list: want 1 directory path");
      }
      const p = asString(args[0]!);
      if (p === null) {
        return new ErrorObj("fs_list: path must be string");
      }
      try {
        return new StringObj(fs.readdirSync(p).join("\n"));
      } catch (err) {
        return new ErrorObj(
          `fs_list failed: ${err instanceof Error ? err.message : String(err)}`,
        );
      }
    }),
  );
}
