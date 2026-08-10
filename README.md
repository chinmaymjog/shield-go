# sourceguard-shield-go

Go rewrite of [sourceguard-shield](../sourceguard-shield): a git pre-commit
hook that scans staged changes for secrets (via gitleaks + trufflehog)
before they're committed.

**Status: work in progress.** This replaces the bash-based
install/update/uninstall scripts with a single static binary distributed via
a Homebrew tap, so `brew install`/`upgrade`/`uninstall` replace
`setup-machine.sh`/`self-update.sh`/`uninstall-machine.sh` entirely.

## Install

```sh
brew tap chinmaymjog/homebrew-shield
brew install chinmaymjog/homebrew-shield/shield
```

## Build

```sh
go build -o bin/shield ./cmd/shield
./bin/shield version
```

## Releasing

Push a `vX.Y.Z` tag to `main`. A GitHub Actions workflow runs
[goreleaser](https://goreleaser.com) (see [.goreleaser.yaml](.goreleaser.yaml)),
which cross-builds `darwin`/`linux` × `amd64`/`arm64` binaries, publishes a
GitHub Release with the archives attached, and pushes the matching
`Formula/shield.rb` to [homebrew-shield](https://github.com/chinmaymjog/homebrew-shield).

## Commands

- `shield hook-run` — run as the git pre-commit hook (scans staged files).
  Not yet implemented.
- `shield check-conflicts` — report repos with a conflicting local
  `core.hooksPath`. Not yet implemented.
- `shield version` — print the shield version.

## Design notes

- No third-party CLI framework (no cobra/pflag) — stdlib `flag` plus a
  small dispatcher is enough for a handful of subcommands, and keeps this
  security tool's dependency graph at zero external Go modules for now.
- Engines (gitleaks, trufflehog) are shelled out to as standalone binaries,
  same as the bash version — not vendored as Go libraries.
