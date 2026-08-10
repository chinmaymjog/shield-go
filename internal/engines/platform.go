package engines

import (
	"fmt"
	"runtime"
)

// arch returns the release-asset architecture string for toolName on this
// machine. gitleaks and trufflehog name amd64 differently in their release
// assets (x64 vs amd64), hence the per-tool switch below. runtime.GOOS
// ("linux"/"darwin") already matches both tools' asset naming directly, so
// unlike arch there's no OS mapping needed.
func arch(toolName string) (string, error) {
	switch runtime.GOARCH {
	case "amd64":
		if toolName == Gitleaks.Name {
			return "x64", nil
		}
		return "amd64", nil
	case "arm64":
		return "arm64", nil
	default:
		return "", fmt.Errorf("unsupported architecture: %s", runtime.GOARCH)
	}
}
