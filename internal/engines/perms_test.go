//go:build unix

package engines

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestBinDir_CreatesPrivateDirs(t *testing.T) {
	home := filepath.Join(t.TempDir(), "shield-home")
	t.Setenv("SOURCEGUARD_SHIELD_HOME", home)

	bin, err := BinDir()
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range []string{home, bin} {
		info, err := os.Stat(d)
		if err != nil {
			t.Fatal(err)
		}
		if got := info.Mode().Perm(); got != 0o700 {
			t.Errorf("%s mode = %v, want 0700", d, got)
		}
	}
}

func TestBinDir_TightensLooseDirsWeOwn(t *testing.T) {
	home := t.TempDir()
	t.Setenv("SOURCEGUARD_SHIELD_HOME", home)
	loose := filepath.Join(home, "bin")
	if err := os.Mkdir(loose, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(home, 0o755); err != nil {
		t.Fatal(err)
	}

	if _, err := BinDir(); err != nil {
		t.Fatal(err)
	}
	for _, d := range []string{home, loose} {
		info, _ := os.Stat(d)
		if got := info.Mode().Perm(); got != 0o700 {
			t.Errorf("%s mode = %v, want 0700", d, got)
		}
	}
}

func TestOwnedBy_RejectsOtherUsers(t *testing.T) {
	info, err := os.Stat(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if !ownedBy(info, os.Getuid()) {
		t.Error("expected the temp dir to be owned by the current user")
	}
	if ownedBy(info, os.Getuid()+1) {
		t.Error("expected a different uid to be rejected")
	}
}

func TestTrustedBinary(t *testing.T) {
	dir := t.TempDir()
	good := filepath.Join(dir, "good")
	if err := os.WriteFile(good, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := trustedBinary(good); err != nil {
		t.Errorf("0755 owned file should be trusted: %v", err)
	}

	writable := filepath.Join(dir, "writable")
	if err := os.WriteFile(writable, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(writable, 0o775); err != nil { // bypass umask
		t.Fatal(err)
	}
	if trustedBinary(writable) == nil {
		t.Error("group-writable file must not be trusted")
	}

	link := filepath.Join(dir, "link")
	if err := os.Symlink(good, link); err != nil {
		t.Fatal(err)
	}
	if trustedBinary(link) == nil {
		t.Error("symlink must not be trusted")
	}
}

// serveFakeReleaseFor is like serveFakeRelease but serves the asset for the
// platform the test is running on, so Ensure (which derives the platform from
// the runtime) can be exercised end to end.
func serveFakeReleaseFor(t *testing.T, assetOS, assetArch string) Spec {
	t.Helper()
	const name, version = "fakescan", "1.0.0"
	asset := fmt.Sprintf("%s_%s_%s_%s.tar.gz", name, version, assetOS, assetArch)
	tarGz := buildTarGz(t, name, []byte(fakeContent))
	checksums := []byte(fmt.Sprintf("%s  %s\n", sha256Hex(tarGz), asset))

	prefix := "/acme/" + name + "/releases/download/v" + version + "/"
	mux := http.NewServeMux()
	mux.HandleFunc(prefix+asset, func(w http.ResponseWriter, _ *http.Request) { w.Write(tarGz) })
	mux.HandleFunc(prefix+fmt.Sprintf("%s_%s_checksums.txt", name, version),
		func(w http.ResponseWriter, _ *http.Request) { w.Write(checksums) })
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	original := githubReleaseBase
	githubReleaseBase = srv.URL
	t.Cleanup(func() { githubReleaseBase = original })

	return Spec{Repo: "acme/" + name, Name: name, Version: version, ChecksumsSHA256: sha256Hex(checksums)}
}

// A cached binary that group/other can write to is discarded and re-fetched
// through the verified download rather than executed.
func TestEnsure_ReplacesUntrustedCachedBinary(t *testing.T) {
	assetArch, err := arch("fakescan")
	if err != nil {
		t.Skip("unsupported platform:", err)
	}
	spec := serveFakeReleaseFor(t, runtime.GOOS, assetArch)

	t.Setenv("SOURCEGUARD_SHIELD_HOME", t.TempDir())
	binDir, err := BinDir()
	if err != nil {
		t.Fatal(err)
	}
	cached := filepath.Join(binDir, fmt.Sprintf("%s-%s-%s-%s", spec.Name, spec.Version, runtime.GOOS, assetArch))
	if err := os.WriteFile(cached, []byte("tampered"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(cached, 0o777); err != nil {
		t.Fatal(err)
	}

	got, err := Ensure(spec)
	if err != nil {
		t.Fatalf("Ensure: %v", err)
	}
	data, err := os.ReadFile(got)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != fakeContent {
		t.Fatalf("tampered cached binary was reused: %q", data)
	}
}
