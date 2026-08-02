package engines

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func buildTarGz(t *testing.T, name string, content []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(content))}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// fakeContent is the payload served as the "binary" inside the fake release.
const fakeContent = "#!/bin/sh\necho fake\n"

// serveFakeRelease spins up an httptest server serving a fake gitleaks-shaped
// release (asset + checksums.txt), points githubReleaseBase at it for the
// duration of the test, and returns the Spec to use.
//
// checksumsPinOverride/assetDigestOverride, when non-empty, replace the
// otherwise-correct digest — used to simulate a tampered pin or a tampered
// per-asset checksums.txt entry.
func serveFakeRelease(t *testing.T, checksumsPinOverride, assetDigestOverride string) Spec {
	t.Helper()
	const name = "fakescan"
	const version = "1.0.0"
	const assetOS, assetArch = "linux", "amd64"
	asset := fmt.Sprintf("%s_%s_%s_%s.tar.gz", name, version, assetOS, assetArch)
	checksumsAsset := fmt.Sprintf("%s_%s_checksums.txt", name, version)

	tarGz := buildTarGz(t, name, []byte(fakeContent))
	assetDigest := sha256Hex(tarGz)
	if assetDigestOverride != "" {
		assetDigest = assetDigestOverride
	}
	checksums := []byte(fmt.Sprintf("%s  %s\n", assetDigest, asset))
	checksumsDigest := sha256Hex(checksums)
	if checksumsPinOverride != "" {
		checksumsDigest = checksumsPinOverride
	}

	prefix := "/acme/" + name + "/releases/download/v" + version + "/"
	mux := http.NewServeMux()
	mux.HandleFunc(prefix+asset, func(w http.ResponseWriter, _ *http.Request) { w.Write(tarGz) })
	mux.HandleFunc(prefix+checksumsAsset, func(w http.ResponseWriter, _ *http.Request) { w.Write(checksums) })
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	original := githubReleaseBase
	githubReleaseBase = srv.URL
	t.Cleanup(func() { githubReleaseBase = original })

	return Spec{
		Repo:            "acme/" + name,
		Name:            name,
		Version:         version,
		ChecksumsSHA256: checksumsDigest,
	}
}

func assertNotInstalled(t *testing.T, dest string) {
	t.Helper()
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Fatalf("expected no binary installed after a failed verification, stat err = %v", err)
	}
}

func TestVerifiedDownload_HappyPath(t *testing.T) {
	spec := serveFakeRelease(t, "", "")
	dest := filepath.Join(t.TempDir(), "out-binary")

	if err := verifiedDownload(spec, "linux", "amd64", dest); err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}

	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != fakeContent {
		t.Fatalf("unexpected installed content: %q", got)
	}
	info, err := os.Stat(dest)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o111 == 0 {
		t.Fatalf("expected installed binary to be executable, got mode %v", info.Mode())
	}
}

func TestVerifiedDownload_TamperedChecksumsPin_FailsClosed(t *testing.T) {
	spec := serveFakeRelease(t, strings.Repeat("0", 64), "")
	dest := filepath.Join(t.TempDir(), "out-binary")

	err := verifiedDownload(spec, "linux", "amd64", dest)
	if err == nil {
		t.Fatal("expected error for a checksums.txt that doesn't match the pinned digest, got nil")
	}
	assertNotInstalled(t, dest)
}

func TestVerifiedDownload_TamperedAssetDigest_FailsClosed(t *testing.T) {
	spec := serveFakeRelease(t, "", strings.Repeat("0", 64))
	dest := filepath.Join(t.TempDir(), "out-binary")

	err := verifiedDownload(spec, "linux", "amd64", dest)
	if err == nil {
		t.Fatal("expected error for an asset that doesn't match its checksums.txt entry, got nil")
	}
	assertNotInstalled(t, dest)
}

func TestVerifiedDownload_MissingPin_FailsClosedWithoutNetworkCall(t *testing.T) {
	spec := serveFakeRelease(t, "", "")
	spec.ChecksumsSHA256 = ""
	// point at an unroutable address: a passing test must not depend on this
	// ever being dialed.
	githubReleaseBase = "http://127.0.0.1:0"
	dest := filepath.Join(t.TempDir(), "out-binary")

	if err := verifiedDownload(spec, "linux", "amd64", dest); err == nil {
		t.Fatal("expected error for a missing pinned checksum, got nil")
	}
	assertNotInstalled(t, dest)
}
