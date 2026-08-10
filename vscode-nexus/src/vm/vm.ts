/**
 * Stack-based bytecode virtual machine for Nexus.
 */

import {
  BUILTIN_NAMES,
  BuiltinObj,
  createBuiltins,
  BuiltinHost,
} from "../language/builtins";
import {
  ArrayObj,
  BooleanObj,
  ClosureObj,
  CompiledFunctionObj,
  FALSE_OBJ,
  HashObj,
  IntegerObj,
  NULL_OBJ,
  NexusObject,
  StringObj,
  TRUE_OBJ,
  isTruthy,
  newError,
  ErrorObj,
} from "../language/values";
import { Opcode, readUint16 } from "./code";
import { Bytecode } from "./compiler";

const STACK_SIZE = 2048;
const GLOBALS_SIZE = 65536;
const MAX_FRAMES = 1024;

class Frame {
  ip = -1;
  constructor(
    readonly cl: ClosureObj,
    readonly basePointer: number,
  ) {}

  instructions(): Uint8Array {
    return this.cl.fn.instructions;
  }
}

export class VirtualMachine {
  private readonly constants: NexusObject[];
  private readonly stack: (NexusObject | undefined)[] = new Array(STACK_SIZE);
  private sp = 0;
  globals: (NexusObject | undefined)[];
  private readonly frames: (Frame | undefined)[] = new Array(MAX_FRAMES);
  private framesIndex = 1;
  private readonly builtins: Map<string, BuiltinObj>;
  private lastPopped: NexusObject = NULL_OBJ;
  readonly output: string[] = [];

  constructor(bytecode: Bytecode, globals?: (NexusObject | undefined)[]) {
    const host: BuiltinHost = {
      writeOutput: (line) => this.output.push(line),
      applyFunction: (fn, args) => this.applyForBuiltin(fn, args),
    };
    this.builtins = createBuiltins(host);
    this.constants = bytecode.constants;
    this.globals = globals ?? new Array(GLOBALS_SIZE);
    const mainFn = new CompiledFunctionObj(bytecode.instructions, 0, 0);
    const mainClosure = new ClosureObj(mainFn);
    this.frames[0] = new Frame(mainClosure, 0);
  }

  lastPoppedStackElem(): NexusObject {
    return this.lastPopped;
  }

  run(): string | null {
    while (true) {
      const frame = this.currentFrame();
      if (frame.ip >= frame.instructions().length - 1) {
        break;
      }
      frame.ip++;
      const ip = frame.ip;
      const ins = frame.instructions();
      const op = ins[ip] as Opcode;

      switch (op) {
        case Opcode.OpConstant: {
          const constIndex = readUint16(ins, ip + 1);
          frame.ip += 2;
          const err = this.push(this.constants[constIndex]!);
          if (err) {
            return err;
          }
          break;
        }
        case Opcode.OpPop:
          this.lastPopped = this.pop();
          break;
        case Opcode.OpAdd:
        case Opcode.OpSub:
        case Opcode.OpMul:
        case Opcode.OpDiv:
        case Opcode.OpMod: {
          const err = this.executeBinary(op);
          if (err) {
            return err;
          }
          break;
        }
        case Opcode.OpTrue: {
          const err = this.push(TRUE_OBJ);
          if (err) {
            return err;
          }
          break;
        }
        case Opcode.OpFalse: {
          const err = this.push(FALSE_OBJ);
          if (err) {
            return err;
          }
          break;
        }
        case Opcode.OpNull: {
          const err = this.push(NULL_OBJ);
          if (err) {
            return err;
          }
          break;
        }
        case Opcode.OpEqual:
        case Opcode.OpNotEqual:
        case Opcode.OpGreaterThan:
        case Opcode.OpGreaterEqual:
        case Opcode.OpLessThan:
        case Opcode.OpLessEqual: {
          const err = this.executeComparison(op);
          if (err) {
            return err;
          }
          break;
        }
        case Opcode.OpBang: {
          const err = this.push(isTruthy(this.pop()) ? FALSE_OBJ : TRUE_OBJ);
          if (err) {
            return err;
          }
          break;
        }
        case Opcode.OpMinus: {
          const operand = this.pop();
          if (!(operand instanceof IntegerObj)) {
            return `unsupported type for negation: ${operand.type}`;
          }
          const err = this.push(new IntegerObj(-operand.value));
          if (err) {
            return err;
          }
          break;
        }
        case Opcode.OpJump: {
          const pos = readUint16(ins, ip + 1);
          frame.ip = pos - 1;
          break;
        }
        case Opcode.OpJumpNotTruthy: {
          const pos = readUint16(ins, ip + 1);
          frame.ip += 2;
          const condition = this.pop();
          if (!isTruthy(condition)) {
            frame.ip = pos - 1;
          }
          break;
        }
        case Opcode.OpSetGlobal: {
          const globalIndex = readUint16(ins, ip + 1);
          frame.ip += 2;
          this.globals[globalIndex] = this.pop();
          break;
        }
        case Opcode.OpGetGlobal: {
          const globalIndex = readUint16(ins, ip + 1);
          frame.ip += 2;
          const err = this.push(this.globals[globalIndex] ?? NULL_OBJ);
          if (err) {
            return err;
          }
          break;
        }
        case Opcode.OpSetLocal: {
          const localIndex = ins[ip + 1]!;
          frame.ip++;
          this.stack[frame.basePointer + localIndex] = this.pop();
          break;
        }
        case Opcode.OpGetLocal: {
          const localIndex = ins[ip + 1]!;
          frame.ip++;
          const err = this.push(
            this.stack[frame.basePointer + localIndex] ?? NULL_OBJ,
          );
          if (err) {
            return err;
          }
          break;
        }
        case Opcode.OpGetBuiltin: {
          const builtinIndex = ins[ip + 1]!;
          frame.ip++;
          const name = BUILTIN_NAMES[builtinIndex];
          const builtin = name ? this.builtins.get(name) : undefined;
          if (!builtin) {
            return `invalid builtin index ${builtinIndex}`;
          }
          const err = this.push(builtin);
          if (err) {
            return err;
          }
          break;
        }
        case Opcode.OpArray: {
          const numElements = readUint16(ins, ip + 1);
          frame.ip += 2;
          const array = this.buildArray(this.sp - numElements, this.sp);
          this.sp -= numElements;
          const err = this.push(array);
          if (err) {
            return err;
          }
          break;
        }
        case Opcode.OpHash: {
          const numElements = readUint16(ins, ip + 1);
          frame.ip += 2;
          const hash = this.buildHash(this.sp - numElements, this.sp);
          this.sp -= numElements;
          const err = this.push(hash);
          if (err) {
            return err;
          }
          break;
        }
        case Opcode.OpIndex: {
          const index = this.pop();
          const left = this.pop();
          const err = this.executeIndex(left, index);
          if (err) {
            return err;
          }
          break;
        }
        case Opcode.OpMember: {
          const fieldIndex = readUint16(ins, ip + 1);
          frame.ip += 2;
          const fieldObj = this.constants[fieldIndex];
          if (!(fieldObj instanceof StringObj)) {
            return "member field constant must be string";
          }
          const left = this.pop();
          const err = this.executeMember(left, fieldObj.value);
          if (err) {
            return err;
          }
          break;
        }
        case Opcode.OpSetIndex: {
          const val = this.pop();
          const index = this.pop();
          const left = this.pop();
          const setErr = this.executeSetIndex(left, index, val);
          if (setErr) {
            return setErr;
          }
          const err = this.push(val);
          if (err) {
            return err;
          }
          break;
        }
        case Opcode.OpCall: {
          const numArgs = ins[ip + 1]!;
          frame.ip++;
          const err = this.executeCall(numArgs);
          if (err) {
            return err;
          }
          break;
        }
        case Opcode.OpReturnValue: {
          const returnValue = this.pop();
          const popped = this.popFrame();
          this.sp = popped.basePointer - 1;
          const err = this.push(returnValue);
          if (err) {
            return err;
          }
          break;
        }
        case Opcode.OpReturn: {
          const popped = this.popFrame();
          this.sp = popped.basePointer - 1;
          const err = this.push(NULL_OBJ);
          if (err) {
            return err;
          }
          break;
        }
        case Opcode.OpClosure: {
          const constIndex = readUint16(ins, ip + 1);
          const numFree = ins[ip + 3]!;
          frame.ip += 3;
          const err = this.pushClosure(constIndex, numFree);
          if (err) {
            return err;
          }
          break;
        }
        case Opcode.OpGetFree: {
          const freeIndex = ins[ip + 1]!;
          frame.ip++;
          const err = this.push(
            this.currentFrame().cl.free[freeIndex] ?? NULL_OBJ,
          );
          if (err) {
            return err;
          }
          break;
        }
        case Opcode.OpCurrentClosure: {
          const err = this.push(this.currentFrame().cl);
          if (err) {
            return err;
          }
          break;
        }
        default:
          return `unknown opcode ${op}`;
      }
    }
    return null;
  }

  private applyForBuiltin(fn: NexusObject, args: NexusObject[]): NexusObject {
    if (fn instanceof BuiltinObj) {
      return fn.fn(...args);
    }
    if (fn instanceof ClosureObj) {
      // Run a nested mini-call via stack (limited: use recursive VM call pattern)
      return newError("vm map/filter of closures: use tree-walk engine");
    }
    return newError(`not a function: ${fn.type}`);
  }

  private currentFrame(): Frame {
    return this.frames[this.framesIndex - 1]!;
  }

  private pushFrame(f: Frame): void {
    this.frames[this.framesIndex] = f;
    this.framesIndex++;
  }

  private popFrame(): Frame {
    this.framesIndex--;
    return this.frames[this.framesIndex]!;
  }

  private push(obj: NexusObject): string | null {
    if (this.sp >= STACK_SIZE) {
      return "stack overflow";
    }
    this.stack[this.sp] = obj;
    this.sp++;
    return null;
  }

  private pop(): NexusObject {
    this.sp--;
    const obj = this.stack[this.sp] ?? NULL_OBJ;
    this.lastPopped = obj;
    return obj;
  }

  private pushClosure(constIndex: number, numFree: number): string | null {
    const constant = this.constants[constIndex];
    if (!(constant instanceof CompiledFunctionObj)) {
      return `not a function: ${constant?.type}`;
    }
    const free: NexusObject[] = [];
    for (let i = 0; i < numFree; i++) {
      free.push(this.stack[this.sp - numFree + i] ?? NULL_OBJ);
    }
    this.sp -= numFree;
    return this.push(new ClosureObj(constant, free));
  }

  private executeCall(numArgs: number): string | null {
    const callee = this.stack[this.sp - 1 - numArgs];
    if (callee instanceof ClosureObj) {
      return this.callClosure(callee, numArgs);
    }
    if (callee instanceof BuiltinObj) {
      return this.callBuiltin(callee, numArgs);
    }
    return `calling non-function: ${callee?.type}`;
  }

  private callClosure(cl: ClosureObj, numArgs: number): string | null {
    if (numArgs !== cl.fn.numParameters) {
      return `wrong number of arguments: want=${cl.fn.numParameters}, got=${numArgs}`;
    }
    const frame = new Frame(cl, this.sp - numArgs);
    this.pushFrame(frame);
    this.sp = frame.basePointer + cl.fn.numLocals;
    return null;
  }

  private callBuiltin(builtin: BuiltinObj, numArgs: number): string | null {
    const args: NexusObject[] = [];
    for (let i = 0; i < numArgs; i++) {
      args.push(this.stack[this.sp - numArgs + i] ?? NULL_OBJ);
    }
    const result = builtin.fn(...args);
    this.sp = this.sp - numArgs - 1;
    if (result instanceof ErrorObj) {
      return result.message;
    }
    return this.push(result);
  }

  private buildArray(start: number, end: number): ArrayObj {
    const elements: NexusObject[] = [];
    for (let i = start; i < end; i++) {
      elements.push(this.stack[i] ?? NULL_OBJ);
    }
    return new ArrayObj(elements);
  }

  private buildHash(start: number, end: number): HashObj {
    const hash = new HashObj();
    for (let i = start; i < end; i += 2) {
      hash.set(this.stack[i] ?? NULL_OBJ, this.stack[i + 1] ?? NULL_OBJ);
    }
    return hash;
  }

  private executeIndex(left: NexusObject, index: NexusObject): string | null {
    if (left instanceof ArrayObj && index instanceof IntegerObj) {
      const el = left.elements[index.value];
      return this.push(el ?? NULL_OBJ);
    }
    if (left instanceof HashObj) {
      return this.push(left.get(index));
    }
    if (left instanceof StringObj && index instanceof IntegerObj) {
      if (index.value < 0 || index.value >= left.value.length) {
        return this.push(NULL_OBJ);
      }
      return this.push(new StringObj(left.value[index.value]!));
    }
    return `index operator not supported: ${left.type}`;
  }

  private executeMember(left: NexusObject, field: string): string | null {
    if (!(left instanceof HashObj)) {
      return `member access on non-hash: ${left.type}`;
    }
    return this.push(left.getString(field));
  }

  private executeSetIndex(
    left: NexusObject,
    index: NexusObject,
    val: NexusObject,
  ): string | null {
    if (left instanceof ArrayObj) {
      if (!(index instanceof IntegerObj)) {
        return "array index must be INTEGER";
      }
      if (index.value < 0 || index.value >= left.elements.length) {
        return "array index out of bounds";
      }
      left.elements[index.value] = val;
      return null;
    }
    if (left instanceof HashObj) {
      left.set(index, val);
      return null;
    }
    return `index assignment not supported on ${left.type}`;
  }

  private executeBinary(op: Opcode): string | null {
    const right = this.pop();
    const left = this.pop();
    if (left instanceof IntegerObj && right instanceof IntegerObj) {
      return this.executeBinaryInteger(op, left, right);
    }
    if (
      left instanceof StringObj &&
      right instanceof StringObj &&
      op === Opcode.OpAdd
    ) {
      return this.push(new StringObj(left.value + right.value));
    }
    return `unsupported types for binary operation: ${left.type} ${right.type}`;
  }

  private executeBinaryInteger(
    op: Opcode,
    left: IntegerObj,
    right: IntegerObj,
  ): string | null {
    let result: number;
    switch (op) {
      case Opcode.OpAdd:
        result = left.value + right.value;
        break;
      case Opcode.OpSub:
        result = left.value - right.value;
        break;
      case Opcode.OpMul:
        result = left.value * right.value;
        break;
      case Opcode.OpDiv:
        if (right.value === 0) {
          return "division by zero";
        }
        result = Math.trunc(left.value / right.value);
        break;
      case Opcode.OpMod:
        if (right.value === 0) {
          return "modulo by zero";
        }
        result = left.value % right.value;
        break;
      default:
        return `unknown integer operator ${op}`;
    }
    return this.push(new IntegerObj(result));
  }

  private executeComparison(op: Opcode): string | null {
    const right = this.pop();
    const left = this.pop();
    if (left instanceof IntegerObj && right instanceof IntegerObj) {
      switch (op) {
        case Opcode.OpEqual:
          return this.push(nativeBool(left.value === right.value));
        case Opcode.OpNotEqual:
          return this.push(nativeBool(left.value !== right.value));
        case Opcode.OpGreaterThan:
          return this.push(nativeBool(left.value > right.value));
        case Opcode.OpGreaterEqual:
          return this.push(nativeBool(left.value >= right.value));
        case Opcode.OpLessThan:
          return this.push(nativeBool(left.value < right.value));
        case Opcode.OpLessEqual:
          return this.push(nativeBool(left.value <= right.value));
      }
    }
    switch (op) {
      case Opcode.OpEqual:
        return this.push(nativeBool(left === right || left.inspect() === right.inspect()));
      case Opcode.OpNotEqual:
        return this.push(
          nativeBool(!(left === right || left.inspect() === right.inspect())),
        );
      default:
        return `unsupported comparison on ${left.type}`;
    }
  }
}

function nativeBool(v: boolean): BooleanObj {
  return v ? TRUE_OBJ : FALSE_OBJ;
}

export function runBytecode(
  bytecode: Bytecode,
  globals?: (NexusObject | undefined)[],
): { value: NexusObject; output: string[]; error?: string } {
  const vm = new VirtualMachine(bytecode, globals);
  const error = vm.run();
  if (error) {
    return { value: newError(error), output: vm.output, error };
  }
  return { value: vm.lastPoppedStackElem(), output: vm.output };
}
