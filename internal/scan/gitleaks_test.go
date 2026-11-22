package scan

import (
	"os"
	"os/exec"
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

// commitFile stages and commits name with content, with a fixed identity so
// the test does not depend on the machine's git config.
func commitFile(t *testing.T, dir, name, content string) {
	t.Helper()
	stageFile(t, dir, name, content)
	out, err := exec.Command("git", "-C", dir,
		"-c", "user.name=t", "-c", "user.email=t@example.com", "-c", "commit.gpgsign=false",
		"commit", "-q", "--no-verify", "-m", "add "+name).CombinedOutput()
	if err != nil {
		t.Fatalf("git commit failed: %v\n%s", err, out)
	}
}

func TestResolveIgnore(t *testing.T) {
	const committedEntries = "aaaa:file:rule:1\n"

	t.Run("no ignore file anywhere", func(t *testing.T) {
		dir := initTestRepo(t)
		commitFile(t, dir, "a.txt", "x")
		path, warning, cleanup := resolveIgnore(dir)
		defer cleanup()
		if path != "" || warning != "" {
			t.Fatalf("got path=%q warning=%q, want both empty", path, warning)
		}
	})

	t.Run("uncommitted file is not honoured and warns", func(t *testing.T) {
		dir := initTestRepo(t)
		commitFile(t, dir, "a.txt", "x")
		stageFile(t, dir, ".gitleaksignore", committedEntries)
		path, warning, cleanup := resolveIgnore(dir)
		defer cleanup()
		if path != "" {
			t.Fatalf("an uncommitted ignore file must not be honoured, got %q", path)
		}
		if warning == "" {
			t.Fatal("expected a warning for an uncommitted ignore file")
		}
	})

	t.Run("first commit with a staged ignore file is not honoured", func(t *testing.T) {
		dir := initTestRepo(t) // no HEAD yet
		stageFile(t, dir, ".gitleaksignore", committedEntries)
		path, warning, cleanup := resolveIgnore(dir)
		defer cleanup()
		if path != "" || warning == "" {
			t.Fatalf("got path=%q warning=%q, want no path and a warning", path, warning)
		}
	})

	t.Run("committed and unchanged is honoured silently", func(t *testing.T) {
		dir := initTestRepo(t)
		commitFile(t, dir, ".gitleaksignore", committedEntries)
		path, warning, cleanup := resolveIgnore(dir)
		defer cleanup()
		if warning != "" {
			t.Fatalf("unexpected warning: %q", warning)
		}
		got, err := os.ReadFile(path)
		if err != nil || string(got) != committedEntries {
			t.Fatalf("honoured file = %q (err %v), want %q", got, err, committedEntries)
		}
	})

	t.Run("working-tree edits are ignored and warn", func(t *testing.T) {
		dir := initTestRepo(t)
		commitFile(t, dir, ".gitleaksignore", committedEntries)
		stageFile(t, dir, ".gitleaksignore", committedEntries+"bbbb:other:rule:2\n")
		path, warning, cleanup := resolveIgnore(dir)
		defer cleanup()
		got, err := os.ReadFile(path)
		if err != nil || string(got) != committedEntries {
			t.Fatalf("honoured file = %q (err %v); the edited entry must not apply", got, err)
		}
		if warning == "" {
			t.Fatal("expected a warning when the working-tree file differs from HEAD")
		}
	})

	t.Run("cleanup removes the temp copy", func(t *testing.T) {
		dir := initTestRepo(t)
		commitFile(t, dir, ".gitleaksignore", committedEntries)
		path, _, cleanup := resolveIgnore(dir)
		cleanup()
		if fileExists(path) {
			t.Fatal("temp ignore copy still present after cleanup")
		}
	})
}
