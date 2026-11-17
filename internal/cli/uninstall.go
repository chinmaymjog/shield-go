package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/chinmaymjog/shield-go/internal/conflicts"
	"github.com/chinmaymjog/shield-go/internal/engines"
)

// runUninstall reverses runInstall: unset the global core.hooksPath (only
// if it's still ours — never touch a hooksPath a developer has since
// pointed elsewhere) and remove the shield home directory (cached
// binaries, hook script, everything). The shield binary itself is left
// alone; removing it is `brew uninstall`'s job.
func runUninstall(args []string) int {
	fmt.Println("🛡️  Uninstalling SourceGuard Shield...")

	hooksDir, err := conflicts.HooksDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, "shield:", err)
		return 1
	}

	current, _ := currentGlobalHooksPath()
	unset, msg := hooksPathAction(current, hooksDir)
	fmt.Println("  ", msg)
	if unset {
		if err := exec.Command("git", "config", "--global", "--unset", "core.hooksPath").Run(); err != nil {
			fmt.Fprintln(os.Stderr, "shield: could not unset git core.hooksPath:", err)
			return 1
		}
	}

	home, err := engines.Home()
	if err != nil {
		fmt.Fprintln(os.Stderr, "shield:", err)
		return 1
	}
	if info, err := os.Stat(home); err == nil && info.IsDir() {
		if err := checkSafeHome(home); err != nil {
			fmt.Fprintln(os.Stderr, "shield: refusing to remove", home+":", err)
			return 1
		}
		if err := removeShieldHome(home); err != nil {
			fmt.Fprintln(os.Stderr, "shield: could not remove", home, ":", err)
			return 1
		}
		fmt.Printf("   Removed shield files from %s (cached binaries, hook)\n", home)
	} else {
		fmt.Printf("   %s did not exist — nothing to remove.\n", home)
	}

	fmt.Println("✅ SourceGuard Shield uninstalled.")
	fmt.Println("   No repo on this machine runs local secret scanning until `shield install` is run again.")
	return 0
}

// hooksPathAction decides what runUninstall should do with the current
// global core.hooksPath value: unset it only if it's still shield's own
// hooksDir, otherwise leave it untouched (it may point at Husky/lint-staged
// or some other hook manager a developer has since switched to).
func hooksPathAction(current, ours string) (unset bool, msg string) {
	switch {
	case current == ours:
		return true, fmt.Sprintf("Unset global core.hooksPath (was %s)", ours)
	case current == "":
		return false, "global core.hooksPath was already unset — nothing to do there."
	default:
		return false, fmt.Sprintf("global core.hooksPath is %q, not ours — leaving it untouched.", current)
	}
}

// currentGlobalHooksPath returns the current global core.hooksPath, or ""
// if it's unset (git exits non-zero in that case, which is not an error
// here).
func currentGlobalHooksPath() (string, error) {
	out, err := exec.Command("git", "config", "--global", "--get", "core.hooksPath").Output()
	if err != nil {
		return "", nil
	}
	return strings.TrimSpace(string(out)), nil
}

// shieldChildren are the only entries shield creates inside its home
// directory. Uninstall removes just these, never the whole directory, so a
// SOURCEGUARD_SHIELD_HOME pointing at a shared folder can't cause data loss.
var shieldChildren = []string{"bin", "hooks"}

// checkSafeHome refuses homes that resolve to the filesystem root, the
// user's home directory, or the current working directory.
func checkSafeHome(home string) error {
	resolved, err := canonical(home)
	if err != nil {
		return err
	}
	if filepath.Dir(resolved) == resolved {
		return fmt.Errorf("%s is a filesystem root", resolved)
	}
	protected := map[string]string{}
	if uh, err := os.UserHomeDir(); err == nil {
		protected["the user's home directory"] = uh
	}
	if wd, err := os.Getwd(); err == nil {
		protected["the current working directory"] = wd
	}
	for what, p := range protected {
		if cp, err := canonical(p); err == nil && cp == resolved {
			return fmt.Errorf("%s is %s", resolved, what)
		}
	}
	return nil
}

func canonical(p string) (string, error) {
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(abs)
}

// removeShieldHome deletes only shieldChildren under home, then removes home
// itself if (and only if) that leaves it empty.
func removeShieldHome(home string) error {
	for _, c := range shieldChildren {
		if err := os.RemoveAll(filepath.Join(home, c)); err != nil {
			return err
		}
	}
	// os.Remove fails on a non-empty directory, which is what we want.
	_ = os.Remove(home)
	return nil
}
