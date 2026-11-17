package conflicts

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindRepoHooks(t *testing.T) {
	root := t.TempDir()
	hooks := filepath.Join(root, "a", ".git", "hooks")
	os.MkdirAll(hooks, 0o755)
	os.WriteFile(filepath.Join(hooks, "pre-commit"), []byte("#!/bin/sh\n"), 0o755)
	os.WriteFile(filepath.Join(hooks, "pre-push"), []byte("x"), 0o644)
	os.WriteFile(filepath.Join(hooks, "commit-msg.sample"), []byte("x"), 0o755)
	os.MkdirAll(filepath.Join(root, "b", ".git", "hooks"), 0o755)

	got := FindRepoHooks([]string{root})
	if len(got) != 1 || got[0].Repo != filepath.Join(root, "a") ||
		len(got[0].Hooks) != 1 || got[0].Hooks[0] != "pre-commit" {
		t.Fatalf("FindRepoHooks = %+v", got)
	}
}
