package scan

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// initTestRepo creates a fresh git repository in a new temp dir and returns
// its path. No commits are made — RepoRoot/HasStagedChanges/ChangedFiles/
// Snapshot all work against a bare `git init` plus staged content, mirroring
// a hook running on someone's very first commit.
func initTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if out, err := exec.Command("git", "-C", dir, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init failed: %v\n%s", err, out)
	}
	return dir
}

// stageFile writes content to name under dir and `git add`s it.
func stageFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "-C", dir, "add", "--", name).CombinedOutput(); err != nil {
		t.Fatalf("git add %s failed: %v\n%s", name, err, out)
	}
}

// chdir temporarily changes the process working directory to dir, restoring
// the original on test cleanup. RepoRoot (unlike the rest of this package)
// takes no directory argument — it always operates on the process cwd, same
// as it does in production as a hook run from inside the repo being
// committed to.
func chdir(t *testing.T, dir string) {
	t.Helper()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(orig); err != nil {
			t.Fatal(err)
		}
	})
}

func TestRepoRoot_InsideRepo(t *testing.T) {
	repo := initTestRepo(t)
	// Resolve symlinks (macOS's /tmp is a symlink to /private/tmp) so the
	// comparison isn't thrown off by git printing the resolved path.
	want, err := filepath.EvalSymlinks(repo)
	if err != nil {
		t.Fatal(err)
	}
	chdir(t, repo)

	got, err := RepoRoot()
	if err != nil {
		t.Fatalf("RepoRoot() error = %v", err)
	}
	gotResolved, err := filepath.EvalSymlinks(got)
	if err != nil {
		t.Fatal(err)
	}
	if gotResolved != want {
		t.Errorf("RepoRoot() = %q, want %q", gotResolved, want)
	}
}

func TestRepoRoot_OutsideRepo(t *testing.T) {
	chdir(t, t.TempDir())

	got, err := RepoRoot()
	if err != nil {
		t.Fatalf("RepoRoot() error = %v", err)
	}
	if got != "" {
		t.Errorf("RepoRoot() = %q, want empty string outside a git repo", got)
	}
}

func TestHasStagedChanges(t *testing.T) {
	repo := initTestRepo(t)

	if HasStagedChanges(repo) {
		t.Error("HasStagedChanges() = true on a fresh repo with nothing staged")
	}

	stageFile(t, repo, "example.txt", "hello world\n")

	if !HasStagedChanges(repo) {
		t.Error("HasStagedChanges() = false after staging a file")
	}
}

func TestChangedFiles(t *testing.T) {
	repo := initTestRepo(t)
	stageFile(t, repo, "a.txt", "a")
	stageFile(t, repo, "sub/b.txt", "b")

	got, err := ChangedFiles(repo)
	if err != nil {
		t.Fatalf("ChangedFiles() error = %v", err)
	}
	if !equalUnordered(got, []string{"a.txt", "sub/b.txt"}) {
		t.Errorf("ChangedFiles() = %v, want [a.txt sub/b.txt]", got)
	}
}

func TestChangedFiles_NothingStaged(t *testing.T) {
	repo := initTestRepo(t)

	got, err := ChangedFiles(repo)
	if err != nil {
		t.Fatalf("ChangedFiles() error = %v", err)
	}
	if len(got) != 0 {
		t.Errorf("ChangedFiles() = %v, want empty", got)
	}
}

func TestSnapshot_UsesStagedContentNotWorkingTree(t *testing.T) {
	repo := initTestRepo(t)
	stageFile(t, repo, "creds.txt", "staged-secret\n")

	// Dirty the working tree after staging — Snapshot must reflect what's
	// in the index, not what's currently on disk. This is the exact
	// property the hook relies on: "scan what's staged" is meaningless if a
	// working-tree edit made after `git add` leaks into what gets scanned.
	if err := os.WriteFile(filepath.Join(repo, "creds.txt"), []byte("working-tree-secret\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	dir, cleanup, err := Snapshot(repo, []string{"creds.txt"})
	if err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}
	defer cleanup()

	got, err := os.ReadFile(filepath.Join(dir, "creds.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "staged-secret\n" {
		t.Errorf("Snapshot() captured %q, want the staged content %q", got, "staged-secret\n")
	}

	cleanup()
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("Snapshot() cleanup did not remove %s", dir)
	}
}

func TestSnapshot_NestedPath(t *testing.T) {
	repo := initTestRepo(t)
	stageFile(t, repo, "a/b/c.txt", "nested\n")

	dir, cleanup, err := Snapshot(repo, []string{"a/b/c.txt"})
	if err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}
	defer cleanup()

	got, err := os.ReadFile(filepath.Join(dir, "a/b/c.txt"))
	if err != nil {
		t.Fatalf("Snapshot() did not recreate the nested path: %v", err)
	}
	if string(got) != "nested\n" {
		t.Errorf("Snapshot() captured %q, want %q", got, "nested\n")
	}
}

func equalUnordered(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	counts := map[string]int{}
	for _, s := range a {
		counts[s]++
	}
	for _, s := range b {
		counts[s]--
	}
	for _, v := range counts {
		if v != 0 {
			return false
		}
	}
	return true
}
