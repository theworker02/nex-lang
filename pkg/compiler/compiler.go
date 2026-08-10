// Package compiler lowers a Nexus AST to bytecode for the VM.
package compiler

import (
	"fmt"

	"nex-lang/pkg/code"
	"nex-lang/pkg/evaluator"
	"nex-lang/pkg/lexer"
	"nex-lang/pkg/parser"
)

// Compiler produces bytecode from an AST.
type Compiler struct {
	constants   []evaluator.Object
	symbolTable *SymbolTable
	scopes      []CompilationScope
	scopeIndex  int
}

// CompilationScope holds instructions for one function/program body.
type CompilationScope struct {
	instructions        code.Instructions
	lastInstruction     EmittedInstruction
	previousInstruction EmittedInstruction
}

// EmittedInstruction tracks the last emitted opcode for peephole tweaks.
type EmittedInstruction struct {
	Opcode   code.Opcode
	Position int
}

// Bytecode is the compiler output.
type Bytecode struct {
	Instructions code.Instructions
	Constants    []evaluator.Object
}

// New creates a compiler with builtins pre-registered.
func New() *Compiler {
	st := NewSymbolTable()
	for i, name := range evaluator.BuiltinNames {
		st.DefineBuiltin(i, name)
	}
	return &Compiler{
		symbolTable: st,
		scopes:      []CompilationScope{{}},
		scopeIndex:  0,
	}
}

// NewWithState continues compilation with an existing symbol table and constants (REPL).
func NewWithState(st *SymbolTable, constants []evaluator.Object) *Compiler {
	c := New()
	c.symbolTable = st
	c.constants = constants
	return c
}

// Compile lowers node to bytecode.
func (c *Compiler) Compile(node parser.Node) error {
	switch node := node.(type) {
	case *parser.Program:
		for _, s := range node.Statements {
			if err := c.Compile(s); err != nil {
				return err
			}
		}

	case *parser.ExpressionStatement:
		if err := c.Compile(node.Expression); err != nil {
			return err
		}
		c.emit(code.OpPop)

	case *parser.BlockStatement:
		for _, s := range node.Statements {
			if err := c.Compile(s); err != nil {
				return err
			}
		}

	case *parser.LetStatement:
		symbol := c.symbolTable.Define(node.Name.Value)
		if fn, ok := node.Value.(*parser.FunctionLiteral); ok {
			if err := c.compileFunction(fn, node.Name.Value); err != nil {
				return err
			}
		} else if err := c.Compile(node.Value); err != nil {
			return err
		}
		if symbol.Scope == GlobalScope {
			c.emit(code.OpSetGlobal, symbol.Index)
		} else {
			c.emit(code.OpSetLocal, symbol.Index)
		}

	case *parser.ReturnStatement:
		if node.ReturnValue != nil {
			if err := c.Compile(node.ReturnValue); err != nil {
				return err
			}
			c.emit(code.OpReturnValue)
		} else {
			c.emit(code.OpReturn)
		}

	case *parser.WhileStatement:
		return c.compileWhile(node)

	case *parser.BreakStatement:
		return fmt.Errorf("break is only valid inside while (compiler)")

	case *parser.ContinueStatement:
		return fmt.Errorf("continue is only valid inside while (compiler)")

	case *parser.StructStatement:
		return c.compileStruct(node)

	case *parser.ImportStatement:
		return fmt.Errorf("import is not supported by the bytecode engine; use the tree-walk runtime (nex run without --vm)")

	case *parser.IntegerLiteral:
		c.emit(code.OpConstant, c.addConstant(&evaluator.Integer{Value: node.Value}))

	case *parser.StringLiteral:
		c.emit(code.OpConstant, c.addConstant(&evaluator.String{Value: node.Value}))

	case *parser.Boolean:
		if node.Value {
			c.emit(code.OpTrue)
		} else {
			c.emit(code.OpFalse)
		}

	case *parser.NullLiteral:
		c.emit(code.OpNull)

	case *parser.PrefixExpression:
		if err := c.Compile(node.Right); err != nil {
			return err
		}
		switch node.Operator {
		case "!":
			c.emit(code.OpBang)
		case "-":
			c.emit(code.OpMinus)
		default:
			return fmt.Errorf("unknown operator %s", node.Operator)
		}

	case *parser.InfixExpression:
		return c.compileInfix(node)

	case *parser.IfExpression:
		return c.compileIf(node)

	case *parser.Identifier:
		return c.compileIdentifier(node)

	case *parser.AssignExpression:
		return c.compileAssign(node)

	case *parser.FunctionLiteral:
		return c.compileFunction(node, "")

	case *parser.CallExpression:
		if err := c.Compile(node.Function); err != nil {
			return err
		}
		for _, a := range node.Arguments {
			if err := c.Compile(a); err != nil {
				return err
			}
		}
		c.emit(code.OpCall, len(node.Arguments))

	case *parser.ArrayLiteral:
		for _, el := range node.Elements {
			if err := c.Compile(el); err != nil {
				return err
			}
		}
		c.emit(code.OpArray, len(node.Elements))

	case *parser.HashLiteral:
		// Map iteration order is unstable; sort keys by String() for determinism.
		keys := make([]parser.Expression, 0, len(node.Pairs))
		for k := range node.Pairs {
			keys = append(keys, k)
		}
		sortExprs(keys)
		for _, k := range keys {
			if err := c.Compile(k); err != nil {
				return err
			}
			if err := c.Compile(node.Pairs[k]); err != nil {
				return err
			}
		}
		c.emit(code.OpHash, len(node.Pairs)*2)

	case *parser.IndexExpression:
		if err := c.Compile(node.Left); err != nil {
			return err
		}
		if err := c.Compile(node.Index); err != nil {
			return err
		}
		c.emit(code.OpIndex)

	case *parser.MemberExpression:
		if err := c.Compile(node.Left); err != nil {
			return err
		}
		idx := c.addConstant(&evaluator.String{Value: node.Field.Value})
		c.emit(code.OpMember, idx)

	case *parser.PipeExpression:
		return c.compilePipe(node)

	case *parser.MatchExpression:
		return c.compileMatch(node)

	case *parser.TryExpression:
		return c.compileTry(node)

	default:
		return fmt.Errorf("compilation not implemented for %T", node)
	}
	return nil
}

func (c *Compiler) compileInfix(node *parser.InfixExpression) error {
	// Short-circuit logical ops.
	if node.Operator == "&&" {
		if err := c.Compile(node.Left); err != nil {
			return err
		}
		jumpFalse := c.emit(code.OpJumpNotTruthy, 9999)
		c.emit(code.OpPop)
		if err := c.Compile(node.Right); err != nil {
			return err
		}
		c.changeOperand(jumpFalse, len(c.currentInstructions()))
		return nil
	}
	if node.Operator == "||" {
		if err := c.Compile(node.Left); err != nil {
			return err
		}
		jumpElse := c.emit(code.OpJumpNotTruthy, 9999)
		jumpEnd := c.emit(code.OpJump, 9999)
		c.changeOperand(jumpElse, len(c.currentInstructions()))
		c.emit(code.OpPop)
		if err := c.Compile(node.Right); err != nil {
			return err
		}
		c.changeOperand(jumpEnd, len(c.currentInstructions()))
		return nil
	}

	if err := c.Compile(node.Left); err != nil {
		return err
	}
	if err := c.Compile(node.Right); err != nil {
		return err
	}
	switch node.Operator {
	case "+":
		c.emit(code.OpAdd)
	case "-":
		c.emit(code.OpSub)
	case "*":
		c.emit(code.OpMul)
	case "/":
		c.emit(code.OpDiv)
	case "%":
		c.emit(code.OpMod)
	case "==":
		c.emit(code.OpEqual)
	case "!=":
		c.emit(code.OpNotEqual)
	case ">":
		c.emit(code.OpGreaterThan)
	case ">=":
		c.emit(code.OpGreaterEqual)
	case "<":
		c.emit(code.OpLessThan)
	case "<=":
		c.emit(code.OpLessEqual)
	default:
		return fmt.Errorf("unknown operator %s", node.Operator)
	}
	return nil
}

func (c *Compiler) compileIf(node *parser.IfExpression) error {
	if err := c.Compile(node.Condition); err != nil {
		return err
	}
	jumpNotTruthyPos := c.emit(code.OpJumpNotTruthy, 9999)
	if err := c.Compile(node.Consequence); err != nil {
		return err
	}
	if c.lastInstructionIs(code.OpPop) {
		c.removeLastPop()
	}
	jumpPos := c.emit(code.OpJump, 9999)
	c.changeOperand(jumpNotTruthyPos, len(c.currentInstructions()))
	if node.Alternative == nil {
		c.emit(code.OpNull)
	} else {
		if err := c.Compile(node.Alternative); err != nil {
			return err
		}
		if c.lastInstructionIs(code.OpPop) {
			c.removeLastPop()
		}
	}
	c.changeOperand(jumpPos, len(c.currentInstructions()))
	return nil
}

type loopContext struct {
	breakPositions    []int
	continuePositions []int
	loopStart         int
}

func (c *Compiler) compileWhile(node *parser.WhileStatement) error {
	loopStart := len(c.currentInstructions())
	ctx := &loopContext{loopStart: loopStart}

	if err := c.Compile(node.Condition); err != nil {
		return err
	}
	exitJump := c.emit(code.OpJumpNotTruthy, 9999)

	// Compile body with break/continue patching via a temporary approach:
	// walk statements manually so break/continue can emit placeholder jumps.
	if err := c.compileWhileBody(node.Body, ctx); err != nil {
		return err
	}
	if c.lastInstructionIs(code.OpPop) {
		c.removeLastPop()
	} else {
		// body may leave a value; discard it
		// only pop if something was left — while body statements already OpPop via ExpressionStatement
	}

	c.emit(code.OpJump, loopStart)
	after := len(c.currentInstructions())
	c.changeOperand(exitJump, after)
	for _, pos := range ctx.breakPositions {
		c.changeOperand(pos, after)
	}
	for _, pos := range ctx.continuePositions {
		c.changeOperand(pos, loopStart)
	}
	return nil
}

func (c *Compiler) compileWhileBody(body *parser.BlockStatement, ctx *loopContext) error {
	for _, stmt := range body.Statements {
		switch s := stmt.(type) {
		case *parser.BreakStatement:
			pos := c.emit(code.OpJump, 9999)
			ctx.breakPositions = append(ctx.breakPositions, pos)
		case *parser.ContinueStatement:
			pos := c.emit(code.OpJump, 9999)
			ctx.continuePositions = append(ctx.continuePositions, pos)
		default:
			if err := c.Compile(s); err != nil {
				return err
			}
		}
	}
	return nil
}

func (c *Compiler) compileIdentifier(node *parser.Identifier) error {
	symbol, ok := c.symbolTable.Resolve(node.Value)
	if !ok {
		return fmt.Errorf("undefined variable %s", node.Value)
	}
	c.loadSymbol(symbol)
	return nil
}

func (c *Compiler) compileAssign(node *parser.AssignExpression) error {
	switch name := node.Name.(type) {
	case *parser.Identifier:
		symbol, ok := c.symbolTable.Resolve(name.Value)
		if !ok {
			symbol = c.symbolTable.Define(name.Value)
		}
		if err := c.Compile(node.Value); err != nil {
			return err
		}
		switch symbol.Scope {
		case GlobalScope:
			c.emit(code.OpSetGlobal, symbol.Index)
			c.emit(code.OpGetGlobal, symbol.Index)
		case LocalScope:
			c.emit(code.OpSetLocal, symbol.Index)
			c.emit(code.OpGetLocal, symbol.Index)
		case FreeScope:
			return fmt.Errorf("cannot assign to free variable %s", name.Value)
		default:
			return fmt.Errorf("cannot assign to %s", name.Value)
		}
	case *parser.IndexExpression:
		if err := c.Compile(name.Left); err != nil {
			return err
		}
		if err := c.Compile(name.Index); err != nil {
			return err
		}
		if err := c.Compile(node.Value); err != nil {
			return err
		}
		c.emit(code.OpSetIndex)
	default:
		return fmt.Errorf("invalid assignment target")
	}
	return nil
}

func (c *Compiler) compileFunction(node *parser.FunctionLiteral, name string) error {
	c.enterScope()
	if name != "" {
		c.symbolTable.DefineFunctionName(name)
	}
	for _, p := range node.Parameters {
		c.symbolTable.Define(p.Name.Value)
	}
	if err := c.Compile(node.Body); err != nil {
		return err
	}
	if c.lastInstructionIs(code.OpPop) {
		c.replaceLastPopWithReturn()
	}
	if !c.lastInstructionIs(code.OpReturnValue) && !c.lastInstructionIs(code.OpReturn) {
		c.emit(code.OpReturn)
	}
	freeSymbols := c.symbolTable.FreeSymbols
	numLocals := c.symbolTable.numDefinitions
	instructions := c.leaveScope()

	for _, s := range freeSymbols {
		c.loadSymbol(s)
	}
	fn := &evaluator.CompiledFunction{
		Instructions:  []byte(instructions),
		NumLocals:     numLocals,
		NumParameters: len(node.Parameters),
	}
	c.emit(code.OpClosure, c.addConstant(fn), len(freeSymbols))
	return nil
}

func (c *Compiler) compileStruct(node *parser.StructStatement) error {
	// struct Point { x, y } => let Point = fn(x, y) { {"__struct": "Point", "x": x, "y": y} }
	name := node.Name.Value
	fields := node.Fields
	c.enterScope()
	for _, f := range fields {
		c.symbolTable.Define(f.Value)
	}
	// build hash: pairs of key const + get local
	for i, f := range fields {
		c.emit(code.OpConstant, c.addConstant(&evaluator.String{Value: f.Value}))
		c.emit(code.OpGetLocal, i)
	}
	// also __struct
	c.emit(code.OpConstant, c.addConstant(&evaluator.String{Value: "__struct"}))
	c.emit(code.OpConstant, c.addConstant(&evaluator.String{Value: name}))
	c.emit(code.OpHash, len(fields)*2+2)
	c.emit(code.OpReturnValue)
	numLocals := c.symbolTable.numDefinitions
	instructions := c.leaveScope()
	fn := &evaluator.CompiledFunction{
		Instructions:  []byte(instructions),
		NumLocals:     numLocals,
		NumParameters: len(fields),
	}
	c.emit(code.OpClosure, c.addConstant(fn), 0)
	symbol := c.symbolTable.Define(name)
	if symbol.Scope == GlobalScope {
		c.emit(code.OpSetGlobal, symbol.Index)
	} else {
		c.emit(code.OpSetLocal, symbol.Index)
	}
	return nil
}

func (c *Compiler) compilePipe(node *parser.PipeExpression) error {
	// left |> f  => f(left)
	// left |> f(a, b) => f(left, a, b)
	switch right := node.Right.(type) {
	case *parser.CallExpression:
		if err := c.Compile(right.Function); err != nil {
			return err
		}
		if err := c.Compile(node.Left); err != nil {
			return err
		}
		for _, a := range right.Arguments {
			if err := c.Compile(a); err != nil {
				return err
			}
		}
		c.emit(code.OpCall, len(right.Arguments)+1)
	default:
		if err := c.Compile(node.Right); err != nil {
			return err
		}
		if err := c.Compile(node.Left); err != nil {
			return err
		}
		c.emit(code.OpCall, 1)
	}
	return nil
}

func (c *Compiler) compileMatch(node *parser.MatchExpression) error {
	// Compile as chained ifs. Wildcard _ and bind patterns supported.
	if err := c.Compile(node.Value); err != nil {
		return err
	}
	// Keep subject in a temp global-like local by duplicating via set/get of a synthetic local.
	// Simpler approach: subject stays on stack; each arm duplicates via re-eval of value — but that
	// re-evaluates side effects. Use a synthetic local in current scope.
	subject := c.symbolTable.Define("__match_subject")
	if subject.Scope == GlobalScope {
		c.emit(code.OpSetGlobal, subject.Index)
	} else {
		c.emit(code.OpSetLocal, subject.Index)
	}

	endJumps := []int{}
	for i, arm := range node.Arms {
		// load subject
		c.loadSymbol(subject)
		matched, bindName, err := c.emitPatternTest(arm.Pattern)
		if err != nil {
			return err
		}
		_ = matched
		jumpNext := c.emit(code.OpJumpNotTruthy, 9999)
		if bindName != "" {
			bind := c.symbolTable.Define(bindName)
			c.loadSymbol(subject)
			if bind.Scope == GlobalScope {
				c.emit(code.OpSetGlobal, bind.Index)
			} else {
				c.emit(code.OpSetLocal, bind.Index)
			}
		}
		if err := c.Compile(arm.Body); err != nil {
			return err
		}
		endJumps = append(endJumps, c.emit(code.OpJump, 9999))
		c.changeOperand(jumpNext, len(c.currentInstructions()))
		if i == len(node.Arms)-1 {
			c.emit(code.OpNull)
		}
	}
	after := len(c.currentInstructions())
	for _, j := range endJumps {
		c.changeOperand(j, after)
	}
	return nil
}

func (c *Compiler) emitPatternTest(pattern parser.Expression) (bool, string, error) {
	switch p := pattern.(type) {
	case *parser.Identifier:
		if p.Value == "_" {
			c.emit(code.OpPop) // discard subject
			c.emit(code.OpTrue)
			return true, "", nil
		}
		// bind pattern: always matches
		c.emit(code.OpPop)
		c.emit(code.OpTrue)
		return true, p.Value, nil
	case *parser.IntegerLiteral, *parser.StringLiteral, *parser.Boolean, *parser.NullLiteral:
		if err := c.Compile(p); err != nil {
			return false, "", err
		}
		c.emit(code.OpEqual)
		return true, "", nil
	default:
		return false, "", fmt.Errorf("unsupported match pattern %T", pattern)
	}
}

func (c *Compiler) compileTry(node *parser.TryExpression) error {
	// try expr => evaluate Result; if ok push value; else return err-result from function
	if err := c.Compile(node.Value); err != nil {
		return err
	}
	// Duplicate result: store to temp
	tmp := c.symbolTable.Define("__try_tmp")
	if tmp.Scope == GlobalScope {
		c.emit(code.OpSetGlobal, tmp.Index)
		c.emit(code.OpGetGlobal, tmp.Index)
	} else {
		c.emit(code.OpSetLocal, tmp.Index)
		c.emit(code.OpGetLocal, tmp.Index)
	}
	// get .ok field
	c.emit(code.OpMember, c.addConstant(&evaluator.String{Value: "ok"}))
	jumpErr := c.emit(code.OpJumpNotTruthy, 9999)
	// ok path: load value
	c.loadSymbol(tmp)
	c.emit(code.OpMember, c.addConstant(&evaluator.String{Value: "value"}))
	jumpEnd := c.emit(code.OpJump, 9999)
	c.changeOperand(jumpErr, len(c.currentInstructions()))
	// err path: return the whole result (early return)
	c.loadSymbol(tmp)
	c.emit(code.OpReturnValue)
	c.changeOperand(jumpEnd, len(c.currentInstructions()))
	return nil
}

func (c *Compiler) loadSymbol(s Symbol) {
	switch s.Scope {
	case GlobalScope:
		c.emit(code.OpGetGlobal, s.Index)
	case LocalScope:
		c.emit(code.OpGetLocal, s.Index)
	case BuiltinScope:
		c.emit(code.OpGetBuiltin, s.Index)
	case FreeScope:
		c.emit(code.OpGetFree, s.Index)
	case FunctionScope:
		c.emit(code.OpCurrentClosure)
	}
}

// Bytecode returns the compiled program.
func (c *Compiler) Bytecode() *Bytecode {
	return &Bytecode{
		Instructions: c.currentInstructions(),
		Constants:    c.constants,
	}
}

// SymbolTable returns the compiler's symbol table (for REPL persistence).
func (c *Compiler) SymbolTable() *SymbolTable {
	return c.symbolTable
}

// Constants returns the constant pool.
func (c *Compiler) Constants() []evaluator.Object {
	return c.constants
}

func (c *Compiler) addConstant(obj evaluator.Object) int {
	c.constants = append(c.constants, obj)
	return len(c.constants) - 1
}

func (c *Compiler) emit(op code.Opcode, operands ...int) int {
	ins := code.Make(op, operands...)
	pos := c.addInstruction(ins)
	c.setLastInstruction(op, pos)
	return pos
}

func (c *Compiler) addInstruction(ins []byte) int {
	pos := len(c.currentInstructions())
	c.scopes[c.scopeIndex].instructions = append(c.currentInstructions(), ins...)
	return pos
}

func (c *Compiler) setLastInstruction(op code.Opcode, pos int) {
	previous := c.scopes[c.scopeIndex].lastInstruction
	last := EmittedInstruction{Opcode: op, Position: pos}
	c.scopes[c.scopeIndex].previousInstruction = previous
	c.scopes[c.scopeIndex].lastInstruction = last
}

func (c *Compiler) lastInstructionIs(op code.Opcode) bool {
	if len(c.currentInstructions()) == 0 {
		return false
	}
	return c.scopes[c.scopeIndex].lastInstruction.Opcode == op
}

func (c *Compiler) removeLastPop() {
	last := c.scopes[c.scopeIndex].lastInstruction
	previous := c.scopes[c.scopeIndex].previousInstruction
	c.scopes[c.scopeIndex].instructions = c.currentInstructions()[:last.Position]
	c.scopes[c.scopeIndex].lastInstruction = previous
}

func (c *Compiler) replaceLastPopWithReturn() {
	lastPos := c.scopes[c.scopeIndex].lastInstruction.Position
	c.replaceInstruction(lastPos, code.Make(code.OpReturnValue))
	c.scopes[c.scopeIndex].lastInstruction.Opcode = code.OpReturnValue
}

func (c *Compiler) replaceInstruction(pos int, newInstruction []byte) {
	ins := c.currentInstructions()
	for i := 0; i < len(newInstruction); i++ {
		ins[pos+i] = newInstruction[i]
	}
}

func (c *Compiler) changeOperand(opPos int, operand int) {
	op := code.Opcode(c.currentInstructions()[opPos])
	newInstruction := code.Make(op, operand)
	c.replaceInstruction(opPos, newInstruction)
}

func (c *Compiler) currentInstructions() code.Instructions {
	return c.scopes[c.scopeIndex].instructions
}

func (c *Compiler) enterScope() {
	c.scopes = append(c.scopes, CompilationScope{})
	c.scopeIndex++
	c.symbolTable = NewEnclosedSymbolTable(c.symbolTable)
}

func (c *Compiler) leaveScope() code.Instructions {
	instructions := c.currentInstructions()
	c.scopes = c.scopes[:len(c.scopes)-1]
	c.scopeIndex--
	c.symbolTable = c.symbolTable.Outer
	return instructions
}

func sortExprs(exprs []parser.Expression) {
	// insertion sort by String()
	for i := 1; i < len(exprs); i++ {
		j := i
		for j > 0 && exprs[j-1].String() > exprs[j].String() {
			exprs[j-1], exprs[j] = exprs[j], exprs[j-1]
			j--
		}
	}
}

// CompileSource lexes, parses, and compiles source.
func CompileSource(source string) (*Bytecode, error) {
	l := lexer.New(source)
	p := parser.New(l)
	program := p.ParseProgram()
	if len(p.Errors()) > 0 {
		return nil, fmt.Errorf("parse error: %s", p.Errors()[0])
	}
	comp := New()
	if err := comp.Compile(program); err != nil {
		return nil, err
	}
	return comp.Bytecode(), nil
}
