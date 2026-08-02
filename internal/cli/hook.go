package cli

import "fmt"

// runHook is invoked by git as the pre-commit hook. It will scan staged
// files with gitleaks and trufflehog, mirroring hooks/pre-commit from the
// bash implementation.
//
// TODO (next pairing module): port the checksum-verified binary download
// (ensure_gitleaks/ensure_trufflehog + verified_download) from
// sourceguard-shield/hooks/pre-commit.
//
// TODO (module after that): port the staged-file snapshot + dual-engine
// scan (gitleaks protect --staged, trufflehog filesystem) and combined
// exit-code handling.
func runHook(args []string) int {
	fmt.Println("shield hook-run: not implemented yet")
	return 0
}
