package denylist

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const zeroSHA = "0000000000000000000000000000000000000000"

// maxPushCommits caps how many outgoing commits a push scan inspects.
const maxPushCommits = 2000

func gitOut(dir string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	return cmd.Output()
}

type repository struct {
	root string
	urls []string
}

func currentRepo() (repository, bool) {
	out, err := gitOut("", "rev-parse", "--show-toplevel")
	if err != nil {
		return repository{}, false // outside a repository: never block
	}
	root := strings.TrimSpace(string(out))
	return repository{root: root, urls: remoteURLs(root)}, true
}

func remoteURLs(root string) []string {
	out, err := gitOut(root, "remote", "-v")
	if err != nil {
		return nil
	}
	seen := map[string]bool{}
	var urls []string
	for _, l := range strings.Split(string(out), "\n") {
		f := strings.Fields(l)
		if len(f) >= 2 && !seen[f[1]] {
			seen[f[1]] = true
			urls = append(urls, f[1])
		}
	}
	return urls
}

// load reads the config. An unreadable or invalid file blocks (exit 1) so a typo
// cannot silently switch the guard off.
func load() (*Config, int) {
	c, err := Load(FilePath())
	if err != nil {
		fmt.Fprintln(os.Stderr, "❌ shield denylist:", err)
		return nil, 1
	}
	return c, 0
}

// diffLocations runs a git command that prints a unified diff and scans it.
func diffLocations(c *Config, dir string, args ...string) []string {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	pipe, err := cmd.StdoutPipe()
	if err != nil || cmd.Start() != nil {
		return nil
	}
	locs := c.ScanDiff(pipe)
	_, _ = io.Copy(io.Discard, pipe)
	_ = cmd.Wait()
	return locs
}

// PreCommit checks what is about to be committed: staged added lines, staged
// file paths, and the author and committer identity. Returns 1 to block.
func PreCommit() int {
	c, rc := load()
	if rc != 0 {
		return rc
	}
	rp, ok := currentRepo()
	if !ok || !c.Active(rp.root, rp.urls...) {
		return 0
	}
	rep := &report{w: os.Stderr}
	for _, v := range []struct{ env, label string }{{"GIT_AUTHOR_IDENT", "author"}, {"GIT_COMMITTER_IDENT", "committer"}} {
		if out, err := gitOut(rp.root, "var", v.env); err == nil && c.MatchText(string(out)) {
			rep.add("the %s identity matches the deny-list (check user.name and user.email for this repository)", v.label)
		}
	}
	if out, err := gitOut(rp.root, "diff", "--cached", "--name-only", "--diff-filter=ACMR", "-z"); err == nil {
		paths := strings.Split(strings.Trim(string(out), "\x00"), "\x00")
		if n := c.MatchingPaths(paths); n > 0 {
			rep.add("%d staged file path(s) match the deny-list", n)
		}
	}
	rep.addLocations("staged line: ", diffLocations(c, rp.root, "diff", "--cached", "-U0", "--no-ext-diff", "--diff-filter=ACMR"))
	if rep.count() == 0 {
		return 0
	}
	rep.print("❌ shield denylist: commit blocked.")
	return 1
}

// CommitMsg checks the commit message file git passes to the commit-msg hook.
func CommitMsg(file string) int {
	c, rc := load()
	if rc != 0 {
		return rc
	}
	rp, ok := currentRepo()
	if !ok || !c.Active(rp.root, rp.urls...) {
		return 0
	}
	b, err := os.ReadFile(file)
	if err != nil {
		return 0
	}
	var msg []string
	for _, l := range strings.Split(string(b), "\n") {
		if !strings.HasPrefix(l, "#") {
			msg = append(msg, l)
		}
	}
	if c.MatchText(strings.Join(msg, "\n")) {
		rep := &report{w: os.Stderr}
		rep.add("the commit message matches the deny-list")
		rep.print("❌ shield denylist: commit blocked.")
		return 1
	}
	return 0
}

// scanCommit adds a finding for every way commit sha matches.
func scanCommit(c *Config, root, sha string, rep *report) {
	short := sha
	if len(short) > 10 {
		short = short[:10]
	}
	out, err := gitOut(root, "log", "-1", "--format=%an%x00%ae%x00%cn%x00%ce%x00%B", sha)
	if err == nil {
		parts := strings.SplitN(string(out), "\x00", 5)
		if len(parts) == 5 {
			if c.MatchText(strings.Join(parts[:4], "\n")) {
				rep.add("commit %s: author or committer identity matches", short)
			}
			if c.MatchText(parts[4]) {
				rep.add("commit %s: message matches", short)
			}
		}
	}
	if out, err := gitOut(root, "diff-tree", "--no-commit-id", "--name-only", "-r", "-m", "--root", "--diff-filter=ACMR", "-z", sha); err == nil {
		if n := c.MatchingPaths(strings.Split(strings.Trim(string(out), "\x00"), "\x00")); n > 0 {
			rep.add("commit %s: %d file path(s) match", short, n)
		}
	}
	for _, loc := range diffLocations(c, root, "show", "-U0", "--no-ext-diff", "--format=", sha) {
		rep.add("commit %s: %s", short, loc)
	}
}

// PrePush checks the commits a push would send. git passes the remote name and
// URL as arguments and "<local ref> <local sha> <remote ref> <remote sha>" lines
// on stdin. Returns 1 to block.
func PrePush(remoteName, remoteURL string, in io.Reader) int {
	c, rc := load()
	if rc != 0 {
		return rc
	}
	rp, ok := currentRepo()
	if !ok || !c.Active(rp.root, remoteURL) {
		return 0
	}
	commits := []string{}
	seen := map[string]bool{}
	sc := bufio.NewScanner(in)
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		if len(f) != 4 || f[1] == zeroSHA {
			continue // blank line or a branch deletion
		}
		var out []byte
		var err error
		if f[3] != zeroSHA {
			out, err = gitOut(rp.root, "rev-list", "-n", fmt.Sprint(maxPushCommits), f[3]+".."+f[1])
		}
		if f[3] == zeroSHA || err != nil {
			args := []string{"rev-list", "-n", fmt.Sprint(maxPushCommits), f[1], "--not"}
			if remoteName != "" {
				args = append(args, "--remotes="+remoteName)
			}
			out, _ = gitOut(rp.root, args...)
		}
		for _, s := range strings.Fields(string(out)) {
			if !seen[s] {
				seen[s] = true
				commits = append(commits, s)
			}
		}
	}
	rep := &report{w: os.Stderr}
	for _, s := range commits {
		scanCommit(c, rp.root, s, rep)
	}
	if rep.count() == 0 {
		return 0
	}
	rep.print(fmt.Sprintf("❌ shield denylist: push blocked (%d commit(s) checked).", len(commits)))
	fmt.Fprintln(os.Stderr, "   Rewrite the commits (git rebase / filter-repo) before pushing.")
	return 1
}

// Check scans the whole local history of the current repository, regardless of
// scope. It is for auditing a repository by hand.
func Check() int {
	c, rc := load()
	if rc != 0 {
		return rc
	}
	if c == nil || c.Patterns == nil {
		fmt.Fprintf(os.Stderr, "shield denylist: no patterns configured (%s)\n", FilePath())
		return 1
	}
	rp, ok := currentRepo()
	if !ok {
		fmt.Fprintln(os.Stderr, "shield denylist: not inside a git repository")
		return 1
	}
	out, err := gitOut(rp.root, "rev-list", "--branches", "--tags")
	if err != nil {
		fmt.Fprintln(os.Stderr, "shield denylist:", err)
		return 1
	}
	commits := strings.Fields(string(out))
	rep := &report{w: os.Stdout}
	for _, s := range commits {
		scanCommit(c, rp.root, s, rep)
	}
	if rep.count() == 0 {
		fmt.Printf("✅ shield denylist: %d commit(s) on all branches and tags are clean.\n", len(commits))
		return 0
	}
	rep.print(fmt.Sprintf("❌ shield denylist: %d finding(s) in %d commit(s).", rep.count(), len(commits)))
	return 1
}

// Status prints the effective configuration without printing any term.
func Status() int {
	path := FilePath()
	c, err := Load(path)
	if err != nil {
		fmt.Println("❌", err)
		return 1
	}
	if c == nil {
		fmt.Printf("shield denylist: not configured. Run `shield denylist init` to create %s.\n", path)
		return 0
	}
	fmt.Printf("file:      %s\n", c.Path)
	if fi, err := os.Stat(c.Path); err == nil && fi.Mode().Perm()&0o077 != 0 {
		fmt.Printf("warning:   the file is readable by other users; run: chmod 600 %s\n", c.Path)
	}
	fmt.Printf("patterns:  %d\n", c.Count)
	fmt.Printf("scope:     remote=%t path=%t\n", c.Remote != nil, c.Dir != nil)
	if c.Remote == nil && c.Dir == nil {
		fmt.Println("           (no scope line: the guard is inactive everywhere)")
	}
	if rp, ok := currentRepo(); ok {
		fmt.Printf("this repo: %s -> guard %s\n", filepath.Base(rp.root), map[bool]string{true: "ACTIVE", false: "not active"}[c.Active(rp.root, rp.urls...)])
	}
	return 0
}

// Init writes the starter file (mode 0600) unless one exists.
func Init() int {
	path := FilePath()
	if path == "" {
		fmt.Fprintln(os.Stderr, "shield denylist: cannot determine the config path; set SHIELD_DENYLIST_FILE")
		return 1
	}
	if _, err := os.Stat(path); err == nil {
		fmt.Printf("shield denylist: %s already exists; not touching it.\n", path)
		return 0
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		fmt.Fprintln(os.Stderr, "shield denylist:", err)
		return 1
	}
	if err := os.WriteFile(path, []byte(Template), 0o600); err != nil {
		fmt.Fprintln(os.Stderr, "shield denylist:", err)
		return 1
	}
	fmt.Printf("Created %s (mode 600).\nAdd a remote:/path: scope line and your terms, then run `shield denylist status`.\n", path)
	return 0
}
