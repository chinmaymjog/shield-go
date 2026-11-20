package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// shimEnv builds a repo whose own hooks record their calls, plus a fake `shield`
// on PATH that logs its arguments and exits with $FAKE_DENY_RC for denylist
// commands (0 for everything else).
type shimEnv struct {
	t     *testing.T
	repo  string
	bin   string
	log   string
	stdin string
}

func newShimEnv(t *testing.T) *shimEnv {
	t.Helper()
	e := &shimEnv{t: t, repo: t.TempDir(), bin: t.TempDir()}
	e.log = filepath.Join(e.bin, "calls.log")
	e.stdin = filepath.Join(e.bin, "shield.stdin")
	run := func(dir string, args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run(e.repo, "init", "-q")
	fake := "#!/bin/sh\n" +
		"echo \"shield $*\" >> \"" + e.log + "\"\n" +
		"if [ \"$1\" = denylist ]; then cat > \"" + e.stdin + "\"; exit ${FAKE_DENY_RC:-0}; fi\n" +
		"exit 0\n"
	if err := os.WriteFile(filepath.Join(e.bin, "shield"), []byte(fake), 0o755); err != nil {
		t.Fatal(err)
	}
	return e
}

func (e *shimEnv) repoHook(name string) {
	e.t.Helper()
	body := "#!/bin/sh\necho \"repo-hook " + name + " $*\" >> \"" + e.log + "\"\ncat > \"" + e.log + ".repo-stdin\"\nexit ${REPO_HOOK_RC:-0}\n"
	p := filepath.Join(e.repo, ".git", "hooks", name)
	if err := os.WriteFile(p, []byte(body), 0o755); err != nil {
		e.t.Fatal(err)
	}
}

func (e *shimEnv) run(script string, stdin string, env ...string) (int, string) {
	e.t.Helper()
	cmd := exec.Command("sh", "-c", script, "hook", "arg1", "arg2")
	cmd.Dir = e.repo
	cmd.Env = append([]string{"PATH=" + e.bin + ":/usr/bin:/bin:/usr/sbin:/sbin", "HOME=" + e.bin}, env...)
	cmd.Stdin = strings.NewReader(stdin)
	out, err := cmd.CombinedOutput()
	rc := 0
	if ee, ok := err.(*exec.ExitError); ok {
		rc = ee.ExitCode()
	} else if err != nil {
		e.t.Fatal(err)
	}
	b, _ := os.ReadFile(e.log)
	return rc, string(b) + string(out)
}

func TestDenylistShimsAreValidShell(t *testing.T) {
	for _, name := range []string{"commit-msg", "pre-push"} {
		script := denylistForwardScript(name, "/weird 'path/with spaces/shield")
		cmd := exec.Command("sh", "-n", "/dev/stdin")
		cmd.Stdin = strings.NewReader(script)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%s shim is not valid shell: %v\n%s", name, err, out)
		}
	}
}

func TestCommitMsgShimBlocksOnlyOnExit1(t *testing.T) {
	e := newShimEnv(t)
	e.repoHook("commit-msg")
	script := denylistForwardScript("commit-msg", "")

	rc, log := e.run(script, "", "FAKE_DENY_RC=1")
	if rc != 1 || strings.Contains(log, "repo-hook") {
		t.Errorf("exit 1 from shield must block before the repo hook: rc=%d\n%s", rc, log)
	}
	// An older shield without the subcommand exits 2; that must not block.
	rc, log = e.run(script, "", "FAKE_DENY_RC=2")
	if rc != 0 || !strings.Contains(log, "repo-hook commit-msg arg1 arg2") {
		t.Errorf("exit 2 must be ignored and forwarded: rc=%d\n%s", rc, log)
	}
	rc, log = e.run(script, "", "FAKE_DENY_RC=0", "REPO_HOOK_RC=3")
	if rc != 3 || !strings.Contains(log, "shield denylist hook commit-msg arg1 arg2") {
		t.Errorf("the repo hook's exit code must propagate: rc=%d\n%s", rc, log)
	}
}

func TestPrePushShimFeedsStdinToBothAndBlocks(t *testing.T) {
	e := newShimEnv(t)
	e.repoHook("pre-push")
	script := denylistForwardScript("pre-push", "")
	input := "refs/heads/main aaa refs/heads/main bbb\n"

	rc, log := e.run(script, input, "FAKE_DENY_RC=0")
	if rc != 0 {
		t.Fatalf("rc=%d\n%s", rc, log)
	}
	for _, f := range []string{e.stdin, e.log + ".repo-stdin"} {
		b, _ := os.ReadFile(f)
		if string(b) != input {
			t.Errorf("%s received %q, want %q", filepath.Base(f), b, input)
		}
	}
	os.Remove(e.log + ".repo-stdin")
	os.Remove(e.log)
	rc, log = e.run(script, input, "FAKE_DENY_RC=1")
	if rc != 1 || strings.Contains(log, "repo-hook") {
		t.Errorf("exit 1 must block before the repo hook: rc=%d\n%s", rc, log)
	}
}

func TestPreCommitScriptRunsDenylistFirstAndBlocksOnExit1(t *testing.T) {
	e := newShimEnv(t)
	script := hookScript("")
	rc, log := e.run(script, "", "FAKE_DENY_RC=1")
	if rc != 1 || strings.Contains(log, "hook-run") {
		t.Errorf("deny-list block must stop before the scan: rc=%d\n%s", rc, log)
	}
	rc, log = e.run(script, "", "FAKE_DENY_RC=2")
	if rc != 0 || !strings.Contains(log, "shield hook-run") {
		t.Errorf("an unsupported deny-list command must not block: rc=%d\n%s", rc, log)
	}
	if i, j := strings.Index(log, "denylist hook pre-commit"), strings.Index(log, "hook-run"); i < 0 || j < 0 || i > j {
		t.Errorf("deny-list check must run before the scan:\n%s", log)
	}
}
