package scan

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

// noisyTrufflehogLine matches trufflehog stderr lines with no actionable
// info: the ASCII-art banner and the routine "running source"/"finished
// scanning" status lines. Trufflehog has no flag to lower its own verbosity,
// so hooks/pre-commit filtered these by hand — same here.
var noisyTrufflehogLine = regexp.MustCompile(`🐷|running source|finished scanning`)

// runTrufflehog runs `trufflehog filesystem` against the staged-file
// snapshot — trufflehog has no staged-diff mode, so scanDir stands in for
// "only what's about to be committed".
func runTrufflehog(binPath, scanDir string) (bool, error) {
	cmd := exec.Command(binPath, "filesystem", scanDir, "--only-verified", "--no-update", "--fail")
	cmd.Stdout = os.Stdout

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	ok, err := runPasses(cmd)
	filterTrufflehogNoise(stderr.Bytes())
	return ok, err
}

// filterTrufflehogNoise prints raw's lines to stderr, dropping banner/status
// noise. Anything else (a genuine finding detail, error, or warning) is left
// intact.
func filterTrufflehogNoise(raw []byte) {
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" || noisyTrufflehogLine.MatchString(line) {
			continue
		}
		fmt.Fprintln(os.Stderr, line)
	}
}
