# Threat model

shield installs a git hook on every repository on a developer's machine and
downloads and executes third-party binaries on each commit. This document
states what it is trying to protect, who it is defending against, what it
relies on, and what it does not cover. It describes the code as of the commit
that introduced it; where a control is new or unproven, that is said.

Report a vulnerability through [SECURITY.md](../SECURITY.md), not a public issue.

## What shield is for

shield is a **seatbelt against accidents**, not a defence against a determined
insider. Its main job is stopping a developer from committing a credential they
did not mean to commit. Everything in the second half of this document is about
keeping the tool itself from becoming a way to attack the machine it runs on.

## Assets

| Asset | Why it matters |
| --- | --- |
| Secrets in staged changes | The thing shield exists to keep out of git history. |
| The developer machine | A global hook runs on every commit in every repo, and it executes downloaded binaries, so shield is code execution on every `git commit`. |
| The engine cache (`~/.sourceguard-shield/bin`) | Whatever is in it gets executed. |
| The hook scripts (`~/.sourceguard-shield/hooks`) | Git executes them directly. |
| Deny-list terms (`~/.config/shield/denylist`) | Internal project and customer names; leaking them defeats the point. |
| Release integrity | Users install shield and trust it to be what the maintainer built. |

## Actors

| Actor | Capability assumed |
| --- | --- |
| **Accidental committer** | The primary case. Pastes a key into a file and stages it. |
| **Malicious or compromised repository** | Controls files in a repo the developer clones: `.gitleaksignore`, repo hooks, hook-manager config. Cannot run code until the developer runs git there. |
| **Network attacker** | Can block or alter requests to GitHub while an engine is being downloaded. |
| **Compromised upstream release** | gitleaks or trufflehog (or their GitHub release) ships a malicious binary or checksums file. |
| **Compromised shield release** | Someone alters a shield release or the Homebrew tap. |
| **Other local user** | A different account on the same machine, with write access to loosely-protected directories. |
| **Same-user process** | Any process running as the developer, including a malicious dependency. |

Not in scope as an adversary: a developer who deliberately wants to bypass the
hook (see [Non-goals](#non-goals-and-known-gaps)).

## Trust boundaries and assumptions

shield trusts:

- **GitHub's TLS endpoint** to serve release assets, but never to *authenticate*
  them. Authenticity comes from the pins below.
- **Homebrew** and the maintainer's tap to deliver the shield binary itself.
- **The developer's own user account.** A process running as the developer can
  already read their secrets and edit their git config; shield does not try to
  defend against it.
- **The embedded gitleaks ruleset.** It ships inside the binary and is passed
  with `--config`, so a repository cannot substitute its own rules.

shield does not trust:

- Anything fetched from the network until it matches a pinned digest.
- Anything in the engine cache that is not a regular file owned by the current
  user and closed to group and other.
- The contents of a repository, other than a committed `.gitleaksignore` (see the gap below).

## Threats and mitigations

### Secrets reach git history

- **Mitigation:** gitleaks scans the staged diff (`protect --staged`) and
  trufflehog scans a snapshot of exactly the staged files with
  `--only-verified`. Either failing blocks the commit. gitleaks runs with
  `--redact` so findings do not print the secret.
- **Residual:** detection is shape- and verification-based. See the README's
  [What shield catches](../README.md#what-shield-catches-and-what-it-wont).

### A network attacker substitutes an engine binary

- **Mitigation:** each engine version is pinned in `internal/engines/spec.go`
  by the sha256 of its `checksums.txt`. The download fails closed: a missing
  pin, a failed fetch or any digest mismatch installs nothing. The asset is then
  checked against the (now authenticated) `checksums.txt`.
- **Why it is built this way:** an implementation that falls back to "trust it"
  when `checksums.txt` cannot be fetched lets an attacker who can block that one
  request install anything. shield never falls back.

### A malicious archive abuses extraction

- **Mitigation:** extraction accepts only a regular-file entry with the expected
  name, writes at most 1 GiB, and creates the output exclusively with mode 0700.
  Symlinks, hardlinks and directories are rejected. (#37)
- **Residual:** this is defence in depth. The checksum pin is the primary
  control.

### The engine cache is tampered with

- **Mitigation:** the state and `bin` directories are created 0700, loose
  permissions on directories the user owns are tightened, and directories owned
  by someone else are refused. A cached binary that is not a regular file owned
  by the user, or is group/world-writable, is deleted and re-fetched through the
  verified download rather than executed. This also covers
  `SOURCEGUARD_SHIELD_HOME` pointing at a directory owned by someone else. (#38)
- **Residual:** a same-user process can still modify the cache. The cached
  binary is not re-hashed against a stored digest, because an attacker who can
  write the binary can write the digest.

### A shield release is tampered with

- **Mitigation:** releases are signed keylessly with cosign, carry a per-archive
  SBOM and a GitHub build-provenance attestation, so users can check a download
  against the release workflow that built it. Verification steps are in the
  README. (#39)
- **Status:** configured but **not yet exercised by a real release**. Treat the
  controls as unproven until the first signed release has been verified.
- **Residual:** releases made before signing was added have only
  `checksums.txt`, which anyone able to alter a release can also alter.

### An upstream engine release is compromised

- **Mitigation:** only the pin. The pin is a digest of a file the maintainer
  saw at bump time; it does not prove the upstream project signed it. A pin taken
  from an already-compromised release would be trusted.
- **Gap:** shield does not verify upstream signatures. Tracked in #43.

### The hook is silently bypassed

A global `core.hooksPath` has the lowest precedence in git's config, so these
all stop the hook from running with no error:

- A repository with its own `core.hooksPath` (Husky and similar).
- `git commit --no-verify`.
- A `git -c core.hooksPath=...` override or `GIT_CONFIG_*` environment.

**Mitigation:** `shield install` and `shield check-conflicts` walk common
developer roots (`~/repos`, `~/src`, `~/work`, `~/workspace`, or the whole home
directory with `SG_SHIELD_DEEP_CONFLICT_SCAN=true`) and report repos whose local
`core.hooksPath` overrides shield's. The check is informational and only runs
when invoked, so a repo that adds an override later is not noticed until it is
run again. `--no-verify` is not defended against (see below).

Because a global hooks path disables each repo's own `.git/hooks`, shield also
installs shims that forward every standard hook to the repo's own copy. This
keeps repo hooks working but means a malicious repo's hook runs exactly as it
would without shield.

### The deny-list leaks the terms it protects

- **Mitigation:** matches are reported as `file:line` or a commit hash, never as
  text, so terms do not land in terminal scrollback, CI logs or screenshots. A
  matching path is shown as `[path hidden]`. Parse errors never repeat a pattern.
  The file lives outside every repository (`shield denylist init` creates it with
  mode 600) and only applies to repositories matching a configured `remote:` or
  `path:` scope.
- **Residual:** lock files and SVGs are skipped to avoid hash noise, so a term in
  those files is not caught.

## Non-goals and known gaps

These are deliberate limits or open weaknesses, listed so no one assumes
coverage that is not there.

- **`git commit --no-verify` and hook overrides.** A developer who wants to
  bypass the hook can. shield catches accidents, not intent. Server-side or CI
  scanning is the control for that.
- **A committed `.gitleaksignore` is trusted.** Only the version at `HEAD` is
  honoured, so an entry cannot suppress a finding in the commit that adds it, and
  shield warns when the working-tree copy differs. But a repository that ships a
  `.gitleaksignore` in its history can still suppress findings in later commits
  without the developer having reviewed it, and shield does not report when an
  entry actually suppressed something. (#46)
- **History.** Only what is staged in this commit is scanned. Secrets already in
  history, or in unstaged files, are out of scope.
- **Deletions and mode changes are not scanned.** The staged-file filter is
  add/copy/modify/rename only.
- **Verification sends candidate secrets to their provider.** trufflehog's
  `--only-verified` confirms a credential by trying it against the live service.
  That means a candidate secret is sent to its own provider's API from the
  developer's machine. Offline, verification cannot succeed, so a commit with a
  real, unrevoked secret may pass trufflehog; gitleaks still applies.
- **A trufflehog failure blocks the commit.** The hook fails closed when an
  engine cannot be prepared or run, including offline on first use. The cost is a
  nudge toward `--no-verify`.
- **Staged content is written to a temporary directory.** The snapshot trufflehog
  scans is created under the OS temp directory (via `os.MkdirTemp`, mode 0700) and
  removed when the hook exits. A hook killed mid-run can leave it behind.
- **Same-user compromise.** A malicious dependency or script running as the
  developer can edit the cache, the hook scripts, the global git config and the
  deny-list. shield does not defend against it.
- **The hooks directory is created 0755 by `shield install`.** It sits inside the
  0700 state directory once the cache has been used, but `install` itself does not
  yet restrict it.
- **`$PATH` lookup.** The hook script finds `shield` on `$PATH` first, then a few
  well-known install locations. An attacker who controls `$PATH` can run a
  different `shield`.
- **trufflehog output.** Whether trufflehog prints the raw matched credential to
  the terminal when it reports a finding has not been checked here. If it does,
  that is a leak into scrollback that gitleaks's `--redact` avoids.

## Open items

| Item | Tracking |
| --- | --- |
| Verify upstream engine signatures, not only a pinned digest | #43 |
| Confirm the first signed release verifies as documented | follow-up to #39 |
| Create the hooks directory 0700 in `shield install` | #47 |
| Report when a committed `.gitleaksignore` entry suppresses a finding | follow-up to #46 |
| Check what trufflehog prints for a finding | not yet filed |
| Surface `core.hooksPath` overrides that appear after install | #48 |
