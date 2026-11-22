//go:build !unix

package engines

import "os"

// shield only ships for macOS and Linux; these keep the package compiling
// elsewhere without pretending to enforce Unix ownership rules.

func ownedBy(os.FileInfo, int) bool { return true }

func securePrivateDir(dir string) error { return os.MkdirAll(dir, 0o700) }

func trustedBinary(path string) error {
	_, err := os.Lstat(path)
	return err
}
