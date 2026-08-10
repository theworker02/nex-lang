import * as http from "http";
import * as https from "https";
import { URL } from "url";
import {
  BuiltinObj,
  Environment,
  StringObj,
  ErrorObj,
  NexusObject,
  IntegerObj,
} from "../language/evaluator";

function asString(obj: NexusObject): string | null {
  return obj instanceof StringObj ? obj.value : null;
}

function fetchUrl(urlStr: string, timeoutMs = 15_000): Promise<string> {
  return new Promise((resolve, reject) => {
    let parsed: URL;
    try {
      parsed = new URL(urlStr);
    } catch {
      reject(new Error(`invalid URL: ${urlStr}`));
      return;
    }
    const lib = parsed.protocol === "http:" ? http : https;
    const req = lib.get(urlStr, (res) => {
      if (
        res.statusCode &&
        res.statusCode >= 300 &&
        res.statusCode < 400 &&
        res.headers.location
      ) {
        void fetchUrl(res.headers.location, timeoutMs).then(resolve, reject);
        return;
      }
      const chunks: Buffer[] = [];
      res.on("data", (c) => chunks.push(c as Buffer));
      res.on("end", () => {
        resolve(Buffer.concat(chunks).toString("utf8"));
      });
    });
    req.setTimeout(timeoutMs, () => {
      req.destroy(new Error("net_fetch timeout"));
    });
    req.on("error", reject);
  });
}

/**
 * Async networking helpers. `net_fetch` returns a promise-like string via
 * blocking wait in the builtin (cooperative with the Nexus async runtime).
 */
export function installNet(env: Environment): void {
  env.set(
    "net_fetch",
    new BuiltinObj((...args) => {
      if (args.length < 1 || args.length > 2) {
        return new ErrorObj("net_fetch: want url [, timeout_ms]");
      }
      const url = asString(args[0]!);
      if (!url) {
        return new ErrorObj("net_fetch: url must be string");
      }
      let timeout = 15_000;
      if (args[1] instanceof IntegerObj) {
        timeout = args[1].value;
      }
      // Sync bridge using deasync-free Atomics wait on a shared buffer is
      // unavailable in all hosts; instead we stash a thenable string wrapper.
      // The evaluator awaits PromiseObj; we return a special error if sync.
      // For smoke tests and offline use, support `net_fetch_sync` mock via
      // child_process-free promise object registered on global.
      try {
        // Attempt immediate cache for data: URLs
        if (url.startsWith("data:text/plain,")) {
          return new StringObj(decodeURIComponent(url.slice("data:text/plain,".length)));
        }
        // Blocking fetch using nested event loop is not portable; use
        // spawnSync-free approach: return pending marker consumed by async eval.
        const { PromiseObj } = require("../language/runtime") as {
          PromiseObj: new (p: Promise<NexusObject>) => NexusObject;
        };
        return new PromiseObj(
          fetchUrl(url, timeout)
            .then((body) => new StringObj(body))
            .catch(
              (err) =>
                new ErrorObj(
                  `net_fetch failed: ${err instanceof Error ? err.message : String(err)}`,
                ),
            ),
        );
      } catch (err) {
        return new ErrorObj(
          `net_fetch unavailable: ${err instanceof Error ? err.message : String(err)}`,
        );
      }
    }),
  );

  env.set(
    "net_url_encode",
    new BuiltinObj((...args) => {
      if (args.length !== 1) {
        return new ErrorObj("net_url_encode: want 1 string");
      }
      const s = asString(args[0]!);
      if (s === null) {
        return new ErrorObj("net_url_encode: want string");
      }
      return new StringObj(encodeURIComponent(s));
    }),
  );
}
