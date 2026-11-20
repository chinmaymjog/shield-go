// Package denylist blocks commits and pushes that add terms from a private
// list: internal project names, customer names, addresses you never want in a
// repository. The list lives in a file outside every repository and is applied
// only to repositories that match a configured scope, so repositories where
// those terms are normal are left alone.
//
// Matches are reported as file:line (or a commit hash), never as text, so the
// terms do not end up in terminal scrollback, CI logs or screenshots.
package denylist

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Config is the parsed deny-list file.
type Config struct {
	Path     string         // file the config was read from
	Patterns *regexp.Regexp // case-insensitive OR of every pattern line; nil if none
	Count    int            // number of pattern lines
	Remote   *regexp.Regexp // scope: matches a remote URL
	Dir      *regexp.Regexp // scope: matches the repository's top-level path
}

// FilePath returns where the deny-list file is expected: $SHIELD_DENYLIST_FILE,
// else $XDG_CONFIG_HOME/shield/denylist, else ~/.config/shield/denylist.
func FilePath() string {
	if p := os.Getenv("SHIELD_DENYLIST_FILE"); p != "" {
		return p
	}
	if x := os.Getenv("XDG_CONFIG_HOME"); x != "" {
		return filepath.Join(x, "shield", "denylist")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "shield", "denylist")
}

// Load reads and parses the deny-list file. A missing file is not an error: it
// returns (nil, nil), meaning the guard is not configured. Error text never
// repeats a pattern.
//
// File format, one entry per line:
//
//	# comment
//	remote: <regex>   scope: apply when a remote URL matches
//	path: <regex>     scope: apply when the repository path matches
//	<regex>           a term to block (Go RE2 syntax, case-insensitive)
//
// Without a remote: or path: line the guard is inactive everywhere.
func Load(path string) (*Config, error) {
	if path == "" {
		return nil, nil
	}
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("denylist: cannot read %s: %w", path, err)
	}
	defer f.Close()

	c := &Config{Path: path}
	var patterns []string
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	n := 0
	for sc.Scan() {
		n++
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		switch {
		case strings.HasPrefix(line, "remote:"):
			re, err := regexp.Compile("(?i)" + strings.TrimSpace(strings.TrimPrefix(line, "remote:")))
			if err != nil {
				return nil, fmt.Errorf("denylist: %s line %d: invalid remote pattern", path, n)
			}
			c.Remote = re
		case strings.HasPrefix(line, "path:"):
			re, err := regexp.Compile("(?i)" + strings.TrimSpace(strings.TrimPrefix(line, "path:")))
			if err != nil {
				return nil, fmt.Errorf("denylist: %s line %d: invalid path pattern", path, n)
			}
			c.Dir = re
		default:
			if _, err := regexp.Compile("(?i)" + line); err != nil {
				return nil, fmt.Errorf("denylist: %s line %d: invalid pattern", path, n)
			}
			patterns = append(patterns, line)
		}
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("denylist: cannot read %s: %w", path, err)
	}
	if len(patterns) > 0 {
		c.Count = len(patterns)
		c.Patterns = regexp.MustCompile("(?i)(?:" + strings.Join(patterns, ")|(?:") + ")")
	}
	return c, nil
}

// Active reports whether the guard applies given the repository path and the
// remote URLs in play. It is false without patterns or without any scope line.
func (c *Config) Active(repoPath string, urls ...string) bool {
	if c == nil || c.Patterns == nil {
		return false
	}
	if c.Dir != nil && repoPath != "" && c.Dir.MatchString(filepath.ToSlash(repoPath)) {
		return true
	}
	if c.Remote != nil {
		for _, u := range urls {
			if u != "" && c.Remote.MatchString(u) {
				return true
			}
		}
	}
	return false
}

// MatchText reports whether s contains a deny-listed term.
func (c *Config) MatchText(s string) bool {
	return c != nil && c.Patterns != nil && c.Patterns.MatchString(s)
}

// Template is the starter file written by `shield denylist init`. It contains no
// real terms.
const Template = `# Deny-list for shield. Keep this file private; it must not live in a repository.
#
# remote: and path: lines say where the guard applies. Without at least one of
# them it does nothing, so repositories where these terms are normal are safe.
#
#   remote: github\.com[:/]yourname/
#   path: /repos/personal/
#
# Every other line is a term to block: a Go (RE2) regular expression, matched
# case-insensitively. \b marks a word boundary, which avoids matches inside hashes.
#
#   \bprojectcodename
#   customername
`
