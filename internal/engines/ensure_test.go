package engines

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// TestEnsure_PrunesOtherVersionsOfSameEngine seeds the cache with the
// pinned binary already present (so Ensure takes the cached path and makes
// no network call), alongside stale versions and unrelated files, and checks
// that only the stale versions of the same engine are removed.
func TestEnsure_PrunesOtherVersionsOfSameEngine(t *testing.T) {
	t.Setenv("SOURCEGUARD_SHIELD_HOME", t.TempDir())
	binDir, err := BinDir()
	if err != nil {
		t.Fatal(err)
	}

	spec := Spec{Repo: "gitleaks/gitleaks", Name: "gitleaks", Version: "9.9.9", ChecksumsSHA256: "unused-cached-path"}
	assetArch, err := arch(spec.Name)
	if err != nil {
		t.Skip("unsupported platform:", err)
	}
	current := fmt.Sprintf("gitleaks-9.9.9-%s-%s", runtime.GOOS, assetArch)

	seed := []string{
		current,
		"gitleaks-8.30.0-" + runtime.GOOS + "-" + assetArch, // stale: should go
		"gitleaks-1.0.0-linux-x64",                          // stale, other platform: should go
		"trufflehog-3.82.13-darwin-arm64",                   // other engine: must stay
		"README",                                            // unrelated: must stay
	}
	for _, name := range seed {
		if err := os.WriteFile(filepath.Join(binDir, name), []byte("x"), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	got, err := Ensure(spec)
	if err != nil {
		t.Fatalf("Ensure: %v", err)
	}
	if want := filepath.Join(binDir, current); got != want {
		t.Fatalf("Ensure returned %q, want %q", got, want)
	}

	entries, err := os.ReadDir(binDir)
	if err != nil {
		t.Fatal(err)
	}
	remaining := map[string]bool{}
	for _, e := range entries {
		remaining[e.Name()] = true
	}
	for _, keep := range []string{current, "trufflehog-3.82.13-darwin-arm64", "README"} {
		if !remaining[keep] {
			t.Errorf("%s was deleted, should have been kept", keep)
		}
	}
	for _, gone := range []string{"gitleaks-8.30.0-" + runtime.GOOS + "-" + assetArch, "gitleaks-1.0.0-linux-x64"} {
		if remaining[gone] {
			t.Errorf("%s still present, should have been pruned", gone)
		}
	}
}
