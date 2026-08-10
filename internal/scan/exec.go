package scan

import (
	"errors"
	"os/exec"
)

// runPasses runs cmd and reports whether it exited 0 (passed). A non-zero
// exit is treated as "failed" regardless of cause (findings vs. a tool
// error) — the caller surfaces remediation text for both cases without
// needing to distinguish them.
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
