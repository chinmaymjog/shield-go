package engines

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCopyAtomicReplacesDestAndLeavesNoTemp(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	dest := filepath.Join(dir, "bin", "engine")
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(src, []byte("new-binary"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dest, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := copyAtomic(src, dest); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(dest)
	if string(got) != "new-binary" {
		t.Fatalf("dest = %q", got)
	}
	fi, _ := os.Stat(dest)
	if fi.Mode().Perm() != 0o755 {
		t.Fatalf("mode = %v, want 0755", fi.Mode().Perm())
	}
	entries, _ := os.ReadDir(filepath.Dir(dest))
	if len(entries) != 1 {
		t.Fatalf("leftover files in bin dir: %v", entries)
	}
}

// A failed copy must leave any existing dest untouched and no dest at all if
// there was none — never a partial file.
func TestCopyAtomicFailureLeavesDestIntact(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "engine")
	if err := os.WriteFile(dest, []byte("good"), 0o755); err != nil {
		t.Fatal(err)
	}
	// src is a directory: Open succeeds but io.Copy fails mid-way.
	if err := copyAtomic(dir, dest); err == nil {
		t.Fatal("expected error")
	}
	got, _ := os.ReadFile(dest)
	if string(got) != "good" {
		t.Fatalf("dest modified: %q", got)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatalf("temp file left behind: %v", entries)
	}
}

func TestInstallBinaryMovesFile(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	dest := filepath.Join(dir, "dest")
	os.WriteFile(src, []byte("x"), 0o755)
	if err := installBinary(src, dest); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dest); err != nil {
		t.Fatal(err)
	}
}
