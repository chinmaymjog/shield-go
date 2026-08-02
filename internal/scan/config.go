package scan

import (
	_ "embed"
	"os"
)

//go:embed gitleaks.toml
var gitleaksConfig []byte

// writeConfig writes the embedded gitleaks ruleset to a temp file and
// returns its path plus a cleanup func. The ruleset ships inside the shield
// binary — like the engine version pins, changing it means shipping a new
// shield release, not editing a file out of band.
func writeConfig() (string, func(), error) {
	f, err := os.CreateTemp("", "sg-shield-gitleaks-*.toml")
	if err != nil {
		return "", nil, err
	}
	cleanup := func() { os.Remove(f.Name()) }

	if _, err := f.Write(gitleaksConfig); err != nil {
		f.Close()
		cleanup()
		return "", nil, err
	}
	if err := f.Close(); err != nil {
		cleanup()
		return "", nil, err
	}
	return f.Name(), cleanup, nil
}
