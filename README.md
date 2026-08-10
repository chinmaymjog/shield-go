# shield

A git pre-commit hook that scans staged changes for secrets — API keys,
tokens, credentials — using [gitleaks](https://github.com/gitleaks/gitleaks)
and [trufflehog](https://github.com/trufflesecurity/trufflehog), before
they ever reach a commit.

Ships as a single static Go binary with no runtime dependencies beyond git
itself. It installs **once per machine**, not once per repo: `shield
install` points git's global `core.hooksPath` at a hook script shield
writes, so every repository on that machine — current and future clones —
gets scanned automatically. There's no `.pre-commit-config.yaml`, no
per-repo setup, nothing for a consumer repo to opt into or maintain.

**Platforms:** macOS and Linux, `amd64`/`arm64`. No Windows build —
goreleaser only cross-compiles `darwin`/`linux` (see
[`.goreleaser.yaml`](.goreleaser.yaml)).

## Quick start

```sh
brew tap chinmaymjog/homebrew-shield
brew trust chinmaymjog/homebrew-shield  # one-time, newer Homebrew versions require this for new taps
brew install chinmaymjog/homebrew-shield/shield
shield install
```

That's it. Every `git commit` from then on runs gitleaks + trufflehog
against exactly what's staged. A clean commit passes through silently
(after a short one-time download the first time each engine runs); a
commit containing a likely secret is blocked with output telling you what
was found and how to respond.

```sh
shield uninstall   # run this first, while `shield` is still on PATH
brew uninstall chinmaymjog/homebrew-shield/shield
```

## How scanning works

On each `git commit`, the installed hook runs `shield hook-run`, which:

1. Exits immediately (no download, no scan) if there's nothing staged, or
   if you're not inside a git repo at all.
2. Downloads and caches gitleaks/trufflehog on first use — see
   [Engine versions](#engine-versions-gitleaks--trufflehog) below.
3. Runs `gitleaks protect --staged` directly against the git index diff.
4. Snapshots the staged files (as they are in the index, not the working
   tree) into a temp directory and runs `trufflehog filesystem
   --only-verified` against it — trufflehog has no staged-diff mode, so
   this scopes it to exactly what's about to be committed.
5. Blocks the commit (non-zero exit) if either tool reports a finding.

Only what's staged in *this* commit is scanned — pre-existing secrets
elsewhere in a repo's history are out of scope for a pre-commit hook by
design; that's a job for a separate history/continuous scanner.

A gitleaks false positive can be suppressed per-repo via a
`.gitleaksignore` file (fingerprint-based, checked automatically if
present). A trufflehog finding is a *verified* credential — the tool
actually confirmed it works against the live provider — so the answer is
to rotate/revoke it, not suppress it.

## Local hooksPath conflicts

`core.hooksPath` is git's lowest-precedence hook setting: any repo with its
own **local** `core.hooksPath` (most commonly set by Husky or another
JS-based hook manager) silently overrides shield's global one for that repo
— no error, no warning, nothing in the commit output to indicate it's not
being scanned.

```sh
shield check-conflicts [root]
```

scans for exactly this (`shield install` also runs it automatically right
after installing). With no argument it walks a handful of common developer
directories under `$HOME` (`repos`, `src`, `work`, `workspace`); pass an
explicit root to scope it, or set `SG_SHIELD_DEEP_CONFLICT_SCAN=true` to
walk the whole home directory. If it finds a conflict, the fix is to have
that repo's existing hook manager also invoke shield's hook — `shield
check-conflicts` prints the exact line to add (e.g. to `.husky/pre-commit`)
for each conflicting repo it finds.

## Engine versions (gitleaks / trufflehog)

Nothing needs to be pre-installed — gitleaks and trufflehog are downloaded
on demand, not expected to already be on the machine or on `$PATH`.

The exact trusted version of each tool is pinned in
[`internal/engines/spec.go`](internal/engines/spec.go) — hardcoded in the
shield source, not a config file read at runtime. `shield`'s own version
*is* the pin: bumping an engine version means editing `spec.go` and cutting
a new shield release.

The first `hook-run` after a fresh install (or after a shield version bump
that changes a pin) downloads that version once and caches it at
`~/.sourceguard-shield/bin/<tool>-<version>-<os>-<arch>`; every commit
after that reuses the cached binary with no network call. Downloads are
checksum-verified against a trust anchor baked into `spec.go` before
anything is extracted or executed — see the comments in
[`internal/engines/download.go`](internal/engines/download.go) for exactly
what's verified and why it fails closed on any mismatch.

Caching is keyed by version, so a pin bump downloads the new version
alongside the old one rather than replacing it in place; `shield uninstall`
clears the whole cache regardless of version.

## Forking this for your own org

Shield has no dependency on any other repo and no assumptions baked in
about who's using it — forking it to run against your own repos, under
your own name, with your own rules, is the expected way to adopt it beyond
personal use. Things you'll likely want to change:

- **Your own Homebrew tap.** Update `brews.repository` (owner/name) in
  [`.goreleaser.yaml`](.goreleaser.yaml) to your own tap repo, and
  `release.github` to your own GitHub repo. CI needs a `GITHUB_TOKEN` (repo
  scope) and a `HOMEBREW_TAP_TOKEN` (repo-scope PAT on the tap repo) — see
  [`.github/workflows/release.yml`](.github/workflows/release.yml).
- **Your own detection ruleset.** The gitleaks config is embedded straight
  into the binary from
  [`internal/scan/gitleaks.toml`](internal/scan/gitleaks.toml) (see
  [`internal/scan/config.go`](internal/scan/config.go)) — edit it and cut a
  release; there's nothing to configure at runtime.
- **Your own engine version pins**, if you want a newer/older
  gitleaks/trufflehog than the versions this repo currently trusts — see
  [Engine versions](#engine-versions-gitleaks--trufflehog) above.
- **The module path**, if you want `go install`/imports to point at your
  fork rather than `github.com/chinmaymjog/shield-go` — update `go.mod` and
  the import paths under `internal/`.
- **`SOURCEGUARD_SHIELD_HOME`**, if you don't want the default
  `~/.sourceguard-shield` state directory — every command in this repo
  reads it from that one env var (`internal/engines/home.go`), so it's a
  single override point, useful for testing without touching a real
  machine's global git config (see [Development](#development) below).

None of this requires touching the install/uninstall/scan logic itself —
it's all either a config value or a file swap.

## Development

```sh
go build -o bin/shield ./cmd/shield
go vet ./...
go test ./...
```

To exercise `install`/`uninstall` without touching your real machine's
global git config or `~/.sourceguard-shield`, sandbox both:

```sh
export GIT_CONFIG_GLOBAL=$(mktemp)          # isolates git's global config
export SOURCEGUARD_SHIELD_HOME=$(mktemp -d) # isolates shield's state dir
./bin/shield install
# ...test...
./bin/shield uninstall
```

## Releasing

Push a `vX.Y.Z` tag to `main`. A GitHub Actions workflow runs
[goreleaser](https://goreleaser.com) (see
[`.goreleaser.yaml`](.goreleaser.yaml)), which cross-builds
`darwin`/`linux` × `amd64`/`arm64` binaries, publishes a GitHub Release
with the archives attached, and pushes the matching `Formula/shield.rb` to
the configured Homebrew tap.

## Commands

- `shield install` — set up shield as the global git pre-commit hook
  (`core.hooksPath` + hook script). Run once per machine after
  `brew install`; safe to re-run any time (e.g. after upgrading shield).
- `shield uninstall` — reverse `shield install`: unset `core.hooksPath`
  (only if it's still shield's own — a hooksPath you've since pointed
  elsewhere is left untouched) and remove `~/.sourceguard-shield`.
- `shield hook-run` — run as the git pre-commit hook (scans staged files).
  Invoked automatically by the hook script `shield install` writes; not
  meant to be run by hand.
- `shield check-conflicts [root]` — report repos with a conflicting local
  `core.hooksPath`.
- `shield version` — print the shield version.

## Design notes

- No third-party CLI framework (no cobra/pflag) — stdlib `flag` plus a
  small dispatcher is enough for a handful of subcommands, and keeps this
  security tool's dependency graph at zero external Go modules.
- gitleaks and trufflehog are shelled out to as standalone downloaded
  binaries, not vendored as Go libraries — each stays independently
  updatable via its own upstream release process.
- `shield install` doesn't happen automatically as a Homebrew
  `post_install` step, on purpose: it mutates git's *global* config, which
  reaches beyond this formula's install prefix and would fire in any
  environment `brew install` runs in (including CI/containers that just
  want the binary). Homebrew also has no matching `post_uninstall` hook for
  Formulas, so an automatic install would be asymmetric with an explicit
  `shield uninstall`. Keeping it an explicit, auditable command is a
  deliberate tradeoff for a security tool — the Homebrew formula's caveats
  message points at it so it isn't easy to miss.
