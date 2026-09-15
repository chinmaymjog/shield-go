package scan

import (
	"io"
	"os"
	"strings"
	"testing"
)

func TestNoisyTrufflehogLine(t *testing.T) {
	noisy := []string{
		"🐷🐷🐷  TruffleHog. Unearth your secrets.  🐷🐷🐷",
		`{"level":"info-0","ts":"2026-01-01T00:00:00Z","logger":"trufflehog","msg":"running source"}`,
		`{"level":"info-0","ts":"2026-01-01T00:00:01Z","logger":"trufflehog","msg":"finished scanning"}`,
	}
	for _, line := range noisy {
		if !noisyTrufflehogLine.MatchString(line) {
			t.Errorf("noisyTrufflehogLine did not match expected noise: %q", line)
		}
	}

	real := []string{
		"a genuine finding or error line",
		`{"level":"error","msg":"unexpected status 403"}`,
	}
	for _, line := range real {
		if noisyTrufflehogLine.MatchString(line) {
			t.Errorf("noisyTrufflehogLine incorrectly matched a non-noise line: %q", line)
		}
	}
}

func TestFilterTrufflehogNoise(t *testing.T) {
	input := []byte(strings.Join([]string{
		"🐷🐷🐷  TruffleHog. Unearth your secrets.  🐷🐷🐷",
		`{"msg":"running source"}`,
		`{"msg":"finished scanning"}`,
		"",
		"a genuine finding or error line that must survive",
	}, "\n") + "\n")

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	orig := os.Stderr
	os.Stderr = w
	t.Cleanup(func() { os.Stderr = orig })

	filterTrufflehogNoise(input)

	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}

	want := "a genuine finding or error line that must survive\n"
	if string(out) != want {
		t.Errorf("filterTrufflehogNoise() wrote %q, want %q", out, want)
	}
}
