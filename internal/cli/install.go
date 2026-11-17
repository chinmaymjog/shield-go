package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/chinmaymjog/shield-go/internal/conflicts"
)

// wellKnownShieldLocations are absolute paths checked, in order, by the
// generated hook script when "shield" isn't resolvable via $PATH — GUI git
// clients (Tower, SourceTree, VS Code, Finder-launched apps) commonly run
// hooks with a minimal PATH that excludes Homebrew's bin dir.
var wellKnownShieldLocations = []string{
	"/opt/homebrew/bin/shield",              // Homebrew, Apple Silicon
	"/usr/local/bin/shield",                 // Homebrew, Intel mac / generic
	"/home/linuxbrew/.linuxbrew/bin/shield", // Linuxbrew
}

// runInstall wires shield into git as a global pre-commit hook: writes a
// hook script under shield's hooks dir and points git's global
// core.hooksPath at it. Every repo on the machine — current and future
// clones — is covered from then on; there's nothing to change per-repo.
// Safe to re-run any time (e.g. after upgrading shield).
func runInstall(args []string) int {
	hooksDir, err := conflicts.HooksDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, "shield:", err)
		return 1
	}
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "shield: could not create", hooksDir, ":", err)
		return 1
	}

	hookPath := filepath.Join(hooksDir, "pre-commit")
	if err := os.WriteFile(hookPath, []byte(hookScript(selfPathHint())), 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "shield: could not write", hookPath, ":", err)
		return 1
	}

	for _, name := range forwardedHooks {
		shimPath := filepath.Join(hooksDir, name)
		if err := os.WriteFile(shimPath, []byte(forwardScript(name)), 0o755); err != nil {
			fmt.Fprintln(os.Stderr, "shield: could not write", shimPath, ":", err)
			return 1
		}
	}

	if err := exec.Command("git", "config", "--global", "core.hooksPath", hooksDir).Run(); err != nil {
		fmt.Fprintln(os.Stderr, "shield: could not set git core.hooksPath:", err)
		return 1
	}

	fmt.Println("✅ SourceGuard Shield installed.")
	fmt.Println("   core.hooksPath =", hooksDir)
	fmt.Println("   Every repo on this machine is now protected — no per-repo setup needed.")
	fmt.Println()

	// core.hooksPath is the LOWEST-precedence git hook setting: any repo
	// with its own local core.hooksPath (Husky, lint-staged, etc.) silently
	// overrides this global one. Surface known conflicts right away.
	// Non-fatal — install still succeeds either way.
	runCheckConflicts(nil)
	return 0
}

// hookScript is the pre-commit script installed at <hooksDir>/pre-commit.
// It's a plain, readable POSIX script (not a symlink to the shield binary,
// and not a path baked in via a symlink-resolved os.Executable()) so a
// developer can `cat` it to see exactly what runs, and so it keeps working
// across a `brew upgrade` — resolving through Homebrew's symlink into a
// specific Cellar version's path would break the moment `brew cleanup`
// removes that old version. selfHint, if non-empty, is tried last: a
// best-effort extra location for non-Homebrew installs (e.g. a local
// `go build` whose output directory isn't on PATH or in the list above).
func hookScript(selfHint string) string {
	candidates := append(append([]string{}, wellKnownShieldLocations...), selfHint)

	var fallback strings.Builder
	for _, c := range candidates {
		if c == "" {
			continue
		}
		fmt.Fprintf(&fallback, "[ -z \"$SHIELD\" ] && [ -x %s ] && SHIELD=%s\n", shq(c), shq(c))
	}

	return fmt.Sprintf(`#!/bin/sh
# Installed by "shield install". Resolves the shield binary robustly:
# $PATH first (the common case for terminal-driven commits), then a short
# list of well-known install locations for GUI git clients that run hooks
# with a minimal PATH. After the scan passes, chains to the repo's own
# pre-commit hook (core.hooksPath would otherwise silently disable it).
SHIELD=
if command -v shield >/dev/null 2>&1; then
  SHIELD=shield
else
%s
fi
if [ -z "$SHIELD" ]; then
  echo "shield: could not locate the shield binary (checked \$PATH and common install locations)." >&2
  echo "shield: re-run 'shield install', or make sure shield is on \$PATH, then retry the commit." >&2
  exit 1
fi
"$SHIELD" hook-run "$@" || exit $?
repo_hook="$(git rev-parse --git-common-dir 2>/dev/null)/hooks/pre-commit"
if [ -f "$repo_hook" ] && [ -x "$repo_hook" ]; then
  exec "$repo_hook" "$@"
fi
exit 0
`, fallback.String())
}

// forwardedHooks are the standard git hooks (other than pre-commit, which
// runs the scan) for which shield installs a thin shim. With a global
// core.hooksPath git ignores <repo>/.git/hooks entirely, so each shim
// forwards to the repo's own hook of the same name.
var forwardedHooks = []string{
	"applypatch-msg", "pre-applypatch", "post-applypatch",
	"prepare-commit-msg", "commit-msg", "post-commit",
	"pre-merge-commit", "post-merge", "pre-rebase", "post-rewrite",
	"post-checkout", "pre-push", "pre-auto-gc", "reference-transaction",
	"push-to-checkout", "fsmonitor-watchman", "post-index-change",
}

// forwardScript returns a shim that execs the repo's own hook <name>, if
// present and executable. exec keeps stdin (pre-push, post-rewrite) and
// the exit status intact.
func forwardScript(name string) string {
	return fmt.Sprintf(`#!/bin/sh
# Installed by "shield install": forwards to the repo's own %[1]s hook,
# which a global core.hooksPath would otherwise silently disable.
repo_hook="$(git rev-parse --git-common-dir 2>/dev/null)/hooks/%[1]s"
if [ -f "$repo_hook" ] && [ -x "$repo_hook" ]; then
  exec "$repo_hook" "$@"
fi
exit 0
`, name)
}

// shq shell-quotes s for safe embedding in the generated hook script.
func shq(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// selfPathHint returns the running shield binary's path as reported by
// os.Executable(), or "" if it can't be determined. Best-effort only — see
// hookScript's doc comment for why this isn't symlink-resolved.
func selfPathHint() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	return exe
}
