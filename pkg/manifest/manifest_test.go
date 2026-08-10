package manifest

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseAndValidate(t *testing.T) {
	data := []byte(`
name = "demo"
version = "1.2.3"
author = "Ada"
description = "example"

[dependencies]
leftpad = "0.1.0"
`)

	m, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	if m.Name != "demo" || m.Version != "1.2.3" || m.Author != "Ada" {
		t.Fatalf("unexpected manifest: %+v", m)
	}
	if m.Dependencies["leftpad"] != "0.1.0" {
		t.Fatalf("unexpected dependencies: %+v", m.Dependencies)
	}
}

func TestValidateRejectsBadName(t *testing.T) {
	m := &Manifest{Name: "1bad", Version: "0.1.0", Author: "x"}
	if err := m.Validate(); err == nil {
		t.Fatal("expected validation error for bad name")
	}
}

func TestWriteAndLoad(t *testing.T) {
	dir := t.TempDir()
	m := Default("sample", "tester")
	m.Dependencies["other"] = "2.0.0"

	if err := m.WriteToDir(dir); err != nil {
		t.Fatalf("WriteToDir() error: %v", err)
	}

	loaded, err := LoadFromDir(dir)
	if err != nil {
		t.Fatalf("LoadFromDir() error: %v", err)
	}
	if loaded.Name != "sample" || loaded.Dependencies["other"] != "2.0.0" {
		t.Fatalf("unexpected loaded manifest: %+v", loaded)
	}

	path := filepath.Join(dir, FileName)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("manifest file missing: %v", err)
	}
}
