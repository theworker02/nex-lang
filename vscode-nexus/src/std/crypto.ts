import * as crypto from "crypto";
import {
  BuiltinObj,
  Environment,
  StringObj,
  ErrorObj,
  NexusObject,
} from "../language/evaluator";

function asString(obj: NexusObject): string | null {
  return obj instanceof StringObj ? obj.value : null;
}

/**
 * Cryptographic hashing builtins backed by Node crypto.
 */
export function installCrypto(env: Environment): void {
  env.set(
    "sha256",
    new BuiltinObj((...args) => {
      if (args.length !== 1) {
        return new ErrorObj("sha256: want 1 string");
      }
      const s = asString(args[0]!);
      if (s === null) {
        return new ErrorObj("sha256: want string");
      }
      return new StringObj(
        crypto.createHash("sha256").update(s, "utf8").digest("hex"),
      );
    }),
  );

  env.set(
    "sha512",
    new BuiltinObj((...args) => {
      if (args.length !== 1) {
        return new ErrorObj("sha512: want 1 string");
      }
      const s = asString(args[0]!);
      if (s === null) {
        return new ErrorObj("sha512: want string");
      }
      return new StringObj(
        crypto.createHash("sha512").update(s, "utf8").digest("hex"),
      );
    }),
  );

  env.set(
    "md5",
    new BuiltinObj((...args) => {
      if (args.length !== 1) {
        return new ErrorObj("md5: want 1 string");
      }
      const s = asString(args[0]!);
      if (s === null) {
        return new ErrorObj("md5: want string");
      }
      return new StringObj(
        crypto.createHash("md5").update(s, "utf8").digest("hex"),
      );
    }),
  );

  env.set(
    "hmac_sha256",
    new BuiltinObj((...args) => {
      if (args.length !== 2) {
        return new ErrorObj("hmac_sha256: want key, data");
      }
      const key = asString(args[0]!);
      const data = asString(args[1]!);
      if (key === null || data === null) {
        return new ErrorObj("hmac_sha256: want strings");
      }
      return new StringObj(
        crypto.createHmac("sha256", key).update(data, "utf8").digest("hex"),
      );
    }),
  );

  env.set(
    "random_bytes_hex",
    new BuiltinObj((...args) => {
      let n = 16;
      if (args.length === 1 && args[0] && "value" in args[0]) {
        n = Number((args[0] as { value: number }).value);
      }
      if (!Number.isFinite(n) || n < 0 || n > 65536) {
        return new ErrorObj("random_bytes_hex: invalid length");
      }
      return new StringObj(crypto.randomBytes(n).toString("hex"));
    }),
  );
}
