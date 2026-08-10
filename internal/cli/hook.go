package cli

import "github.com/chinmaymjog/shield-go/internal/scan"

// runHook is invoked by git as the pre-commit hook: scans staged files
// with gitleaks and trufflehog before letting the commit through.
func runHook(args []string) int {
	return scan.Run()
}
