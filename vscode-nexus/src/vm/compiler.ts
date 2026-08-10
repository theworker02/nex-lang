/**
 * Bytecode compiler: lowers Nexus AST to instructions for the TS VM.
 */

import {
  ArrayLiteral,
  AssignExpression,
  BlockStatement,
  BooleanLiteral,
  CallExpression,
  ExpressionStatement,
  FunctionLiteral,
  HashLiteral,
  Identifier,
  IfExpression,
  IndexExpression,
  InfixExpression,
  IntegerLiteral,
  LetStatement,
  MemberExpression,
  Node,
  PrefixExpression,
  Program,
  ReturnStatement,
  StringLiteral,
  WhileStatement,
} from "../language/parser";
import {
  CompiledFunctionObj,
  IntegerObj,
  NexusObject,
  StringObj,
} from "../language/values";
import { BUILTIN_NAMES } from "../language/builtins";
import { make, Opcode } from "./code";
import { SymbolTable } from "./symbols";

interface EmittedInstruction {
  opcode: Opcode;
  position: number;
}

interface CompilationScope {
  instructions: number[];
  lastInstruction: EmittedInstruction;
  previousInstruction: EmittedInstruction;
}

export interface Bytecode {
  instructions: Uint8Array;
  constants: NexusObject[];
}

export class BytecodeCompiler {
  constants: NexusObject[] = [];
  symbolTable: SymbolTable;
  private scopes: CompilationScope[] = [
    {
      instructions: [],
      lastInstruction: { opcode: Opcode.OpPop, position: -1 },
      previousInstruction: { opcode: Opcode.OpPop, position: -1 },
    },
  ];
  private scopeIndex = 0;

  constructor(symbolTable?: SymbolTable, constants?: NexusObject[]) {
    this.symbolTable = symbolTable ?? new SymbolTable();
    if (!symbolTable) {
      for (let i = 0; i < BUILTIN_NAMES.length; i++) {
        this.symbolTable.defineBuiltin(i, BUILTIN_NAMES[i]!);
      }
    }
    if (constants) {
      this.constants = constants;
    }
  }

  bytecode(): Bytecode {
    return {
      instructions: Uint8Array.from(this.currentInstructions()),
      constants: this.constants,
    };
  }

  compile(node: Node): string | null {
    switch (node.type) {
      case "Program":
        for (const s of (node as Program).statements) {
          const err = this.compile(s);
          if (err) {
            return err;
          }
        }
        return null;

      case "ExpressionStatement": {
        const es = node as ExpressionStatement;
        if (es.expression) {
          const err = this.compile(es.expression);
          if (err) {
            return err;
          }
          this.emit(Opcode.OpPop);
        }
        return null;
      }

      case "BlockStatement":
        for (const s of (node as BlockStatement).statements) {
          const err = this.compile(s);
          if (err) {
            return err;
          }
        }
        return null;

      case "LetStatement": {
        const ls = node as LetStatement;
        const symbol = this.symbolTable.define(ls.name.value);
        if (ls.value?.type === "FunctionLiteral") {
          const err = this.compileFunction(
            ls.value as FunctionLiteral,
            ls.name.value,
          );
          if (err) {
            return err;
          }
        } else if (ls.value) {
          const err = this.compile(ls.value);
          if (err) {
            return err;
          }
        } else {
          this.emit(Opcode.OpNull);
        }
        if (symbol.scope === "GLOBAL") {
          this.emit(Opcode.OpSetGlobal, symbol.index);
        } else {
          this.emit(Opcode.OpSetLocal, symbol.index);
        }
        return null;
      }

      case "ReturnStatement": {
        const rs = node as ReturnStatement;
        if (rs.returnValue) {
          const err = this.compile(rs.returnValue);
          if (err) {
            return err;
          }
          this.emit(Opcode.OpReturnValue);
        } else {
          this.emit(Opcode.OpReturn);
        }
        return null;
      }

      case "WhileStatement":
        return this.compileWhile(node as WhileStatement);

      case "ImportStatement":
        return "import is not supported by the bytecode VM; use the tree-walk engine (nexus.executionEngine=eval)";

      case "IntegerLiteral":
        this.emit(
          Opcode.OpConstant,
          this.addConstant(new IntegerObj((node as IntegerLiteral).value)),
        );
        return null;

      case "StringLiteral":
        this.emit(
          Opcode.OpConstant,
          this.addConstant(new StringObj((node as StringLiteral).value)),
        );
        return null;

      case "BooleanLiteral":
        this.emit(
          (node as BooleanLiteral).value ? Opcode.OpTrue : Opcode.OpFalse,
        );
        return null;

      case "NullLiteral":
        this.emit(Opcode.OpNull);
        return null;

      case "PrefixExpression": {
        const p = node as PrefixExpression;
        if (!p.right) {
          return "prefix missing operand";
        }
        const err = this.compile(p.right);
        if (err) {
          return err;
        }
        if (p.operator === "!") {
          this.emit(Opcode.OpBang);
        } else if (p.operator === "-") {
          this.emit(Opcode.OpMinus);
        } else {
          return `unknown prefix operator ${p.operator}`;
        }
        return null;
      }

      case "InfixExpression":
        return this.compileInfix(node as InfixExpression);

      case "IfExpression":
        return this.compileIf(node as IfExpression);

      case "Identifier":
        return this.compileIdentifier(node as Identifier);

      case "AssignExpression":
        return this.compileAssign(node as AssignExpression);

      case "FunctionLiteral":
        return this.compileFunction(node as FunctionLiteral, "");

      case "CallExpression": {
        const call = node as CallExpression;
        let err = this.compile(call.function);
        if (err) {
          return err;
        }
        for (const a of call.arguments) {
          err = this.compile(a);
          if (err) {
            return err;
          }
        }
        this.emit(Opcode.OpCall, call.arguments.length);
        return null;
      }

      case "ArrayLiteral": {
        const arr = node as ArrayLiteral;
        for (const el of arr.elements) {
          const err = this.compile(el);
          if (err) {
            return err;
          }
        }
        this.emit(Opcode.OpArray, arr.elements.length);
        return null;
      }

      case "HashLiteral": {
        const hash = node as HashLiteral;
        for (const { key, value } of hash.pairs) {
          let err = this.compile(key);
          if (err) {
            return err;
          }
          err = this.compile(value);
          if (err) {
            return err;
          }
        }
        this.emit(Opcode.OpHash, hash.pairs.length * 2);
        return null;
      }

      case "IndexExpression": {
        const ix = node as IndexExpression;
        let err = this.compile(ix.left);
        if (err) {
          return err;
        }
        if (!ix.index) {
          return "index missing";
        }
        err = this.compile(ix.index);
        if (err) {
          return err;
        }
        this.emit(Opcode.OpIndex);
        return null;
      }

      case "MemberExpression": {
        const m = node as MemberExpression;
        const err = this.compile(m.object);
        if (err) {
          return err;
        }
        const fieldIdx = this.addConstant(new StringObj(m.property.value));
        this.emit(Opcode.OpMember, fieldIdx);
        return null;
      }

      case "EnumDeclaration":
      case "StructDeclaration":
      case "EffectDeclaration":
      case "MacroDefinition":
      case "ExternDeclaration":
        return null;

      default:
        return `compilation not supported for ${node.type}`;
    }
  }

  private compileInfix(node: InfixExpression): string | null {
    if (node.operator === "<") {
      if (!node.right) {
        return "missing rhs";
      }
      let err = this.compile(node.right);
      if (err) {
        return err;
      }
      err = this.compile(node.left);
      if (err) {
        return err;
      }
      this.emit(Opcode.OpGreaterThan);
      return null;
    }
    if (node.operator === "<=") {
      if (!node.right) {
        return "missing rhs";
      }
      let err = this.compile(node.right);
      if (err) {
        return err;
      }
      err = this.compile(node.left);
      if (err) {
        return err;
      }
      this.emit(Opcode.OpGreaterEqual);
      return null;
    }

    let err = this.compile(node.left);
    if (err) {
      return err;
    }
    if (!node.right) {
      return "missing rhs";
    }
    err = this.compile(node.right);
    if (err) {
      return err;
    }

    switch (node.operator) {
      case "+":
        this.emit(Opcode.OpAdd);
        break;
      case "-":
        this.emit(Opcode.OpSub);
        break;
      case "*":
        this.emit(Opcode.OpMul);
        break;
      case "/":
        this.emit(Opcode.OpDiv);
        break;
      case "%":
        this.emit(Opcode.OpMod);
        break;
      case "==":
        this.emit(Opcode.OpEqual);
        break;
      case "!=":
        this.emit(Opcode.OpNotEqual);
        break;
      case ">":
        this.emit(Opcode.OpGreaterThan);
        break;
      case ">=":
        this.emit(Opcode.OpGreaterEqual);
        break;
      default:
        return `unknown operator ${node.operator}`;
    }
    return null;
  }

  private compileIf(node: IfExpression): string | null {
    if (!node.condition) {
      return "if missing condition";
    }
    let err = this.compile(node.condition);
    if (err) {
      return err;
    }
    const jumpNotTruthyPos = this.emit(Opcode.OpJumpNotTruthy, 9999);
    err = this.compile(node.consequence);
    if (err) {
      return err;
    }
    if (this.lastInstructionIs(Opcode.OpPop)) {
      this.removeLastPop();
    }
    const jumpPos = this.emit(Opcode.OpJump, 9999);
    this.changeOperand(jumpNotTruthyPos, this.currentInstructions().length);
    if (node.alternative) {
      err = this.compile(node.alternative);
      if (err) {
        return err;
      }
      if (this.lastInstructionIs(Opcode.OpPop)) {
        this.removeLastPop();
      }
    } else {
      this.emit(Opcode.OpNull);
    }
    this.changeOperand(jumpPos, this.currentInstructions().length);
    return null;
  }

  private compileWhile(node: WhileStatement): string | null {
    const loopStart = this.currentInstructions().length;
    if (!node.condition) {
      return "while missing condition";
    }
    let err = this.compile(node.condition);
    if (err) {
      return err;
    }
    const jumpNotTruthyPos = this.emit(Opcode.OpJumpNotTruthy, 9999);
    err = this.compile(node.body);
    if (err) {
      return err;
    }
    if (this.lastInstructionIs(Opcode.OpPop)) {
      this.removeLastPop();
    }
    this.emit(Opcode.OpJump, loopStart);
    this.changeOperand(jumpNotTruthyPos, this.currentInstructions().length);
    this.emit(Opcode.OpNull);
    return null;
  }

  private compileIdentifier(node: Identifier): string | null {
    const symbol = this.symbolTable.resolve(node.value);
    if (!symbol) {
      return `undefined variable ${node.value}`;
    }
    return this.loadSymbol(symbol);
  }

  private loadSymbol(symbol: {
    scope: string;
    index: number;
  }): string | null {
    switch (symbol.scope) {
      case "GLOBAL":
        this.emit(Opcode.OpGetGlobal, symbol.index);
        break;
      case "LOCAL":
        this.emit(Opcode.OpGetLocal, symbol.index);
        break;
      case "BUILTIN":
        this.emit(Opcode.OpGetBuiltin, symbol.index);
        break;
      case "FREE":
        this.emit(Opcode.OpGetFree, symbol.index);
        break;
      case "FUNCTION":
        this.emit(Opcode.OpCurrentClosure);
        break;
      default:
        return `unknown symbol scope ${symbol.scope}`;
    }
    return null;
  }

  private compileAssign(node: AssignExpression): string | null {
    if (node.name.type === "Identifier") {
      const name = (node.name as Identifier).value;
      const symbol = this.symbolTable.resolve(name);
      if (!symbol) {
        return `undefined variable ${name}`;
      }
      if (!node.value) {
        return "assign missing value";
      }
      const err = this.compile(node.value);
      if (err) {
        return err;
      }
      if (symbol.scope === "GLOBAL") {
        this.emit(Opcode.OpSetGlobal, symbol.index);
        this.emit(Opcode.OpGetGlobal, symbol.index);
      } else if (symbol.scope === "LOCAL") {
        this.emit(Opcode.OpSetLocal, symbol.index);
        this.emit(Opcode.OpGetLocal, symbol.index);
      } else {
        return `cannot assign to ${symbol.scope} binding`;
      }
      return null;
    }
    if (node.name.type === "IndexExpression") {
      const ix = node.name as IndexExpression;
      let err = this.compile(ix.left);
      if (err) {
        return err;
      }
      if (!ix.index) {
        return "index missing";
      }
      err = this.compile(ix.index);
      if (err) {
        return err;
      }
      if (!node.value) {
        return "assign missing value";
      }
      err = this.compile(node.value);
      if (err) {
        return err;
      }
      this.emit(Opcode.OpSetIndex);
      return null;
    }
    return "invalid assignment target";
  }

  private compileFunction(
    node: FunctionLiteral,
    name: string,
  ): string | null {
    this.enterScope();
    if (name) {
      this.symbolTable.defineFunctionName(name);
    }
    for (const p of node.parameters) {
      this.symbolTable.define(p.value);
    }
    const err = this.compile(node.body);
    if (err) {
      return err;
    }
    if (this.lastInstructionIs(Opcode.OpPop)) {
      this.replaceLastPopWithReturn();
    }
    if (!this.lastInstructionIs(Opcode.OpReturnValue) &&
      !this.lastInstructionIs(Opcode.OpReturn)) {
      this.emit(Opcode.OpReturn);
    }
    const freeSymbols = [...this.symbolTable.freeSymbols];
    const numLocals = this.symbolTable.numDefinitions;
    const instructions = this.leaveScope();

    for (const s of freeSymbols) {
      const loadErr = this.loadSymbol(s);
      if (loadErr) {
        return loadErr;
      }
    }

    const compiled = new CompiledFunctionObj(
      Uint8Array.from(instructions),
      numLocals,
      node.parameters.length,
    );
    const fnIndex = this.addConstant(compiled);
    this.emit(Opcode.OpClosure, fnIndex, freeSymbols.length);
    return null;
  }

  private addConstant(obj: NexusObject): number {
    this.constants.push(obj);
    return this.constants.length - 1;
  }

  private emit(op: Opcode, ...operands: number[]): number {
    const ins = make(op, ...operands);
    const pos = this.addInstruction(ins);
    this.setLastInstruction(op, pos);
    return pos;
  }

  private addInstruction(ins: number[]): number {
    const pos = this.currentInstructions().length;
    this.scopes[this.scopeIndex]!.instructions.push(...ins);
    return pos;
  }

  private setLastInstruction(op: Opcode, pos: number): void {
    const prev = this.scopes[this.scopeIndex]!.lastInstruction;
    this.scopes[this.scopeIndex]!.previousInstruction = prev;
    this.scopes[this.scopeIndex]!.lastInstruction = { opcode: op, position: pos };
  }

  private lastInstructionIs(op: Opcode): boolean {
    if (this.currentInstructions().length === 0) {
      return false;
    }
    return this.scopes[this.scopeIndex]!.lastInstruction.opcode === op;
  }

  private removeLastPop(): void {
    const last = this.scopes[this.scopeIndex]!.lastInstruction;
    const prev = this.scopes[this.scopeIndex]!.previousInstruction;
    this.scopes[this.scopeIndex]!.instructions =
      this.currentInstructions().slice(0, last.position);
    this.scopes[this.scopeIndex]!.lastInstruction = prev;
  }

  private replaceLastPopWithReturn(): void {
    const lastPos = this.scopes[this.scopeIndex]!.lastInstruction.position;
    this.replaceInstruction(lastPos, make(Opcode.OpReturnValue));
    this.scopes[this.scopeIndex]!.lastInstruction.opcode =
      Opcode.OpReturnValue;
  }

  private replaceInstruction(pos: number, newInstruction: number[]): void {
    const ins = this.scopes[this.scopeIndex]!.instructions;
    for (let i = 0; i < newInstruction.length; i++) {
      ins[pos + i] = newInstruction[i]!;
    }
  }

  private changeOperand(opPos: number, operand: number): void {
    const op = this.currentInstructions()[opPos] as Opcode;
    const newInstruction = make(op, operand);
    this.replaceInstruction(opPos, newInstruction);
  }

  private currentInstructions(): number[] {
    return this.scopes[this.scopeIndex]!.instructions;
  }

  private enterScope(): void {
    this.scopes.push({
      instructions: [],
      lastInstruction: { opcode: Opcode.OpPop, position: -1 },
      previousInstruction: { opcode: Opcode.OpPop, position: -1 },
    });
    this.scopeIndex++;
    this.symbolTable = new SymbolTable(this.symbolTable);
  }

  private leaveScope(): number[] {
    const instructions = this.currentInstructions();
    this.scopes.pop();
    this.scopeIndex--;
    this.symbolTable = this.symbolTable.outer!;
    return instructions;
  }
}

export function compileProgram(program: Program): {
  bytecode?: Bytecode;
  error?: string;
} {
  const c = new BytecodeCompiler();
  const err = c.compile(program);
  if (err) {
    return { error: err };
  }
  return { bytecode: c.bytecode() };
}
