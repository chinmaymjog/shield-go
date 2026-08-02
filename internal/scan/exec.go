package scan

import (
	"errors"
	"os/exec"
)

// runPasses runs cmd and reports whether it exited 0 (passed). A non-zero
// exit is treated as "failed" regardless of cause (findings vs. a tool
// error) — matching hooks/pre-commit's aggregation, which never
// distinguishes the two and just surfaces both kinds of remediation text.
func runPasses(cmd *exec.Cmd) (bool, error) {
	err := cmd.Run()
	if err == nil {
		return true, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return false, nil
	}
	return false, err
}
