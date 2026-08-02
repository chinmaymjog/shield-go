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
	Version:         "8.30.0",
	ChecksumsSHA256: "78e53de2429bde6500a6f22793546babe6ae75634a0c250c37e3a07703856a90",
}

var Trufflehog = Spec{
	Repo:            "trufflesecurity/trufflehog",
	Name:            "trufflehog",
	Version:         "3.82.13",
	ChecksumsSHA256: "8f57a662a64d82316d1e784a6d199ef8a03fd92aba2b0e809f2b8d578985e49b",
}
