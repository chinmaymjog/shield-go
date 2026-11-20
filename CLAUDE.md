# shield-go

Local git pre-commit secret scanning (gitleaks + trufflehog), installed once per machine. Go 1.23.

## Layout
- `cmd/` — CLI entrypoints
- `internal/` — implementation packages (`denylist/` is the private-terms guard: it must never print matched text)
- `.goreleaser.yaml` — release config (do not change without being asked)

## Working on issues
- Verify before finishing: `go vet ./...` and `go test ./...` (same as CI in `.github/workflows/test.yml`).
- Add or update tests for every bug fix.
- Keep changes minimal and scoped to the issue; one issue per PR.
- Open a PR against `main` that references the issue (`Fixes #N`). Never push to `main` directly.
- Don't touch `.github/workflows/` or release/packaging config unless the issue asks for it.
