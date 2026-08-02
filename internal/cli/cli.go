// Package cli dispatches shield's subcommands.
//
// There's no cobra/pflag here on purpose: shield only has a handful of
// subcommands, and the stdlib "flag" package plus a small switch statement
// covers that with zero third-party dependencies. For a security tool whose
// entire job is catching leaked secrets, a smaller dependency graph is a
// smaller supply-chain attack surface — worth the small amount of extra
// hand-written wiring.
package cli

import (
	"fmt"
	"io"
	"os"

	"gitlab.com/mydemoorg/sourceguard1/shield-go/internal/version"
)

// Run executes the subcommand named by args[0] and returns a process exit
// code (following the Unix convention: 0 = success, non-zero = failure).
func Run(args []string) int {
	if len(args) == 0 {
		printUsage(os.Stderr)
		return 2
	}

	switch args[0] {
	case "version":
		fmt.Println(version.String())
		return 0
	case "hook-run":
		return runHook(args[1:])
	case "check-conflicts":
		return runCheckConflicts(args[1:])
	case "-h", "--help", "help":
		printUsage(os.Stdout)
		return 0
	default:
		fmt.Fprintf(os.Stderr, "shield: unknown command %q\n\n", args[0])
		printUsage(os.Stderr)
		return 2
	}
}

func printUsage(w io.Writer) {
	fmt.Fprint(w, `Usage: shield <command> [flags]

Commands:
  hook-run          run as the git pre-commit hook (scans staged files)
  check-conflicts   report repos with a conflicting local core.hooksPath
  version           print the shield version

`)
}
