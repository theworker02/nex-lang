/**
 * Nexus bytecode opcodes and instruction encoding (TypeScript VM).
 */

export type Instructions = Uint8Array;

export enum Opcode {
  OpConstant = 0,
  OpPop,
  OpAdd,
  OpSub,
  OpMul,
  OpDiv,
  OpMod,
  OpTrue,
  OpFalse,
  OpNull,
  OpEqual,
  OpNotEqual,
  OpGreaterThan,
  OpGreaterEqual,
  OpLessThan,
  OpLessEqual,
  OpMinus,
  OpBang,
  OpJump,
  OpJumpNotTruthy,
  OpGetGlobal,
  OpSetGlobal,
  OpGetLocal,
  OpSetLocal,
  OpGetBuiltin,
  OpArray,
  OpHash,
  OpIndex,
  OpCall,
  OpReturnValue,
  OpReturn,
  OpClosure,
  OpGetFree,
  OpCurrentClosure,
  OpSetIndex,
  OpMember,
  OpAnd,
  OpOr,
}

export interface Definition {
  name: string;
  operandWidths: number[];
}

const DEFINITIONS: Record<number, Definition> = {
  [Opcode.OpConstant]: { name: "OpConstant", operandWidths: [2] },
  [Opcode.OpPop]: { name: "OpPop", operandWidths: [] },
  [Opcode.OpAdd]: { name: "OpAdd", operandWidths: [] },
  [Opcode.OpSub]: { name: "OpSub", operandWidths: [] },
  [Opcode.OpMul]: { name: "OpMul", operandWidths: [] },
  [Opcode.OpDiv]: { name: "OpDiv", operandWidths: [] },
  [Opcode.OpMod]: { name: "OpMod", operandWidths: [] },
  [Opcode.OpTrue]: { name: "OpTrue", operandWidths: [] },
  [Opcode.OpFalse]: { name: "OpFalse", operandWidths: [] },
  [Opcode.OpNull]: { name: "OpNull", operandWidths: [] },
  [Opcode.OpEqual]: { name: "OpEqual", operandWidths: [] },
  [Opcode.OpNotEqual]: { name: "OpNotEqual", operandWidths: [] },
  [Opcode.OpGreaterThan]: { name: "OpGreaterThan", operandWidths: [] },
  [Opcode.OpGreaterEqual]: { name: "OpGreaterEqual", operandWidths: [] },
  [Opcode.OpLessThan]: { name: "OpLessThan", operandWidths: [] },
  [Opcode.OpLessEqual]: { name: "OpLessEqual", operandWidths: [] },
  [Opcode.OpMinus]: { name: "OpMinus", operandWidths: [] },
  [Opcode.OpBang]: { name: "OpBang", operandWidths: [] },
  [Opcode.OpJump]: { name: "OpJump", operandWidths: [2] },
  [Opcode.OpJumpNotTruthy]: { name: "OpJumpNotTruthy", operandWidths: [2] },
  [Opcode.OpGetGlobal]: { name: "OpGetGlobal", operandWidths: [2] },
  [Opcode.OpSetGlobal]: { name: "OpSetGlobal", operandWidths: [2] },
  [Opcode.OpGetLocal]: { name: "OpGetLocal", operandWidths: [1] },
  [Opcode.OpSetLocal]: { name: "OpSetLocal", operandWidths: [1] },
  [Opcode.OpGetBuiltin]: { name: "OpGetBuiltin", operandWidths: [1] },
  [Opcode.OpArray]: { name: "OpArray", operandWidths: [2] },
  [Opcode.OpHash]: { name: "OpHash", operandWidths: [2] },
  [Opcode.OpIndex]: { name: "OpIndex", operandWidths: [] },
  [Opcode.OpCall]: { name: "OpCall", operandWidths: [1] },
  [Opcode.OpReturnValue]: { name: "OpReturnValue", operandWidths: [] },
  [Opcode.OpReturn]: { name: "OpReturn", operandWidths: [] },
  [Opcode.OpClosure]: { name: "OpClosure", operandWidths: [2, 1] },
  [Opcode.OpGetFree]: { name: "OpGetFree", operandWidths: [1] },
  [Opcode.OpCurrentClosure]: { name: "OpCurrentClosure", operandWidths: [] },
  [Opcode.OpSetIndex]: { name: "OpSetIndex", operandWidths: [] },
  [Opcode.OpMember]: { name: "OpMember", operandWidths: [2] },
  [Opcode.OpAnd]: { name: "OpAnd", operandWidths: [] },
  [Opcode.OpOr]: { name: "OpOr", operandWidths: [] },
};

export function lookup(op: Opcode): Definition | undefined {
  return DEFINITIONS[op];
}

export function make(op: Opcode, ...operands: number[]): number[] {
  const def = DEFINITIONS[op];
  if (!def) {
    return [];
  }
  let instructionLen = 1;
  for (const w of def.operandWidths) {
    instructionLen += w;
  }
  const instruction = new Array<number>(instructionLen);
  instruction[0] = op;
  let offset = 1;
  for (let i = 0; i < operands.length; i++) {
    const width = def.operandWidths[i]!;
    const o = operands[i]!;
    if (width === 1) {
      instruction[offset] = o & 0xff;
    } else if (width === 2) {
      instruction[offset] = (o >> 8) & 0xff;
      instruction[offset + 1] = o & 0xff;
    }
    offset += width;
  }
  return instruction;
}

export function readUint16(ins: Uint8Array | number[], offset: number): number {
  return ((ins[offset]! & 0xff) << 8) | (ins[offset + 1]! & 0xff);
}

export function readUint8(ins: Uint8Array | number[], offset: number): number {
  return ins[offset]! & 0xff;
}

export function readOperands(
  def: Definition,
  ins: Uint8Array | number[],
  start: number,
): { operands: number[]; bytesRead: number } {
  const operands: number[] = [];
  let offset = 0;
  for (const width of def.operandWidths) {
    if (width === 1) {
      operands.push(readUint8(ins, start + offset));
    } else if (width === 2) {
      operands.push(readUint16(ins, start + offset));
    }
    offset += width;
  }
  return { operands, bytesRead: offset };
}

export function concatInstructions(...parts: number[][]): Uint8Array {
  const total = parts.reduce((n, p) => n + p.length, 0);
  const out = new Uint8Array(total);
  let offset = 0;
  for (const p of parts) {
    out.set(p, offset);
    offset += p.length;
  }
  return out;
}
