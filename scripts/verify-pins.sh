#!/bin/sh
# Verify the engine pins in internal/engines/spec.go against upstream:
#   - the pinned checksums.txt digest matches what upstream serves, and
#   - for engines that sign releases, the signature checks out under cosign
#     against the upstream release workflow's identity.
# Needs network access and, for signed engines (trufflehog), cosign on PATH.
# Run from anywhere; CI runs it when spec.go changes and weekly.
set -eu
cd "$(dirname "$0")/.."
exec go test -tags verifypins -run '^TestVerifyPins$' -count=1 -v ./internal/engines
