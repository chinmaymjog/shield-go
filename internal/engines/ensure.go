package engines

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Ensure returns the path to spec's cached binary, downloading and
// verifying it first if it isn't already cached, then prunes any other
// cached versions of the same engine.
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

	if _, err := os.Stat(destPath); err != nil {
		if err := verifiedDownload(spec, assetOS, assetArch, destPath); err != nil {
			return "", err
		}
	}

	pruneStale(binDir, spec.Name, destPath)
	return destPath, nil
}

// pruneStale deletes cached binaries for every other version of the same
// engine once the pinned one is in place, so the cache doesn't grow by a
// full engine binary on every pin bump. It runs on every Ensure, not just
// after a fresh download, so machines that already hold several versions
// get cleaned up on their next commit too. Best-effort: a failed delete is
// ignored, since it's only wasted disk, never a correctness problem.
//
// Safe against an in-flight download: verifiedDownload stages everything in
// os.TempDir and only moves the verified binary into binDir at the end.
// Deleting a binary another process is currently executing is also safe on
// macOS/Linux — the running process keeps its open inode.
func pruneStale(binDir, name, keep string) {
	entries, err := os.ReadDir(binDir)
	if err != nil {
		return
	}
	prefix := name + "-"
	for _, e := range entries {
		path := filepath.Join(binDir, e.Name())
		if e.IsDir() || path == keep || !strings.HasPrefix(e.Name(), prefix) {
			continue
		}
		os.Remove(path)
	}
}
