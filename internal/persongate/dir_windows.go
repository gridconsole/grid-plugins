//go:build windows

package persongate

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"unsafe"

	"github.com/gridconsole/grid-plugins/internal/fslock"
)

func private(i os.FileInfo) error {
	if i.Mode()&os.ModeSymlink != 0 {
		return errors.New("person entry is a link")
	}
	return nil // Windows ownership is checked on the opened handle below.
}

var getSecurityInfo = syscall.NewLazyDLL("advapi32.dll").NewProc("GetSecurityInfo")
var localFree = syscall.NewLazyDLL("kernel32.dll").NewProc("LocalFree")

func owner(f *os.File) error {
	var sid *syscall.SID
	var acl uintptr
	var descriptor uintptr
	code, _, _ := getSecurityInfo.Call(f.Fd(), 1, 5, uintptr(unsafe.Pointer(&sid)), 0, uintptr(unsafe.Pointer(&acl)), 0, uintptr(unsafe.Pointer(&descriptor)))
	if code != 0 {
		return syscall.Errno(code)
	}
	defer localFree.Call(descriptor)
	process, err := syscall.GetCurrentProcess()
	if err != nil {
		return err
	}
	var token syscall.Token
	if err = syscall.OpenProcessToken(process, syscall.TOKEN_QUERY, &token); err != nil {
		return err
	}
	defer token.Close()
	u, err := token.GetTokenUser()
	if err != nil {
		return err
	}
	a, err := sid.String()
	if err != nil {
		return err
	}
	b, err := u.User.Sid.String()
	if err != nil {
		return err
	}
	if a != b {
		return errors.New("foreign person entry owner")
	}

	if acl == 0 {
		return errors.New("person entry has unrestricted access")
	}
	// Reject write grants to any SID other than this account, SYSTEM and
	// Administrators. Those two privileged principals can always take ownership.
	type aclHeader struct {
		Revision, Reserved     byte
		Size, Count, Reserved2 uint16
	}
	header := (*aclHeader)(unsafe.Pointer(acl))
	offset := uintptr(8)
	for i := uint16(0); i < header.Count; i++ {
		if offset+8 > uintptr(header.Size) {
			return errors.New("invalid person ACL")
		}
		ace := unsafe.Slice((*byte)(unsafe.Pointer(acl+offset)), 8)
		size := uintptr(ace[2]) | uintptr(ace[3])<<8
		if size < 8 || offset+size > uintptr(header.Size) {
			return errors.New("invalid person ACE")
		}
		if ace[0] == 0 && ace[1]&8 == 0 { // ACCESS_ALLOWED, not inherit-only
			mask := *(*uint32)(unsafe.Pointer(acl + offset + 4))
			if mask&(0x40000000|0x10000000|0x000d0116) != 0 {
				allowed := (*syscall.SID)(unsafe.Pointer(acl + offset + 8))
				value, e := allowed.String()
				if e != nil {
					return e
				}
				if value != b && value != "S-1-5-18" && value != "S-1-5-32-544" {
					return errors.New("person entry grants foreign write access")
				}
			}
		}
		offset += size
	}
	return nil
}
func openFile(r *os.Root, n string, flags int) (*os.File, error) {
	// Root constrains the path; OpenNoFollow refuses the final reparse point.
	f, err := fslock.OpenNoFollow(filepath.Join(r.Name(), n), flags, 0600)
	if err != nil {
		return nil, err
	}
	if err = owner(f); err != nil {
		f.Close()
		return nil, err
	}
	return f, nil
}
func lockFile(f *os.File) error { return fslock.LockExclusive(f) }
func unlockFile(f *os.File)     { fslock.Unlock(f) }

var convertSDDL = syscall.NewLazyDLL("advapi32.dll").NewProc("ConvertStringSecurityDescriptorToSecurityDescriptorW")

func makeDir(path string) error {
	process, err := syscall.GetCurrentProcess()
	if err != nil {
		return err
	}
	var token syscall.Token
	if err = syscall.OpenProcessToken(process, syscall.TOKEN_QUERY, &token); err != nil {
		return err
	}
	defer token.Close()
	u, err := token.GetTokenUser()
	if err != nil {
		return err
	}
	sid, err := u.User.Sid.String()
	if err != nil {
		return err
	}
	sddl, err := syscall.UTF16PtrFromString("D:P(A;OICI;FA;;;" + sid + ")")
	if err != nil {
		return err
	}
	var descriptor uintptr
	ok, _, err := convertSDDL.Call(uintptr(unsafe.Pointer(sddl)), 1, uintptr(unsafe.Pointer(&descriptor)), 0)
	if ok == 0 {
		return err
	}
	defer localFree.Call(descriptor)
	name, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	sa := syscall.SecurityAttributes{Length: uint32(unsafe.Sizeof(syscall.SecurityAttributes{})), SecurityDescriptor: descriptor}
	err = syscall.CreateDirectory(name, &sa)
	if err == syscall.ERROR_ALREADY_EXISTS {
		return os.ErrExist
	}
	return err
}
func checkDir(r *os.Root) error {
	f, err := r.Open(".")
	if err != nil {
		return err
	}
	defer f.Close()
	return owner(f)
}
