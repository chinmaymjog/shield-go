package conflicts

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Conflict is a repo whose local core.hooksPath overrides shield's global
// hook, so it never gets scanned.
type Conflict struct {
	Repo      string
	LocalPath string
}

// Find scans the given roots (see ScanRoots) for repos with a conflicting
// local core.hooksPath, returning every conflict found. Best-effort: an
// unreadable root or repo is skipped rather than aborting the whole scan.
func Find(roots []string, hooksDir string) []Conflict {
	var conflicts []Conflict
	for _, root := range roots {
		if info, err := os.Stat(root); err != nil || !info.IsDir() {
			continue
		}
		gitDirs, err := findGitDirs(root)
		if err != nil {
			continue
		}
		for _, gitDir := range gitDirs {
			repo := filepath.Dir(gitDir)
			localPath, ok := localHooksPath(repo)
			if !ok || localPath == "" {
				continue
			}
			if resolveAbs(repo, localPath) != hooksDir {
				conflicts = append(conflicts, Conflict{Repo: repo, LocalPath: localPath})
			}
		}
	}
	return conflicts
}

func localHooksPath(repo string) (string, bool) {
	out, err := exec.Command("git", "-C", repo, "config", "--local", "--get", "core.hooksPath").Output()
	if err != nil {
		return "", false
	}
	return strings.TrimSpace(string(out)), true
}

// resolveAbs resolves repo/localPath to an absolute path only when it's
// actually a directory (matching how git itself resolves a relative
// core.hooksPath); otherwise it's compared as the raw configured value.
func resolveAbs(repo, localPath string) string {
	candidate := repo + "/" + localPath
	if info, err := os.Stat(candidate); err == nil && info.IsDir() {
		if abs, err := filepath.Abs(candidate); err == nil {
			return abs
		}
	}
	return localPath
}
