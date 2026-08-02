// Command shield is the SourceGuard Shield CLI: a git pre-commit hook that
// scans staged changes for secrets before they're committed.
package main

import (
	"os"

	"gitlab.com/mydemoorg/sourceguard1/shield-go/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:]))
}
