package engines

import (
	"os"
	"path/filepath"
)

// Home returns shield's state directory (cached engine binaries, the
// installed hook script), honoring the SOURCEGUARD_SHIELD_HOME override.
func Home() (string, error) {
	if h := os.Getenv("SOURCEGUARD_SHIELD_HOME"); h != "" {
		return h, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".sourceguard-shield"), nil
}

// BinDir returns the directory cached engine binaries live in, creating it
// if necessary. shield executes whatever is cached here on every commit, so
// both the state directory and this one must be owned by the current user and
// closed to group/other: loose permissions on a directory we own are
// tightened to 0700, and a directory owned by someone else is refused.
func BinDir() (string, error) {
	home, err := Home()
	if err != nil {
		return "", err
	}
	if err := securePrivateDir(home); err != nil {
		return "", err
	}
	dir := filepath.Join(home, "bin")
	if err := securePrivateDir(dir); err != nil {
		return "", err
	}
	return dir, nil
}
