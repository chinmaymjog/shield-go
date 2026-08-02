package cli

import "gitlab.com/mydemoorg/sourceguard1/shield-go/internal/scan"

// runHook is invoked by git as the pre-commit hook: scans staged files with
// gitleaks and trufflehog, mirroring hooks/pre-commit from the bash
// implementation.
func runHook(args []string) int {
	return scan.Run()
}
