package engines

import (
	"os"
	"path/filepath"
)

// Home returns the shield home directory, honoring the same
// SOURCEGUARD_SHIELD_HOME override the bash implementation uses.
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
// if necessary.
func BinDir() (string, error) {
	home, err := Home()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, "bin")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}
