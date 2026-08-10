package runtime

import (
	"os"
	"path/filepath"
	"testing"

	"nex-lang/pkg/evaluator"
)

func TestResolveLocalAndModules(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "helper.nex"), []byte("let helper_val = 42;\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	modDir := filepath.Join(root, ModulesDir, "demo")
	if err := os.MkdirAll(modDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(modDir, "mod.nex"), []byte("let demo_val = 7;\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	rt := New(root, evaluator.NewEnvironment())
	env := rt.Env
	env.Set("__dir__", &evaluator.String{Value: root})

	full, err := rt.ResolvePath("helper.nex", env)
	if err != nil {
		t.Fatalf("resolve helper: %v", err)
	}
	if filepath.Base(full) != "helper.nex" {
		t.Fatalf("unexpected helper path %s", full)
	}

	full, err = rt.ResolvePath("demo", env)
	if err != nil {
		t.Fatalf("resolve demo package: %v", err)
	}
	if filepath.Base(full) != "mod.nex" {
		t.Fatalf("expected mod.nex, got %s", full)
	}
}

func TestImportEvaluatesModule(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "lib.nex"), []byte("let answer = 21;\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	entry := filepath.Join(root, "main.nex")
	if err := os.WriteFile(entry, []byte("import \"lib.nex\";\nanswer * 2\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	rt := New(root, evaluator.NewEnvironment())
	result := rt.LoadFile(entry)
	if errObj, ok := result.(*evaluator.Error); ok {
		t.Fatalf("eval error: %s", errObj.Message)
	}
	n, ok := result.(*evaluator.Integer)
	if !ok || n.Value != 42 {
		t.Fatalf("want 42, got %#v", result)
	}
}
