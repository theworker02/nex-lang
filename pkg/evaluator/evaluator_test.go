package evaluator

import (
	"strings"
	"testing"

	"nex-lang/pkg/lexer"
	"nex-lang/pkg/parser"
)

func TestEvalIntegerExpression(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
	}{
		{"5", 5},
		{"10", 10},
		{"-5", -5},
		{"5 + 5 + 5 + 5 - 10", 10},
		{"2 * 2 * 2 * 2 * 2", 32},
		{"5 * 2 + 10", 20},
		{"50 / 2 * 2 + 10", 60},
		{"(5 + 10 * 2 + 15 / 3) * 2 + -10", 50},
	}

	for _, tt := range tests {
		evaluated := testEval(tt.input)
		testIntegerObject(t, evaluated, tt.expected)
	}
}

func TestEvalBooleanExpression(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"true", true},
		{"false", false},
		{"1 < 2", true},
		{"1 > 2", false},
		{"1 == 1", true},
		{"1 != 1", false},
		{"true == true", true},
		{"false != true", true},
	}

	for _, tt := range tests {
		evaluated := testEval(tt.input)
		testBooleanObject(t, evaluated, tt.expected)
	}
}

func TestLetStatements(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
	}{
		{"let a = 5; a;", 5},
		{"let a = 5 * 5; a;", 25},
		{"let a = 5; let b = a; b;", 5},
		{"let a = 5; let b = a; let c = a + b + 5; c;", 15},
	}

	for _, tt := range tests {
		testIntegerObject(t, testEval(tt.input), tt.expected)
	}
}

func TestFunctionApplication(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
	}{
		{"let identity = fn(x) { x; }; identity(5);", 5},
		{"let identity = fn(x) { return x; }; identity(5);", 5},
		{"let double = fn(x) { x * 2; }; double(5);", 10},
		{"let add = fn(x, y) { x + y; }; add(5, 5);", 10},
		{"let add = fn(x, y) { x + y; }; add(5 + 5, add(5, 5));", 20},
		{"fn(x) { x; }(5)", 5},
	}

	for _, tt := range tests {
		testIntegerObject(t, testEval(tt.input), tt.expected)
	}
}

func TestIfElseExpressions(t *testing.T) {
	tests := []struct {
		input    string
		expected interface{}
	}{
		{"if (true) { 10 }", 10},
		{"if (false) { 10 }", nil},
		{"if (1 < 2) { 10 }", 10},
		{"if (1 > 2) { 10 } else { 20 }", 20},
	}

	for _, tt := range tests {
		evaluated := testEval(tt.input)
		integer, ok := tt.expected.(int)
		if ok {
			testIntegerObject(t, evaluated, int64(integer))
		} else {
			testNullObject(t, evaluated)
		}
	}
}

func TestStringConcatenation(t *testing.T) {
	input := `"Hello" + " " + "World!"`
	evaluated := testEval(input)
	str, ok := evaluated.(*String)
	if !ok {
		t.Fatalf("object is not String. got=%T (%+v)", evaluated, evaluated)
	}
	if str.Value != "Hello World!" {
		t.Errorf("String has wrong value. got=%q", str.Value)
	}
}

func TestErrorHandling(t *testing.T) {
	tests := []struct {
		input           string
		expectedMessage string
	}{
		{"5 + true;", "type mismatch: INTEGER + BOOLEAN"},
		{"foobar", "identifier not found: foobar"},
		{`"Hello" - "World"`, "unknown operator: STRING - STRING"},
	}

	for _, tt := range tests {
		evaluated := testEval(tt.input)
		errObj, ok := evaluated.(*Error)
		if !ok {
			t.Errorf("no error object returned. got=%T(%+v)", evaluated, evaluated)
			continue
		}
		if errObj.Message != tt.expectedMessage {
			t.Errorf("wrong error message. expected=%q, got=%q", tt.expectedMessage, errObj.Message)
		}
	}
}

func TestTypeAnnotations(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
	}{
		{"let n: int = 7; n;", 7},
		{"let add = fn(a: int, b: int) -> int { a + b }; add(3, 4);", 7},
	}
	for _, tt := range tests {
		testIntegerObject(t, testEval(tt.input), tt.expected)
	}

	errObj := testEval(`let n: int = "nope";`)
	if _, ok := errObj.(*Error); !ok {
		t.Fatalf("expected type error, got %T (%+v)", errObj, errObj)
	}
}

func TestMatchExpression(t *testing.T) {
	input := `
let grade = fn(n) {
  match (n) {
    10 -> 100,
    9 -> 90,
    _ -> 0
  }
};
grade(9);
`
	testIntegerObject(t, testEval(input), 90)
}

func TestStructAndMember(t *testing.T) {
	input := `
struct Point { x, y };
let p = Point(3, 4);
p.x + p.y;
`
	testIntegerObject(t, testEval(input), 7)
}

func TestPipeAndMap(t *testing.T) {
	input := `
let double = fn(x) { x * 2 };
5 |> double;
`
	testIntegerObject(t, testEval(input), 10)

	mapped := testEval(`[1, 2, 3] |> map(fn(x) { x + 1 });`)
	arr, ok := mapped.(*Array)
	if !ok || len(arr.Elements) != 3 {
		t.Fatalf("expected array of 3, got %T (%+v)", mapped, mapped)
	}
	testIntegerObject(t, arr.Elements[2], 4)
}

func TestResultTry(t *testing.T) {
	okVal := testEval(`unwrap(ok(42));`)
	testIntegerObject(t, okVal, 42)

	input := `
let parse = fn(n) {
  if (n > 0) { return ok(n); }
  return err("bad");
};
let run = fn(n) {
  let v = try parse(n);
  return v * 2;
};
run(5);
`
	testIntegerObject(t, testEval(input), 10)

	errRes := testEval(`
let parse = fn(n) {
  if (n > 0) { return ok(n); }
  return err("bad");
};
let run = fn(n) {
  let v = try parse(n);
  return v * 2;
};
run(0);
`)
	h, ok := errRes.(*Hash)
	if !ok {
		t.Fatalf("expected Result hash from try err, got %T", errRes)
	}
	flag, _ := h.Get("ok").(*Boolean)
	if flag == nil || flag.Value {
		t.Fatalf("expected Err result, got %s", errRes.Inspect())
	}
}

func testEval(input string) Object {
	l := lexer.New(input)
	p := parser.New(l)
	program := p.ParseProgram()
	if len(p.Errors()) > 0 {
		panic("parse errors: " + strings.Join(p.Errors(), "; "))
	}
	env := NewEnvironment()
	return Eval(program, env)
}

func testIntegerObject(t *testing.T, obj Object, expected int64) bool {
	t.Helper()
	result, ok := obj.(*Integer)
	if !ok {
		t.Errorf("object is not Integer. got=%T (%+v)", obj, obj)
		return false
	}
	if result.Value != expected {
		t.Errorf("object has wrong value. got=%d, want=%d", result.Value, expected)
		return false
	}
	return true
}

func testBooleanObject(t *testing.T, obj Object, expected bool) bool {
	t.Helper()
	result, ok := obj.(*Boolean)
	if !ok {
		t.Errorf("object is not Boolean. got=%T (%+v)", obj, obj)
		return false
	}
	if result.Value != expected {
		t.Errorf("object has wrong value. got=%t, want=%t", result.Value, expected)
		return false
	}
	return true
}

func testNullObject(t *testing.T, obj Object) bool {
	t.Helper()
	if obj != NULL {
		t.Errorf("object is not NULL. got=%T (%+v)", obj, obj)
		return false
	}
	return true
}
