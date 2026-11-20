package denylist

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const term = "zzforbidden"

func writeConfig(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "denylist")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SHIELD_DENYLIST_FILE", p)
	return p
}

func TestLoadParsesPatternsAndScope(t *testing.T) {
	p := writeConfig(t, "# c\n\nremote: github\\.com/me/\npath: /personal/\n"+term+"\n\\bsecondterm\n")
	c, err := Load(p)
	if err != nil || c == nil {
		t.Fatalf("Load: %v %v", c, err)
	}
	if c.Count != 2 || c.Remote == nil || c.Dir == nil {
		t.Fatalf("unexpected config: %+v", c)
	}
	if !c.MatchText("see ZZFORBIDDEN here") || !c.MatchText("a secondterm b") || c.MatchText("nothing") {
		t.Error("pattern matching is wrong")
	}
}

func TestLoadMissingFileMeansNotConfigured(t *testing.T) {
	c, err := Load(filepath.Join(t.TempDir(), "absent"))
	if c != nil || err != nil {
		t.Fatalf("want (nil, nil), got %v %v", c, err)
	}
}

func TestLoadInvalidPatternErrorDoesNotRepeatIt(t *testing.T) {
	p := writeConfig(t, "remote: x\n(unclosed"+term+"\n")
	_, err := Load(p)
	if err == nil {
		t.Fatal("invalid pattern accepted")
	}
	if strings.Contains(err.Error(), term) {
		t.Errorf("error text leaks the pattern: %v", err)
	}
}

func TestActiveNeedsScope(t *testing.T) {
	noScope, _ := Load(writeConfig(t, term+"\n"))
	if noScope.Active("/repos/personal/x", "git@github.com:me/x.git") {
		t.Error("guard must be inactive without a scope line")
	}
	scoped, _ := Load(writeConfig(t, "remote: github\\.com[:/]me/\npath: /repos/personal/\n"+term+"\n"))
	cases := []struct {
		path string
		urls []string
		want bool
	}{
		{"/repos/work/x", []string{"git@gitlab.example:team/x.git"}, false},
		{"/repos/personal/x", nil, true},
		{"/repos/work/x", []string{"git@github.com:me/x.git"}, true},
		{"/repos/work/x", []string{"https://github.com/other/x"}, false},
	}
	for _, tc := range cases {
		if got := scoped.Active(tc.path, tc.urls...); got != tc.want {
			t.Errorf("Active(%q, %v) = %v, want %v", tc.path, tc.urls, got, tc.want)
		}
	}
}

const sampleDiff = `diff --git a/app.go b/app.go
--- a/app.go
+++ b/app.go
@@ -1,0 +5,2 @@
+clean line
+uses zzforbidden here
diff --git a/go.sum b/go.sum
--- a/go.sum
+++ b/go.sum
@@ -1,0 +1,1 @@
+zzforbidden-in-a-hash
diff --git a/zzforbidden.txt b/zzforbidden.txt
--- /dev/null
+++ b/zzforbidden.txt
@@ -0,0 +1,1 @@
+zzforbidden
`

func TestScanDiffReportsLocationsOnly(t *testing.T) {
	c, _ := Load(writeConfig(t, term+"\n"))
	locs := c.ScanDiff(strings.NewReader(sampleDiff))
	got := strings.Join(locs, "|")
	if got != "app.go:6|[path hidden]:1" {
		t.Fatalf("locations = %q", got)
	}
	if strings.Contains(strings.ToLower(got), term) {
		t.Error("locations contain the matched term")
	}
}

// ---- git integration ----

func gitEnv(t *testing.T) {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_AUTHOR_NAME", "Dev")
	t.Setenv("GIT_AUTHOR_EMAIL", "dev@example.invalid")
	t.Setenv("GIT_COMMITTER_NAME", "Dev")
	t.Setenv("GIT_COMMITTER_EMAIL", "dev@example.invalid")
}

func run(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func newRepo(t *testing.T) string {
	t.Helper()
	gitEnv(t)
	dir := t.TempDir()
	run(t, dir, "init", "-q", "-b", "main")
	t.Chdir(dir)
	return dir
}

func commitFile(t *testing.T, dir, name, content, msg string) string {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	run(t, dir, "add", name)
	run(t, dir, "commit", "-q", "--no-verify", "-m", msg)
	return run(t, dir, "rev-parse", "HEAD")
}

// capture runs f with stderr redirected and returns the exit code and output.
func capture(t *testing.T, f func() int) (int, string) {
	t.Helper()
	old := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w
	rc := f()
	w.Close()
	os.Stderr = old
	b, _ := io.ReadAll(r)
	return rc, string(b)
}

func inScope(t *testing.T, dir string) {
	writeConfig(t, "path: "+regexpQuote(dir)+"\n"+term+"\n")
}

func regexpQuote(s string) string {
	return strings.NewReplacer(`\`, `\\`, `.`, `\.`, `+`, `\+`, `(`, `\(`, `)`, `\)`).Replace(s)
}

func TestPreCommitBlocksStagedTermAndStaysSilentOnIt(t *testing.T) {
	dir := newRepo(t)
	inScope(t, dir)
	commitFile(t, dir, "ok.txt", "fine\n", "base")
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("x\nthe "+term+" word\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(t, dir, "add", "a.txt")
	rc, out := capture(t, PreCommit)
	if rc != 1 {
		t.Fatalf("rc = %d, output: %s", rc, out)
	}
	if !strings.Contains(out, "a.txt:2") {
		t.Errorf("location missing: %s", out)
	}
	if strings.Contains(strings.ToLower(out), term) {
		t.Errorf("output prints the term: %s", out)
	}
}

func TestPreCommitPassesCleanAndOutOfScope(t *testing.T) {
	dir := newRepo(t)
	inScope(t, dir)
	commitFile(t, dir, "ok.txt", "fine\n", "base")
	_ = os.WriteFile(filepath.Join(dir, "b.txt"), []byte("clean\n"), 0o644)
	run(t, dir, "add", "b.txt")
	if rc, out := capture(t, PreCommit); rc != 0 {
		t.Fatalf("clean change blocked: %s", out)
	}
	// Same content, but the repo is not in scope.
	writeConfig(t, "path: /somewhere/else/\n"+term+"\n")
	_ = os.WriteFile(filepath.Join(dir, "c.txt"), []byte(term+"\n"), 0o644)
	run(t, dir, "add", "c.txt")
	if rc, out := capture(t, PreCommit); rc != 0 {
		t.Fatalf("out-of-scope repo blocked: %s", out)
	}
}

func TestPreCommitBlocksMatchingIdentity(t *testing.T) {
	dir := newRepo(t)
	inScope(t, dir)
	commitFile(t, dir, "ok.txt", "fine\n", "base")
	t.Setenv("GIT_AUTHOR_EMAIL", "dev@"+term+".example")
	_ = os.WriteFile(filepath.Join(dir, "n.txt"), []byte("clean\n"), 0o644)
	run(t, dir, "add", "n.txt")
	rc, out := capture(t, PreCommit)
	if rc != 1 || !strings.Contains(out, "author identity") {
		t.Fatalf("rc=%d out=%s", rc, out)
	}
	if strings.Contains(strings.ToLower(out), term) {
		t.Errorf("output prints the term: %s", out)
	}
}

func TestCommitMsg(t *testing.T) {
	dir := newRepo(t)
	inScope(t, dir)
	msg := filepath.Join(t.TempDir(), "MSG")
	_ = os.WriteFile(msg, []byte("fine message\n# comment mentioning "+term+"\n"), 0o644)
	if rc, out := capture(t, func() int { return CommitMsg(msg) }); rc != 0 {
		t.Fatalf("comment lines must be ignored: %s", out)
	}
	_ = os.WriteFile(msg, []byte("fix the "+term+" thing\n"), 0o644)
	rc, out := capture(t, func() int { return CommitMsg(msg) })
	if rc != 1 || strings.Contains(strings.ToLower(out), term) {
		t.Fatalf("rc=%d out=%s", rc, out)
	}
}

func TestPrePushScansOnlyOutgoingCommits(t *testing.T) {
	dir := newRepo(t)
	remote := t.TempDir()
	run(t, remote, "init", "-q", "--bare", "-b", "main")
	run(t, dir, "remote", "add", "origin", remote)
	writeConfig(t, "remote: "+regexpQuote(remote)+"\n"+term+"\n")

	base := commitFile(t, dir, "a.txt", "one\n", "base")
	run(t, dir, "push", "-q", "--no-verify", "origin", "main")
	bad := commitFile(t, dir, "b.txt", "has "+term+"\n", "adds a term")
	zero := strings.Repeat("0", 40)

	rc, out := capture(t, func() int {
		return PrePush("origin", remote, strings.NewReader("refs/heads/main "+bad+" refs/heads/main "+base+"\n"))
	})
	if rc != 1 || !strings.Contains(out, "b.txt:1") {
		t.Fatalf("outgoing term not blocked: rc=%d out=%s", rc, out)
	}
	if strings.Contains(strings.ToLower(out), term) {
		t.Errorf("output prints the term: %s", out)
	}
	// The old, clean range is fine; so is a branch deletion.
	if rc, out := capture(t, func() int {
		return PrePush("origin", remote, strings.NewReader("refs/heads/main "+base+" refs/heads/main "+base+"\n"))
	}); rc != 0 {
		t.Fatalf("clean range blocked: %s", out)
	}
	if rc, _ := capture(t, func() int {
		return PrePush("origin", remote, strings.NewReader("(delete) "+zero+" refs/heads/x "+bad+"\n"))
	}); rc != 0 {
		t.Fatal("branch deletion blocked")
	}
	// A new branch with no remote counterpart scans everything not on the remote.
	if rc, _ := capture(t, func() int {
		return PrePush("origin", remote, strings.NewReader("refs/heads/feat "+bad+" refs/heads/feat "+zero+"\n"))
	}); rc != 1 {
		t.Fatal("new branch carrying the term not blocked")
	}
	// A destination outside the scope is not checked.
	writeConfig(t, "remote: gitlab\\.example\n"+term+"\n")
	if rc, _ := capture(t, func() int {
		return PrePush("origin", remote, strings.NewReader("refs/heads/main "+bad+" refs/heads/main "+base+"\n"))
	}); rc != 0 {
		t.Fatal("out-of-scope remote blocked")
	}
}

func TestCheckFindsHistory(t *testing.T) {
	dir := newRepo(t)
	writeConfig(t, term+"\n")
	commitFile(t, dir, "a.txt", "x "+term+"\n", "add")
	run(t, dir, "rm", "-q", "a.txt")
	run(t, dir, "commit", "-q", "--no-verify", "-m", "remove")
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	rc := Check()
	w.Close()
	os.Stdout = old
	out, _ := io.ReadAll(r)
	if rc != 1 || strings.Contains(strings.ToLower(string(out)), term) {
		t.Fatalf("rc=%d out=%s", rc, out)
	}
}

func TestInitWritesPrivateFileOnceAndInvalidConfigBlocks(t *testing.T) {
	p := filepath.Join(t.TempDir(), "cfg", "denylist")
	t.Setenv("SHIELD_DENYLIST_FILE", p)
	if Init() != 0 {
		t.Fatal("Init failed")
	}
	fi, err := os.Stat(p)
	if err != nil || fi.Mode().Perm() != 0o600 {
		t.Fatalf("file mode: %v %v", fi, err)
	}
	_ = os.WriteFile(p, []byte("keep me\n"), 0o600)
	Init()
	b, _ := os.ReadFile(p)
	if string(b) != "keep me\n" {
		t.Error("Init overwrote an existing file")
	}
	_ = os.WriteFile(p, []byte("(broken\n"), 0o600)
	newRepo(t)
	if rc, _ := capture(t, PreCommit); rc != 1 {
		t.Error("an invalid config must block, not silently disable the guard")
	}
}
