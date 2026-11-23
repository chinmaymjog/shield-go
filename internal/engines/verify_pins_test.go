//go:build verifypins

// This test talks to the real GitHub release endpoints and shells out to
// cosign, so it is excluded from the normal `go test ./...`. Run it with
// scripts/verify-pins.sh; CI runs it when spec.go changes and weekly.
//
// It answers: is the checksums digest pinned in spec.go really what upstream
// published, and for engines that sign their releases, was it signed by the
// upstream release workflow? Doing this when a pin is bumped (rather than on
// each user's machine) avoids a runtime dependency on Sigstore's transparency
// log while still checking the thing users actually trust — the pin.
package engines

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestVerifyPins(t *testing.T) {
	for _, spec := range []Spec{Gitleaks, Trufflehog} {
		t.Run(spec.Name, func(t *testing.T) { verifyPin(t, spec) })
	}
}

func verifyPin(t *testing.T, spec Spec) {
	dir := t.TempDir()
	base := fmt.Sprintf("%s/%s/releases/download/v%s", githubReleaseBase, spec.Repo, spec.Version)
	checksumsName := fmt.Sprintf("%s_%s_checksums.txt", spec.Name, spec.Version)
	checksums := filepath.Join(dir, checksumsName)

	if err := download(base+"/"+checksumsName, checksums, 30*time.Second); err != nil {
		t.Fatalf("download %s: %v", checksumsName, err)
	}

	// 1. The pin must equal what upstream serves. Releases are not immutable,
	// so this also catches an asset replaced under an existing tag.
	got, err := sha256File(checksums)
	if err != nil {
		t.Fatal(err)
	}
	if got != spec.ChecksumsSHA256 {
		t.Fatalf("%s %s: pinned checksums digest %s does not match upstream's %s",
			spec.Name, spec.Version, spec.ChecksumsSHA256, got)
	}

	if spec.Signing == nil {
		// 2a. Unsigned upstream. Make sure that is still true: if a newer
		// release starts publishing a signature, the pin should be verified.
		for _, suffix := range []string{".sig", ".pem", ".sigstore.json", ".sigstore", ".asc"} {
			probe := filepath.Join(dir, "probe"+suffix)
			if err := download(base+"/"+checksumsName+suffix, probe, 30*time.Second); err == nil {
				t.Fatalf("%s now publishes %s%s: set Spec.Signing so the pin is signature-verified",
					spec.Name, checksumsName, suffix)
			}
		}
		t.Logf("NOTICE: %s publishes no release signature; its pin is trust-on-first-use", spec.Name)
		fmt.Printf("::notice title=%s release is unsigned::The pin for %s %s is trust-on-first-use; upstream publishes no signature.\n",
			spec.Name, spec.Name, spec.Version)
		return
	}

	// 2b. Signed upstream: verify the keyless signature against the release
	// workflow's identity.
	if _, err := exec.LookPath("cosign"); err != nil {
		t.Fatal("cosign not found on PATH; it is required to verify signed engines")
	}
	sig := filepath.Join(dir, checksumsName+".sig")
	cert := filepath.Join(dir, checksumsName+".pem")
	for path, name := range map[string]string{sig: checksumsName + ".sig", cert: checksumsName + ".pem"} {
		if err := download(base+"/"+name, path, 30*time.Second); err != nil {
			t.Fatalf("download %s: %v", name, err)
		}
	}
	cmd := exec.Command("cosign", "verify-blob",
		"--certificate", cert,
		"--signature", sig,
		"--certificate-identity", spec.Identity(),
		"--certificate-oidc-issuer", spec.Signing.OIDCIssuer,
		checksums)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("cosign verify-blob failed for %s %s (identity %s): %v",
			spec.Name, spec.Version, spec.Identity(), err)
	}
}
