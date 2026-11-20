#!/usr/bin/env bash
# End-to-end test: build shield, install it as the global git hook in an
# isolated HOME, then check that a clean commit passes and a commit containing
# a (randomly generated, fake) credential is blocked. Touches nothing outside
# the temp directory. Needs network: shield downloads gitleaks/trufflehog on
# first use.
set -euo pipefail

root="$(cd "$(dirname "$0")/.." && pwd)"
work="$(mktemp -d)"
trap 'chmod -R u+w "$work" 2>/dev/null; rm -rf "$work"' EXIT
mkdir -p "$work/bin"

# Build first, with the real HOME, so Go's module/toolchain cache is reused.
echo "==> build"
(cd "$root" && go build -o "$work/bin/shield" ./cmd/shield)
export PATH="$work/bin:$PATH"

export HOME="$work/home"
export SOURCEGUARD_SHIELD_HOME="$work/home/.sourceguard-shield"
export GIT_CONFIG_NOSYSTEM=1
export GIT_AUTHOR_NAME=e2e GIT_AUTHOR_EMAIL=e2e@example.invalid
export GIT_COMMITTER_NAME=e2e GIT_COMMITTER_EMAIL=e2e@example.invalid
mkdir -p "$HOME"

echo "==> shield install"
shield install
test "$(git config --global core.hooksPath)" = "$SOURCEGUARD_SHIELD_HOME/hooks" \
  || { echo "FAIL: core.hooksPath not set to shield's hooks dir"; exit 1; }

repo="$work/repo"
git init -q "$repo"
cd "$repo"

echo "==> clean commit must pass"
echo "hello world" > README.md
git add README.md
git commit -q -m "clean commit"

echo "==> commit with a credential must be blocked"
# Built at runtime so no secret-shaped literal exists in this repository.
token="ghp_$(openssl rand -hex 18)"
printf 'GITHUB_TOKEN=%s\n' "$token" > config.env
git add config.env
if git commit -q -m "should be blocked" >"$work/out.txt" 2>&1; then
  echo "FAIL: commit with a credential was allowed"; cat "$work/out.txt"; exit 1
fi
if grep -q "$token" "$work/out.txt"; then
  echo "FAIL: hook output leaked the full credential"; exit 1
fi
test "$(git rev-list --count HEAD)" = "1" || { echo "FAIL: blocked commit was recorded"; exit 1; }

echo "==> deny-list guard: blocks in scope, leaves other repos alone, never prints the term"
fake="zzforbidden$(openssl rand -hex 3)"
# Explicit path: CI runners set XDG_CONFIG_HOME, which would otherwise win.
export SHIELD_DENYLIST_FILE="$HOME/.config/shield/denylist"
mkdir -p "$(dirname "$SHIELD_DENYLIST_FILE")"
cat > "$SHIELD_DENYLIST_FILE" <<DENY
path: $work/dl
$fake
DENY
chmod 600 "$SHIELD_DENYLIST_FILE"
status_out="$(shield denylist status)"
grep -q "patterns:  1" <<< "$status_out" || { echo "FAIL: denylist status"; echo "$status_out"; exit 1; }

git init -q -b main "$work/dl"
git init -q --bare "$work/dl-remote.git"
git -C "$work/dl" remote add origin "$work/dl-remote.git"
cd "$work/dl"
echo "clean" > a.txt
git add a.txt
git commit -q -m "clean commit"
git push -q origin main

echo "uses $fake here" > b.txt
git add b.txt
if git commit -q -m "adds a term" >"$work/dl.out" 2>&1; then
  echo "FAIL: staged term was allowed"; cat "$work/dl.out"; exit 1
fi
grep -q "b.txt:1" "$work/dl.out" || { echo "FAIL: location not reported"; cat "$work/dl.out"; exit 1; }
if grep -q "$fake" "$work/dl.out"; then echo "FAIL: output printed the term"; exit 1; fi
git reset -q

echo "ok" > c.txt
git add c.txt
if git commit -q -m "mentions $fake" >"$work/dl.out" 2>&1; then
  echo "FAIL: commit message with a term was allowed"; exit 1
fi
if grep -q "$fake" "$work/dl.out"; then echo "FAIL: output printed the term"; exit 1; fi

# --no-verify skips pre-commit and commit-msg; the pre-push hook still blocks.
echo "uses $fake" > d.txt
git add d.txt
git commit -q --no-verify -m "bypass"
if git push -q origin main >"$work/dl.out" 2>&1; then
  echo "FAIL: push carrying a term was allowed"; exit 1
fi
if grep -q "$fake" "$work/dl.out"; then echo "FAIL: output printed the term"; exit 1; fi
test "$(git --git-dir="$work/dl-remote.git" rev-list --count main)" = "1" || { echo "FAIL: blocked push reached the remote"; exit 1; }

# A repository outside the configured scope is untouched.
git init -q -b main "$work/other"
cd "$work/other"
echo "uses $fake" > e.txt
git add e.txt
git commit -q -m "out of scope"
cd "$repo"

echo "==> shield uninstall"
shield uninstall
if git config --global core.hooksPath >/dev/null 2>&1; then
  echo "FAIL: core.hooksPath still set after uninstall"; exit 1
fi

echo "OK: end-to-end checks passed"
