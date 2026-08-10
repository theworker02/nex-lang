package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCreateExtractAndChecksum(t *testing.T) {
	src := t.TempDir()
	if err := os.WriteFile(filepath.Join(src, "main.nex"), []byte("let x = 1;"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "nexus.toml"), []byte("name=\"demo\"\nversion=\"0.1.0\"\nauthor=\"t\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(src, ".modules", "skip"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, ".modules", "skip", "x.txt"), []byte("nope"), 0o644); err != nil {
		t.Fatal(err)
	}

	archive := filepath.Join(t.TempDir(), "pkg.tar.gz")
	if err := createTarGz(src, archive); err != nil {
		t.Fatalf("createTarGz: %v", err)
	}

	sum1, err := sha256File(archive)
	if err != nil {
		t.Fatal(err)
	}
	sum2, err := sha256File(archive)
	if err != nil {
		t.Fatal(err)
	}
	if sum1 != sum2 || sum1 == "" {
		t.Fatalf("unexpected checksums: %q vs %q", sum1, sum2)
	}

	dest := t.TempDir()
	if err := extractTarGz(archive, dest); err != nil {
		t.Fatalf("extractTarGz: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dest, "main.nex")); err != nil {
		t.Fatalf("expected main.nex extracted: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dest, ".modules")); !os.IsNotExist(err) {
		t.Fatal("expected .modules to be skipped from archive")
	}
}

func TestSafeJoinRejectsTraversal(t *testing.T) {
	if _, err := safeJoin(t.TempDir(), "../evil.txt"); err == nil {
		t.Fatal("expected traversal to be rejected")
	}
}
