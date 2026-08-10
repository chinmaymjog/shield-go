// Package version holds build-time metadata.
//
// These vars are overridden at build time via -ldflags, e.g.:
//
//	go build -ldflags "-X github.com/chinmaymjog/shield-go/internal/version.Version=1.2.3" ./cmd/shield
//
// goreleaser will do this automatically once the release pipeline (a later
// module) is wired up. Until then, local builds report "dev".
package version

var (
	// Version is the shield release version (e.g. "1.2.3"), or "dev" for
	// unreleased local builds.
	Version = "dev"
	// Commit is the git SHA the binary was built from.
	Commit = "none"
	// Date is the build timestamp (RFC3339), set by the release pipeline.
	Date = "unknown"
)

// String returns a one-line, human-readable version summary.
func String() string {
	return "shield " + Version + " (" + Commit + ", built " + Date + ")"
}
