package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/chinmaymjog/shield-go/internal/conflicts"
)

// runCheckConflicts reports repos where a local core.hooksPath shadows
// shield's global hook, mirroring check-hooks-conflicts.sh. Purely
// informational — like the bash script, it always exits 0.
func runCheckConflicts(args []string) int {
	var root string
	if len(args) > 0 {
		root = args[0]
	}

	hooksDir, err := conflicts.HooksDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, "shield:", err)
		return 1
	}

	roots := conflicts.ScanRoots(root)
	fmt.Println("🔎 Scanning for repos with a conflicting local core.hooksPath...")
	fmt.Println("   Roots:", strings.Join(roots, " "))

	found := conflicts.Find(roots, hooksDir)
	for _, c := range found {
		fmt.Printf("⚠️  %s\n", c.Repo)
		fmt.Printf("    local core.hooksPath = %s  (overrides SourceGuard Shield — NOT scanned)\n", c.LocalPath)
	}

	fmt.Println()
	if len(found) == 0 {
		fmt.Println("✅ No conflicts found under the scanned roots.")
	} else {
		fmt.Printf("%d repo(s) found with a local core.hooksPath that bypasses SourceGuard Shield.\n", len(found))
		fmt.Println()
		fmt.Println("Fix: have that repo's existing hook manager also invoke the shield hook, e.g.")
		fmt.Println("add this line to its pre-commit script (.husky/pre-commit or equivalent):")
		fmt.Println()
		fmt.Printf("  \"%s/pre-commit\" || exit 1\n", hooksDir)
	}
	return 0
}
