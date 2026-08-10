import { NexusObject, newError } from "../language/values";

/**
 * Automatic zero-overhead memory engine: deterministic refcounting with
 * smart pointers (Rc/Arc) and an incremental cycle collector.
 */

export type ObjectId = number;

export interface ManagedHeader {
  id: ObjectId;
  strong: number;
  weak: number;
  color: "black" | "gray" | "white" | "purple";
  buffered: boolean;
  children: ObjectId[];
}

export interface ManagedObject {
  header: ManagedHeader;
  value: NexusObject;
}

/** Shared ownership smart pointer (single-threaded Rc). */
export class Rc<T extends NexusObject = NexusObject> {
  constructor(private readonly heap: MemoryEngine, readonly id: ObjectId) {
    this.heap.retain(id);
  }

  get(): T {
    return this.heap.deref(this.id) as T;
  }

  clone(): Rc<T> {
    return new Rc<T>(this.heap, this.id);
  }

  drop(): void {
    this.heap.release(this.id);
  }
}

/** Atomic-style shared pointer API (cooperative Arc in the extension host). */
export class Arc<T extends NexusObject = NexusObject> {
  private readonly lock = { taken: false };

  constructor(private readonly heap: MemoryEngine, readonly id: ObjectId) {
    this.heap.retain(id);
  }

  get(): T {
    return this.heap.deref(this.id) as T;
  }

  clone(): Arc<T> {
    return new Arc<T>(this.heap, this.id);
  }

  /** Cooperative critical section for host-thread safety. */
  withLock<R>(fn: (value: T) => R): R {
    if (this.lock.taken) {
      throw new Error("Arc lock contention");
    }
    this.lock.taken = true;
    try {
      return fn(this.get());
    } finally {
      this.lock.taken = false;
    }
  }

  drop(): void {
    this.heap.release(this.id);
  }
}

/**
 * Heap with strong/weak refcounts and incremental cycle detection
 * (Bacon–Rajan style purple buffering).
 */
export class MemoryEngine {
  private nextId: ObjectId = 1;
  private readonly objects = new Map<ObjectId, ManagedObject>();
  private readonly cycleBuffer: ObjectId[] = [];
  private collectBudget = 32;
  private freed = 0;
  private allocated = 0;

  stats(): { allocated: number; live: number; freed: number; buffered: number } {
    return {
      allocated: this.allocated,
      live: this.objects.size,
      freed: this.freed,
      buffered: this.cycleBuffer.length,
    };
  }

  alloc(value: NexusObject, children: ObjectId[] = []): ObjectId {
    const id = this.nextId++;
    this.objects.set(id, {
      header: {
        id,
        strong: 1,
        weak: 0,
        color: "black",
        buffered: false,
        children: [...children],
      },
      value,
    });
    this.allocated += 1;
    return id;
  }

  retain(id: ObjectId): void {
    const obj = this.objects.get(id);
    if (!obj) {
      return;
    }
    obj.header.strong += 1;
    obj.header.color = "black";
  }

  release(id: ObjectId): void {
    const obj = this.objects.get(id);
    if (!obj) {
      return;
    }
    obj.header.strong -= 1;
    if (obj.header.strong === 0) {
      this.free(id);
      return;
    }
    // Possible cycle participant
    if (obj.header.color !== "purple") {
      obj.header.color = "purple";
      if (!obj.header.buffered) {
        obj.header.buffered = true;
        this.cycleBuffer.push(id);
      }
    }
    this.collectIncremental();
  }

  weakRetain(id: ObjectId): void {
    const obj = this.objects.get(id);
    if (obj) {
      obj.header.weak += 1;
    }
  }

  weakRelease(id: ObjectId): void {
    const obj = this.objects.get(id);
    if (!obj) {
      return;
    }
    obj.header.weak -= 1;
    if (obj.header.strong === 0 && obj.header.weak === 0) {
      this.objects.delete(id);
    }
  }

  deref(id: ObjectId): NexusObject {
    const obj = this.objects.get(id);
    if (!obj) {
      return newError(`dangling pointer to object #${id}`);
    }
    return obj.value;
  }

  setChildren(id: ObjectId, children: ObjectId[]): void {
    const obj = this.objects.get(id);
    if (obj) {
      obj.header.children = [...children];
    }
  }

  updateValue(id: ObjectId, value: NexusObject): void {
    const obj = this.objects.get(id);
    if (obj) {
      obj.value = value;
    }
  }

  rc<T extends NexusObject>(value: T, children: ObjectId[] = []): Rc<T> {
    const id = this.alloc(value, children);
    // alloc starts at strong=1; Rc constructor retains → bump then drop one
    const ptr = new Rc<T>(this, id);
    this.release(id); // balance the initial alloc retain vs Rc retain
    return ptr;
  }

  arc<T extends NexusObject>(value: T, children: ObjectId[] = []): Arc<T> {
    const id = this.alloc(value, children);
    const ptr = new Arc<T>(this, id);
    this.release(id);
    return ptr;
  }

  /** Run a bounded amount of cycle collection work. */
  collectIncremental(budget = this.collectBudget): number {
    let work = 0;
    while (work < budget && this.cycleBuffer.length > 0) {
      const id = this.cycleBuffer.shift()!;
      const obj = this.objects.get(id);
      if (!obj) {
        continue;
      }
      obj.header.buffered = false;
      if (obj.header.color === "purple") {
        this.markGray(id);
        this.scan(id);
        this.collectWhite(id);
        work += 1;
      }
    }
    return work;
  }

  /** Force a full cycle collection pass. */
  collectCycles(): number {
    let total = 0;
    let n = 0;
    do {
      n = this.collectIncremental(10_000);
      total += n;
    } while (n > 0);
    return total;
  }

  private free(id: ObjectId): void {
    const obj = this.objects.get(id);
    if (!obj) {
      return;
    }
    for (const child of obj.header.children) {
      this.release(child);
    }
    if (obj.header.weak === 0) {
      this.objects.delete(id);
    } else {
      // Keep shell for weak refs; mark dead by clearing children
      obj.header.children = [];
      obj.header.color = "black";
    }
    this.freed += 1;
  }

  private markGray(id: ObjectId): void {
    const obj = this.objects.get(id);
    if (!obj || obj.header.color === "gray") {
      return;
    }
    obj.header.color = "gray";
    for (const child of obj.header.children) {
      const c = this.objects.get(child);
      if (!c) {
        continue;
      }
      c.header.strong -= 1;
      this.markGray(child);
    }
  }

  private scan(id: ObjectId): void {
    const obj = this.objects.get(id);
    if (!obj || obj.header.color !== "gray") {
      return;
    }
    if (obj.header.strong > 0) {
      this.scanBlack(id);
      return;
    }
    obj.header.color = "white";
    for (const child of obj.header.children) {
      this.scan(child);
    }
  }

  private scanBlack(id: ObjectId): void {
    const obj = this.objects.get(id);
    if (!obj) {
      return;
    }
    obj.header.color = "black";
    for (const child of obj.header.children) {
      const c = this.objects.get(child);
      if (!c) {
        continue;
      }
      c.header.strong += 1;
      if (c.header.color !== "black") {
        this.scanBlack(child);
      }
    }
  }

  private collectWhite(id: ObjectId): void {
    const obj = this.objects.get(id);
    if (!obj || obj.header.color !== "white") {
      return;
    }
    obj.header.color = "black";
    for (const child of [...obj.header.children]) {
      this.collectWhite(child);
    }
    this.objects.delete(id);
    this.freed += 1;
  }
}

/** Process-wide default engine used by the evaluator/runtime. */
let defaultEngine: MemoryEngine | null = null;

export function getMemoryEngine(): MemoryEngine {
  if (!defaultEngine) {
    defaultEngine = new MemoryEngine();
  }
  return defaultEngine;
}

export function resetMemoryEngine(): MemoryEngine {
  defaultEngine = new MemoryEngine();
  return defaultEngine;
}
