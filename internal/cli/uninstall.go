package cli

import (
	"fmt"
	"os"
	"os/exec"
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
		if err := os.RemoveAll(home); err != nil {
			fmt.Fprintln(os.Stderr, "shield: could not remove", home, ":", err)
			return 1
		}
		fmt.Printf("   Removed %s (cached binaries, hook, config)\n", home)
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
