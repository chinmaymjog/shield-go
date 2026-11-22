//go:build unix

package engines

import (
	"fmt"
	"os"
	"syscall"
)

// ownedBy reports whether info's owner is uid.
func ownedBy(info os.FileInfo, uid int) bool {
	st, ok := info.Sys().(*syscall.Stat_t)
	return ok && int(st.Uid) == uid
}

// securePrivateDir makes sure dir exists, is a directory owned by the current
// user, and is not accessible to group/other. A directory we own with loose
// permissions is tightened to 0700; one owned by someone else is refused,
// because nothing in it can be trusted.
func securePrivateDir(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	info, err := os.Stat(dir)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", dir)
	}
	if !ownedBy(info, os.Getuid()) {
		return fmt.Errorf("%s is not owned by the current user — refusing to use it for engine binaries", dir)
	}
	if info.Mode().Perm() != 0o700 {
		if err := os.Chmod(dir, 0o700); err != nil {
			return fmt.Errorf("could not restrict %s to 0700: %w", dir, err)
		}
	}
	return nil
}

// trustedBinary reports whether the cached binary at path is a regular file
// owned by the current user that group/other cannot write to.
func trustedBinary(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%s is not a regular file", path)
	}
	if !ownedBy(info, os.Getuid()) {
		return fmt.Errorf("%s is not owned by the current user", path)
	}
	if info.Mode().Perm()&0o022 != 0 {
		return fmt.Errorf("%s is writable by group or others", path)
	}
	return nil
}
