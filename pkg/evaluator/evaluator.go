package evaluator

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"nex-lang/pkg/lexer"
	"nex-lang/pkg/parser"
)

var (
	NULL     = &Null{}
	TRUE     = &Boolean{Value: true}
	FALSE    = &Boolean{Value: false}
	BREAK    = &BreakSignal{}
	CONTINUE = &ContinueSignal{}
)

// Importer resolves and evaluates import paths. Set by the runtime package.
var Importer func(path string, env *Environment) Object

// ExtraBuiltins can be populated by the host before evaluation.
var ExtraBuiltins = map[string]*Builtin{}

// Eval evaluates a node in the given environment.
func Eval(node parser.Node, env *Environment) Object {
	switch node := node.(type) {
	case *parser.Program:
		return evalProgram(node, env)

	case *parser.BlockStatement:
		return evalBlockStatement(node, env)

	case *parser.ExpressionStatement:
		return Eval(node.Expression, env)

	case *parser.ReturnStatement:
		if node.ReturnValue == nil {
			return &ReturnValue{Value: NULL}
		}
		val := Eval(node.ReturnValue, env)
		if isError(val) {
			return val
		}
		return &ReturnValue{Value: val}

	case *parser.LetStatement:
		val := Eval(node.Value, env)
		if isError(val) {
			return val
		}
		if node.TypeName != "" {
			if err := checkTypeAnnotation(node.TypeName, val); err != nil {
				return err
			}
		}
		env.Set(node.Name.Value, val)
		return val

	case *parser.StructStatement:
		return evalStructStatement(node, env)

	case *parser.WhileStatement:
		return evalWhile(node, env)

	case *parser.ImportStatement:
		if Importer == nil {
			return newError("imports are not enabled")
		}
		return Importer(node.Path, env)

	case *parser.BreakStatement:
		return BREAK

	case *parser.ContinueStatement:
		return CONTINUE

	case *parser.IntegerLiteral:
		return &Integer{Value: node.Value}

	case *parser.StringLiteral:
		return &String{Value: node.Value}

	case *parser.Boolean:
		return nativeBoolToBooleanObject(node.Value)

	case *parser.NullLiteral:
		return NULL

	case *parser.PrefixExpression:
		right := Eval(node.Right, env)
		if isError(right) {
			return right
		}
		return evalPrefixExpression(node.Operator, right)

	case *parser.InfixExpression:
		if node.Operator == "&&" {
			left := Eval(node.Left, env)
			if isError(left) {
				return left
			}
			if !isTruthy(left) {
				return left
			}
			return Eval(node.Right, env)
		}
		if node.Operator == "||" {
			left := Eval(node.Left, env)
			if isError(left) {
				return left
			}
			if isTruthy(left) {
				return left
			}
			return Eval(node.Right, env)
		}
		left := Eval(node.Left, env)
		if isError(left) {
			return left
		}
		right := Eval(node.Right, env)
		if isError(right) {
			return right
		}
		return evalInfixExpression(node.Operator, left, right)

	case *parser.AssignExpression:
		return evalAssign(node, env)

	case *parser.IfExpression:
		return evalIfExpression(node, env)

	case *parser.Identifier:
		return evalIdentifier(node, env)

	case *parser.FunctionLiteral:
		return &Function{
			Parameters: node.Parameters,
			ReturnType: node.ReturnType,
			Body:       node.Body,
			Env:        env,
		}

	case *parser.CallExpression:
		function := Eval(node.Function, env)
		if isError(function) {
			return function
		}
		args := evalExpressions(node.Arguments, env)
		if len(args) == 1 && isError(args[0]) {
			return args[0]
		}
		return applyFunction(function, args)

	case *parser.MatchExpression:
		return evalMatchExpression(node, env)

	case *parser.TryExpression:
		return evalTryExpression(node, env)

	case *parser.PipeExpression:
		return evalPipeExpression(node, env)

	case *parser.MemberExpression:
		return evalMemberExpression(node, env)

	case *parser.ArrayLiteral:
		elements := evalExpressions(node.Elements, env)
		if len(elements) == 1 && isError(elements[0]) {
			return elements[0]
		}
		return &Array{Elements: elements}

	case *parser.HashLiteral:
		return evalHashLiteral(node, env)

	case *parser.IndexExpression:
		left := Eval(node.Left, env)
		if isError(left) {
			return left
		}
		index := Eval(node.Index, env)
		if isError(index) {
			return index
		}
		return evalIndexExpression(left, index)
	}

	return newError("unknown node type: %T", node)
}

func evalProgram(program *parser.Program, env *Environment) Object {
	var result Object = NULL

	for _, statement := range program.Statements {
		result = Eval(statement, env)

		switch result := result.(type) {
		case *ReturnValue:
			return result.Value
		case *Error:
			return result
		}
	}

	return result
}

func evalBlockStatement(block *parser.BlockStatement, env *Environment) Object {
	var result Object = NULL

	for _, statement := range block.Statements {
		result = Eval(statement, env)

		if result != nil {
			rt := result.Type()
			if rt == ReturnValueObj || rt == ErrorObj || rt == BreakObj || rt == ContinueObj {
				return result
			}
		}
	}

	return result
}

func evalWhile(node *parser.WhileStatement, env *Environment) Object {
	var result Object = NULL
	for {
		cond := Eval(node.Condition, env)
		if isError(cond) {
			return cond
		}
		if !isTruthy(cond) {
			break
		}
		result = Eval(node.Body, env)
		if isError(result) {
			return result
		}
		if result != nil {
			switch result.Type() {
			case ReturnValueObj:
				return result
			case BreakObj:
				return NULL
			case ContinueObj:
				continue
			}
		}
	}
	return result
}

func evalAssign(node *parser.AssignExpression, env *Environment) Object {
	val := Eval(node.Value, env)
	if isError(val) {
		return val
	}

	switch name := node.Name.(type) {
	case *parser.Identifier:
		if !env.Update(name.Value, val) {
			env.Set(name.Value, val)
		}
		return val
	case *parser.IndexExpression:
		left := Eval(name.Left, env)
		if isError(left) {
			return left
		}
		index := Eval(name.Index, env)
		if isError(index) {
			return index
		}
		return assignIndex(left, index, val)
	default:
		return newError("invalid assignment target")
	}
}

func assignIndex(left, index, val Object) Object {
	switch left := left.(type) {
	case *Array:
		idx, ok := index.(*Integer)
		if !ok {
			return newError("array index must be INTEGER, got %s", index.Type())
		}
		if idx.Value < 0 || int(idx.Value) >= len(left.Elements) {
			return newError("array index out of bounds")
		}
		left.Elements[idx.Value] = val
		return val
	case *Hash:
		hashKey, ok := index.(Hashable)
		if !ok {
			return newError("unusable as hash key: %s", index.Type())
		}
		left.Pairs[hashKey.HashKey()] = HashPair{Key: index, Value: val}
		return val
	default:
		return newError("index assignment not supported on %s", left.Type())
	}
}

func nativeBoolToBooleanObject(input bool) *Boolean {
	if input {
		return TRUE
	}
	return FALSE
}

func evalPrefixExpression(operator string, right Object) Object {
	switch operator {
	case "!":
		return evalBangOperatorExpression(right)
	case "-":
		return evalMinusPrefixOperatorExpression(right)
	default:
		return newError("unknown operator: %s%s", operator, right.Type())
	}
}

func evalBangOperatorExpression(right Object) Object {
	switch right {
	case TRUE:
		return FALSE
	case FALSE:
		return TRUE
	case NULL:
		return TRUE
	default:
		return FALSE
	}
}

func evalMinusPrefixOperatorExpression(right Object) Object {
	if right.Type() != IntegerObj {
		return newError("unknown operator: -%s", right.Type())
	}
	value := right.(*Integer).Value
	return &Integer{Value: -value}
}

func evalInfixExpression(operator string, left, right Object) Object {
	switch {
	case left.Type() == IntegerObj && right.Type() == IntegerObj:
		return evalIntegerInfixExpression(operator, left, right)
	case left.Type() == StringObj && right.Type() == StringObj:
		return evalStringInfixExpression(operator, left, right)
	case left.Type() == StringObj && right.Type() == IntegerObj:
		return evalStringInfixExpression(operator, left, &String{Value: fmt.Sprintf("%d", right.(*Integer).Value)})
	case left.Type() == IntegerObj && right.Type() == StringObj:
		return evalStringInfixExpression(operator, &String{Value: fmt.Sprintf("%d", left.(*Integer).Value)}, right)
	case operator == "==":
		return nativeBoolToBooleanObject(objectsEqual(left, right))
	case operator == "!=":
		return nativeBoolToBooleanObject(!objectsEqual(left, right))
	case left.Type() != right.Type():
		return newError("type mismatch: %s %s %s", left.Type(), operator, right.Type())
	default:
		return newError("unknown operator: %s %s %s", left.Type(), operator, right.Type())
	}
}

func objectsEqual(left, right Object) bool {
	if left == nil || right == nil {
		return left == right
	}
	if left.Type() != right.Type() {
		return false
	}
	switch l := left.(type) {
	case *Integer:
		return l.Value == right.(*Integer).Value
	case *String:
		return l.Value == right.(*String).Value
	case *Boolean:
		return l.Value == right.(*Boolean).Value
	case *Null:
		return true
	case *Array:
		r := right.(*Array)
		if len(l.Elements) != len(r.Elements) {
			return false
		}
		for i := range l.Elements {
			if !objectsEqual(l.Elements[i], r.Elements[i]) {
				return false
			}
		}
		return true
	case *Hash:
		r := right.(*Hash)
		if len(l.Pairs) != len(r.Pairs) {
			return false
		}
		for k, lp := range l.Pairs {
			rp, ok := r.Pairs[k]
			if !ok || !objectsEqual(lp.Value, rp.Value) {
				return false
			}
		}
		return true
	default:
		return left == right
	}
}

func evalIntegerInfixExpression(operator string, left, right Object) Object {
	leftVal := left.(*Integer).Value
	rightVal := right.(*Integer).Value

	switch operator {
	case "+":
		return &Integer{Value: leftVal + rightVal}
	case "-":
		return &Integer{Value: leftVal - rightVal}
	case "*":
		return &Integer{Value: leftVal * rightVal}
	case "/":
		if rightVal == 0 {
			return newError("division by zero")
		}
		return &Integer{Value: leftVal / rightVal}
	case "%":
		if rightVal == 0 {
			return newError("modulo by zero")
		}
		return &Integer{Value: leftVal % rightVal}
	case "<":
		return nativeBoolToBooleanObject(leftVal < rightVal)
	case ">":
		return nativeBoolToBooleanObject(leftVal > rightVal)
	case "<=":
		return nativeBoolToBooleanObject(leftVal <= rightVal)
	case ">=":
		return nativeBoolToBooleanObject(leftVal >= rightVal)
	case "==":
		return nativeBoolToBooleanObject(leftVal == rightVal)
	case "!=":
		return nativeBoolToBooleanObject(leftVal != rightVal)
	default:
		return newError("unknown operator: %s %s %s", left.Type(), operator, right.Type())
	}
}

func evalStringInfixExpression(operator string, left, right Object) Object {
	leftVal := left.(*String).Value
	rightVal := right.(*String).Value

	switch operator {
	case "+":
		return &String{Value: leftVal + rightVal}
	case "==":
		return nativeBoolToBooleanObject(leftVal == rightVal)
	case "!=":
		return nativeBoolToBooleanObject(leftVal != rightVal)
	case "<":
		return nativeBoolToBooleanObject(leftVal < rightVal)
	case ">":
		return nativeBoolToBooleanObject(leftVal > rightVal)
	default:
		return newError("unknown operator: %s %s %s", left.Type(), operator, right.Type())
	}
}

func evalIfExpression(ie *parser.IfExpression, env *Environment) Object {
	condition := Eval(ie.Condition, env)
	if isError(condition) {
		return condition
	}

	if isTruthy(condition) {
		return Eval(ie.Consequence, env)
	} else if ie.Alternative != nil {
		return Eval(ie.Alternative, env)
	}
	return NULL
}

func evalIdentifier(node *parser.Identifier, env *Environment) Object {
	if val, ok := env.Get(node.Value); ok {
		return val
	}
	if builtin, ok := builtins[node.Value]; ok {
		return builtin
	}
	if builtin, ok := ExtraBuiltins[node.Value]; ok {
		return builtin
	}
	return newError("identifier not found: %s", node.Value)
}

func evalExpressions(exps []parser.Expression, env *Environment) []Object {
	var result []Object

	for _, e := range exps {
		evaluated := Eval(e, env)
		if isError(evaluated) {
			return []Object{evaluated}
		}
		result = append(result, evaluated)
	}

	return result
}

func evalHashLiteral(node *parser.HashLiteral, env *Environment) Object {
	pairs := make(map[HashKey]HashPair)

	for keyNode, valueNode := range node.Pairs {
		key := Eval(keyNode, env)
		if isError(key) {
			return key
		}
		hashKey, ok := key.(Hashable)
		if !ok {
			return newError("unusable as hash key: %s", key.Type())
		}
		value := Eval(valueNode, env)
		if isError(value) {
			return value
		}
		hashed := hashKey.HashKey()
		pairs[hashed] = HashPair{Key: key, Value: value}
	}

	return &Hash{Pairs: pairs}
}

func evalIndexExpression(left, index Object) Object {
	switch {
	case left.Type() == ArrayObj && index.Type() == IntegerObj:
		return evalArrayIndexExpression(left, index)
	case left.Type() == HashObj:
		return evalHashIndexExpression(left, index)
	case left.Type() == StringObj && index.Type() == IntegerObj:
		str := left.(*String).Value
		idx := index.(*Integer).Value
		if idx < 0 || int(idx) >= len(str) {
			return NULL
		}
		return &String{Value: string(str[idx])}
	default:
		return newError("index operator not supported: %s", left.Type())
	}
}

func evalArrayIndexExpression(array, index Object) Object {
	arrayObject := array.(*Array)
	idx := index.(*Integer).Value
	max := int64(len(arrayObject.Elements) - 1)
	if idx < 0 || idx > max {
		return NULL
	}
	return arrayObject.Elements[idx]
}

func evalHashIndexExpression(hash, index Object) Object {
	hashObject := hash.(*Hash)
	key, ok := index.(Hashable)
	if !ok {
		return newError("unusable as hash key: %s", index.Type())
	}
	pair, ok := hashObject.Pairs[key.HashKey()]
	if !ok {
		return NULL
	}
	return pair.Value
}

func applyFunction(fn Object, args []Object) Object {
	return ApplyFunction(fn, args)
}

// ApplyFunction calls a Nexus function or builtin with args.
func ApplyFunction(fn Object, args []Object) Object {
	switch fn := fn.(type) {
	case *Function:
		extendedEnv, err := extendFunctionEnv(fn, args)
		if err != nil {
			return err
		}
		evaluated := Eval(fn.Body, extendedEnv)
		return unwrapReturnValue(evaluated)
	case *Builtin:
		return fn.Fn(args...)
	default:
		return newError("not a function: %s", fn.Type())
	}
}

func extendFunctionEnv(fn *Function, args []Object) (*Environment, Object) {
	env := NewEnclosedEnvironment(fn.Env)
	for i, param := range fn.Parameters {
		var arg Object = NULL
		if i < len(args) {
			arg = args[i]
		}
		if param.TypeName != "" {
			if err := checkTypeAnnotation(param.TypeName, arg); err != nil {
				return nil, err
			}
		}
		env.Set(param.Name.Value, arg)
	}
	return env, nil
}

func checkTypeAnnotation(typeName string, val Object) Object {
	switch strings.ToLower(typeName) {
	case "any", "auto":
		return nil
	case "int", "integer":
		if _, ok := val.(*Integer); !ok {
			return newError("type error: expected %s, got %s", typeName, val.Type())
		}
	case "string", "str":
		if _, ok := val.(*String); !ok {
			return newError("type error: expected %s, got %s", typeName, val.Type())
		}
	case "bool", "boolean":
		if _, ok := val.(*Boolean); !ok {
			return newError("type error: expected %s, got %s", typeName, val.Type())
		}
	case "array", "list":
		if _, ok := val.(*Array); !ok {
			return newError("type error: expected %s, got %s", typeName, val.Type())
		}
	case "hash", "map", "object", "result":
		if _, ok := val.(*Hash); !ok {
			return newError("type error: expected %s, got %s", typeName, val.Type())
		}
	case "fn", "function":
		switch val.(type) {
		case *Function, *Builtin:
		default:
			return newError("type error: expected %s, got %s", typeName, val.Type())
		}
	case "null":
		if _, ok := val.(*Null); !ok {
			return newError("type error: expected null, got %s", val.Type())
		}
	}
	return nil
}

func evalStructStatement(node *parser.StructStatement, env *Environment) Object {
	fields := make([]string, len(node.Fields))
	for i, f := range node.Fields {
		fields[i] = f.Value
	}
	name := node.Name.Value
	ctor := &Builtin{
		Fn: func(args ...Object) Object {
			if len(args) != len(fields) {
				return newError("%s expects %d fields, got %d", name, len(fields), len(args))
			}
			h := NewHash()
			h.SetString("__struct", &String{Value: name})
			for i, field := range fields {
				h.SetString(field, args[i])
			}
			return h
		},
	}
	env.Set(name, ctor)
	return ctor
}

func evalMatchExpression(node *parser.MatchExpression, env *Environment) Object {
	val := Eval(node.Value, env)
	if isError(val) {
		return val
	}
	for _, arm := range node.Arms {
		matched, bindName := matchPattern(arm.Pattern, val)
		if !matched {
			continue
		}
		armEnv := env
		if bindName != "" {
			armEnv = NewEnclosedEnvironment(env)
			armEnv.Set(bindName, val)
		}
		return Eval(arm.Body, armEnv)
	}
	return newError("no matching arm in match expression")
}

func matchPattern(pattern parser.Expression, val Object) (bool, string) {
	switch p := pattern.(type) {
	case *parser.Identifier:
		if p.Value == "_" {
			return true, ""
		}
		// bare identifier binds the value
		return true, p.Value
	case *parser.IntegerLiteral:
		i, ok := val.(*Integer)
		return ok && i.Value == p.Value, ""
	case *parser.StringLiteral:
		s, ok := val.(*String)
		return ok && s.Value == p.Value, ""
	case *parser.Boolean:
		b, ok := val.(*Boolean)
		return ok && b.Value == p.Value, ""
	case *parser.NullLiteral:
		_, ok := val.(*Null)
		return ok, ""
	default:
		return false, ""
	}
}

func evalTryExpression(node *parser.TryExpression, env *Environment) Object {
	val := Eval(node.Value, env)
	if isError(val) {
		return val
	}
	h, ok := val.(*Hash)
	if !ok {
		return newError("try expects a Result hash, got %s", val.Type())
	}
	okFlag := h.Get("ok")
	b, isBool := okFlag.(*Boolean)
	if !isBool {
		return newError("try expects Result with boolean ok field")
	}
	if b.Value {
		return h.Get("value")
	}
	return &ReturnValue{Value: h}
}

func evalPipeExpression(node *parser.PipeExpression, env *Environment) Object {
	left := Eval(node.Left, env)
	if isError(left) {
		return left
	}
	switch right := node.Right.(type) {
	case *parser.CallExpression:
		fn := Eval(right.Function, env)
		if isError(fn) {
			return fn
		}
		args := evalExpressions(right.Arguments, env)
		if len(args) == 1 && isError(args[0]) {
			return args[0]
		}
		piped := make([]Object, 0, len(args)+1)
		piped = append(piped, left)
		piped = append(piped, args...)
		return applyFunction(fn, piped)
	default:
		fn := Eval(node.Right, env)
		if isError(fn) {
			return fn
		}
		return applyFunction(fn, []Object{left})
	}
}

func evalMemberExpression(node *parser.MemberExpression, env *Environment) Object {
	left := Eval(node.Left, env)
	if isError(left) {
		return left
	}
	h, ok := left.(*Hash)
	if !ok {
		return newError("member access on non-hash: %s", left.Type())
	}
	return h.Get(node.Field.Value)
}

func unwrapReturnValue(obj Object) Object {
	if returnValue, ok := obj.(*ReturnValue); ok {
		return returnValue.Value
	}
	return obj
}

func isTruthy(obj Object) bool {
	switch obj {
	case NULL:
		return false
	case TRUE:
		return true
	case FALSE:
		return false
	default:
		if i, ok := obj.(*Integer); ok {
			return i.Value != 0
		}
		if s, ok := obj.(*String); ok {
			return s.Value != ""
		}
		return true
	}
}

func newError(format string, a ...interface{}) *Error {
	return &Error{Message: fmt.Sprintf(format, a...)}
}

func isError(obj Object) bool {
	if obj != nil {
		return obj.Type() == ErrorObj
	}
	return false
}

// EvalSource lexes, parses, and evaluates source text.
func EvalSource(source string, env *Environment) Object {
	l := lexer.New(source)
	p := parser.New(l)
	program := p.ParseProgram()
	if len(p.Errors()) > 0 {
		return newError("parse error: %s", strings.Join(p.Errors(), "; "))
	}
	return Eval(program, env)
}

// EvalFile loads and evaluates a .nex file.
func EvalFile(path string, env *Environment) Object {
	data, err := os.ReadFile(path)
	if err != nil {
		return newError("cannot read %s: %s", path, err)
	}
	abs, err := filepath.Abs(path)
	if err == nil {
		env.Set("__file__", &String{Value: abs})
		env.Set("__dir__", &String{Value: filepath.Dir(abs)})
	}
	return EvalSource(string(data), env)
}
