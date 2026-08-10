/**
 * Synchronous Postgres queries via a dedicated worker + Atomics.wait.
 * Keeps db_* host builtins sync while using the real `pg` driver.
 */

import {
  MessageChannel,
  Worker,
  receiveMessageOnPort,
} from "worker_threads";

export interface SyncPgResult {
  rows: Array<Record<string, unknown>>;
  rowCount: number;
}

interface WorkerReply {
  ok: boolean;
  error?: string;
  rows?: Array<Record<string, unknown>>;
  rowCount?: number;
}

const WORKER_SOURCE = `
const { parentPort, workerData } = require("worker_threads");
const { Pool } = require("pg");

const pool = new Pool({
  connectionString: workerData.connectionString,
  max: 6,
  idleTimeoutMillis: 30000,
});
const lock = new Int32Array(workerData.sab);

parentPort.on("message", async (msg) => {
  const port = msg.port;
  try {
    if (msg.type === "end") {
      await pool.end();
      port.postMessage({ ok: true, rows: [], rowCount: 0 });
    } else {
      const res = await pool.query(msg.text, msg.params || []);
      // A multi-statement query resolves to an array of results; report the last one.
      const last = Array.isArray(res) ? res[res.length - 1] : res;
      const rows = (last && last.rows) || [];
      port.postMessage({
        ok: true,
        rows,
        rowCount:
          last && last.rowCount != null ? last.rowCount : rows.length,
      });
    }
  } catch (err) {
    port.postMessage({
      ok: false,
      error: err && err.message ? err.message : String(err),
      rows: [],
      rowCount: 0,
    });
  } finally {
    Atomics.store(lock, 0, 1);
    Atomics.notify(lock, 0);
    try { port.close(); } catch (_) {}
  }
});
`;

export class SyncPg {
  private worker: Worker;
  private lock: Int32Array;
  private closed = false;

  constructor(connectionString: string) {
    const sab = new SharedArrayBuffer(4);
    this.lock = new Int32Array(sab);
    this.worker = new Worker(WORKER_SOURCE, {
      eval: true,
      workerData: { connectionString, sab },
    });
    this.worker.on("error", (err) => {
      // eslint-disable-next-line no-console
      console.error("[sync-pg] worker error:", err);
    });
  }

  query(text: string, params: unknown[] = []): SyncPgResult {
    if (this.closed) {
      throw new Error("sync pg: pool closed");
    }
    const { port1, port2 } = new MessageChannel();
    Atomics.store(this.lock, 0, 0);
    this.worker.postMessage({ type: "query", port: port2, text, params }, [
      port2,
    ]);
    let msg = receiveMessageOnPort(port1);
    const started = Date.now();
    while (msg === undefined) {
      Atomics.wait(this.lock, 0, 0, 250);
      msg = receiveMessageOnPort(port1);
      if (msg === undefined && Atomics.load(this.lock, 0) === 1) {
        // The worker signalled completion; the message may still be in flight.
        for (let i = 0; i < 200 && msg === undefined; i++) {
          msg = receiveMessageOnPort(port1);
        }
        break;
      }
      if (Date.now() - started > 60_000) {
        port1.close();
        throw new Error("sync pg: query timed out");
      }
    }
    port1.close();
    const body = msg?.message as WorkerReply | undefined;
    if (!body) {
      throw new Error("sync pg: empty response");
    }
    if (!body.ok) {
      throw new Error(body.error || "query failed");
    }
    return { rows: body.rows ?? [], rowCount: body.rowCount ?? 0 };
  }

  exec(text: string, params: unknown[] = []): number {
    return this.query(text, params).rowCount;
  }

  close(): void {
    if (this.closed) {
      return;
    }
    this.closed = true;
    try {
      const { port1, port2 } = new MessageChannel();
      Atomics.store(this.lock, 0, 0);
      this.worker.postMessage({ type: "end", port: port2 }, [port2]);
      receiveMessageOnPort(port1);
      port1.close();
    } catch {
      // ignore
    }
    void this.worker.terminate();
  }
}
