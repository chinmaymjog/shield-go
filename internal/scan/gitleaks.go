package scan

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// ignoreFileName is the per-repo gitleaks suppression file.
const ignoreFileName = ".gitleaksignore"

// runGitleaks runs `gitleaks protect --staged` against the git index diff
// directly — naturally scoped to what's about to be committed, no manual
// file copy needed. --verbose prints the File/Line/RuleID for each finding
// (without it, only a "leaks found: N" count is shown); --log-level warn
// drops the routine "N commits scanned" noise; --no-color avoids raw ANSI
// escapes around --redact's REDACTED text.
func runGitleaks(binPath, configPath, repoRoot string) (bool, error) {
	args := []string{
		"protect", "--staged", "--no-banner", "--verbose", "--log-level", "warn", "--no-color",
		"--config", configPath, "--source", repoRoot,
	}
	ignorePath, warning, cleanup := resolveIgnore(repoRoot)
	defer cleanup()
	if warning != "" {
		fmt.Fprintln(os.Stderr, warning)
	}
	if ignorePath != "" {
		args = append(args, "--gitleaks-ignore-path", ignorePath)
	}
	args = append(args, "--redact")

	cmd := exec.Command(binPath, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return runPasses(cmd)
}

// resolveIgnore decides which .gitleaksignore gitleaks may honour. Only the
// version already committed at HEAD counts: honouring the working-tree file
// would let a commit that adds an ignore entry also suppress the finding it
// is committing, and would let a file nobody has reviewed silence the scan.
//
// It returns the path of a temp copy of the committed file (empty if there is
// none), a warning for the user if the working-tree file differs from the
// committed one (empty otherwise), and a cleanup func. Any failure to read
// HEAD means no ignore file is honoured, which can only produce more
// findings, never fewer.
func resolveIgnore(repoRoot string) (path, warning string, cleanup func()) {
	cleanup = func() {}

	committed, hasCommitted := committedIgnore(repoRoot)
	if hasCommitted {
		f, err := os.CreateTemp("", "sg-shield-ignore-*")
		if err == nil {
			tmp := f.Name()
			_, werr := f.Write(committed)
			cerr := f.Close()
			if werr == nil && cerr == nil {
				path = tmp
				cleanup = func() { os.Remove(tmp) }
			} else {
				os.Remove(tmp)
			}
		}
	}

	working, err := os.ReadFile(filepath.Join(repoRoot, ignoreFileName))
	switch {
	case err != nil:
		// no working-tree file; if one is committed and was deleted, nothing
		// to warn about — the committed version is still what applies.
	case !hasCommitted:
		warning = fmt.Sprintf("shield: %s is not committed yet, so it is ignored for this commit. "+
			"Commit it on its own first; its entries apply from the next commit.", ignoreFileName)
	case !bytes.Equal(working, committed):
		warning = fmt.Sprintf("shield: %s differs from the committed version; only the committed version is honoured. "+
			"Commit your change to it on its own first.", ignoreFileName)
	}
	return path, warning, cleanup
}

// committedIgnore returns the contents of .gitleaksignore at HEAD, and whether
// it exists there. A repo with no commits yet, or no such file at HEAD, reports
// false.
func committedIgnore(repoRoot string) ([]byte, bool) {
	out, err := exec.Command("git", "-C", repoRoot, "show", "HEAD:"+ignoreFileName).Output()
	if err != nil {
		return nil, false
	}
	return out, true
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
