// Package vm executes Nexus bytecode produced by the compiler.
package vm

import (
	"fmt"

	"nex-lang/pkg/code"
	"nex-lang/pkg/compiler"
	"nex-lang/pkg/evaluator"
)

const (
	StackSize   = 2048
	GlobalsSize = 65536
	MaxFrames   = 1024
)

// Frame is a call frame for a closure.
type Frame struct {
	cl          *evaluator.Closure
	ip          int
	basePointer int
}

func NewFrame(cl *evaluator.Closure, basePointer int) *Frame {
	return &Frame{cl: cl, ip: -1, basePointer: basePointer}
}

func (f *Frame) Instructions() code.Instructions {
	return code.Instructions(f.cl.Fn.Instructions)
}

// VM is a stack-based bytecode virtual machine.
type VM struct {
	constants   []evaluator.Object
	stack       []evaluator.Object
	sp          int
	globals     []evaluator.Object
	frames      []*Frame
	framesIndex int
}

// New creates a VM from bytecode.
func New(bytecode *compiler.Bytecode) *VM {
	mainFn := &evaluator.CompiledFunction{Instructions: []byte(bytecode.Instructions)}
	mainClosure := &evaluator.Closure{Fn: mainFn}
	frames := make([]*Frame, MaxFrames)
	frames[0] = NewFrame(mainClosure, 0)
	return &VM{
		constants:   bytecode.Constants,
		stack:       make([]evaluator.Object, StackSize),
		globals:     make([]evaluator.Object, GlobalsSize),
		frames:      frames,
		framesIndex: 1,
	}
}

// NewWithGlobalsState continues a VM with existing globals (REPL).
func NewWithGlobalsState(bytecode *compiler.Bytecode, globals []evaluator.Object) *VM {
	v := New(bytecode)
	v.globals = globals
	return v
}

func (v *VM) currentFrame() *Frame {
	return v.frames[v.framesIndex-1]
}

func (v *VM) pushFrame(f *Frame) {
	v.frames[v.framesIndex] = f
	v.framesIndex++
}

func (v *VM) popFrame() *Frame {
	v.framesIndex--
	return v.frames[v.framesIndex]
}

// Run executes the bytecode until completion or error.
func (v *VM) Run() error {
	var ip int
	var ins code.Instructions
	var op code.Opcode

	for v.currentFrame().ip < len(v.currentFrame().Instructions())-1 {
		v.currentFrame().ip++
		ip = v.currentFrame().ip
		ins = v.currentFrame().Instructions()
		op = code.Opcode(ins[ip])

		switch op {
		case code.OpConstant:
			constIndex := code.ReadUint16(ins[ip+1:])
			v.currentFrame().ip += 2
			if err := v.push(v.constants[constIndex]); err != nil {
				return err
			}

		case code.OpPop:
			v.pop()

		case code.OpAdd, code.OpSub, code.OpMul, code.OpDiv, code.OpMod:
			if err := v.executeBinaryOperation(op); err != nil {
				return err
			}

		case code.OpTrue:
			if err := v.push(evaluator.TRUE); err != nil {
				return err
			}
		case code.OpFalse:
			if err := v.push(evaluator.FALSE); err != nil {
				return err
			}
		case code.OpNull:
			if err := v.push(evaluator.NULL); err != nil {
				return err
			}

		case code.OpEqual, code.OpNotEqual, code.OpGreaterThan, code.OpGreaterEqual, code.OpLessThan, code.OpLessEqual:
			if err := v.executeComparison(op); err != nil {
				return err
			}

		case code.OpBang:
			if err := v.executeBangOperator(); err != nil {
				return err
			}
		case code.OpMinus:
			if err := v.executeMinusOperator(); err != nil {
				return err
			}

		case code.OpJump:
			pos := int(code.ReadUint16(ins[ip+1:]))
			v.currentFrame().ip = pos - 1

		case code.OpJumpNotTruthy:
			pos := int(code.ReadUint16(ins[ip+1:]))
			v.currentFrame().ip += 2
			condition := v.pop()
			if !isTruthy(condition) {
				v.currentFrame().ip = pos - 1
			}

		case code.OpSetGlobal:
			globalIndex := int(code.ReadUint16(ins[ip+1:]))
			v.currentFrame().ip += 2
			v.globals[globalIndex] = v.pop()

		case code.OpGetGlobal:
			globalIndex := int(code.ReadUint16(ins[ip+1:]))
			v.currentFrame().ip += 2
			if err := v.push(v.globals[globalIndex]); err != nil {
				return err
			}

		case code.OpSetLocal:
			localIndex := int(ins[ip+1])
			v.currentFrame().ip++
			frame := v.currentFrame()
			v.stack[frame.basePointer+localIndex] = v.pop()

		case code.OpGetLocal:
			localIndex := int(ins[ip+1])
			v.currentFrame().ip++
			frame := v.currentFrame()
			if err := v.push(v.stack[frame.basePointer+localIndex]); err != nil {
				return err
			}

		case code.OpGetBuiltin:
			builtinIndex := int(ins[ip+1])
			v.currentFrame().ip++
			builtin, ok := evaluator.GetBuiltinByIndex(builtinIndex)
			if !ok {
				return fmt.Errorf("invalid builtin index %d", builtinIndex)
			}
			if err := v.push(builtin); err != nil {
				return err
			}

		case code.OpArray:
			numElements := int(code.ReadUint16(ins[ip+1:]))
			v.currentFrame().ip += 2
			array := v.buildArray(v.sp-numElements, v.sp)
			v.sp -= numElements
			if err := v.push(array); err != nil {
				return err
			}

		case code.OpHash:
			numElements := int(code.ReadUint16(ins[ip+1:]))
			v.currentFrame().ip += 2
			hash, err := v.buildHash(v.sp-numElements, v.sp)
			if err != nil {
				return err
			}
			v.sp -= numElements
			if err := v.push(hash); err != nil {
				return err
			}

		case code.OpIndex:
			index := v.pop()
			left := v.pop()
			if err := v.executeIndexExpression(left, index); err != nil {
				return err
			}

		case code.OpMember:
			fieldIndex := code.ReadUint16(ins[ip+1:])
			v.currentFrame().ip += 2
			fieldObj := v.constants[fieldIndex]
			field, ok := fieldObj.(*evaluator.String)
			if !ok {
				return fmt.Errorf("member field constant must be string")
			}
			left := v.pop()
			if err := v.executeMember(left, field.Value); err != nil {
				return err
			}

		case code.OpSetIndex:
			val := v.pop()
			index := v.pop()
			left := v.pop()
			if err := v.executeSetIndex(left, index, val); err != nil {
				return err
			}
			if err := v.push(val); err != nil {
				return err
			}

		case code.OpCall:
			numArgs := int(ins[ip+1])
			v.currentFrame().ip++
			if err := v.executeCall(numArgs); err != nil {
				return err
			}

		case code.OpReturnValue:
			returnValue := v.pop()
			frame := v.popFrame()
			v.sp = frame.basePointer - 1
			if err := v.push(returnValue); err != nil {
				return err
			}

		case code.OpReturn:
			frame := v.popFrame()
			v.sp = frame.basePointer - 1
			if err := v.push(evaluator.NULL); err != nil {
				return err
			}

		case code.OpClosure:
			constIndex := int(code.ReadUint16(ins[ip+1:]))
			numFree := int(ins[ip+3])
			v.currentFrame().ip += 3
			if err := v.pushClosure(constIndex, numFree); err != nil {
				return err
			}

		case code.OpGetFree:
			freeIndex := int(ins[ip+1])
			v.currentFrame().ip++
			currentClosure := v.currentFrame().cl
			if err := v.push(currentClosure.Free[freeIndex]); err != nil {
				return err
			}

		case code.OpCurrentClosure:
			if err := v.push(v.currentFrame().cl); err != nil {
				return err
			}

		default:
			return fmt.Errorf("unknown opcode %d", op)
		}
	}
	return nil
}

func (v *VM) pushClosure(constIndex, numFree int) error {
	constant := v.constants[constIndex]
	fn, ok := constant.(*evaluator.CompiledFunction)
	if !ok {
		return fmt.Errorf("not a function: %T", constant)
	}
	free := make([]evaluator.Object, numFree)
	for i := 0; i < numFree; i++ {
		free[i] = v.stack[v.sp-numFree+i]
	}
	v.sp -= numFree
	return v.push(&evaluator.Closure{Fn: fn, Free: free})
}

func (v *VM) executeCall(numArgs int) error {
	callee := v.stack[v.sp-1-numArgs]
	switch callee := callee.(type) {
	case *evaluator.Closure:
		return v.callClosure(callee, numArgs)
	case *evaluator.Builtin:
		return v.callBuiltin(callee, numArgs)
	default:
		return fmt.Errorf("calling non-function: %s", callee.Type())
	}
}

func (v *VM) callClosure(cl *evaluator.Closure, numArgs int) error {
	if numArgs != cl.Fn.NumParameters {
		return fmt.Errorf("wrong number of arguments: want=%d, got=%d", cl.Fn.NumParameters, numArgs)
	}
	frame := NewFrame(cl, v.sp-numArgs)
	v.pushFrame(frame)
	v.sp = frame.basePointer + cl.Fn.NumLocals
	return nil
}

func (v *VM) callBuiltin(builtin *evaluator.Builtin, numArgs int) error {
	args := v.stack[v.sp-numArgs : v.sp]
	result := builtin.Fn(args...)
	v.sp = v.sp - numArgs - 1
	if errObj, ok := result.(*evaluator.Error); ok {
		return fmt.Errorf("%s", errObj.Message)
	}
	return v.push(result)
}

func (v *VM) buildArray(start, end int) *evaluator.Array {
	elements := make([]evaluator.Object, end-start)
	copy(elements, v.stack[start:end])
	return &evaluator.Array{Elements: elements}
}

func (v *VM) buildHash(start, end int) (*evaluator.Hash, error) {
	hash := evaluator.NewHash()
	for i := start; i < end; i += 2 {
		key := v.stack[i]
		value := v.stack[i+1]
		hashable, ok := key.(evaluator.Hashable)
		if !ok {
			return nil, fmt.Errorf("unusable as hash key: %s", key.Type())
		}
		hash.Pairs[hashable.HashKey()] = evaluator.HashPair{Key: key, Value: value}
	}
	return hash, nil
}

func (v *VM) executeIndexExpression(left, index evaluator.Object) error {
	switch {
	case left.Type() == evaluator.ArrayObj && index.Type() == evaluator.IntegerObj:
		return v.push(evalArrayIndex(left, index))
	case left.Type() == evaluator.HashObj:
		return v.push(evalHashIndex(left, index))
	case left.Type() == evaluator.StringObj && index.Type() == evaluator.IntegerObj:
		str := left.(*evaluator.String).Value
		idx := index.(*evaluator.Integer).Value
		if idx < 0 || int(idx) >= len(str) {
			return v.push(evaluator.NULL)
		}
		return v.push(&evaluator.String{Value: string(str[idx])})
	default:
		return fmt.Errorf("index operator not supported: %s", left.Type())
	}
}

func evalArrayIndex(array, index evaluator.Object) evaluator.Object {
	arr := array.(*evaluator.Array)
	idx := index.(*evaluator.Integer).Value
	if idx < 0 || int(idx) >= len(arr.Elements) {
		return evaluator.NULL
	}
	return arr.Elements[idx]
}

func evalHashIndex(hash, index evaluator.Object) evaluator.Object {
	h := hash.(*evaluator.Hash)
	key, ok := index.(evaluator.Hashable)
	if !ok {
		return &evaluator.Error{Message: fmt.Sprintf("unusable as hash key: %s", index.Type())}
	}
	pair, ok := h.Pairs[key.HashKey()]
	if !ok {
		return evaluator.NULL
	}
	return pair.Value
}

func (v *VM) executeMember(left evaluator.Object, field string) error {
	hash, ok := left.(*evaluator.Hash)
	if !ok {
		return fmt.Errorf("member access on non-hash: %s", left.Type())
	}
	return v.push(hash.Get(field))
}

func (v *VM) executeSetIndex(left, index, val evaluator.Object) error {
	switch left := left.(type) {
	case *evaluator.Array:
		idx, ok := index.(*evaluator.Integer)
		if !ok {
			return fmt.Errorf("array index must be INTEGER")
		}
		if idx.Value < 0 || int(idx.Value) >= len(left.Elements) {
			return fmt.Errorf("array index out of bounds")
		}
		left.Elements[idx.Value] = val
		return nil
	case *evaluator.Hash:
		key, ok := index.(evaluator.Hashable)
		if !ok {
			return fmt.Errorf("unusable as hash key: %s", index.Type())
		}
		left.Pairs[key.HashKey()] = evaluator.HashPair{Key: index, Value: val}
		return nil
	default:
		return fmt.Errorf("index assignment not supported on %s", left.Type())
	}
}

func (v *VM) executeBinaryOperation(op code.Opcode) error {
	right := v.pop()
	left := v.pop()
	if left.Type() == evaluator.IntegerObj && right.Type() == evaluator.IntegerObj {
		return v.executeBinaryInteger(op, left, right)
	}
	if left.Type() == evaluator.StringObj && right.Type() == evaluator.StringObj && op == code.OpAdd {
		leftVal := left.(*evaluator.String).Value
		rightVal := right.(*evaluator.String).Value
		return v.push(&evaluator.String{Value: leftVal + rightVal})
	}
	return fmt.Errorf("unsupported types for binary operation: %s %s", left.Type(), right.Type())
}

func (v *VM) executeBinaryInteger(op code.Opcode, left, right evaluator.Object) error {
	leftVal := left.(*evaluator.Integer).Value
	rightVal := right.(*evaluator.Integer).Value
	var result int64
	switch op {
	case code.OpAdd:
		result = leftVal + rightVal
	case code.OpSub:
		result = leftVal - rightVal
	case code.OpMul:
		result = leftVal * rightVal
	case code.OpDiv:
		if rightVal == 0 {
			return fmt.Errorf("division by zero")
		}
		result = leftVal / rightVal
	case code.OpMod:
		if rightVal == 0 {
			return fmt.Errorf("modulo by zero")
		}
		result = leftVal % rightVal
	default:
		return fmt.Errorf("unknown integer operator %d", op)
	}
	return v.push(&evaluator.Integer{Value: result})
}

func (v *VM) executeComparison(op code.Opcode) error {
	right := v.pop()
	left := v.pop()
	if left.Type() == evaluator.IntegerObj && right.Type() == evaluator.IntegerObj {
		return v.executeIntegerComparison(op, left, right)
	}
	switch op {
	case code.OpEqual:
		return v.push(boolObj(objectsEqual(left, right)))
	case code.OpNotEqual:
		return v.push(boolObj(!objectsEqual(left, right)))
	default:
		return fmt.Errorf("unknown operator: %d (%s %s)", op, left.Type(), right.Type())
	}
}

func (v *VM) executeIntegerComparison(op code.Opcode, left, right evaluator.Object) error {
	leftVal := left.(*evaluator.Integer).Value
	rightVal := right.(*evaluator.Integer).Value
	switch op {
	case code.OpEqual:
		return v.push(boolObj(leftVal == rightVal))
	case code.OpNotEqual:
		return v.push(boolObj(leftVal != rightVal))
	case code.OpGreaterThan:
		return v.push(boolObj(leftVal > rightVal))
	case code.OpGreaterEqual:
		return v.push(boolObj(leftVal >= rightVal))
	case code.OpLessThan:
		return v.push(boolObj(leftVal < rightVal))
	case code.OpLessEqual:
		return v.push(boolObj(leftVal <= rightVal))
	default:
		return fmt.Errorf("unknown operator %d", op)
	}
}

func (v *VM) executeBangOperator() error {
	operand := v.pop()
	switch operand {
	case evaluator.TRUE:
		return v.push(evaluator.FALSE)
	case evaluator.FALSE:
		return v.push(evaluator.TRUE)
	case evaluator.NULL:
		return v.push(evaluator.TRUE)
	default:
		return v.push(evaluator.FALSE)
	}
}

func (v *VM) executeMinusOperator() error {
	operand := v.pop()
	if operand.Type() != evaluator.IntegerObj {
		return fmt.Errorf("unsupported type for negation: %s", operand.Type())
	}
	value := operand.(*evaluator.Integer).Value
	return v.push(&evaluator.Integer{Value: -value})
}

func (v *VM) push(o evaluator.Object) error {
	if v.sp >= StackSize {
		return fmt.Errorf("stack overflow")
	}
	v.stack[v.sp] = o
	v.sp++
	return nil
}

func (v *VM) pop() evaluator.Object {
	o := v.stack[v.sp-1]
	v.sp--
	return o
}

// LastPoppedStackElem returns the last value popped (expression result).
func (v *VM) LastPoppedStackElem() evaluator.Object {
	return v.stack[v.sp]
}

// Globals returns the global bindings (for REPL persistence).
func (v *VM) Globals() []evaluator.Object {
	return v.globals
}

func boolObj(b bool) *evaluator.Boolean {
	if b {
		return evaluator.TRUE
	}
	return evaluator.FALSE
}

func isTruthy(obj evaluator.Object) bool {
	switch obj {
	case evaluator.NULL, evaluator.FALSE:
		return false
	default:
		if i, ok := obj.(*evaluator.Integer); ok && i.Value == 0 {
			return false
		}
		if s, ok := obj.(*evaluator.String); ok && s.Value == "" {
			return false
		}
		return true
	}
}

func objectsEqual(a, b evaluator.Object) bool {
	if a == nil || b == nil {
		return a == b
	}
	if a.Type() != b.Type() {
		return false
	}
	switch a := a.(type) {
	case *evaluator.Integer:
		return a.Value == b.(*evaluator.Integer).Value
	case *evaluator.String:
		return a.Value == b.(*evaluator.String).Value
	case *evaluator.Boolean:
		return a.Value == b.(*evaluator.Boolean).Value
	case *evaluator.Null:
		return true
	case *evaluator.Array:
		other := b.(*evaluator.Array)
		if len(a.Elements) != len(other.Elements) {
			return false
		}
		for i := range a.Elements {
			if !objectsEqual(a.Elements[i], other.Elements[i]) {
				return false
			}
		}
		return true
	default:
		return a == b
	}
}

// RunSource compiles and runs source on a fresh VM.
func RunSource(source string) (evaluator.Object, error) {
	bc, err := compiler.CompileSource(source)
	if err != nil {
		return nil, err
	}
	machine := New(bc)
	if err := machine.Run(); err != nil {
		return nil, err
	}
	result := machine.LastPoppedStackElem()
	if result == nil {
		return evaluator.NULL, nil
	}
	return result, nil
}
