//go:build !windows

package security

import (
	"fmt"
	"os"
)

// RestrictPath applies the requested POSIX mode to a file or directory. The
// caller uses this for data-key, snapshot, and generated configuration paths
// whose permissions are part of the storage contract.
func RestrictPath(path string, mode os.FileMode) error {
	if err := os.Chmod(path, mode.Perm()); err != nil {
		return fmt.Errorf("chmod %s: %w", path, err)
	}
	return nil
}

// NormalizeFileMode preserves the mode bits on filesystems where they are
// meaningful.
func NormalizeFileMode(mode os.FileMode) os.FileMode {
	return mode.Perm()
}

// VerifyPrivatePath checks the native representation of the requested mode.
// Windows uses ACLs instead; see permissions_windows.go.
func VerifyPrivatePath(path string, mode os.FileMode) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if got := info.Mode().Perm(); got != mode.Perm() {
		return fmt.Errorf("permissions for %s = %o, want %o", path, got, mode.Perm())
	}
	return nil
}
