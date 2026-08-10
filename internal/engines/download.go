package engines

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// githubReleaseBase is a var, not a const, so tests can point it at an
// httptest server instead of hitting real GitHub.
var githubReleaseBase = "https://github.com"

// verifiedDownload fetches an engine release asset and its checksums.txt,
// authenticates checksums.txt against the pinned digest in spec (the trust
// anchor — see Spec's doc comment), then authenticates the asset against
// checksums.txt, and only then extracts binName into destPath.
//
// This fails closed: a missing pin, a failed fetch, or any digest mismatch
// returns an error and installs nothing. That matters more than it might
// look: a naive implementation that fell back to "trust it" whenever
// checksums.txt couldn't be fetched would let an attacker who can block or
// tamper with just that one request install anything they want.
func verifiedDownload(spec Spec, assetOS, assetArch, destPath string) error {
	if spec.ChecksumsSHA256 == "" {
		return fmt.Errorf("%s: no pinned checksums digest — refusing to install unverified", spec.Name)
	}

	tmp, err := os.MkdirTemp("", "shield-download-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	asset := fmt.Sprintf("%s_%s_%s_%s.tar.gz", spec.Name, spec.Version, assetOS, assetArch)
	checksumsAsset := fmt.Sprintf("%s_%s_checksums.txt", spec.Name, spec.Version)
	base := fmt.Sprintf("%s/%s/releases/download/v%s", githubReleaseBase, spec.Repo, spec.Version)

	// A stalled connection must fail fast and loudly, not hang `git commit`
	// with no feedback — matches curl's --connect-timeout/--max-time split.
	assetPath := filepath.Join(tmp, asset)
	if err := download(base+"/"+asset, assetPath, 300*time.Second); err != nil {
		return fmt.Errorf("failed to download %s: %w", asset, err)
	}

	checksumsPath := filepath.Join(tmp, "checksums.txt")
	if err := download(base+"/"+checksumsAsset, checksumsPath, 30*time.Second); err != nil {
		return fmt.Errorf("failed to download %s checksums.txt: %w", spec.Name, err)
	}

	checksumsDigest, err := sha256File(checksumsPath)
	if err != nil {
		return err
	}
	if checksumsDigest != spec.ChecksumsSHA256 {
		return fmt.Errorf("%s checksums.txt digest %s does not match pinned %s — refusing to trust an unrecognised checksums file",
			spec.Name, checksumsDigest, spec.ChecksumsSHA256)
	}

	expected, err := expectedDigest(checksumsPath, asset)
	if err != nil {
		return err
	}
	actual, err := sha256File(assetPath)
	if err != nil {
		return err
	}
	if expected == "" || expected != actual {
		return fmt.Errorf("checksum mismatch for %s: expected %s, got %s — refusing to install a binary that doesn't match its published checksum",
			asset, orNotListed(expected), actual)
	}

	extracted := filepath.Join(tmp, spec.Name)
	if err := extractFile(assetPath, spec.Name, extracted); err != nil {
		return err
	}
	if err := os.Chmod(extracted, 0o755); err != nil {
		return err
	}
	return installBinary(extracted, destPath)
}

func orNotListed(s string) string {
	if s == "" {
		return "<not listed>"
	}
	return s
}

// download fetches url to destPath, failing if the connection can't be
// established within 10s or the whole transfer doesn't finish within
// overallTimeout.
func download(url, destPath string, overallTimeout time.Duration) error {
	client := &http.Client{
		Timeout: overallTimeout,
		Transport: &http.Transport{
			DialContext: (&net.Dialer{Timeout: 10 * time.Second}).DialContext,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), overallTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %s", resp.Status)
	}

	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, resp.Body)
	return err
}

func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// expectedDigest parses a checksums.txt line ("<sha256>  <filename>") for
// asset's digest.
func expectedDigest(checksumsPath, asset string) (string, error) {
	data, err := os.ReadFile(checksumsPath)
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[1] == asset {
			return fields[0], nil
		}
	}
	return "", nil
}

// extractFile pulls the single file named "name" out of a .tar.gz archive.
func extractFile(archivePath, name, destPath string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return fmt.Errorf("%s not found in %s", name, filepath.Base(archivePath))
		}
		if err != nil {
			return err
		}
		if hdr.Name != name {
			continue
		}
		out, err := os.Create(destPath)
		if err != nil {
			return err
		}
		defer out.Close()
		// archivePath was already verified against a pinned checksum above,
		// so this isn't decompressing attacker-controlled input.
		_, err = io.Copy(out, tr)
		return err
	}
}

// installBinary moves src to dest, falling back to copy+remove if they're on
// different filesystems (unlike `mv`, os.Rename doesn't fall back on its own).
func installBinary(src, dest string) error {
	if err := os.Rename(src, dest); err == nil {
		return nil
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return os.Remove(src)
}
