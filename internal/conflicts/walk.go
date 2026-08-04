package conflicts

import (
	"io/fs"
	"path/filepath"
)

// findGitDirs walks root looking for .git directories, mirroring
// `find root -type d -name node_modules -prune -o -type d -name .git -print0 -prune`:
// node_modules is skipped entirely (never descended into), and .git is
// recorded but not descended into either.
func findGitDirs(root string) ([]string, error) {
	var dirs []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // best-effort, like the shell find's `2>/dev/null`
		}
		if !d.IsDir() {
			return nil
		}
		switch d.Name() {
		case "node_modules":
			return filepath.SkipDir
		case ".git":
			dirs = append(dirs, path)
			return filepath.SkipDir
		}
		return nil
	})
	return dirs, err
}
