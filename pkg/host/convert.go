package host

import (
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	"nex-lang/pkg/evaluator"
)

// FromGo converts common Go values into Nexus objects.
func FromGo(v any) evaluator.Object {
	if v == nil {
		return evaluator.NULL
	}
	switch val := v.(type) {
	case evaluator.Object:
		return val
	case bool:
		if val {
			return evaluator.TRUE
		}
		return evaluator.FALSE
	case int:
		return &evaluator.Integer{Value: int64(val)}
	case int32:
		return &evaluator.Integer{Value: int64(val)}
	case int64:
		return &evaluator.Integer{Value: val}
	case uint64:
		return &evaluator.Integer{Value: int64(val)}
	case float64:
		return &evaluator.Integer{Value: int64(val)}
	case string:
		return &evaluator.String{Value: val}
	case []byte:
		return &evaluator.String{Value: string(val)}
	case time.Time:
		return &evaluator.String{Value: val.UTC().Format(time.RFC3339)}
	case *time.Time:
		if val == nil {
			return evaluator.NULL
		}
		return &evaluator.String{Value: val.UTC().Format(time.RFC3339)}
	case []string:
		arr := make([]evaluator.Object, len(val))
		for i, s := range val {
			arr[i] = &evaluator.String{Value: s}
		}
		return &evaluator.Array{Elements: arr}
	case map[string]any:
		h := evaluator.NewHash()
		for k, vv := range val {
			h.SetString(k, FromGo(vv))
		}
		return h
	case map[string]string:
		h := evaluator.NewHash()
		for k, vv := range val {
			h.SetString(k, &evaluator.String{Value: vv})
		}
		return h
	case json.RawMessage:
		var decoded any
		if err := json.Unmarshal(val, &decoded); err != nil {
			return &evaluator.String{Value: string(val)}
		}
		return FromGo(decoded)
	case error:
		return &evaluator.Error{Message: val.Error()}
	}

	rv := reflect.ValueOf(v)
	if !rv.IsValid() {
		return evaluator.NULL
	}
	if rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return evaluator.NULL
		}
		return FromGo(rv.Elem().Interface())
	}

	switch rv.Kind() {
	case reflect.Slice, reflect.Array:
		arr := make([]evaluator.Object, rv.Len())
		for i := 0; i < rv.Len(); i++ {
			arr[i] = FromGo(rv.Index(i).Interface())
		}
		return &evaluator.Array{Elements: arr}
	case reflect.Map:
		h := evaluator.NewHash()
		for _, key := range rv.MapKeys() {
			k := fmt.Sprint(key.Interface())
			h.SetString(k, FromGo(rv.MapIndex(key).Interface()))
		}
		return h
	case reflect.Struct:
		return structToHash(rv)
	default:
		return &evaluator.String{Value: fmt.Sprint(v)}
	}
}

func structToHash(rv reflect.Value) *evaluator.Hash {
	h := evaluator.NewHash()
	rt := rv.Type()
	for i := 0; i < rv.NumField(); i++ {
		field := rt.Field(i)
		if field.PkgPath != "" && !field.Anonymous { // unexported
			continue
		}
		fv := rv.Field(i)
		if !fv.CanInterface() {
			continue
		}
		// Promote anonymous embedded structs so templates see .Name, not .Package.Name.
		if field.Anonymous {
			for fv.Kind() == reflect.Ptr {
				if fv.IsNil() {
					fv = reflect.Value{}
					break
				}
				fv = fv.Elem()
			}
			if fv.IsValid() && fv.Kind() == reflect.Struct {
				inner := structToHash(fv)
				for _, pair := range inner.Pairs {
					key := pair.Key.Inspect()
					if s, ok := pair.Key.(*evaluator.String); ok {
						key = s.Value
					}
					h.SetString(key, pair.Value)
				}
				continue
			}
		}
		h.SetString(field.Name, FromGo(fv.Interface()))
	}
	return h
}

// ToGo converts a Nexus object into a Go value suitable for JSON/templates.
func ToGo(obj evaluator.Object) any {
	if obj == nil {
		return nil
	}
	switch v := obj.(type) {
	case *evaluator.Null:
		return nil
	case *evaluator.Boolean:
		return v.Value
	case *evaluator.Integer:
		return v.Value
	case *evaluator.String:
		return v.Value
	case *evaluator.Array:
		out := make([]any, len(v.Elements))
		for i, el := range v.Elements {
			out[i] = ToGo(el)
		}
		return out
	case *evaluator.Hash:
		out := make(map[string]any, len(v.Pairs))
		for _, pair := range v.Pairs {
			key := pair.Key.Inspect()
			if s, ok := pair.Key.(*evaluator.String); ok {
				key = s.Value
			}
			out[key] = ToGo(pair.Value)
		}
		return out
	case *evaluator.Error:
		return map[string]any{"error": v.Message}
	default:
		return v.Inspect()
	}
}

// AsString extracts a string from a Nexus object.
func AsString(obj evaluator.Object) (string, bool) {
	if s, ok := obj.(*evaluator.String); ok {
		return s.Value, true
	}
	return "", false
}

// AsInt extracts an int64 from a Nexus object.
func AsInt(obj evaluator.Object) (int64, bool) {
	if i, ok := obj.(*evaluator.Integer); ok {
		return i.Value, true
	}
	return 0, false
}

// AsBool extracts a bool from a Nexus object.
func AsBool(obj evaluator.Object) bool {
	switch obj {
	case evaluator.TRUE:
		return true
	case evaluator.FALSE, evaluator.NULL:
		return false
	default:
		if i, ok := obj.(*evaluator.Integer); ok {
			return i.Value != 0
		}
		if s, ok := obj.(*evaluator.String); ok {
			return s.Value != ""
		}
		return obj != nil && obj.Type() != evaluator.NullObj
	}
}

// HashGetString gets a string field from a hash.
func HashGetString(h *evaluator.Hash, key string) string {
	v := h.Get(key)
	if s, ok := v.(*evaluator.String); ok {
		return s.Value
	}
	if i, ok := v.(*evaluator.Integer); ok {
		return fmt.Sprintf("%d", i.Value)
	}
	if v == evaluator.NULL || v == nil {
		return ""
	}
	return v.Inspect()
}

// ExpectArgs checks arity.
func ExpectArgs(name string, n int, args []evaluator.Object) *evaluator.Error {
	if len(args) != n {
		return &evaluator.Error{Message: fmt.Sprintf("%s: wrong number of arguments. got=%d, want=%d", name, len(args), n)}
	}
	return nil
}

// ExpectMinArgs checks minimum arity.
func ExpectMinArgs(name string, n int, args []evaluator.Object) *evaluator.Error {
	if len(args) < n {
		return &evaluator.Error{Message: fmt.Sprintf("%s: wrong number of arguments. got=%d, want>=%d", name, len(args), n)}
	}
	return nil
}
