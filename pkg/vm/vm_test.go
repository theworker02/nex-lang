package vm

import (
	"testing"

	"nex-lang/pkg/compiler"
	"nex-lang/pkg/evaluator"
	"nex-lang/pkg/lexer"
	"nex-lang/pkg/parser"
)

func TestIntegerArithmetic(t *testing.T) {
	tests := []vmTestCase{
		{"1", 1},
		{"2", 2},
		{"1 + 2", 3},
		{"1 - 2", -1},
		{"1 * 2", 2},
		{"4 / 2", 2},
		{"50 / 2 * 2 + 10 - 5", 55},
		{"5 % 2", 1},
		{"-5", -5},
		{"-50 + 100 + -50", 0},
	}
	runVmTests(t, tests)
}

func TestBooleanExpressions(t *testing.T) {
	tests := []vmTestCase{
		{"true", true},
		{"false", false},
		{"1 < 2", true},
		{"1 > 2", false},
		{"1 == 1", true},
		{"1 != 2", true},
		{"true == true", true},
		{"!true", false},
		{"!!true", true},
		{"1 <= 1", true},
		{"2 >= 3", false},
	}
	runVmTests(t, tests)
}

func TestConditionals(t *testing.T) {
	tests := []vmTestCase{
		{"if (true) { 10 }", 10},
		{"if (true) { 10 } else { 20 }", 10},
		{"if (false) { 10 } else { 20 }", 20},
		{"if (1) { 10 }", 10},
		{"if (1 < 2) { 10 } else { 20 }", 10},
		{"if (false) { 10 }", nil},
	}
	runVmTests(t, tests)
}

func TestGlobalLetStatements(t *testing.T) {
	tests := []vmTestCase{
		{"let one = 1; one", 1},
		{"let one = 1; let two = 2; one + two", 3},
		{"let one = 1; let two = one + one; one + two", 3},
	}
	runVmTests(t, tests)
}

func TestStringExpressions(t *testing.T) {
	tests := []vmTestCase{
		{`"nex"`, "nex"},
		{`"ne" + "x"`, "nex"},
	}
	runVmTests(t, tests)
}

func TestArrayLiterals(t *testing.T) {
	tests := []vmTestCase{
		{"[]", []int{}},
		{"[1, 2, 3]", []int{1, 2, 3}},
		{"[1 + 2, 3 * 4]", []int{3, 12}},
	}
	runVmTests(t, tests)
}

func TestHashLiterals(t *testing.T) {
	tests := []vmTestCase{
		{"{}", map[string]int64{}},
		{`{"n": 1 + 1}`, map[string]int64{"n": 2}},
	}
	runVmTests(t, tests)
}

func TestIndexExpressions(t *testing.T) {
	tests := []vmTestCase{
		{"[1, 2, 3][1]", 2},
		{`{"foo": 5}["foo"]`, 5},
	}
	runVmTests(t, tests)
}

func TestCallingFunctions(t *testing.T) {
	tests := []vmTestCase{
		{"let fivePlusTen = fn() { 5 + 10; }; fivePlusTen();", 15},
		{"let one = fn() { 1; }; let two = fn() { 2; }; one() + two()", 3},
		{"let early = fn() { return 99; 100; }; early();", 99},
		{"let id = fn(x) { x; }; id(42);", 42},
		{"let add = fn(a, b) { a + b; }; add(1, 2);", 3},
	}
	runVmTests(t, tests)
}

func TestClosures(t *testing.T) {
	tests := []vmTestCase{
		{
			`
			let newAdder = fn(a) {
			  fn(b) { a + b };
			};
			let addTwo = newAdder(2);
			addTwo(3);
			`,
			5,
		},
	}
	runVmTests(t, tests)
}

func TestBuiltinFunctions(t *testing.T) {
	tests := []vmTestCase{
		{`len("")`, 0},
		{`len("four")`, 4},
		{`len([1, 2, 3])`, 3},
	}
	runVmTests(t, tests)
}

func TestWhileLoop(t *testing.T) {
	tests := []vmTestCase{
		{
			`
			let i = 0;
			let sum = 0;
			while (i < 5) {
			  sum = sum + i;
			  i = i + 1;
			};
			sum
			`,
			10,
		},
	}
	runVmTests(t, tests)
}

func TestStructsAndMatch(t *testing.T) {
	tests := []vmTestCase{
		{
			`
			struct Point { x, y };
			let p = Point(3, 4);
			p.x + p.y
			`,
			7,
		},
		{
			`
			let classify = fn(n) {
			  return match (n) {
			    0 -> 0,
			    1 -> 1,
			    _ -> n * n
			  };
			};
			classify(4)
			`,
			16,
		},
	}
	runVmTests(t, tests)
}

func TestPipes(t *testing.T) {
	tests := []vmTestCase{
		{
			`
			let add1 = fn(x) { x + 1 };
			5 |> add1
			`,
			6,
		},
	}
	runVmTests(t, tests)
}

type vmTestCase struct {
	input    string
	expected interface{}
}

func runVmTests(t *testing.T, tests []vmTestCase) {
	t.Helper()
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			program := parse(t, tt.input)
			comp := compiler.New()
			if err := comp.Compile(program); err != nil {
				t.Fatalf("compiler error: %s", err)
			}
			machine := New(comp.Bytecode())
			if err := machine.Run(); err != nil {
				t.Fatalf("vm error: %s", err)
			}
			stackElem := machine.LastPoppedStackElem()
			testExpectedObject(t, tt.expected, stackElem)
		})
	}
}

func parse(t *testing.T, input string) *parser.Program {
	t.Helper()
	l := lexer.New(input)
	p := parser.New(l)
	program := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parse errors: %v", p.Errors())
	}
	return program
}

func testExpectedObject(t *testing.T, expected interface{}, actual evaluator.Object) {
	t.Helper()
	switch expected := expected.(type) {
	case int:
		testIntegerObject(t, actual, int64(expected))
	case int64:
		testIntegerObject(t, actual, expected)
	case bool:
		testBooleanObject(t, actual, expected)
	case string:
		s, ok := actual.(*evaluator.String)
		if !ok {
			t.Fatalf("object is not String: %T (%+v)", actual, actual)
		}
		if s.Value != expected {
			t.Fatalf("string wrong. want=%q got=%q", expected, s.Value)
		}
	case []int:
		arr, ok := actual.(*evaluator.Array)
		if !ok {
			t.Fatalf("object not Array: %T (%+v)", actual, actual)
		}
		if len(arr.Elements) != len(expected) {
			t.Fatalf("array len wrong. want=%d got=%d", len(expected), len(arr.Elements))
		}
		for i, el := range expected {
			testIntegerObject(t, arr.Elements[i], int64(el))
		}
	case map[string]int64:
		hash, ok := actual.(*evaluator.Hash)
		if !ok {
			t.Fatalf("object is not Hash: %T (%+v)", actual, actual)
		}
		if len(hash.Pairs) != len(expected) {
			t.Fatalf("hash has wrong num pairs. want=%d got=%d", len(expected), len(hash.Pairs))
		}
		for k, want := range expected {
			pair, ok := hash.Pairs[(&evaluator.String{Value: k}).HashKey()]
			if !ok {
				t.Fatalf("no pair for key %q", k)
			}
			testIntegerObject(t, pair.Value, want)
		}
	case nil:
		if actual != evaluator.NULL && actual != nil {
			if _, ok := actual.(*evaluator.Null); !ok {
				t.Fatalf("expected null, got %T (%s)", actual, actual.Inspect())
			}
		}
	default:
		t.Fatalf("unhandled expected type %T", expected)
	}
}

func testIntegerObject(t *testing.T, obj evaluator.Object, expected int64) {
	t.Helper()
	result, ok := obj.(*evaluator.Integer)
	if !ok {
		t.Fatalf("object is not Integer. got=%T (%+v)", obj, obj)
	}
	if result.Value != expected {
		t.Fatalf("object has wrong value. got=%d want=%d", result.Value, expected)
	}
}

func testBooleanObject(t *testing.T, obj evaluator.Object, expected bool) {
	t.Helper()
	result, ok := obj.(*evaluator.Boolean)
	if !ok {
		t.Fatalf("object is not Boolean. got=%T (%+v)", obj, obj)
	}
	if result.Value != expected {
		t.Fatalf("object has wrong value. got=%t want=%t", result.Value, expected)
	}
}
