package scan

import (
	"bytes"
	"os"
	"testing"
)

func TestWriteConfig(t *testing.T) {
	path, cleanup, err := writeConfig()
	if err != nil {
		t.Fatalf("writeConfig() error = %v", err)
	}
	defer cleanup()

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, gitleaksConfig) {
		t.Errorf("writeConfig() wrote %d bytes, want the embedded gitleaks.toml (%d bytes) unchanged", len(got), len(gitleaksConfig))
	}

	cleanup()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("writeConfig() cleanup did not remove %s", path)
	}
}
