package cli

import (
	"fmt"
	"os"

	"github.com/chinmaymjog/shield-go/internal/denylist"
)

// runDenylist handles `shield denylist ...`. Exit code 1 means "block"; 2 is a
// usage error. The generated hook shims treat only 1 as a block, so a hook
// installed by a newer shield never blocks commits when run by an older binary.
func runDenylist(args []string) int {
	if len(args) == 0 {
		denylistUsage()
		return 2
	}
	switch args[0] {
	case "hook":
		if len(args) < 2 {
			denylistUsage()
			return 2
		}
		switch args[1] {
		case "pre-commit":
			return denylist.PreCommit()
		case "commit-msg":
			if len(args) < 3 {
				return 0
			}
			return denylist.CommitMsg(args[2])
		case "pre-push":
			remote, url := "", ""
			if len(args) > 2 {
				remote = args[2]
			}
			if len(args) > 3 {
				url = args[3]
			}
			return denylist.PrePush(remote, url, os.Stdin)
		}
		denylistUsage()
		return 2
	case "check":
		return denylist.Check()
	case "status":
		return denylist.Status()
	case "init":
		return denylist.Init()
	}
	denylistUsage()
	return 2
}

func denylistUsage() {
	fmt.Fprint(os.Stderr, `Usage: shield denylist <command>

Commands:
  init      create a starter deny-list file (mode 600) outside any repository
  status    show the effective config and whether it applies to this repo
  check     scan this repository's whole local history by hand
  hook      run as a git hook (pre-commit, commit-msg, pre-push); used by 'shield install'
`)
}
