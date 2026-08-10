package scan

import (
	"os"
	"os/exec"
	"strings"
)

// RepoRoot returns the current git repository's top level, or "" if the
// current directory isn't inside a git repo. Silent, not an error: this
// hook must never block a developer for running a command outside a repo.
func RepoRoot() (string, error) {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", nil
	}
	return strings.TrimSpace(string(out)), nil
}

// HasStagedChanges reports whether there is any staged add/copy/modify/
// rename change to scan. Mirrors `git diff --cached --quiet
// --diff-filter=ACMR`: nothing staged means skip the hook entirely,
// including the engine downloads.
func HasStagedChanges(repoRoot string) bool {
	cmd := exec.Command("git", "-C", repoRoot, "diff", "--cached", "--quiet", "--diff-filter=ACMR")
	return cmd.Run() != nil
}

// ChangedFiles lists the files staged in this commit (add/copy/modify/
// rename only) — the exact set the hook snapshots and scans.
func ChangedFiles(repoRoot string) ([]string, error) {
	out, err := exec.Command("git", "-C", repoRoot, "diff", "--cached", "--name-only", "--diff-filter=ACMR", "-z").Output()
	if err != nil {
		return nil, err
	}
	raw := strings.Trim(string(out), "\x00")
	if raw == "" {
		return nil, nil
	}
	return strings.Split(raw, "\x00"), nil
}

// Snapshot checks out the given staged files (as they are in the index,
// not the working tree) into a fresh temp directory, so trufflehog — which
// has no staged-diff mode — can be scoped to exactly what's about to be
// committed. Returns the directory and a cleanup func.
func Snapshot(repoRoot string, files []string) (string, func(), error) {
	dir, err := os.MkdirTemp("", "sg-shield-staged-*")
	if err != nil {
		return "", nil, err
	}
	cleanup := func() { os.RemoveAll(dir) }

	args := append([]string{"-C", repoRoot, "checkout-index", "--prefix=" + dir + "/", "-f", "--"}, files...)
	cmd := exec.Command("git", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		cleanup()
		return "", nil, err
	}
	return dir, cleanup, nil
}
