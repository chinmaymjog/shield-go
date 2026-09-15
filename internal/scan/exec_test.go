package scan

import (
	"os/exec"
	"testing"
)

func TestRunPasses_ExitZero(t *testing.T) {
	ok, err := runPasses(exec.Command("true"))
	if err != nil {
		t.Fatalf("runPasses() error = %v, want nil", err)
	}
	if !ok {
		t.Error("runPasses() = false for a command that exited 0")
	}
}

func TestRunPasses_NonZeroExit(t *testing.T) {
	ok, err := runPasses(exec.Command("false"))
	if err != nil {
		t.Fatalf("runPasses() error = %v, want nil (a non-zero exit is reported via the bool, not an error)", err)
	}
	if ok {
		t.Error("runPasses() = true for a command that exited non-zero")
	}
}

func TestRunPasses_CommandNotFound(t *testing.T) {
	ok, err := runPasses(exec.Command("shield-go-test-nonexistent-binary"))
	if err == nil {
		t.Fatal("runPasses() error = nil, want an error for a missing binary")
	}
	if ok {
		t.Error("runPasses() = true despite a genuine exec error")
	}
}
