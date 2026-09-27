//go:build windows

package security

import (
	"fmt"
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

// SetEntriesInAcl maps GENERIC_ALL to the concrete full-control mask for a
// file object before it is stored in the ACE. x/sys/windows exposes the
// generic bit but not FILE_ALL_ACCESS; this is FILE_ALL_ACCESS (0x1f01ff).
const privateFileFullControlMask windows.ACCESS_MASK = windows.STANDARD_RIGHTS_REQUIRED | windows.SYNCHRONIZE | 0x1ff

// RestrictPath gives the current service identity the only explicit access
// entry and protects the DACL from inherited entries. Windows does not expose
// POSIX mode bits through os.FileInfo, so an owner-only ACL is the equivalent
// of the private 0600/0700 storage contract used by the Unix implementation.
func RestrictPath(path string, mode os.FileMode) error {
	token, err := windows.OpenCurrentProcessToken()
	if err != nil {
		return fmt.Errorf("open current process token: %w", err)
	}
	defer token.Close()
	user, err := token.GetTokenUser()
	if err != nil {
		return fmt.Errorf("read current process identity: %w", err)
	}
	acl, err := windows.ACLFromEntries([]windows.EXPLICIT_ACCESS{{
		AccessPermissions: windows.GENERIC_ALL,
		AccessMode:        windows.SET_ACCESS,
		Inheritance:       windows.NO_INHERITANCE,
		Trustee: windows.TRUSTEE{
			TrusteeForm:  windows.TRUSTEE_IS_SID,
			TrusteeType:  windows.TRUSTEE_IS_USER,
			TrusteeValue: windows.TrusteeValueFromSID(user.User.Sid),
		},
	}}, nil)
	if err != nil {
		return fmt.Errorf("build private ACL for %s: %w", path, err)
	}
	if err := windows.SetNamedSecurityInfo(
		path,
		windows.SE_FILE_OBJECT,
		windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION,
		user.User.Sid,
		nil,
		acl,
		nil,
	); err != nil {
		return fmt.Errorf("set private ACL for %s: %w", path, err)
	}
	return nil
}

// NormalizeFileMode records the private-file equivalent in manifests. NTFS
// stores access through ACLs rather than POSIX mode bits, so 0600 is the
// stable representation used when a snapshot is moved between hosts.
func NormalizeFileMode(mode os.FileMode) os.FileMode {
	return 0o600
}

// VerifyPrivatePath proves that the current identity owns the path and that
// its protected DACL contains exactly one full-control ACE for that identity.
// The mode argument is intentionally accepted for parity with Unix callers;
// Windows ACLs carry the effective permission semantics.
func VerifyPrivatePath(path string, mode os.FileMode) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("private path %s is a symlink", path)
	}
	token, err := windows.OpenCurrentProcessToken()
	if err != nil {
		return fmt.Errorf("open current process token: %w", err)
	}
	defer token.Close()
	user, err := token.GetTokenUser()
	if err != nil {
		return fmt.Errorf("read current process identity: %w", err)
	}
	descriptor, err := windows.GetNamedSecurityInfo(
		path,
		windows.SE_FILE_OBJECT,
		windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION,
	)
	if err != nil {
		return fmt.Errorf("read private ACL for %s: %w", path, err)
	}
	owner, _, err := descriptor.Owner()
	if err != nil {
		return fmt.Errorf("read private ACL owner for %s: %w", path, err)
	}
	if owner == nil || !owner.Equals(user.User.Sid) {
		return fmt.Errorf("private ACL owner for %s is not the current identity", path)
	}
	control, _, err := descriptor.Control()
	if err != nil {
		return fmt.Errorf("read private ACL control for %s: %w", path, err)
	}
	if control&windows.SE_DACL_PROTECTED == 0 {
		return fmt.Errorf("private ACL for %s is inheritable", path)
	}
	dacl, _, err := descriptor.DACL()
	if err != nil {
		return fmt.Errorf("read private ACL entries for %s: %w", path, err)
	}
	if dacl == nil {
		return fmt.Errorf("private ACL for %s has no DACL, want 1 entry", path)
	}
	if dacl.AceCount != 1 {
		return fmt.Errorf("private ACL for %s has %d entries, want 1", path, dacl.AceCount)
	}
	var ace *windows.ACCESS_ALLOWED_ACE
	if err := windows.GetAce(dacl, 0, &ace); err != nil {
		return fmt.Errorf("read private ACL entry for %s: %w", path, err)
	}
	if ace.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE || ace.Mask != privateFileFullControlMask {
		return fmt.Errorf("private ACL for %s is not owner-only full control", path)
	}
	aceSID := (*windows.SID)(unsafe.Pointer(&ace.SidStart))
	if !aceSID.Equals(user.User.Sid) {
		return fmt.Errorf("private ACL for %s grants a different identity", path)
	}
	return nil
}
