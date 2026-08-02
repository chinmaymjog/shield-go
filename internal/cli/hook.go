package cli

import (
	"fmt"
	"os"

	"gitlab.com/mydemoorg/sourceguard1/shield-go/internal/engines"
)

// runHook is invoked by git as the pre-commit hook. It will scan staged
// files with gitleaks and trufflehog, mirroring hooks/pre-commit from the
// bash implementation.
//
// TODO (next pairing module): port the staged-file snapshot + dual-engine
// scan (gitleaks protect --staged, trufflehog filesystem) and combined
// exit-code handling.
func runHook(args []string) int {
	gitleaksPath, err := engines.Ensure(engines.Gitleaks)
	if err != nil {
		fmt.Fprintln(os.Stderr, "shield: failed to prepare gitleaks:", err)
		return 1
	}
	trufflehogPath, err := engines.Ensure(engines.Trufflehog)
	if err != nil {
		fmt.Fprintln(os.Stderr, "shield: failed to prepare trufflehog:", err)
		return 1
	}
	fmt.Println("gitleaks ready at", gitleaksPath)
	fmt.Println("trufflehog ready at", trufflehogPath)
	fmt.Println("shield hook-run: scan logic not implemented yet")
	return 0
}
