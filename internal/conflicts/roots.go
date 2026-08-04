// Package conflicts detects repos where a local core.hooksPath (most
// commonly set by Husky or another JS-based hook manager) silently
// overrides SourceGuard Shield's global hook. Git's config precedence is
// local > global, so any such repo never runs the shield hook at all — with
// no error, no warning, nothing in the commit output to indicate it.
package conflicts

import (
	"os"
	"path/filepath"
	"strings"

	"gitlab.com/mydemoorg/sourceguard1/shield-go/internal/engines"
)

// HooksDir returns the directory shield's global core.hooksPath points at.
func HooksDir() (string, error) {
	home, err := engines.Home()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "hooks"), nil
}

// ScanRoots decides which directories to walk. explicitRoot, if non-empty,
// is used verbatim. Otherwise the default is intentionally narrow (common
// developer roots) for speed on large home directories; set
// SG_SHIELD_DEEP_CONFLICT_SCAN=true to force a full-home walk instead.
func ScanRoots(explicitRoot string) []string {
	if explicitRoot != "" {
		return []string{explicitRoot}
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}

	if deepScanEnabled() {
		return []string{home}
	}

	var roots []string
	for _, sub := range []string{"repos", "src", "work", "workspace"} {
		dir := filepath.Join(home, sub)
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			roots = append(roots, dir)
		}
	}
	if len(roots) == 0 {
		roots = []string{home}
	}
	return roots
}

func deepScanEnabled() bool {
	switch strings.ToLower(os.Getenv("SG_SHIELD_DEEP_CONFLICT_SCAN")) {
	case "1", "true", "yes", "y", "on":
		return true
	}
	return false
}
