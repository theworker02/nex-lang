import {
  ErrorObj,
  IntegerObj,
  NexusObject,
  NullObj,
  NULL_OBJ,
  newError,
} from "./values";

export type TaskFn = () => Promise<void> | void;

export interface TaskHandle {
  readonly id: number;
  readonly done: Promise<void>;
}

/**
 * Lightweight cooperative async scheduler with green tasks and channels.
 */
export class NexusRuntime {
  private nextTaskId = 1;
  private readonly ready: Array<() => void> = [];
  private running = false;
  private readonly pending = new Set<Promise<unknown>>();

  /**
   * Spawn a green task. Returns a handle whose `done` promise resolves when
   * the task body finishes.
   */
  spawn(fn: TaskFn): TaskHandle {
    const id = this.nextTaskId++;
    let resolveDone!: () => void;
    const done = new Promise<void>((resolve) => {
      resolveDone = resolve;
    });

    const run = async (): Promise<void> => {
      try {
        await fn();
      } catch {
        // Task errors are isolated.
      } finally {
        resolveDone();
      }
    };

    const promise = run();
    this.pending.add(promise);
    void promise.finally(() => this.pending.delete(promise));

    return { id, done };
  }

  /** Yield to the event loop (cooperative scheduling point). */
  async yield(): Promise<void> {
    await new Promise<void>((resolve) => setImmediate(resolve));
  }

  /** Drain until all spawned tasks and pending work settle. */
  async drain(): Promise<void> {
    if (this.running) {
      return;
    }
    this.running = true;
    try {
      // Allow newly spawned work to register.
      await this.yield();
      let spins = 0;
      while ((this.pending.size > 0 || this.ready.length > 0) && spins < 100000) {
        spins += 1;
        while (this.ready.length > 0) {
          const job = this.ready.shift()!;
          job();
        }
        if (this.pending.size === 0) {
          break;
        }
        await Promise.race([
          Promise.allSettled([...this.pending]),
          new Promise<void>((r) => setImmediate(r)),
        ]);
      }
    } finally {
      this.running = false;
    }
  }

  enqueue(job: () => void): void {
    this.ready.push(job);
  }

  createChannel(capacity = 0): ChannelObj {
    return new ChannelObj(this, Math.max(0, capacity));
  }
}

interface Waiter<T> {
  resolve: (value: T) => void;
  reject: (err: Error) => void;
}

/**
 * Message-passing channel. Capacity 0 is unbuffered (rendezvous);
 * capacity > 0 is a bounded buffer.
 */
export class ChannelObj implements NexusObject {
  readonly type = "CHANNEL" as const;
  private readonly buffer: NexusObject[] = [];
  private readonly sendWaiters: Array<{
    value: NexusObject;
    resolve: () => void;
    reject: (err: Error) => void;
  }> = [];
  private readonly recvWaiters: Array<Waiter<NexusObject>> = [];
  private closed = false;

  constructor(
    private readonly runtime: NexusRuntime,
    readonly capacity: number,
  ) {}

  inspect(): string {
    return `chan(capacity=${this.capacity}, len=${this.buffer.length}, closed=${this.closed})`;
  }

  async send(value: NexusObject): Promise<void> {
    if (this.closed) {
      throw new Error("send on closed channel");
    }

    if (this.recvWaiters.length > 0) {
      const waiter = this.recvWaiters.shift()!;
      waiter.resolve(value);
      await this.runtime.yield();
      return;
    }

    if (this.buffer.length < this.capacity) {
      this.buffer.push(value);
      await this.runtime.yield();
      return;
    }

    await new Promise<void>((resolve, reject) => {
      this.sendWaiters.push({ value, resolve, reject });
    });
    await this.runtime.yield();
  }

  async recv(): Promise<NexusObject> {
    if (this.buffer.length > 0) {
      const value = this.buffer.shift()!;
      if (this.sendWaiters.length > 0 && this.buffer.length < this.capacity) {
        const sender = this.sendWaiters.shift()!;
        this.buffer.push(sender.value);
        sender.resolve();
      }
      await this.runtime.yield();
      return value;
    }

    if (this.sendWaiters.length > 0) {
      const sender = this.sendWaiters.shift()!;
      sender.resolve();
      await this.runtime.yield();
      return sender.value;
    }

    if (this.closed) {
      return NULL_OBJ;
    }

    const value = await new Promise<NexusObject>((resolve, reject) => {
      this.recvWaiters.push({ resolve, reject });
    });
    await this.runtime.yield();
    return value;
  }

  close(): void {
    this.closed = true;
    while (this.sendWaiters.length > 0) {
      const s = this.sendWaiters.shift()!;
      s.reject(new Error("send on closed channel"));
    }
    while (this.recvWaiters.length > 0) {
      const r = this.recvWaiters.shift()!;
      r.resolve(NULL_OBJ);
    }
  }
}

/** Runtime-backed promise wrapper for async Nexus values. */
export class PromiseObj implements NexusObject {
  readonly type = "PROMISE" as const;

  constructor(readonly promise: Promise<NexusObject>) {}

  inspect(): string {
    return "promise";
  }

  async awaitValue(): Promise<NexusObject> {
    return this.promise;
  }
}

export class RefObj implements NexusObject {
  readonly type = "REF" as const;

  constructor(
    public target: NexusObject,
    readonly mutable: boolean,
  ) {}

  inspect(): string {
    return `${this.mutable ? "&mut " : "&"}${this.target.inspect()}`;
  }
}

export class EnumValueObj implements NexusObject {
  readonly type = "ENUM_VALUE" as const;

  constructor(
    readonly enumName: string | null,
    readonly variant: string,
    readonly fields: NexusObject[],
  ) {}

  inspect(): string {
    const prefix = this.enumName ? `${this.enumName}::` : "";
    if (this.fields.length === 0) {
      return `${prefix}${this.variant}`;
    }
    return `${prefix}${this.variant}(${this.fields.map((f) => f.inspect()).join(", ")})`;
  }
}

export class ExternFnObj implements NexusObject {
  readonly type = "EXTERN_FN" as const;

  constructor(
    readonly library: string,
    readonly symbol: string,
    readonly returnType: string,
    readonly call: (...args: NexusObject[]) => NexusObject,
  ) {}

  inspect(): string {
    return `extern fn ${this.symbol} from ${JSON.stringify(this.library)}`;
  }
}

export function isChannel(obj: NexusObject): obj is ChannelObj {
  return obj instanceof ChannelObj || obj.type === "CHANNEL";
}

export function isPromiseObj(obj: NexusObject): obj is PromiseObj {
  return obj instanceof PromiseObj || obj.type === "PROMISE";
}

export function isRefObj(obj: NexusObject): obj is RefObj {
  return obj instanceof RefObj || obj.type === "REF";
}

export function isEnumValue(obj: NexusObject): obj is EnumValueObj {
  return obj instanceof EnumValueObj || obj.type === "ENUM_VALUE";
}

export function asInteger(obj: NexusObject): number | null {
  if (obj instanceof IntegerObj) {
    return obj.value;
  }
  return null;
}

export function runtimeError(message: string): ErrorObj {
  return newError(message);
}

export { NullObj, IntegerObj };
