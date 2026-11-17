package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHooksPathAction(t *testing.T) {
	const ours = "/home/dev/.sourceguard-shield/hooks"

	cases := []struct {
		name      string
		current   string
		wantUnset bool
	}{
		{"matches ours", ours, true},
		{"already unset", "", false},
		{"foreign hooksPath", "/home/dev/project/.husky", false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			unset, msg := hooksPathAction(c.current, ours)
			if unset != c.wantUnset {
				t.Errorf("hooksPathAction(%q, %q) unset = %v, want %v", c.current, ours, unset, c.wantUnset)
			}
			if msg == "" {
				t.Errorf("hooksPathAction(%q, %q) returned empty message", c.current, ours)
			}
		})
	}
}

func TestRemoveShieldHomeKeepsUnrelatedFiles(t *testing.T) {
	home := t.TempDir()
	for _, d := range shieldChildren {
		if err := os.MkdirAll(filepath.Join(home, d), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(home, d, "x"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	keep := filepath.Join(home, "important.txt")
	keepDir := filepath.Join(home, "projects", "a")
	os.WriteFile(keep, []byte("data"), 0o644)
	os.MkdirAll(keepDir, 0o755)

	if err := removeShieldHome(home); err != nil {
		t.Fatal(err)
	}
	for _, d := range shieldChildren {
		if _, err := os.Stat(filepath.Join(home, d)); !os.IsNotExist(err) {
			t.Errorf("%s should be removed", d)
		}
	}
	for _, p := range []string{keep, keepDir} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("%s should survive: %v", p, err)
		}
	}
}

func TestRemoveShieldHomeRemovesEmptyDir(t *testing.T) {
	home := filepath.Join(t.TempDir(), "shield")
	os.MkdirAll(filepath.Join(home, "bin"), 0o755)
	if err := removeShieldHome(home); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(home); !os.IsNotExist(err) {
		t.Error("empty home should be removed")
	}
}

func TestCheckSafeHome(t *testing.T) {
	if err := checkSafeHome("/"); err == nil {
		t.Error("/ must be refused")
	}

	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)
	if err := checkSafeHome(fakeHome); err == nil {
		t.Error("user home must be refused")
	}

	wd := t.TempDir()
	orig, _ := os.Getwd()
	if err := os.Chdir(wd); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(orig) })
	if err := checkSafeHome("."); err == nil {
		t.Error("cwd must be refused")
	}
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(fakeHome, link); err == nil {
		if err := checkSafeHome(link); err == nil {
			t.Error("symlink to user home must be refused")
		}
	}

	if err := checkSafeHome(t.TempDir()); err != nil {
		t.Errorf("dedicated dir should be allowed: %v", err)
	}
}

func TestRunUninstallRefusesUserHome(t *testing.T) {
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)
	t.Setenv("SOURCEGUARD_SHIELD_HOME", fakeHome)
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(t.TempDir(), "gitconfig"))
	keep := filepath.Join(fakeHome, "keep.txt")
	os.WriteFile(keep, []byte("x"), 0o644)
	os.MkdirAll(filepath.Join(fakeHome, "bin"), 0o755)

	if code := runUninstall(nil); code == 0 {
		t.Error("expected non-zero exit")
	}
	if _, err := os.Stat(keep); err != nil {
		t.Error("unrelated file was deleted")
	}
	if _, err := os.Stat(filepath.Join(fakeHome, "bin")); err != nil {
		t.Error("bin should be untouched when refusing")
	}
}
