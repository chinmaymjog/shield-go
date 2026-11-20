package denylist

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
)

// skipPath matches files that are mostly hashes and give accidental matches.
var skipPath = regexp.MustCompile(`(?i)(^|/)(package-lock\.json|go\.sum|yarn\.lock|pnpm-lock\.yaml)$|\.lock$|\.lock\.hcl$|\.svg$`)

var hunkNew = regexp.MustCompile(`^@@ -\d+(?:,\d+)? \+(\d+)`)

// maxLocations caps how many locations are listed.
const maxLocations = 20

// ScanDiff reads a unified diff (git diff -U0) and returns the locations of
// added lines that match, as "path:line". A path that itself matches is shown as
// "[path hidden]" so the term is not printed.
func (c *Config) ScanDiff(r io.Reader) []string {
	var locs []string
	seen := map[string]bool{}
	file := ""
	skip := false
	line := 0
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 1024*1024), 16*1024*1024)
	for sc.Scan() {
		t := sc.Text()
		switch {
		case strings.HasPrefix(t, "+++ "):
			p := strings.TrimPrefix(t, "+++ ")
			p = strings.Trim(p, `"`)
			p = strings.TrimPrefix(p, "b/")
			if p == "/dev/null" {
				p = ""
			}
			file = p
			skip = p == "" || skipPath.MatchString(p)
		case strings.HasPrefix(t, "@@"):
			if m := hunkNew.FindStringSubmatch(t); m != nil {
				line, _ = strconv.Atoi(m[1])
			}
		case strings.HasPrefix(t, "+") && !strings.HasPrefix(t, "+++"):
			if !skip && c.MatchText(t[1:]) {
				loc := c.location(file, line)
				if !seen[loc] {
					seen[loc] = true
					locs = append(locs, loc)
				}
			}
			line++
		}
	}
	return locs
}

func (c *Config) location(path string, line int) string {
	if c.MatchText(path) {
		path = "[path hidden]"
	}
	return fmt.Sprintf("%s:%d", path, line)
}

// MatchingPaths returns how many of paths contain a deny-listed term.
func (c *Config) MatchingPaths(paths []string) int {
	n := 0
	for _, p := range paths {
		if c.MatchText(p) {
			n++
		}
	}
	return n
}

// report prints locations and counts without ever printing matched text.
type report struct {
	w     io.Writer
	items []string
}

func (r *report) add(format string, a ...any) { r.items = append(r.items, fmt.Sprintf(format, a...)) }
func (r *report) addLocations(prefix string, locs []string) {
	for i, l := range locs {
		if i == maxLocations {
			r.add("%s…and %d more location(s)", prefix, len(locs)-maxLocations)
			return
		}
		r.add("%s%s", prefix, l)
	}
}
func (r *report) count() int { return len(r.items) }
func (r *report) print(header string) {
	fmt.Fprintln(r.w, header)
	for _, it := range r.items {
		fmt.Fprintln(r.w, "   "+it)
	}
	fmt.Fprintln(r.w, "   Matched text is never printed. Check it with your own copy of the deny-list.")
}
