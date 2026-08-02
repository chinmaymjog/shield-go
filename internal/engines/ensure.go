package engines

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// Ensure returns the path to spec's cached binary, downloading and
// verifying it first if it isn't already cached. Mirrors
// ensure_gitleaks/ensure_trufflehog in hooks/pre-commit.
func Ensure(spec Spec) (string, error) {
	assetArch, err := arch(spec.Name)
	if err != nil {
		return "", err
	}
	assetOS := runtime.GOOS

	binDir, err := BinDir()
	if err != nil {
		return "", err
	}
	destPath := filepath.Join(binDir, fmt.Sprintf("%s-%s-%s-%s", spec.Name, spec.Version, assetOS, assetArch))

	if _, err := os.Stat(destPath); err == nil {
		return destPath, nil
	}

	if err := verifiedDownload(spec, assetOS, assetArch, destPath); err != nil {
		return "", err
	}
	return destPath, nil
}
