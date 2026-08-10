export { Opcode, make, lookup, readUint16, readUint8 } from "./code";
export { SymbolTable } from "./symbols";
export { BytecodeCompiler, compileProgram } from "./compiler";
export type { Bytecode } from "./compiler";
export { VirtualMachine, runBytecode } from "./vm";
