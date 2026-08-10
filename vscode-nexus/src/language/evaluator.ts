import { Lexer } from "./lexer";
import {
  ArrayLiteral,
  AssignExpression,
  AsyncExpression,
  AwaitExpression,
  BlockStatement,
  BooleanLiteral,
  CallExpression,
  ChanExpression,
  ConstructorExpression,
  EnumDeclaration,
  Expression,
  ExpressionStatement,
  FunctionLiteral,
  HandleExpression,
  HashLiteral,
  Identifier,
  IfExpression,
  ImportStatement,
  IndexExpression,
  InfixExpression,
  IntegerLiteral,
  LetStatement,
  MatchExpression,
  MemberExpression,
  MoveExpression,
  Node,
  Parser,
  PerformExpression,
  PipeExpression,
  PrefixExpression,
  Program,
  RefExpression,
  ReflectExpression,
  RegionExpression,
  ReturnStatement,
  SpawnExpression,
  StringLiteral,
  StructDeclaration,
  WhileStatement,
  Pattern,
} from "./parser";
import {
  ArrayObj,
  BooleanObj,
  BREAK_SIGNAL,
  CONTINUE_SIGNAL,
  BreakSignal,
  ContinueSignal,
  ErrorObj,
  HashObj,
  IntegerObj,
  NexusObject,
  NullObj,
  ReturnValue,
  StringObj,
  FALSE_OBJ,
  NULL_OBJ,
  TRUE_OBJ,
  isError,
  isTruthy,
  nativeBool,
  newError,
} from "./values";
import { BuiltinObj, installCoreBuiltins } from "./builtins";
import {
  ChannelObj,
  EnumValueObj,
  ExternFnObj,
  NexusRuntime,
  PromiseObj,
  RefObj,
  isChannel,
  isEnumValue,
  isPromiseObj,
  isRefObj,
} from "./runtime";
import { EffectRuntime } from "./effects";
import { RegionStack } from "./regions";
import { getMemoryEngine } from "../runtime/memory";
import { FfiLibraryObj } from "./ffi";
import {
  resolveModulePath,
  readModuleSource,
  findStdlibDir,
  MODULES_DIRNAME,
} from "../runtime/modules";
import { lowerSyntax } from "./syntax";
import * as path from "path";

export type ObjectType = string;

export type {
  NexusObject,
};
export {
  IntegerObj,
  StringObj,
  BooleanObj,
  NullObj,
  ReturnValue,
  BreakSignal,
  ContinueSignal,
  ErrorObj,
  ArrayObj,
  HashObj,
  BuiltinObj,
};
export type BuiltinFunction = (...args: NexusObject[]) => NexusObject;

export class FunctionObj implements NexusObject {
  readonly type = "FUNCTION" as const;
  constructor(
    readonly parameters: Identifier[],
    readonly body: BlockStatement,
    readonly env: Environment,
    readonly isAsync: boolean = false,
  ) {}
  inspect(): string {
    const params = this.parameters.map((p) => p.value).join(", ");
    return `${this.isAsync ? "async " : ""}fn(${params}) { ... }`;
  }
}

export class Environment {
  private readonly store = new Map<string, NexusObject>();

  constructor(private readonly outer: Environment | null = null) {}

  get(name: string): NexusObject | undefined {
    const value = this.store.get(name);
    if (value !== undefined) {
      return value;
    }
    return this.outer?.get(name);
  }

  set(name: string, value: NexusObject): NexusObject {
    this.store.set(name, value);
    return value;
  }
}

export interface EvalResult {
  value: NexusObject;
  output: string[];
}

/**
 * Async tree-walking interpreter with effects, regions, channels, and FFI hooks.
 */
export class Evaluator {
  private readonly output: string[] = [];
  readonly runtime = new NexusRuntime();
  readonly effects = new EffectRuntime();
  readonly regions = new RegionStack();
  private readonly enums = new Map<string, Set<string>>();
  private readonly structs = new Map<string, string[]>();
  private program: Program | null = null;
  private readonly loadedModules = new Set<string>();
  rootDir = process.cwd();
  modulesDir = "";
  stdlibDir = "";

  constructor() {
    this.modulesDir = path.join(this.rootDir, MODULES_DIRNAME);
    this.stdlibDir = findStdlibDir(this.rootDir);
  }

  configureModules(options: {
    rootDir?: string;
    modulesDir?: string;
    stdlibDir?: string;
  }): void {
    if (options.rootDir) {
      this.rootDir = options.rootDir;
    }
    this.modulesDir =
      options.modulesDir ?? path.join(this.rootDir, MODULES_DIRNAME);
    this.stdlibDir =
      options.stdlibDir !== undefined
        ? options.stdlibDir
        : findStdlibDir(this.rootDir);
  }

  async eval(
    node: Node,
    env: Environment = new Environment(),
  ): Promise<EvalResult> {
    if (node.type === "Program") {
      this.program = node as Program;
      this.collectDecls(this.program);
    }
    this.ensureBuiltins(env);
    const value = await this.evalNode(node, env);
    await this.runtime.drain();
    return { value, output: [...this.output] };
  }

  writeOutput(line: string): void {
    this.output.push(line);
  }

  private ensureBuiltins(env: Environment): void {
    if (env.get("puts")) {
      return;
    }
    installCoreBuiltins(env, {
      writeOutput: (line) => this.writeOutput(line),
      applyFunction: (fn, args) => this.applyFunctionSync(fn, args),
    });
  }

  /** Synchronous apply used by map/filter builtins and the HTTP host. */
  applyFunctionSync(fn: NexusObject, args: NexusObject[]): NexusObject {
    if (fn instanceof BuiltinObj) {
      return fn.fn(...args);
    }
    if (fn instanceof FunctionObj) {
      if (args.length !== fn.parameters.length) {
        return newError(
          `wrong number of arguments: got=${args.length}, want=${fn.parameters.length}`,
        );
      }
      const extended = new Environment(fn.env);
      for (let i = 0; i < fn.parameters.length; i++) {
        extended.set(fn.parameters[i]!.value, args[i]!);
      }
      // Use a minimal sync walk for function bodies inside map/filter.
      return syncEvalNode(fn.body, extended, this);
    }
    return newError(`not a function: ${fn.type}`);
  }

  private collectDecls(program: Program): void {
    for (const stmt of program.statements) {
      if (stmt.type === "EnumDeclaration") {
        const d = stmt as EnumDeclaration;
        this.enums.set(
          d.name.value,
          new Set(d.variants.map((v) => v.name.value)),
        );
      }
      if (stmt.type === "StructDeclaration") {
        const d = stmt as StructDeclaration;
        this.structs.set(
          d.name.value,
          d.fields.map((f) => f.name.value),
        );
      }
    }
  }

  private async evalNode(node: Node, env: Environment): Promise<NexusObject> {
    switch (node.type) {
      case "Program":
        return this.evalProgram(node as Program, env);
      case "BlockStatement":
        return this.evalBlock(node as BlockStatement, env);
      case "ExpressionStatement": {
        const es = node as ExpressionStatement;
        return es.expression ? this.evalNode(es.expression, env) : NULL_OBJ;
      }
      case "ReturnStatement": {
        const rs = node as ReturnStatement;
        const val = rs.returnValue
          ? await this.evalNode(rs.returnValue, env)
          : NULL_OBJ;
        if (isError(val)) {
          return val;
        }
        return new ReturnValue(val);
      }
      case "BreakStatement":
        return BREAK_SIGNAL;
      case "ContinueStatement":
        return CONTINUE_SIGNAL;
      case "LetStatement": {
        const ls = node as LetStatement;
        const val = ls.value ? await this.evalNode(ls.value, env) : NULL_OBJ;
        if (isError(val)) {
          return val;
        }
        // Track heap for non-primitives
        if (
          !(val instanceof IntegerObj) &&
          !(val instanceof BooleanObj) &&
          !(val instanceof StringObj) &&
          !(val instanceof NullObj)
        ) {
          const id = getMemoryEngine().alloc(val);
          getMemoryEngine().retain(id);
          const arena = this.regions.current();
          if (arena) {
            arena.alloc(ls.name.value, () => getMemoryEngine().release(id));
          }
        }
        env.set(ls.name.value, val);
        return val;
      }
      case "EnumDeclaration":
      case "StructDeclaration":
      case "EffectDeclaration":
      case "MacroDefinition":
      case "ExternDeclaration":
        return NULL_OBJ;
      case "ImportStatement":
        return this.evalImport(node as ImportStatement, env);
      case "WhileStatement":
        return this.evalWhile(node as WhileStatement, env);
      case "IntegerLiteral":
        return new IntegerObj((node as IntegerLiteral).value);
      case "StringLiteral":
        return new StringObj((node as StringLiteral).value);
      case "BooleanLiteral":
        return nativeBool((node as BooleanLiteral).value);
      case "NullLiteral":
        return NULL_OBJ;
      case "PrefixExpression":
        return this.evalPrefix(node as PrefixExpression, env);
      case "InfixExpression":
        return this.evalInfix(node as InfixExpression, env);
      case "IfExpression":
        return this.evalIf(node as IfExpression, env);
      case "Identifier":
        return this.evalIdentifier(node as Identifier, env);
      case "FunctionLiteral": {
        const fl = node as FunctionLiteral;
        return new FunctionObj(fl.parameters, fl.body, env, fl.isAsync);
      }
      case "CallExpression":
        return this.evalCall(node as CallExpression, env);
      case "ConstructorExpression":
        return this.evalConstructor(node as ConstructorExpression, env);
      case "ArrayLiteral":
        return this.evalArray(node as ArrayLiteral, env);
      case "HashLiteral":
        return this.evalHash(node as HashLiteral, env);
      case "IndexExpression":
        return this.evalIndex(node as IndexExpression, env);
      case "MemberExpression":
        return this.evalMember(node as MemberExpression, env);
      case "AssignExpression":
        return this.evalAssign(node as AssignExpression, env);
      case "PipeExpression":
        return this.evalPipe(node as PipeExpression, env);
      case "MatchExpression":
        return this.evalMatch(node as MatchExpression, env);
      case "RefExpression":
        return this.evalRef(node as RefExpression, env);
      case "MoveExpression": {
        const m = node as MoveExpression;
        return m.value ? this.evalNode(m.value, env) : NULL_OBJ;
      }
      case "AsyncExpression":
        return this.evalAsync(node as AsyncExpression, env);
      case "AwaitExpression":
        return this.evalAwait(node as AwaitExpression, env);
      case "SpawnExpression":
        return this.evalSpawn(node as SpawnExpression, env);
      case "ChanExpression":
        return this.evalChan(node as ChanExpression, env);
      case "PerformExpression":
        return this.evalPerform(node as PerformExpression, env);
      case "HandleExpression":
        return this.evalHandle(node as HandleExpression, env);
      case "RegionExpression":
        return this.evalRegion(node as RegionExpression, env);
      case "ReflectExpression":
        return this.evalReflect(node as ReflectExpression);
      default:
        return newError(`unknown node type: ${node.type}`);
    }
  }

  private async evalProgram(
    program: Program,
    env: Environment,
  ): Promise<NexusObject> {
    let result: NexusObject = NULL_OBJ;
    for (const statement of program.statements) {
      result = await this.evalNode(statement, env);
      if (result instanceof ReturnValue) {
        return result.value;
      }
      if (result instanceof ErrorObj) {
        return result;
      }
    }
    return result;
  }

  private async evalBlock(
    block: BlockStatement,
    env: Environment,
  ): Promise<NexusObject> {
    let result: NexusObject = NULL_OBJ;
    for (const statement of block.statements) {
      result = await this.evalNode(statement, env);
      if (
        result instanceof ReturnValue ||
        result instanceof ErrorObj ||
        result instanceof BreakSignal ||
        result instanceof ContinueSignal
      ) {
        return result;
      }
    }
    return result;
  }

  private async evalPrefix(
    node: PrefixExpression,
    env: Environment,
  ): Promise<NexusObject> {
    if (!node.right) {
      return newError("prefix expression missing operand");
    }
    const right = await this.evalNode(node.right, env);
    if (isError(right)) {
      return right;
    }
    switch (node.operator) {
      case "!":
        return nativeBool(!isTruthy(right));
      case "-":
        if (!(right instanceof IntegerObj)) {
          return newError(`unknown operator: -${right.type}`);
        }
        return new IntegerObj(-right.value);
      default:
        return newError(`unknown operator: ${node.operator}`);
    }
  }

  private async evalInfix(
    node: InfixExpression,
    env: Environment,
  ): Promise<NexusObject> {
    const left = await this.evalNode(node.left, env);
    if (isError(left)) {
      return left;
    }
    if (!node.right) {
      return newError("infix expression missing right operand");
    }

    // Short-circuit logical operators (&& / || / and / or)
    if (
      node.operator === "&&" ||
      node.operator === "AND" ||
      node.operator === "and"
    ) {
      if (!isTruthy(left)) {
        return left;
      }
      return this.evalNode(node.right, env);
    }
    if (
      node.operator === "||" ||
      node.operator === "OR" ||
      node.operator === "or"
    ) {
      if (isTruthy(left)) {
        return left;
      }
      return this.evalNode(node.right, env);
    }

    const right = await this.evalNode(node.right, env);
    if (isError(right)) {
      return right;
    }

    if (left instanceof IntegerObj && right instanceof IntegerObj) {
      return this.evalIntegerInfix(node.operator, left, right);
    }
    if (left instanceof StringObj && right instanceof StringObj) {
      if (node.operator === "+") {
        return new StringObj(left.value + right.value);
      }
      if (node.operator === "==") {
        return nativeBool(left.value === right.value);
      }
      if (node.operator === "!=") {
        return nativeBool(left.value !== right.value);
      }
    }
    if (node.operator === "==") {
      return nativeBool(left === right);
    }
    if (node.operator === "!=") {
      return nativeBool(left !== right);
    }
    return newError(
      `unknown operator: ${left.type} ${node.operator} ${right.type}`,
    );
  }

  private evalIntegerInfix(
    operator: string,
    left: IntegerObj,
    right: IntegerObj,
  ): NexusObject {
    switch (operator) {
      case "+":
        return new IntegerObj(left.value + right.value);
      case "-":
        return new IntegerObj(left.value - right.value);
      case "*":
        return new IntegerObj(left.value * right.value);
      case "/":
        if (right.value === 0) {
          return newError("division by zero");
        }
        return new IntegerObj(Math.trunc(left.value / right.value));
      case "%":
        if (right.value === 0) {
          return newError("modulo by zero");
        }
        return new IntegerObj(left.value % right.value);
      case "<":
        return nativeBool(left.value < right.value);
      case ">":
        return nativeBool(left.value > right.value);
      case "<=":
        return nativeBool(left.value <= right.value);
      case ">=":
        return nativeBool(left.value >= right.value);
      case "==":
        return nativeBool(left.value === right.value);
      case "!=":
        return nativeBool(left.value !== right.value);
      default:
        return newError(`unknown operator: INTEGER ${operator} INTEGER`);
    }
  }

  private async evalIf(
    node: IfExpression,
    env: Environment,
  ): Promise<NexusObject> {
    if (!node.condition) {
      return newError("if expression missing condition");
    }
    const condition = await this.evalNode(node.condition, env);
    if (isError(condition)) {
      return condition;
    }
    if (isTruthy(condition)) {
      return this.evalNode(node.consequence, env);
    }
    if (node.alternative) {
      return this.evalNode(node.alternative, env);
    }
    return NULL_OBJ;
  }

  private evalIdentifier(node: Identifier, env: Environment): NexusObject {
    const value = env.get(node.value);
    if (value) {
      return value;
    }
    const b = this.runtimeBuiltins()[node.value];
    if (b) {
      return b;
    }
    return newError(`identifier not found: ${node.value}`);
  }

  private async evalCall(
    node: CallExpression,
    env: Environment,
  ): Promise<NexusObject> {
    const fn = await this.evalNode(node.function, env);
    if (isError(fn)) {
      return fn;
    }
    const args: NexusObject[] = [];
    for (const arg of node.arguments) {
      const evaluated = await this.evalNode(arg, env);
      if (isError(evaluated)) {
        return evaluated;
      }
      args.push(evaluated);
    }
    return this.applyFunction(fn, args);
  }

  /** Apply a user function (used by HTTP host and call expressions). */
  async applyFunction(
    fn: NexusObject,
    args: NexusObject[],
  ): Promise<NexusObject> {
    if (fn instanceof FunctionObj) {
      if (args.length !== fn.parameters.length) {
        return newError(
          `wrong number of arguments: got=${args.length}, want=${fn.parameters.length}`,
        );
      }
      const extended = new Environment(fn.env);
      for (let i = 0; i < fn.parameters.length; i++) {
        extended.set(fn.parameters[i]!.value, args[i]!);
      }
      const result = await this.evalNode(fn.body, extended);
      if (result instanceof ReturnValue) {
        return result.value;
      }
      return result;
    }
    if (fn instanceof BuiltinObj) {
      return fn.fn(...args);
    }
    if (fn instanceof ExternFnObj) {
      return fn.call(...args);
    }
    return newError(`not a function: ${fn.type}`);
  }

  private async evalConstructor(
    node: ConstructorExpression,
    env: Environment,
  ): Promise<NexusObject> {
    const fields: NexusObject[] = [];
    for (const arg of node.arguments) {
      const v = await this.evalNode(arg, env);
      if (isError(v)) {
        return v;
      }
      fields.push(v);
    }

    const structFields = this.structs.get(node.variant.value);
    if (structFields && !node.enumName) {
      if (fields.length !== structFields.length) {
        return newError(
          `${node.variant.value} expects ${structFields.length} fields, got ${fields.length}`,
        );
      }
      const hash = new HashObj();
      hash.setString("__struct", new StringObj(node.variant.value));
      for (let i = 0; i < structFields.length; i++) {
        hash.setString(structFields[i]!, fields[i]!);
      }
      return hash;
    }

    return new EnumValueObj(node.enumName, node.variant.value, fields);
  }

  private async evalImport(
    node: ImportStatement,
    env: Environment,
  ): Promise<NexusObject> {
    const fromDirObj = env.get("__dir__");
    const fromDir =
      fromDirObj instanceof StringObj ? fromDirObj.value : this.rootDir;
    const resolved = resolveModulePath(node.path, {
      rootDir: this.rootDir,
      modulesDir: this.modulesDir,
      stdlibDir: this.stdlibDir,
      fromDir,
    });
    if (!resolved) {
      return newError(`import failed: cannot resolve ${node.path}`);
    }
    if (this.loadedModules.has(resolved)) {
      return NULL_OBJ;
    }
    this.loadedModules.add(resolved);

    let source: string;
    try {
      source = readModuleSource(resolved);
    } catch (err) {
      this.loadedModules.delete(resolved);
      return newError(
        `import failed: ${err instanceof Error ? err.message : String(err)}`,
      );
    }

    source = lowerSyntax(source);

    const prevFile = env.get("__file__");
    const prevDir = env.get("__dir__");
    env.set("__file__", new StringObj(resolved));
    env.set("__dir__", new StringObj(path.dirname(resolved)));

    const lexer = new Lexer(source);
    const parser = new Parser(lexer);
    const program = parser.parseProgram();
    if (parser.getErrors().length > 0) {
      return newError(
        `parse error in ${resolved}: ${parser.getErrors().join("; ")}`,
      );
    }
    this.collectDecls(program);
    const result = await this.evalNode(program, env);

    if (prevFile) {
      env.set("__file__", prevFile);
    }
    if (prevDir) {
      env.set("__dir__", prevDir);
    }
    return result;
  }

  private async evalWhile(
    node: WhileStatement,
    env: Environment,
  ): Promise<NexusObject> {
    let result: NexusObject = NULL_OBJ;
    while (true) {
      if (!node.condition) {
        return newError("while missing condition");
      }
      const cond = await this.evalNode(node.condition, env);
      if (isError(cond)) {
        return cond;
      }
      if (!isTruthy(cond)) {
        break;
      }
      result = await this.evalNode(node.body, env);
      if (result instanceof ReturnValue || isError(result)) {
        return result;
      }
      if (result instanceof BreakSignal) {
        return NULL_OBJ;
      }
      if (result instanceof ContinueSignal) {
        continue;
      }
    }
    return result;
  }

  private async evalArray(
    node: ArrayLiteral,
    env: Environment,
  ): Promise<NexusObject> {
    const elements: NexusObject[] = [];
    for (const el of node.elements) {
      const v = await this.evalNode(el, env);
      if (isError(v)) {
        return v;
      }
      elements.push(v);
    }
    return new ArrayObj(elements);
  }

  private async evalHash(
    node: HashLiteral,
    env: Environment,
  ): Promise<NexusObject> {
    const hash = new HashObj();
    for (const { key, value } of node.pairs) {
      const k = await this.evalNode(key, env);
      if (isError(k)) {
        return k;
      }
      const v = await this.evalNode(value, env);
      if (isError(v)) {
        return v;
      }
      hash.set(k, v);
    }
    return hash;
  }

  private async evalIndex(
    node: IndexExpression,
    env: Environment,
  ): Promise<NexusObject> {
    const left = await this.evalNode(node.left, env);
    if (isError(left)) {
      return left;
    }
    if (!node.index) {
      return newError("index missing");
    }
    const index = await this.evalNode(node.index, env);
    if (isError(index)) {
      return index;
    }
    if (left instanceof ArrayObj && index instanceof IntegerObj) {
      return left.elements[index.value] ?? NULL_OBJ;
    }
    if (left instanceof HashObj) {
      return left.get(index);
    }
    if (left instanceof StringObj && index instanceof IntegerObj) {
      if (index.value < 0 || index.value >= left.value.length) {
        return NULL_OBJ;
      }
      return new StringObj(left.value[index.value]!);
    }
    return newError(`index operator not supported: ${left.type}`);
  }

  private async evalMember(
    node: MemberExpression,
    env: Environment,
  ): Promise<NexusObject> {
    const obj = await this.evalNode(node.object, env);
    if (isError(obj)) {
      return obj;
    }
    if (obj instanceof HashObj) {
      return obj.getString(node.property.value);
    }
    return newError(`member access on non-hash: ${obj.type}`);
  }

  private async evalAssign(
    node: AssignExpression,
    env: Environment,
  ): Promise<NexusObject> {
    if (!node.value) {
      return newError("assign missing value");
    }
    const val = await this.evalNode(node.value, env);
    if (isError(val)) {
      return val;
    }
    if (node.name.type === "Identifier") {
      env.set((node.name as Identifier).value, val);
      return val;
    }
    if (node.name.type === "IndexExpression") {
      const ix = node.name as IndexExpression;
      const left = await this.evalNode(ix.left, env);
      if (isError(left)) {
        return left;
      }
      if (!ix.index) {
        return newError("index missing");
      }
      const index = await this.evalNode(ix.index, env);
      if (isError(index)) {
        return index;
      }
      if (left instanceof ArrayObj && index instanceof IntegerObj) {
        if (index.value < 0 || index.value >= left.elements.length) {
          return newError("array index out of bounds");
        }
        left.elements[index.value] = val;
        return val;
      }
      if (left instanceof HashObj) {
        left.set(index, val);
        return val;
      }
      return newError(`index assignment not supported on ${left.type}`);
    }
    return newError("invalid assignment target");
  }

  private async evalPipe(
    node: PipeExpression,
    env: Environment,
  ): Promise<NexusObject> {
    const left = await this.evalNode(node.left, env);
    if (isError(left)) {
      return left;
    }
    if (!node.right) {
      return newError("pipe missing right-hand side");
    }
    if (node.right.type === "CallExpression") {
      const call = node.right as CallExpression;
      const fn = await this.evalNode(call.function, env);
      if (isError(fn)) {
        return fn;
      }
      const args: NexusObject[] = [left];
      for (const arg of call.arguments) {
        const evaluated = await this.evalNode(arg, env);
        if (isError(evaluated)) {
          return evaluated;
        }
        args.push(evaluated);
      }
      return this.applyFunction(fn, args);
    }
    const fn = await this.evalNode(node.right, env);
    if (isError(fn)) {
      return fn;
    }
    return this.applyFunction(fn, [left]);
  }

  private async evalMatch(
    node: MatchExpression,
    env: Environment,
  ): Promise<NexusObject> {
    if (!node.scrutinee) {
      return newError("match missing scrutinee");
    }
    const value = await this.evalNode(node.scrutinee, env);
    if (isError(value)) {
      return value;
    }

    for (const arm of node.arms) {
      const matched = this.matchPattern(arm.pattern, value, env);
      if (matched) {
        return this.evalNode(arm.body, matched);
      }
    }
    return newError("non-exhaustive match at runtime");
  }

  private matchPattern(
    pattern: Pattern,
    value: NexusObject,
    env: Environment,
  ): Environment | null {
    switch (pattern.kind) {
      case "wildcard":
        return env;
      case "ident": {
        const extended = new Environment(env);
        extended.set(pattern.name.value, value);
        return extended;
      }
      case "literal": {
        if (
          pattern.value.type === "IntegerLiteral" &&
          value instanceof IntegerObj &&
          pattern.value.value === value.value
        ) {
          return env;
        }
        if (
          pattern.value.type === "StringLiteral" &&
          value instanceof StringObj &&
          pattern.value.value === value.value
        ) {
          return env;
        }
        if (
          pattern.value.type === "BooleanLiteral" &&
          value instanceof BooleanObj &&
          pattern.value.value === value.value
        ) {
          return env;
        }
        return null;
      }
      case "variant": {
        if (!isEnumValue(value)) {
          return null;
        }
        if (value.variant !== pattern.variant.value) {
          return null;
        }
        if (
          pattern.enumName &&
          value.enumName &&
          pattern.enumName !== value.enumName
        ) {
          return null;
        }
        if (pattern.fields.length !== value.fields.length) {
          return null;
        }
        let extended: Environment = env;
        for (let i = 0; i < pattern.fields.length; i++) {
          const nested = this.matchPattern(
            pattern.fields[i]!,
            value.fields[i]!,
            extended,
          );
          if (!nested) {
            return null;
          }
          extended = nested;
        }
        return extended;
      }
      default:
        return null;
    }
  }

  private async evalRef(
    node: RefExpression,
    env: Environment,
  ): Promise<NexusObject> {
    if (!node.value) {
      return newError("ref missing operand");
    }
    const val = await this.evalNode(node.value, env);
    if (isError(val)) {
      return val;
    }
    return new RefObj(val, node.mutable);
  }

  private evalAsync(
    node: AsyncExpression,
    env: Environment,
  ): NexusObject {
    const promise = (async () => {
      if (node.body.type === "BlockStatement") {
        return this.evalNode(node.body as BlockStatement, env);
      }
      return this.evalNode(node.body as Expression, env);
    })();
    return new PromiseObj(promise);
  }

  private async evalAwait(
    node: AwaitExpression,
    env: Environment,
  ): Promise<NexusObject> {
    if (!node.argument) {
      return newError("await missing argument");
    }
    const val = await this.evalNode(node.argument, env);
    if (isError(val)) {
      return val;
    }
    if (isPromiseObj(val)) {
      return val.awaitValue();
    }
    return val;
  }

  private async evalSpawn(
    node: SpawnExpression,
    env: Environment,
  ): Promise<NexusObject> {
    if (!node.argument) {
      return newError("spawn missing argument");
    }
    const arg = node.argument;
    const handle = this.runtime.spawn(async () => {
      const fn = await this.evalNode(arg, env);
      if (fn instanceof FunctionObj) {
        await this.applyFunction(fn, []);
      } else if (isPromiseObj(fn)) {
        await fn.awaitValue();
      }
    });
    return new IntegerObj(handle.id);
  }

  private async evalChan(
    node: ChanExpression,
    env: Environment,
  ): Promise<NexusObject> {
    let capacity = 0;
    if (node.capacity) {
      const c = await this.evalNode(node.capacity, env);
      if (c instanceof IntegerObj) {
        capacity = c.value;
      }
    }
    return this.runtime.createChannel(capacity);
  }

  private async evalPerform(
    node: PerformExpression,
    env: Environment,
  ): Promise<NexusObject> {
    const args: NexusObject[] = [];
    for (const a of node.arguments) {
      const v = await this.evalNode(a, env);
      if (isError(v)) {
        return v;
      }
      args.push(v);
    }
    try {
      const result = this.effects.perform(
        node.effectName,
        node.operation.value,
        args,
      );
      return (result as NexusObject) ?? NULL_OBJ;
    } catch (err) {
      return newError(err instanceof Error ? err.message : String(err));
    }
  }

  private async evalHandle(
    node: HandleExpression,
    env: Environment,
  ): Promise<NexusObject> {
    const handlers = new Map<
      string,
      {
        parameters: string[];
        invoke: (
          args: unknown[],
          cont: { resume: (v: unknown) => unknown },
        ) => unknown;
      }
    >();

    for (const arm of node.handlers) {
      handlers.set(arm.operation.value, {
        parameters: arm.parameters.map((p) => p.value),
        invoke: (args, cont) => {
          const extended = new Environment(env);
          for (let i = 0; i < arm.parameters.length; i++) {
            extended.set(
              arm.parameters[i]!.value,
              (args[i] as NexusObject) ?? NULL_OBJ,
            );
          }
          // resume builtin for handlers
          extended.set(
            "resume",
            new BuiltinObj((...resumeArgs) => {
              const v = resumeArgs[0] ?? NULL_OBJ;
              cont.resume(v);
              return v;
            }),
          );
          // Synchronous bridge: kick async eval without awaiting (handlers
          // are expected to call resume).
          let result: NexusObject = NULL_OBJ;
          void this.evalNode(arm.body, extended).then((r) => {
            result = r;
            if (!(extended.get("resume") as BuiltinObj)) {
              cont.resume(r);
            }
          });
          return result;
        },
      });
    }

    this.effects.push({
      effectName: node.effectName.value,
      handlers,
    });
    try {
      return await this.evalNode(node.body, env);
    } finally {
      this.effects.pop();
    }
  }

  private async evalRegion(
    node: RegionExpression,
    env: Environment,
  ): Promise<NexusObject> {
    this.regions.enter();
    try {
      return await this.evalNode(node.body, env);
    } finally {
      this.regions.exit();
    }
  }

  private evalReflect(node: ReflectExpression): NexusObject {
    const name = node.target.value;
    const fields = this.structs.get(name);
    if (fields) {
      return new StringObj(
        JSON.stringify({ kind: "struct", name, fields }),
      );
    }
    const variants = this.enums.get(name);
    if (variants) {
      return new StringObj(
        JSON.stringify({
          kind: "enum",
          name,
          variants: [...variants],
        }),
      );
    }
    return newError(`reflect: unknown type ${name}`);
  }

  private runtimeBuiltins(): Record<string, BuiltinObj> {
    return {
      send: new BuiltinObj((...args) => {
        if (args.length !== 2 || !isChannel(args[0]!)) {
          return newError("send: want channel, value");
        }
        return new PromiseObj(
          (args[0] as ChannelObj).send(args[1]!).then(() => NULL_OBJ),
        );
      }),
      recv: new BuiltinObj((...args) => {
        if (args.length !== 1 || !isChannel(args[0]!)) {
          return newError("recv: want channel");
        }
        return new PromiseObj((args[0] as ChannelObj).recv());
      }),
      deref: new BuiltinObj((...args) => {
        if (args.length !== 1 || !isRefObj(args[0]!)) {
          return newError("deref: want ref");
        }
        return (args[0] as RefObj).target;
      }),
      reflect: new BuiltinObj((...args) => {
        if (args[0] instanceof StringObj) {
          return this.evalReflect({
            kind: "expression",
            type: "ReflectExpression",
            token: {
              type: "REFLECT",
              literal: "reflect",
              line: 0,
              column: 0,
            },
            target: {
              kind: "expression",
              type: "Identifier",
              token: {
                type: "IDENT",
                literal: args[0].value,
                line: 0,
                column: 0,
              },
              value: args[0].value,
              tokenLiteral: () => args[0]!.inspect(),
            },
            tokenLiteral: () => "reflect",
          });
        }
        return newError("reflect: want type name string");
      }),
    };
  }
}

/** Sync walk for map/filter callbacks (subset of statements/expressions). */
function syncEvalNode(
  node: Node,
  env: Environment,
  host: Evaluator,
): NexusObject {
  switch (node.type) {
    case "BlockStatement": {
      let result: NexusObject = NULL_OBJ;
      for (const s of (node as BlockStatement).statements) {
        result = syncEvalNode(s, env, host);
        if (result instanceof ReturnValue) {
          return result.value;
        }
        if (isError(result)) {
          return result;
        }
      }
      return result;
    }
    case "ExpressionStatement": {
      const es = node as ExpressionStatement;
      return es.expression ? syncEvalNode(es.expression, env, host) : NULL_OBJ;
    }
    case "ReturnStatement": {
      const rs = node as ReturnStatement;
      const val = rs.returnValue
        ? syncEvalNode(rs.returnValue, env, host)
        : NULL_OBJ;
      return isError(val) ? val : new ReturnValue(val);
    }
    case "IntegerLiteral":
      return new IntegerObj((node as IntegerLiteral).value);
    case "StringLiteral":
      return new StringObj((node as StringLiteral).value);
    case "BooleanLiteral":
      return nativeBool((node as BooleanLiteral).value);
    case "NullLiteral":
      return NULL_OBJ;
    case "Identifier": {
      const name = (node as Identifier).value;
      return env.get(name) ?? newError(`identifier not found: ${name}`);
    }
    case "InfixExpression": {
      const inf = node as InfixExpression;
      const left = syncEvalNode(inf.left, env, host);
      if (!inf.right) {
        return newError("missing rhs");
      }
      if (
        inf.operator === "&&" ||
        inf.operator === "AND" ||
        inf.operator === "and"
      ) {
        if (!isTruthy(left)) {
          return left;
        }
        return syncEvalNode(inf.right, env, host);
      }
      if (
        inf.operator === "||" ||
        inf.operator === "OR" ||
        inf.operator === "or"
      ) {
        if (isTruthy(left)) {
          return left;
        }
        return syncEvalNode(inf.right, env, host);
      }
      const right = syncEvalNode(inf.right, env, host);
      if (left instanceof IntegerObj && right instanceof IntegerObj) {
        switch (inf.operator) {
          case "+":
            return new IntegerObj(left.value + right.value);
          case "-":
            return new IntegerObj(left.value - right.value);
          case "*":
            return new IntegerObj(left.value * right.value);
          case "/":
            return right.value === 0
              ? newError("division by zero")
              : new IntegerObj(Math.trunc(left.value / right.value));
          case "==":
            return nativeBool(left.value === right.value);
          case "!=":
            return nativeBool(left.value !== right.value);
          case "<":
            return nativeBool(left.value < right.value);
          case ">":
            return nativeBool(left.value > right.value);
          default:
            return newError(`unknown op ${inf.operator}`);
        }
      }
      if (
        left instanceof StringObj &&
        right instanceof StringObj &&
        inf.operator === "+"
      ) {
        return new StringObj(left.value + right.value);
      }
      if (inf.operator === "==") {
        return nativeBool(left === right);
      }
      if (inf.operator === "!=") {
        return nativeBool(left !== right);
      }
      return newError("type mismatch");
    }
    case "CallExpression": {
      const call = node as CallExpression;
      const fn = syncEvalNode(call.function, env, host);
      const args = call.arguments.map((a) => syncEvalNode(a, env, host));
      return host.applyFunctionSync(fn, args);
    }
    case "PrefixExpression": {
      const p = node as PrefixExpression;
      if (!p.right) {
        return newError("missing operand");
      }
      const right = syncEvalNode(p.right, env, host);
      if (p.operator === "!") {
        return nativeBool(!isTruthy(right));
      }
      if (p.operator === "-" && right instanceof IntegerObj) {
        return new IntegerObj(-right.value);
      }
      return newError(`unknown prefix ${p.operator}`);
    }
    default:
      return newError(`sync eval unsupported for ${node.type}`);
  }
}

/** Synchronous convenience for simple (non-async) scripts. */
export function runSource(source: string): EvalResult {
  const lexer = new Lexer(source);
  const parser = new Parser(lexer);
  let program = parser.parseProgram();
  if (parser.getErrors().length > 0) {
    return {
      value: new ErrorObj(`parse errors:\n  ${parser.getErrors().join("\n  ")}`),
      output: [],
    };
  }
  const { MacroExpander } = require("./macro") as typeof import("./macro");
  const expanded = new MacroExpander().expand(program);
  if (expanded.errors.length > 0) {
    return {
      value: new ErrorObj(`macro errors:\n  ${expanded.errors.join("\n  ")}`),
      output: [],
    };
  }
  program = expanded.program;
  return syncEvalProgram(program, new Environment());
}

function syncEvalProgram(program: Program, env: Environment): EvalResult {
  const output: string[] = [];
  const builtins: Record<string, BuiltinObj> = {
    puts: new BuiltinObj((...args) => {
      for (const a of args) {
        output.push(a.inspect());
      }
      return NULL_OBJ;
    }),
    len: new BuiltinObj((...args) => {
      if (args.length === 1 && args[0] instanceof StringObj) {
        return new IntegerObj(args[0].value.length);
      }
      return newError("len: want string");
    }),
  };

  const evalExpr = (node: Node, localEnv: Environment): NexusObject => {
    switch (node.type) {
      case "Program": {
        let result: NexusObject = NULL_OBJ;
        for (const s of (node as Program).statements) {
          result = evalExpr(s, localEnv);
          if (result instanceof ReturnValue) {
            return result.value;
          }
          if (result instanceof ErrorObj) {
            return result;
          }
        }
        return result;
      }
      case "BlockStatement": {
        let result: NexusObject = NULL_OBJ;
        for (const s of (node as BlockStatement).statements) {
          result = evalExpr(s, localEnv);
          if (result instanceof ReturnValue || result instanceof ErrorObj) {
            return result;
          }
        }
        return result;
      }
      case "ExpressionStatement": {
        const es = node as ExpressionStatement;
        return es.expression ? evalExpr(es.expression, localEnv) : NULL_OBJ;
      }
      case "LetStatement": {
        const ls = node as LetStatement;
        const val = ls.value ? evalExpr(ls.value, localEnv) : NULL_OBJ;
        if (isError(val)) {
          return val;
        }
        localEnv.set(ls.name.value, val);
        return val;
      }
      case "ReturnStatement": {
        const rs = node as ReturnStatement;
        const val = rs.returnValue ? evalExpr(rs.returnValue, localEnv) : NULL_OBJ;
        return isError(val) ? val : new ReturnValue(val);
      }
      case "IntegerLiteral":
        return new IntegerObj((node as IntegerLiteral).value);
      case "StringLiteral":
        return new StringObj((node as StringLiteral).value);
      case "BooleanLiteral":
        return nativeBool((node as BooleanLiteral).value);
      case "Identifier": {
        const name = (node as Identifier).value;
        return localEnv.get(name) ?? builtins[name] ?? newError(`identifier not found: ${name}`);
      }
      case "PrefixExpression": {
        const p = node as PrefixExpression;
        if (!p.right) {
          return newError("missing operand");
        }
        const right = evalExpr(p.right, localEnv);
        if (p.operator === "!") {
          return nativeBool(!isTruthy(right));
        }
        if (p.operator === "-" && right instanceof IntegerObj) {
          return new IntegerObj(-right.value);
        }
        return newError(`unknown prefix ${p.operator}`);
      }
      case "InfixExpression": {
        const inf = node as InfixExpression;
        const left = evalExpr(inf.left, localEnv);
        if (!inf.right) {
          return newError("missing rhs");
        }
        const right = evalExpr(inf.right, localEnv);
        if (left instanceof IntegerObj && right instanceof IntegerObj) {
          switch (inf.operator) {
            case "+":
              return new IntegerObj(left.value + right.value);
            case "-":
              return new IntegerObj(left.value - right.value);
            case "*":
              return new IntegerObj(left.value * right.value);
            case "/":
              return right.value === 0
                ? newError("division by zero")
                : new IntegerObj(Math.trunc(left.value / right.value));
            case "==":
              return nativeBool(left.value === right.value);
            case "!=":
              return nativeBool(left.value !== right.value);
            case "<":
              return nativeBool(left.value < right.value);
            case ">":
              return nativeBool(left.value > right.value);
            default:
              return newError(`unknown op ${inf.operator}`);
          }
        }
        if (
          left instanceof StringObj &&
          right instanceof StringObj &&
          inf.operator === "+"
        ) {
          return new StringObj(left.value + right.value);
        }
        return newError("type mismatch");
      }
      case "IfExpression": {
        const iff = node as IfExpression;
        if (!iff.condition) {
          return NULL_OBJ;
        }
        const cond = evalExpr(iff.condition, localEnv);
        if (isTruthy(cond)) {
          return evalExpr(iff.consequence, localEnv);
        }
        return iff.alternative ? evalExpr(iff.alternative, localEnv) : NULL_OBJ;
      }
      case "FunctionLiteral": {
        const fl = node as FunctionLiteral;
        return new FunctionObj(
          fl.parameters,
          fl.body,
          localEnv,
          fl.isAsync ?? false,
        );
      }
      case "CallExpression": {
        const call = node as CallExpression;
        const fn = evalExpr(call.function, localEnv);
        const args = call.arguments.map((a) => evalExpr(a, localEnv));
        if (fn instanceof FunctionObj) {
          const extended = new Environment(fn.env);
          for (let i = 0; i < fn.parameters.length; i++) {
            extended.set(fn.parameters[i]!.value, args[i]!);
          }
          const result = evalExpr(fn.body, extended);
          return result instanceof ReturnValue ? result.value : result;
        }
        if (fn instanceof BuiltinObj) {
          return fn.fn(...args);
        }
        return newError(`not a function: ${fn.type}`);
      }
      case "ConstructorExpression": {
        const ctor = node as ConstructorExpression;
        const fields = ctor.arguments.map((a) => evalExpr(a, localEnv));
        return new EnumValueObj(ctor.enumName, ctor.variant.value, fields);
      }
      case "MatchExpression": {
        const m = node as MatchExpression;
        if (!m.scrutinee) {
          return newError("match missing scrutinee");
        }
        const value = evalExpr(m.scrutinee, localEnv);
        for (const arm of m.arms) {
          const matched = syncMatchPattern(arm.pattern, value, localEnv);
          if (matched) {
            return evalExpr(arm.body, matched);
          }
        }
        return newError("non-exhaustive match at runtime");
      }
      case "MoveExpression": {
        const mv = node as MoveExpression;
        return mv.value ? evalExpr(mv.value, localEnv) : NULL_OBJ;
      }
      case "EnumDeclaration":
      case "StructDeclaration":
      case "EffectDeclaration":
      case "MacroDefinition":
      case "ExternDeclaration":
        return NULL_OBJ;
      default:
        return newError(
          `sync eval unsupported for ${node.type}; use runSourceAsync`,
        );
    }
  };

  return { value: evalExpr(program, env), output };
}

function syncMatchPattern(
  pattern: Pattern,
  value: NexusObject,
  env: Environment,
): Environment | null {
  switch (pattern.kind) {
    case "wildcard":
      return env;
    case "ident": {
      const extended = new Environment(env);
      extended.set(pattern.name.value, value);
      return extended;
    }
    case "literal":
      return null;
    case "variant": {
      if (!isEnumValue(value)) {
        return null;
      }
      if (value.variant !== pattern.variant.value) {
        return null;
      }
      if (
        pattern.enumName &&
        value.enumName &&
        pattern.enumName !== value.enumName
      ) {
        return null;
      }
      if (pattern.fields.length !== value.fields.length) {
        return null;
      }
      let extended: Environment = env;
      for (let i = 0; i < pattern.fields.length; i++) {
        const nested = syncMatchPattern(
          pattern.fields[i]!,
          value.fields[i]!,
          extended,
        );
        if (!nested) {
          return null;
        }
        extended = nested;
      }
      return extended;
    }
    default:
      return null;
  }
}

export async function runSourceAsync(
  source: string,
  env?: Environment,
  preparsed?: Program,
  options?: {
    rootDir?: string;
    modulesDir?: string;
    stdlibDir?: string;
    filePath?: string;
    /** Called after env is prepared, before evaluation (e.g. install HTTP host). */
    onReady?: (ctx: {
      env: Environment;
      evaluator: Evaluator;
    }) => void;
  },
): Promise<EvalResult> {
  let program = preparsed;
  if (!program) {
    const lexer = new Lexer(source);
    const parser = new Parser(lexer);
    program = parser.parseProgram();
    if (parser.getErrors().length > 0) {
      return {
        value: new ErrorObj(
          `parse errors:\n  ${parser.getErrors().join("\n  ")}`,
        ),
        output: [],
      };
    }
  }
  const evaluator = new Evaluator();
  if (options) {
    evaluator.configureModules(options);
  }
  const environment = env ?? new Environment();
  if (options?.filePath) {
    environment.set("__file__", new StringObj(options.filePath));
    environment.set("__dir__", new StringObj(path.dirname(options.filePath)));
  } else if (!environment.get("__dir__")) {
    environment.set("__dir__", new StringObj(evaluator.rootDir));
    environment.set(
      "__file__",
      new StringObj(path.join(evaluator.rootDir, "<stdin>")),
    );
  }
  options?.onReady?.({ env: environment, evaluator });
  return evaluator.eval(program, environment);
}

export { TRUE_OBJ as TRUE, FALSE_OBJ as FALSE, NULL_OBJ as NULL, FfiLibraryObj };
