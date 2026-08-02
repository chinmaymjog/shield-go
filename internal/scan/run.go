package scan

import (
	"fmt"
	"os"

	"gitlab.com/mydemoorg/sourceguard1/shield-go/internal/engines"
)

// Run performs the full staged-file secret scan — the logic behind
// `shield hook-run`. Returns the process exit code the bash hook would have
// returned: 0 for clean/skipped, 1 for blocked.
func Run() int {
	repoRoot, err := RepoRoot()
	if err != nil {
		fmt.Fprintln(os.Stderr, "shield:", err)
		return 1
	}
	if repoRoot == "" {
		return 0 // not inside a git repo
	}
	if !HasStagedChanges(repoRoot) {
		return 0 // nothing staged — skip the engine downloads entirely
	}

	gitleaksBin, err := engines.Ensure(engines.Gitleaks)
	if err != nil {
		fmt.Fprintln(os.Stderr, "shield: failed to prepare gitleaks:", err)
		return 1
	}
	trufflehogBin, err := engines.Ensure(engines.Trufflehog)
	if err != nil {
		fmt.Fprintln(os.Stderr, "shield: failed to prepare trufflehog:", err)
		return 1
	}

	changedFiles, err := ChangedFiles(repoRoot)
	if err != nil {
		fmt.Fprintln(os.Stderr, "shield:", err)
		return 1
	}

	// Only the files actually staged in THIS commit — not the whole tree.
	// Old/pre-existing secrets elsewhere are SourceGuard Control Plane's
	// job (continuous full-history scan), not this hook's.
	scanDir, cleanupScanDir, err := Snapshot(repoRoot, changedFiles)
	if err != nil {
		fmt.Fprintln(os.Stderr, "shield:", err)
		return 1
	}
	defer cleanupScanDir()

	configPath, cleanupConfig, err := writeConfig()
	if err != nil {
		fmt.Fprintln(os.Stderr, "shield:", err)
		return 1
	}
	defer cleanupConfig()

	fmt.Printf("🛡️  SourceGuard Shield: scanning %d staged file(s) (gitleaks + trufflehog)...\n", len(changedFiles))

	gitleaksOK, err := runGitleaks(gitleaksBin, configPath, repoRoot)
	if err != nil {
		fmt.Fprintln(os.Stderr, "shield: gitleaks:", err)
		gitleaksOK = false
	}
	trufflehogOK, err := runTrufflehog(trufflehogBin, scanDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "shield: trufflehog:", err)
		trufflehogOK = false
	}

	if gitleaksOK && trufflehogOK {
		return 0
	}

	fmt.Fprintln(os.Stderr, "❌ SourceGuard Shield: potential secret(s) detected. Commit blocked.")
	if !gitleaksOK {
		fmt.Fprintln(os.Stderr, "   gitleaks: if a finding is a false positive, add its fingerprint to .gitleaksignore.")
	}
	if !trufflehogOK {
		fmt.Fprintln(os.Stderr, "   trufflehog: findings are verified credentials; rotate/revoke instead of suppressing.")
		fmt.Fprintln(os.Stderr, "   If trufflehog failed without findings, check network/provider access and rerun commit.")
	}
	return 1
}
