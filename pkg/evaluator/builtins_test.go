package evaluator

import "testing"

// merge/escape_html/typeof/get exist in the TypeScript host; the Go runtime was
// missing them, which made `nex run` fail to boot the registry app with
// "identifier not found: merge" (stdlib/design.nex calls merge).
func TestMergeBuiltin(t *testing.T) {
	t.Run("later keys win and inputs are not mutated", func(t *testing.T) {
		evaluated := testEval(`
			let a = { "x": 1, "shared": "from_a" };
			let b = { "y": 2, "shared": "from_b" };
			let m = merge(a, b);
			[m["x"], m["y"], m["shared"], a["shared"], len(a), len(b), len(m)]
		`)
		arr, ok := evaluated.(*Array)
		if !ok {
			t.Fatalf("expected Array, got %T (%s)", evaluated, evaluated.Inspect())
		}
		testIntegerObject(t, arr.Elements[0], 1)
		testIntegerObject(t, arr.Elements[1], 2)
		if got := arr.Elements[2].Inspect(); got != "from_b" {
			t.Errorf("later key should win, got %q", got)
		}
		if got := arr.Elements[3].Inspect(); got != "from_a" {
			t.Errorf("merge must not mutate its first argument, got %q", got)
		}
		testIntegerObject(t, arr.Elements[4], 2) // len(a)
		testIntegerObject(t, arr.Elements[5], 2) // len(b)
		testIntegerObject(t, arr.Elements[6], 3) // len(m): x, y, shared
	})

	t.Run("single hash", func(t *testing.T) {
		evaluated := testEval(`len(merge({ "a": 1 }))`)
		testIntegerObject(t, evaluated, 1)
	})

	t.Run("design_node shape from stdlib/design.nex", func(t *testing.T) {
		// Mirrors: let base = { "_design": kind }; return merge(base, props);
		evaluated := testEval(`merge({ "_design": "card" }, { "title": "Hi" })["_design"]`)
		if got := evaluated.Inspect(); got != "card" {
			t.Errorf("expected card, got %q", got)
		}
	})

	t.Run("errors", func(t *testing.T) {
		for input, want := range map[string]string{
			`merge()`:              "merge expects one or more hashes",
			`merge({ "a": 1 }, 2)`: "merge expects hashes",
			`merge("nope")`:        "merge expects hashes",
		} {
			err, ok := testEval(input).(*Error)
			if !ok {
				t.Fatalf("%s: expected Error, got %T", input, testEval(input))
			}
			if err.Message != want {
				t.Errorf("%s: got %q, want %q", input, err.Message, want)
			}
		}
	})
}

func TestEscapeHTMLBuiltin(t *testing.T) {
	evaluated := testEval(`escape_html("<a href='x'>T&C</a>")`)
	want := "&lt;a href=&#39;x&#39;&gt;T&amp;C&lt;/a&gt;"
	if got := evaluated.Inspect(); got != want {
		t.Errorf("got %q, want %q", got, want)
	}

	// Ampersands introduced by escaping must not be escaped a second time.
	if got := testEval(`escape_html("&")`).Inspect(); got != "&amp;" {
		t.Errorf("double-escaped: got %q", got)
	}

	if _, ok := testEval(`escape_html(1)`).(*Error); !ok {
		t.Error("expected error for non-string argument")
	}
}

func TestTypeofBuiltin(t *testing.T) {
	for input, want := range map[string]string{
		`typeof(1)`:          "INTEGER",
		`typeof("s")`:        "STRING",
		`typeof(true)`:       "BOOLEAN",
		`typeof([1])`:        "ARRAY",
		`typeof({ "a": 1 })`: "HASH",
	} {
		if got := testEval(input).Inspect(); got != want {
			t.Errorf("%s: got %q, want %q", input, got, want)
		}
	}
}

func TestGetBuiltin(t *testing.T) {
	testIntegerObject(t, testEval(`get({ "a": 1 }, "a")`), 1)
	testNullObject(t, testEval(`get({ "a": 1 }, "missing")`))

	if _, ok := testEval(`get([1, 2], 0)`).(*Error); !ok {
		t.Error("expected error when first argument is not a hash")
	}
}

// BuiltinNames backs OpGetBuiltin indices in compiled bytecode, so existing
// entries must keep their positions when new builtins are appended.
func TestBuiltinNamesIndicesAreStable(t *testing.T) {
	head := []string{
		"len", "puts", "str", "int", "type", "push", "first", "last", "rest",
		"keys", "has", "contains", "split", "join", "trim", "lower", "upper",
		"starts_with", "replace", "slice", "ok", "err", "is_ok", "is_err", "unwrap",
		"map", "filter", "assert", "assert_eq", "getenv",
	}
	if len(BuiltinNames) < len(head) {
		t.Fatalf("BuiltinNames shrank: got %d entries", len(BuiltinNames))
	}
	for i, name := range head {
		if BuiltinNames[i] != name {
			t.Errorf("index %d moved: got %q, want %q", i, BuiltinNames[i], name)
		}
	}
	for _, name := range BuiltinNames {
		if _, ok := GetBuiltinByName(name); !ok {
			t.Errorf("%q is listed in BuiltinNames but has no implementation", name)
		}
	}
}
