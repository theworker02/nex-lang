import { Worker } from "worker_threads";
import {
  BuiltinObj,
  Environment,
  IntegerObj,
  StringObj,
  ErrorObj,
  NexusObject,
  NullObj,
} from "../language/evaluator";
import { NexusRuntime, PromiseObj } from "../language/runtime";

const NULL = new NullObj();

function asString(obj: NexusObject): string | null {
  return obj instanceof StringObj ? obj.value : null;
}

/**
 * Concurrent task scheduling — green tasks via NexusRuntime and optional
 * Worker-thread offload for CPU-bound string jobs.
 */
export function installTask(env: Environment, runtime?: NexusRuntime): void {
  const rt = runtime ?? new NexusRuntime();

  env.set(
    "spawn_task",
    new BuiltinObj((...args) => {
      if (args.length !== 1) {
        return new ErrorObj("spawn_task: want 1 function");
      }
      const fn = args[0]!;
      // Functions are applied by evaluator; here we schedule a no-op marker
      // and return a task id. Full fn application happens via spawn expr.
      const handle = rt.spawn(async () => {
        void fn;
      });
      return new IntegerObj(handle.id);
    }),
  );

  env.set(
    "task_yield",
    new BuiltinObj(() => {
      return new PromiseObj(rt.yield().then(() => NULL));
    }),
  );

  env.set(
    "worker_hash",
    new BuiltinObj((...args) => {
      if (args.length !== 1) {
        return new ErrorObj("worker_hash: want 1 string");
      }
      const input = asString(args[0]!);
      if (input === null) {
        return new ErrorObj("worker_hash: want string");
      }

      const promise = new Promise<NexusObject>((resolve) => {
        // Inline worker source to avoid extra files
        const code = `
          const { parentPort, workerData } = require('worker_threads');
          const crypto = require('crypto');
          const digest = crypto.createHash('sha256').update(workerData, 'utf8').digest('hex');
          parentPort.postMessage(digest);
        `;
        try {
          const worker = new Worker(code, {
            eval: true,
            workerData: input,
          });
          worker.on("message", (msg: string) => {
            resolve(new StringObj(msg));
            void worker.terminate();
          });
          worker.on("error", (err) => {
            resolve(new ErrorObj(`worker_hash: ${err.message}`));
          });
        } catch (err) {
          // Fallback on hosts that block workers
          const crypto = require("crypto") as typeof import("crypto");
          resolve(
            new StringObj(
              crypto.createHash("sha256").update(input, "utf8").digest("hex"),
            ),
          );
          void err;
        }
      });

      return new PromiseObj(promise);
    }),
  );

  env.set(
    "task_drain",
    new BuiltinObj(() => new PromiseObj(rt.drain().then(() => NULL))),
  );
}
