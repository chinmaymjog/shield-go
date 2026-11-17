# Contributing

Thanks for helping improve shield-go.

## Basics
- Open an issue before large changes.
- Run `go vet ./...` and `go test ./...` before submitting; CI runs the same.
- Add or update tests for bug fixes.
- One logical change per pull request, referencing the issue (`Fixes #N`).

## AI-assisted contributions
AI tools are welcome, with these rules:
- **You are accountable** for every line you submit. Read and understand the code; unreviewed AI output will be closed.
- **Disclose** substantial AI assistance in the PR description.
- Don't paste secrets, tokens, or private data into AI tools or issues.
- Low-effort, bulk, or off-topic AI-generated PRs and issues may be closed without review.

## Maintainer automation
The `@claude` GitHub workflow is restricted to the repository owner. Comments
from anyone else never trigger it. Agent-authored PRs get the same review and
CI requirements as any other PR and are never auto-merged.

## Security issues
See [SECURITY.md](SECURITY.md); do not file them as public issues.
