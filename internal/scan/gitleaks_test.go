package scan

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFileExists(t *testing.T) {
	dir := t.TempDir()
	present := filepath.Join(dir, "present.txt")
	if err := os.WriteFile(present, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	if !fileExists(present) {
		t.Errorf("fileExists(%q) = false, want true", present)
	}
	if fileExists(filepath.Join(dir, "absent.txt")) {
		t.Error("fileExists() = true for a file that doesn't exist")
	}
}
