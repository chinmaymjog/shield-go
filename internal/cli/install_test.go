package cli

import (
	"os"
	"os/exec"
	"path/filepath"
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
	if !strings.Contains(script, "SHIELD=shield") {
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

func TestHookScriptChainsToRepoHook(t *testing.T) {
	script := hookScript("")
	if !strings.Contains(script, "git rev-parse --git-common-dir") || !strings.Contains(script, "/hooks/pre-commit") {
		t.Errorf("pre-commit shim does not chain to repo hook:\n%s", script)
	}
	if strings.Index(script, "hook-run") > strings.Index(script, "repo_hook=") {
		t.Errorf("scan must run before chaining:\n%s", script)
	}
}

func TestForwardScriptsAreValidShell(t *testing.T) {
	for _, name := range forwardedHooks {
		cmd := exec.Command("sh", "-n", "/dev/stdin")
		cmd.Stdin = strings.NewReader(forwardScript(name))
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%s shim invalid: %v\n%s", name, err, out)
		}
	}
}

// TestShimsRunRepoHooks runs the real shims in a temp repo: the pre-commit
// shim (with a fake shield on PATH) must chain to the repo hook, a failing
// scan must block it, and a forwarding shim must exec the repo's hook.
func TestShimsRunRepoHooks(t *testing.T) {
	dir := t.TempDir()
	run := func(c *exec.Cmd) {
		t.Helper()
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("%v: %v\n%s", c.Args, err, out)
		}
	}
	repo := filepath.Join(dir, "repo")
	run(exec.Command("git", "init", "-q", repo))
	bin := filepath.Join(dir, "bin")
	os.MkdirAll(bin, 0o755)
	os.WriteFile(filepath.Join(bin, "shield"), []byte("#!/bin/sh\nexit ${FAKE_SCAN_RC:-0}\n"), 0o755)
	hooks := filepath.Join(repo, ".git", "hooks")
	marker := filepath.Join(dir, "marker")
	for _, n := range []string{"pre-commit", "pre-push"} {
		os.WriteFile(filepath.Join(hooks, n), []byte("#!/bin/sh\necho "+n+" >> "+marker+"\n"), 0o755)
	}
	shim := func(name, body string) *exec.Cmd {
		p := filepath.Join(dir, name)
		os.WriteFile(p, []byte(body), 0o755)
		c := exec.Command(p)
		c.Dir = repo
		c.Env = append(os.Environ(), "PATH="+bin+":"+os.Getenv("PATH"))
		return c
	}

	run(shim("pc", hookScript("")))
	run(shim("pp", forwardScript("pre-push")))
	got, _ := os.ReadFile(marker)
	if string(got) != "pre-commit\npre-push\n" {
		t.Errorf("marker = %q", got)
	}

	os.Remove(marker)
	c := shim("pc2", hookScript(""))
	c.Env = append(c.Env, "FAKE_SCAN_RC=1")
	if err := c.Run(); err == nil {
		t.Error("failing scan should fail the hook")
	}
	if _, err := os.Stat(marker); err == nil {
		t.Error("repo hook ran despite failed scan")
	}

	// No repo hook: shim is a no-op success.
	os.Remove(filepath.Join(hooks, "pre-push"))
	run(shim("pp2", forwardScript("pre-push")))
}
