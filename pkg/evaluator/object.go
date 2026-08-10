package evaluator

import (
	"bytes"
	"fmt"
	"hash/fnv"
	"strings"

	"nex-lang/pkg/parser"
)

// ObjectType identifies a runtime value kind.
type ObjectType string

const (
	IntegerObj     ObjectType = "INTEGER"
	StringObj      ObjectType = "STRING"
	BooleanObj     ObjectType = "BOOLEAN"
	NullObj        ObjectType = "NULL"
	ReturnValueObj ObjectType = "RETURN_VALUE"
	ErrorObj       ObjectType = "ERROR"
	FunctionObj    ObjectType = "FUNCTION"
	BuiltinObj     ObjectType = "BUILTIN"
	ArrayObj       ObjectType = "ARRAY"
	HashObj        ObjectType = "HASH"
	BreakObj       ObjectType = "BREAK"
	ContinueObj    ObjectType = "CONTINUE"
)

// Object is any runtime value produced by the evaluator.
type Object interface {
	Type() ObjectType
	Inspect() string
}

// Hashable values can be used as hash keys.
type Hashable interface {
	HashKey() HashKey
}

// HashKey uniquely identifies a hashable value.
type HashKey struct {
	Type  ObjectType
	Value uint64
}

// Integer is a 64-bit integer value.
type Integer struct {
	Value int64
}

func (i *Integer) Type() ObjectType { return IntegerObj }
func (i *Integer) Inspect() string  { return fmt.Sprintf("%d", i.Value) }
func (i *Integer) HashKey() HashKey {
	return HashKey{Type: i.Type(), Value: uint64(i.Value)}
}

// String is a string value.
type String struct {
	Value string
}

func (s *String) Type() ObjectType { return StringObj }
func (s *String) Inspect() string  { return s.Value }
func (s *String) HashKey() HashKey {
	h := fnv.New64a()
	_, _ = h.Write([]byte(s.Value))
	return HashKey{Type: s.Type(), Value: h.Sum64()}
}

// Boolean is a boolean value.
type Boolean struct {
	Value bool
}

func (b *Boolean) Type() ObjectType { return BooleanObj }
func (b *Boolean) Inspect() string  { return fmt.Sprintf("%t", b.Value) }
func (b *Boolean) HashKey() HashKey {
	var value uint64
	if b.Value {
		value = 1
	}
	return HashKey{Type: b.Type(), Value: value}
}

// Null represents the absence of a value.
type Null struct{}

func (n *Null) Type() ObjectType { return NullObj }
func (n *Null) Inspect() string  { return "null" }

// ReturnValue wraps a value returned from a function.
type ReturnValue struct {
	Value Object
}

func (rv *ReturnValue) Type() ObjectType { return ReturnValueObj }
func (rv *ReturnValue) Inspect() string  { return rv.Value.Inspect() }

// BreakSignal exits a loop.
type BreakSignal struct{}

func (b *BreakSignal) Type() ObjectType { return BreakObj }
func (b *BreakSignal) Inspect() string  { return "break" }

// ContinueSignal continues a loop.
type ContinueSignal struct{}

func (c *ContinueSignal) Type() ObjectType { return ContinueObj }
func (c *ContinueSignal) Inspect() string  { return "continue" }

// Error is a runtime error object.
type Error struct {
	Message string
}

func (e *Error) Type() ObjectType { return ErrorObj }
func (e *Error) Inspect() string  { return "ERROR: " + e.Message }

// Function is a user-defined closure (tree-walk evaluator).
type Function struct {
	Parameters []*parser.Parameter
	ReturnType string
	Body       *parser.BlockStatement
	Env        *Environment
}

func (f *Function) Type() ObjectType { return FunctionObj }
func (f *Function) Inspect() string {
	params := make([]string, len(f.Parameters))
	for i, p := range f.Parameters {
		params[i] = p.String()
	}
	var out bytes.Buffer
	out.WriteString("fn")
	out.WriteString("(")
	out.WriteString(strings.Join(params, ", "))
	out.WriteString(") {\n")
	out.WriteString(f.Body.String())
	out.WriteString("\n}")
	return out.String()
}

const (
	CompiledFunctionObj ObjectType = "COMPILED_FUNCTION"
	ClosureObj          ObjectType = "CLOSURE"
)

// CompiledFunction is a bytecode function produced by the compiler.
type CompiledFunction struct {
	Instructions  []byte
	NumLocals     int
	NumParameters int
}

func (cf *CompiledFunction) Type() ObjectType { return CompiledFunctionObj }
func (cf *CompiledFunction) Inspect() string  { return "compiled function" }

// Closure wraps a compiled function with free variables.
type Closure struct {
	Fn   *CompiledFunction
	Free []Object
}

func (c *Closure) Type() ObjectType { return ClosureObj }
func (c *Closure) Inspect() string  { return "closure" }

// BuiltinFunction is a host-provided callable.
type BuiltinFunction func(args ...Object) Object

// Builtin wraps a host function.
type Builtin struct {
	Fn BuiltinFunction
}

func (b *Builtin) Type() ObjectType { return BuiltinObj }
func (b *Builtin) Inspect() string  { return "builtin function" }

// Array is a dynamic list.
type Array struct {
	Elements []Object
}

func (a *Array) Type() ObjectType { return ArrayObj }
func (a *Array) Inspect() string {
	elements := make([]string, len(a.Elements))
	for i, e := range a.Elements {
		elements[i] = e.Inspect()
	}
	return "[" + strings.Join(elements, ", ") + "]"
}

// HashPair is a key/value entry in a hash.
type HashPair struct {
	Key   Object
	Value Object
}

// Hash is a map from hashable keys to values.
type Hash struct {
	Pairs map[HashKey]HashPair
}

func (h *Hash) Type() ObjectType { return HashObj }
func (h *Hash) Inspect() string {
	pairs := []string{}
	for _, pair := range h.Pairs {
		pairs = append(pairs, fmt.Sprintf("%s: %s", pair.Key.Inspect(), pair.Value.Inspect()))
	}
	return "{" + strings.Join(pairs, ", ") + "}"
}

// Get returns the value for a string key, or NULL.
func (h *Hash) Get(key string) Object {
	hk := (&String{Value: key}).HashKey()
	if pair, ok := h.Pairs[hk]; ok {
		return pair.Value
	}
	return NULL
}

// SetString sets a string-keyed value.
func (h *Hash) SetString(key string, val Object) {
	k := &String{Value: key}
	h.Pairs[k.HashKey()] = HashPair{Key: k, Value: val}
}

// NewHash creates an empty hash.
func NewHash() *Hash {
	return &Hash{Pairs: make(map[HashKey]HashPair)}
}
