package cli

import "fmt"

// runCheckConflicts will report repos where a local core.hooksPath override
// shadows the global shield hook, mirroring check-hooks-conflicts.sh.
//
// TODO (later pairing module): port the conflict-detection logic.
func runCheckConflicts(args []string) int {
	fmt.Println("shield check-conflicts: not implemented yet")
	return 0
}
