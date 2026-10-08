// Package engines manages the gitleaks and trufflehog binaries shield shells
// out to: which versions are trusted, how they're downloaded, and how their
// checksums are verified before anything is executed.
package engines

// Spec pins one scan engine's exact trusted release: the version this build
// of shield trusts, and the sha256 of that release's checksums.txt — the
// trust anchor that authenticates every per-asset digest inside
// checksums.txt (see verifiedDownload). Bumping a version means bumping this
// file and cutting a new shield release; there's no separate versions.yaml
// to edit out of band, because the shield binary's own version *is* the pin.
type Spec struct {
	Repo            string // GitHub "owner/repo"
	Name            string // tool name; used in asset/checksums filenames and as the extracted binary's name
	Version         string
	ChecksumsSHA256 string
}

var Gitleaks = Spec{
	Repo:            "gitleaks/gitleaks",
	Name:            "gitleaks",
	Version:         "8.30.1",
	ChecksumsSHA256: "061476c21adaf5441516f96f185c1a4706a83cd6329b9b38762271b3d4a52fae",
}

var Trufflehog = Spec{
	Repo:            "trufflesecurity/trufflehog",
	Name:            "trufflehog",
	Version:         "3.99.0",
	ChecksumsSHA256: "77cebeaaf3613b95ac10546e2a3eb8f81a50b5e682d44ba435452c60570e161f",
}
