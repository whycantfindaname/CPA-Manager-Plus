//go:build windows

package security

import (
	"os"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

func TestWindowsPrivatePathRestrictAndVerifyReadsNativeACL(t *testing.T) {
	path := t.TempDir() + `\private.data`
	if err := os.WriteFile(path, []byte("private"), 0o600); err != nil {
		t.Fatalf("write private file: %v", err)
	}
	if err := RestrictPath(path, 0o600); err != nil {
		t.Fatalf("restrict private file: %v", err)
	}
	if err := VerifyPrivatePath(path, 0o600); err != nil {
		t.Fatalf("verify restricted private file: %v", err)
	}

	token, err := windows.OpenCurrentProcessToken()
	if err != nil {
		t.Fatalf("open current process token: %v", err)
	}
	defer token.Close()
	user, err := token.GetTokenUser()
	if err != nil {
		t.Fatalf("read current process identity: %v", err)
	}
	descriptor, err := windows.GetNamedSecurityInfo(
		path,
		windows.SE_FILE_OBJECT,
		windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION,
	)
	if err != nil {
		t.Fatalf("read native security descriptor: %v", err)
	}
	dacl, _, err := descriptor.DACL()
	if err != nil {
		t.Fatalf("read native DACL: %v", err)
	}
	if dacl == nil {
		t.Fatal("restricted file has a null DACL")
	}
	if got, want := dacl.AceCount, uint16(1); got != want {
		t.Fatalf("native DACL ACE count = %d, want %d", got, want)
	}
	var ace *windows.ACCESS_ALLOWED_ACE
	if err := windows.GetAce(dacl, 0, &ace); err != nil {
		t.Fatalf("read native DACL ACE: %v", err)
	}
	if got, want := ace.Mask, privateFileFullControlMask; got != want {
		t.Fatalf("native private ACE mask = %#x, want %#x", got, want)
	}
	aceSID := (*windows.SID)(unsafe.Pointer(&ace.SidStart))
	if !aceSID.Equals(user.User.Sid) {
		t.Fatalf("native private ACE SID = %s, want current identity %s", aceSID, user.User.Sid)
	}
}

func TestWindowsPrivatePathRejectsEmptyDACL(t *testing.T) {
	path := t.TempDir() + `\empty-dacl.data`
	if err := os.WriteFile(path, []byte("private"), 0o600); err != nil {
		t.Fatalf("write private file: %v", err)
	}
	if err := RestrictPath(path, 0o600); err != nil {
		t.Fatalf("restrict private file: %v", err)
	}

	emptyDescriptor, err := windows.SecurityDescriptorFromString("D:")
	if err != nil {
		t.Fatalf("build empty DACL descriptor: %v", err)
	}
	emptyDACL, _, err := emptyDescriptor.DACL()
	if err != nil {
		t.Fatalf("read empty DACL descriptor: %v", err)
	}
	if err := windows.SetNamedSecurityInfo(
		path,
		windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION,
		nil,
		nil,
		emptyDACL,
		nil,
	); err != nil {
		t.Fatalf("set empty DACL: %v", err)
	}
	if err := VerifyPrivatePath(path, 0o600); err == nil {
		t.Fatal("empty DACL was accepted")
	}
}

func TestWindowsPrivatePathRejectsNullDACL(t *testing.T) {
	path := t.TempDir() + `\null-dacl.data`
	if err := os.WriteFile(path, []byte("private"), 0o600); err != nil {
		t.Fatalf("write private file: %v", err)
	}
	if err := RestrictPath(path, 0o600); err != nil {
		t.Fatalf("restrict private file: %v", err)
	}

	// Passing a nil ACL to SetNamedSecurityInfo asks Windows to install a
	// null DACL. VerifyPrivatePath must reject that permissive descriptor.
	if err := windows.SetNamedSecurityInfo(
		path,
		windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION,
		nil,
		nil,
		nil,
		nil,
	); err != nil {
		t.Fatalf("set null DACL: %v", err)
	}
	if err := VerifyPrivatePath(path, 0o600); err == nil {
		t.Fatal("null DACL was accepted")
	}
}

func TestWindowsPrivatePathRejectsInsufficientPermissions(t *testing.T) {
	path := t.TempDir() + `\insufficient-permissions.data`
	if err := os.WriteFile(path, []byte("private"), 0o600); err != nil {
		t.Fatalf("write private file: %v", err)
	}
	if err := RestrictPath(path, 0o600); err != nil {
		t.Fatalf("restrict private file: %v", err)
	}

	token, err := windows.OpenCurrentProcessToken()
	if err != nil {
		t.Fatalf("open current process token: %v", err)
	}
	defer token.Close()
	user, err := token.GetTokenUser()
	if err != nil {
		t.Fatalf("read current process identity: %v", err)
	}
	acl, err := windows.ACLFromEntries([]windows.EXPLICIT_ACCESS{
		{
			AccessPermissions: windows.GENERIC_READ,
			AccessMode:        windows.GRANT_ACCESS,
			Inheritance:       windows.NO_INHERITANCE,
			Trustee: windows.TRUSTEE{
				TrusteeForm:  windows.TRUSTEE_IS_SID,
				TrusteeType:  windows.TRUSTEE_IS_USER,
				TrusteeValue: windows.TrusteeValueFromSID(user.User.Sid),
			},
		},
	}, nil)
	if err != nil {
		t.Fatalf("build insufficient-permissions DACL: %v", err)
	}
	if err := windows.SetNamedSecurityInfo(
		path,
		windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION,
		nil,
		nil,
		acl,
		nil,
	); err != nil {
		t.Fatalf("set insufficient-permissions DACL: %v", err)
	}
	if err := VerifyPrivatePath(path, 0o600); err == nil {
		t.Fatal("DACL with insufficient permissions was accepted")
	}
}

func TestWindowsPrivatePathRejectsExtraACE(t *testing.T) {
	path := t.TempDir() + `\extra-ace.data`
	if err := os.WriteFile(path, []byte("private"), 0o600); err != nil {
		t.Fatalf("write private file: %v", err)
	}
	if err := RestrictPath(path, 0o600); err != nil {
		t.Fatalf("restrict private file: %v", err)
	}

	token, err := windows.OpenCurrentProcessToken()
	if err != nil {
		t.Fatalf("open current process token: %v", err)
	}
	defer token.Close()
	user, err := token.GetTokenUser()
	if err != nil {
		t.Fatalf("read current process identity: %v", err)
	}
	world, err := windows.CreateWellKnownSid(windows.WinWorldSid)
	if err != nil {
		t.Fatalf("create world SID: %v", err)
	}
	acl, err := windows.ACLFromEntries([]windows.EXPLICIT_ACCESS{
		{
			AccessPermissions: windows.GENERIC_ALL,
			AccessMode:        windows.GRANT_ACCESS,
			Inheritance:       windows.NO_INHERITANCE,
			Trustee: windows.TRUSTEE{
				TrusteeForm:  windows.TRUSTEE_IS_SID,
				TrusteeType:  windows.TRUSTEE_IS_USER,
				TrusteeValue: windows.TrusteeValueFromSID(user.User.Sid),
			},
		},
		{
			AccessPermissions: windows.GENERIC_READ,
			AccessMode:        windows.GRANT_ACCESS,
			Inheritance:       windows.NO_INHERITANCE,
			Trustee: windows.TRUSTEE{
				TrusteeForm:  windows.TRUSTEE_IS_SID,
				TrusteeType:  windows.TRUSTEE_IS_WELL_KNOWN_GROUP,
				TrusteeValue: windows.TrusteeValueFromSID(world),
			},
		},
	}, nil)
	if err != nil {
		t.Fatalf("build extra-ACE DACL: %v", err)
	}
	if err := windows.SetNamedSecurityInfo(
		path,
		windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION,
		nil,
		nil,
		acl,
		nil,
	); err != nil {
		t.Fatalf("set extra-ACE DACL: %v", err)
	}
	if err := VerifyPrivatePath(path, 0o600); err == nil {
		t.Fatal("DACL with an extra ACE was accepted")
	}
}
