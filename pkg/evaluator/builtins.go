package evaluator

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// BuiltinNames is the stable index order used by the bytecode VM (OpGetBuiltin).
// Append new names to the end: existing indices are baked into compiled bytecode.
var BuiltinNames = []string{
	"len", "puts", "str", "int", "type", "push", "first", "last", "rest",
	"keys", "has", "contains", "split", "join", "trim", "lower", "upper",
	"starts_with", "replace", "slice", "ok", "err", "is_ok", "is_err", "unwrap",
	"map", "filter", "assert", "assert_eq", "getenv",
	"typeof", "get", "escape_html", "merge",
}

// GetBuiltinByName returns a core builtin by name.
func GetBuiltinByName(name string) (*Builtin, bool) {
	b, ok := builtins[name]
	return b, ok
}

// GetBuiltinByIndex returns a core builtin by stable index.
func GetBuiltinByIndex(index int) (*Builtin, bool) {
	if index < 0 || index >= len(BuiltinNames) {
		return nil, false
	}
	return GetBuiltinByName(BuiltinNames[index])
}

var builtins = map[string]*Builtin{
	"len": {
		Fn: func(args ...Object) Object {
			if len(args) != 1 {
				return newError("wrong number of arguments. got=%d, want=1", len(args))
			}
			switch arg := args[0].(type) {
			case *String:
				return &Integer{Value: int64(len(arg.Value))}
			case *Array:
				return &Integer{Value: int64(len(arg.Elements))}
			case *Hash:
				return &Integer{Value: int64(len(arg.Pairs))}
			default:
				return newError("argument to `len` not supported, got %s", args[0].Type())
			}
		},
	},
	"puts": {
		Fn: func(args ...Object) Object {
			for _, arg := range args {
				fmt.Fprintln(os.Stdout, arg.Inspect())
			}
			return NULL
		},
	},
	"str": {
		Fn: func(args ...Object) Object {
			if len(args) != 1 {
				return newError("wrong number of arguments. got=%d, want=1", len(args))
			}
			return &String{Value: args[0].Inspect()}
		},
	},
	"int": {
		Fn: func(args ...Object) Object {
			if len(args) != 1 {
				return newError("wrong number of arguments. got=%d, want=1", len(args))
			}
			switch a := args[0].(type) {
			case *Integer:
				return a
			case *String:
				s := strings.TrimSpace(a.Value)
				if s == "" {
					return &Integer{Value: 0}
				}
				n, err := strconv.ParseInt(s, 10, 64)
				if err != nil {
					return &Integer{Value: 0}
				}
				return &Integer{Value: n}
			case *Boolean:
				if a.Value {
					return &Integer{Value: 1}
				}
				return &Integer{Value: 0}
			case *Null:
				return &Integer{Value: 0}
			default:
				return newError("cannot convert %s to int", a.Type())
			}
		},
	},
	"type": {
		Fn: func(args ...Object) Object {
			if len(args) != 1 {
				return newError("wrong number of arguments. got=%d, want=1", len(args))
			}
			return &String{Value: string(args[0].Type())}
		},
	},
	"push": {
		Fn: func(args ...Object) Object {
			if len(args) < 2 {
				return newError("wrong number of arguments. got=%d, want>=2", len(args))
			}
			arr, ok := args[0].(*Array)
			if !ok {
				return newError("argument to `push` must be ARRAY, got %s", args[0].Type())
			}
			newElems := make([]Object, len(arr.Elements), len(arr.Elements)+len(args)-1)
			copy(newElems, arr.Elements)
			newElems = append(newElems, args[1:]...)
			return &Array{Elements: newElems}
		},
	},
	"first": {
		Fn: func(args ...Object) Object {
			if len(args) != 1 {
				return newError("wrong number of arguments. got=%d, want=1", len(args))
			}
			arr, ok := args[0].(*Array)
			if !ok {
				return newError("argument to `first` must be ARRAY")
			}
			if len(arr.Elements) == 0 {
				return NULL
			}
			return arr.Elements[0]
		},
	},
	"last": {
		Fn: func(args ...Object) Object {
			if len(args) != 1 {
				return newError("wrong number of arguments. got=%d, want=1", len(args))
			}
			arr, ok := args[0].(*Array)
			if !ok {
				return newError("argument to `last` must be ARRAY")
			}
			if len(arr.Elements) == 0 {
				return NULL
			}
			return arr.Elements[len(arr.Elements)-1]
		},
	},
	"rest": {
		Fn: func(args ...Object) Object {
			if len(args) != 1 {
				return newError("wrong number of arguments. got=%d, want=1", len(args))
			}
			arr, ok := args[0].(*Array)
			if !ok {
				return newError("argument to `rest` must be ARRAY")
			}
			if len(arr.Elements) == 0 {
				return &Array{Elements: []Object{}}
			}
			return &Array{Elements: arr.Elements[1:]}
		},
	},
	"keys": {
		Fn: func(args ...Object) Object {
			if len(args) != 1 {
				return newError("wrong number of arguments. got=%d, want=1", len(args))
			}
			hash, ok := args[0].(*Hash)
			if !ok {
				return newError("argument to `keys` must be HASH")
			}
			keys := make([]Object, 0, len(hash.Pairs))
			for _, pair := range hash.Pairs {
				keys = append(keys, pair.Key)
			}
			return &Array{Elements: keys}
		},
	},
	"has": {
		Fn: func(args ...Object) Object {
			if len(args) != 2 {
				return newError("wrong number of arguments. got=%d, want=2", len(args))
			}
			hash, ok := args[0].(*Hash)
			if !ok {
				return newError("argument to `has` must be HASH")
			}
			key, ok := args[1].(Hashable)
			if !ok {
				return newError("unusable as hash key: %s", args[1].Type())
			}
			_, found := hash.Pairs[key.HashKey()]
			return nativeBoolToBooleanObject(found)
		},
	},
	"contains": {
		Fn: func(args ...Object) Object {
			if len(args) != 2 {
				return newError("wrong number of arguments. got=%d, want=2", len(args))
			}
			switch hay := args[0].(type) {
			case *String:
				needle, ok := args[1].(*String)
				if !ok {
					return newError("needle must be STRING")
				}
				return nativeBoolToBooleanObject(strings.Contains(hay.Value, needle.Value))
			case *Array:
				for _, el := range hay.Elements {
					if objectsEqual(el, args[1]) {
						return TRUE
					}
				}
				return FALSE
			default:
				return newError("contains not supported on %s", args[0].Type())
			}
		},
	},
	"split": {
		Fn: func(args ...Object) Object {
			if len(args) != 2 {
				return newError("wrong number of arguments. got=%d, want=2", len(args))
			}
			s, ok1 := args[0].(*String)
			sep, ok2 := args[1].(*String)
			if !ok1 || !ok2 {
				return newError("split expects (string, string)")
			}
			parts := strings.Split(s.Value, sep.Value)
			out := make([]Object, len(parts))
			for i, p := range parts {
				out[i] = &String{Value: p}
			}
			return &Array{Elements: out}
		},
	},
	"join": {
		Fn: func(args ...Object) Object {
			if len(args) != 2 {
				return newError("wrong number of arguments. got=%d, want=2", len(args))
			}
			arr, ok1 := args[0].(*Array)
			sep, ok2 := args[1].(*String)
			if !ok1 || !ok2 {
				return newError("join expects (array, string)")
			}
			parts := make([]string, len(arr.Elements))
			for i, el := range arr.Elements {
				parts[i] = el.Inspect()
			}
			return &String{Value: strings.Join(parts, sep.Value)}
		},
	},
	"trim": {
		Fn: func(args ...Object) Object {
			if len(args) != 1 {
				return newError("wrong number of arguments. got=%d, want=1", len(args))
			}
			s, ok := args[0].(*String)
			if !ok {
				return newError("trim expects string")
			}
			return &String{Value: strings.TrimSpace(s.Value)}
		},
	},
	"lower": {
		Fn: func(args ...Object) Object {
			if len(args) != 1 {
				return newError("wrong number of arguments. got=%d, want=1", len(args))
			}
			s, ok := args[0].(*String)
			if !ok {
				return newError("lower expects string")
			}
			return &String{Value: strings.ToLower(s.Value)}
		},
	},
	"upper": {
		Fn: func(args ...Object) Object {
			if len(args) != 1 {
				return newError("wrong number of arguments. got=%d, want=1", len(args))
			}
			s, ok := args[0].(*String)
			if !ok {
				return newError("upper expects string")
			}
			return &String{Value: strings.ToUpper(s.Value)}
		},
	},
	"starts_with": {
		Fn: func(args ...Object) Object {
			if len(args) != 2 {
				return newError("wrong number of arguments. got=%d, want=2", len(args))
			}
			s, ok1 := args[0].(*String)
			p, ok2 := args[1].(*String)
			if !ok1 || !ok2 {
				return newError("starts_with expects (string, string)")
			}
			return nativeBoolToBooleanObject(strings.HasPrefix(s.Value, p.Value))
		},
	},
	"replace": {
		Fn: func(args ...Object) Object {
			if len(args) != 3 {
				return newError("wrong number of arguments. got=%d, want=3", len(args))
			}
			s, ok1 := args[0].(*String)
			old, ok2 := args[1].(*String)
			neu, ok3 := args[2].(*String)
			if !ok1 || !ok2 || !ok3 {
				return newError("replace expects (string, string, string)")
			}
			return &String{Value: strings.ReplaceAll(s.Value, old.Value, neu.Value)}
		},
	},
	"slice": {
		Fn: func(args ...Object) Object {
			if len(args) < 2 || len(args) > 3 {
				return newError("wrong number of arguments. got=%d, want=2 or 3", len(args))
			}
			start, ok := args[1].(*Integer)
			if !ok {
				return newError("slice start must be INTEGER")
			}
			switch a := args[0].(type) {
			case *Array:
				end := int64(len(a.Elements))
				if len(args) == 3 {
					e, ok := args[2].(*Integer)
					if !ok {
						return newError("slice end must be INTEGER")
					}
					end = e.Value
				}
				if start.Value < 0 {
					start.Value = 0
				}
				if end > int64(len(a.Elements)) {
					end = int64(len(a.Elements))
				}
				if start.Value >= end {
					return &Array{Elements: []Object{}}
				}
				return &Array{Elements: a.Elements[start.Value:end]}
			case *String:
				end := int64(len(a.Value))
				if len(args) == 3 {
					e, ok := args[2].(*Integer)
					if !ok {
						return newError("slice end must be INTEGER")
					}
					end = e.Value
				}
				if start.Value < 0 {
					start.Value = 0
				}
				if end > int64(len(a.Value)) {
					end = int64(len(a.Value))
				}
				if start.Value >= end {
					return &String{Value: ""}
				}
				return &String{Value: a.Value[start.Value:end]}
			default:
				return newError("slice not supported on %s", args[0].Type())
			}
		},
	},
	// Result helpers: ok(v) / err(e) / is_ok(r) / is_err(r) / unwrap(r)
	"ok": {
		Fn: func(args ...Object) Object {
			if len(args) != 1 {
				return newError("wrong number of arguments. got=%d, want=1", len(args))
			}
			h := NewHash()
			h.SetString("ok", TRUE)
			h.SetString("value", args[0])
			h.SetString("error", NULL)
			return h
		},
	},
	"err": {
		Fn: func(args ...Object) Object {
			if len(args) != 1 {
				return newError("wrong number of arguments. got=%d, want=1", len(args))
			}
			h := NewHash()
			h.SetString("ok", FALSE)
			h.SetString("value", NULL)
			h.SetString("error", args[0])
			return h
		},
	},
	"is_ok": {
		Fn: func(args ...Object) Object {
			if len(args) != 1 {
				return newError("wrong number of arguments. got=%d, want=1", len(args))
			}
			h, ok := args[0].(*Hash)
			if !ok {
				return FALSE
			}
			flag, ok := h.Get("ok").(*Boolean)
			return nativeBoolToBooleanObject(ok && flag.Value)
		},
	},
	"is_err": {
		Fn: func(args ...Object) Object {
			if len(args) != 1 {
				return newError("wrong number of arguments. got=%d, want=1", len(args))
			}
			h, ok := args[0].(*Hash)
			if !ok {
				return FALSE
			}
			flag, ok := h.Get("ok").(*Boolean)
			return nativeBoolToBooleanObject(ok && !flag.Value)
		},
	},
	"unwrap": {
		Fn: func(args ...Object) Object {
			if len(args) != 1 {
				return newError("wrong number of arguments. got=%d, want=1", len(args))
			}
			h, ok := args[0].(*Hash)
			if !ok {
				return newError("unwrap expects Result hash")
			}
			flag, ok := h.Get("ok").(*Boolean)
			if !ok {
				return newError("unwrap expects Result with ok field")
			}
			if flag.Value {
				return h.Get("value")
			}
			return newError("unwrap on Err: %s", h.Get("error").Inspect())
		},
	},
	"assert": {
		Fn: func(args ...Object) Object {
			if len(args) < 1 || len(args) > 2 {
				return newError("wrong number of arguments. got=%d, want=1 or 2", len(args))
			}
			msg := "assertion failed"
			if len(args) == 2 {
				if s, ok := args[1].(*String); ok {
					msg = s.Value
				} else {
					msg = args[1].Inspect()
				}
			}
			if !isTruthy(args[0]) {
				return newError("%s", msg)
			}
			return TRUE
		},
	},
	"assert_eq": {
		Fn: func(args ...Object) Object {
			if len(args) < 2 || len(args) > 3 {
				return newError("wrong number of arguments. got=%d, want=2 or 3", len(args))
			}
			if objectsEqual(args[0], args[1]) {
				return TRUE
			}
			msg := fmt.Sprintf("assert_eq failed: got %s, want %s", args[0].Inspect(), args[1].Inspect())
			if len(args) == 3 {
				if s, ok := args[2].(*String); ok {
					msg = s.Value + ": " + msg
				}
			}
			return newError("%s", msg)
		},
	},
	"getenv": {
		Fn: func(args ...Object) Object {
			if len(args) != 1 {
				return newError("wrong number of arguments. got=%d, want=1", len(args))
			}
			key, ok := args[0].(*String)
			if !ok {
				return newError("getenv expects string")
			}
			return &String{Value: os.Getenv(key.Value)}
		},
	},
	// `type` is a keyword in source position, so call sites use `typeof`.
	"typeof": {
		Fn: func(args ...Object) Object {
			if len(args) != 1 {
				return newError("wrong number of arguments. got=%d, want=1", len(args))
			}
			return &String{Value: string(args[0].Type())}
		},
	},
	"get": {
		Fn: func(args ...Object) Object {
			if len(args) != 2 {
				return newError("get expects (hash, key)")
			}
			hash, ok := args[0].(*Hash)
			if !ok {
				return newError("get expects (hash, key)")
			}
			key, ok := args[1].(Hashable)
			if !ok {
				return NULL
			}
			if pair, found := hash.Pairs[key.HashKey()]; found {
				return pair.Value
			}
			return NULL
		},
	},
	"escape_html": {
		Fn: func(args ...Object) Object {
			if len(args) != 1 {
				return newError("escape_html expects string")
			}
			s, ok := args[0].(*String)
			if !ok {
				return newError("escape_html expects string")
			}
			return &String{Value: htmlEscaper.Replace(s.Value)}
		},
	},
	// Shallow left-to-right hash merge; later keys win. Inputs are not mutated.
	"merge": {
		Fn: func(args ...Object) Object {
			if len(args) < 1 {
				return newError("merge expects one or more hashes")
			}
			out := NewHash()
			for _, arg := range args {
				h, ok := arg.(*Hash)
				if !ok {
					return newError("merge expects hashes")
				}
				for k, pair := range h.Pairs {
					out.Pairs[k] = pair
				}
			}
			return out
		},
	},
}

// Mirrors the escaping performed by the TypeScript host's escape_html.
var htmlEscaper = strings.NewReplacer(
	"&", "&amp;",
	"<", "&lt;",
	">", "&gt;",
	`"`, "&quot;",
	"'", "&#39;",
)

func init() {
	// Registered in init() to avoid a package init cycle with ApplyFunction ↔ builtins.
	builtins["map"] = &Builtin{
		Fn: func(args ...Object) Object {
			if len(args) != 2 {
				return newError("wrong number of arguments. got=%d, want=2", len(args))
			}
			arr, ok := args[0].(*Array)
			if !ok {
				return newError("map expects (array, fn)")
			}
			out := make([]Object, 0, len(arr.Elements))
			for _, el := range arr.Elements {
				mapped := ApplyFunction(args[1], []Object{el})
				if isError(mapped) {
					return mapped
				}
				out = append(out, mapped)
			}
			return &Array{Elements: out}
		},
	}
	builtins["filter"] = &Builtin{
		Fn: func(args ...Object) Object {
			if len(args) != 2 {
				return newError("wrong number of arguments. got=%d, want=2", len(args))
			}
			arr, ok := args[0].(*Array)
			if !ok {
				return newError("filter expects (array, fn)")
			}
			out := make([]Object, 0)
			for _, el := range arr.Elements {
				keep := ApplyFunction(args[1], []Object{el})
				if isError(keep) {
					return keep
				}
				if isTruthy(keep) {
					out = append(out, el)
				}
			}
			return &Array{Elements: out}
		},
	}
}
