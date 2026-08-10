package scan

import (
	"os"
	"os/exec"
)

// runGitleaks runs `gitleaks protect --staged` against the git index diff
// directly — naturally scoped to what's about to be committed, no manual
// file copy needed. --verbose prints the File/Line/RuleID for each finding
// (without it, only a "leaks found: N" count is shown); --log-level warn
// drops the routine "N commits scanned" noise; --no-color avoids raw ANSI
// escapes around --redact's REDACTED text.
func runGitleaks(binPath, configPath, repoRoot string) (bool, error) {
	args := []string{
		"protect", "--staged", "--no-banner", "--verbose", "--log-level", "warn", "--no-color",
		"--config", configPath, "--source", repoRoot,
	}
	if ignorePath := repoRoot + "/.gitleaksignore"; fileExists(ignorePath) {
		args = append(args, "--gitleaks-ignore-path", ignorePath)
	}
	args = append(args, "--redact")

	cmd := exec.Command(binPath, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return runPasses(cmd)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
