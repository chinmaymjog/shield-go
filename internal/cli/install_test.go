package cli

import (
	"os/exec"
	"strings"
	"testing"
)

func TestHookScriptResolvesViaPATHFirst(t *testing.T) {
	script := hookScript("/home/dev/bin/shield")
	if !strings.HasPrefix(script, "#!/bin/sh\n") {
		t.Fatalf("hookScript() does not start with a shebang: %q", script)
	}
	if !strings.Contains(script, "command -v shield") {
		t.Errorf("hookScript() does not check $PATH first: %q", script)
	}
	if !strings.Contains(script, "exec shield hook-run") {
		t.Errorf("hookScript() does not exec the PATH-resolved shield: %q", script)
	}
}

func TestHookScriptFallsBackToWellKnownLocations(t *testing.T) {
	script := hookScript("")
	for _, loc := range wellKnownShieldLocations {
		if !strings.Contains(script, shq(loc)) {
			t.Errorf("hookScript() missing fallback for %q:\n%s", loc, script)
		}
	}
}

func TestHookScriptIncludesSelfHintLast(t *testing.T) {
	script := hookScript("/home/dev/bin/shield")
	if !strings.Contains(script, shq("/home/dev/bin/shield")) {
		t.Errorf("hookScript() does not include the self-path hint:\n%s", script)
	}
}

func TestHookScriptErrorsIfNothingFound(t *testing.T) {
	script := hookScript("")
	if !strings.Contains(script, "exit 1") {
		t.Errorf("hookScript() has no failure path when shield can't be located:\n%s", script)
	}
}

// TestHookScriptIsValidShell exercises the generated script through the
// real shell to catch quoting mistakes that string-matching assertions
// above wouldn't: an unterminated quote or bad escape would make even the
// unreachable branches fail to parse.
func TestHookScriptIsValidShell(t *testing.T) {
	script := hookScript("/weird 'path/with spaces/shield")
	cmd := exec.Command("sh", "-n", "/dev/stdin")
	cmd.Stdin = strings.NewReader(script)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generated hook script is not valid shell: %v\n%s\n---\n%s", err, out, script)
	}
}
