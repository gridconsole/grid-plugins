//go:build windows

package fslock

import (
	"errors"
	"os"
	"syscall"
	"unsafe"
)

const (
	lockfileFailImmediately = 0x00000001
	lockfileExclusiveLock   = 0x00000002
	errorLockViolation      = syscall.Errno(33)
)

var (
	kernel32             = syscall.NewLazyDLL("kernel32.dll")
	procLockFileEx       = kernel32.NewProc("LockFileEx")
	procUnlockFileEx     = kernel32.NewProc("UnlockFileEx")
	procCreateEvent      = kernel32.NewProc("CreateEventW")
	procOverlappedResult = kernel32.NewProc("GetOverlappedResult")
)

// LockShared blocks until f holds a shared lock.
func LockShared(f *os.File) error { return lockWait(f, 0) }

// LockExclusive blocks until f holds an exclusive lock.
func LockExclusive(f *os.File) error { return lockWait(f, lockfileExclusiveLock) }

// TryLockExclusive takes an exclusive lock without waiting.
func TryLockExclusive(f *os.File) error { return tryLockFile(f, lockfileExclusiveLock) }

// lockWait is the blocking lock behind LockShared and LockExclusive, with the
// wait hook of fslock.go in front of it.
func lockWait(f *os.File, flags uint32) error {
	return lockWaiting(f,
		func() error { return tryLockFile(f, flags) },
		func() error { return lockFile(f, flags, false) })
}

// tryLockFile is lockFile without waiting; ErrLocked says that another handle
// holds the lock.
func tryLockFile(f *os.File, flags uint32) error {
	err := lockFile(f, flags|lockfileFailImmediately, false)
	if errors.Is(err, errorLockViolation) {
		return ErrLocked
	}
	return err
}

// Unlock drops the lock; closing f would drop it too.
func Unlock(f *os.File) { _ = lockFile(f, 0, true) }

func lockFile(f *os.File, flags uint32, unlock bool) error {
	conn, err := f.SyscallConn()
	if err != nil {
		return err
	}
	var lockErr error
	// Control keeps the file alive and prevents Close from recycling its handle
	// until the operation (including any pending completion) has finished.
	err = conn.Control(func(fd uintptr) {
		lockErr = lockHandle(fd, flags, unlock)
	})
	if err != nil {
		return err
	}
	return lockErr
}

func lockHandle(fd uintptr, flags uint32, unlock bool) error {
	event, _, err := procCreateEvent.Call(0, 1, 0, 0) // private, manual-reset event
	if event == 0 {
		return err
	}
	defer syscall.CloseHandle(syscall.Handle(event))
	// Windows byte-range locks are mandatory. Reserve one byte at
	// 0x7FFFFFFFFFFFFFFE, far beyond any content, so ordinary reads and writes
	// through other handles remain possible. Every lock and unlock uses it.
	over := &syscall.Overlapped{
		Offset: 0xfffffffe, OffsetHigh: 0x7fffffff, HEvent: syscall.Handle(event),
	}
	var ok uintptr
	if unlock {
		ok, _, err = procUnlockFileEx.Call(fd, 0, 1, 0, uintptr(unsafe.Pointer(over)))
	} else {
		ok, _, err = procLockFileEx.Call(fd, uintptr(flags), 0, 1, 0, uintptr(unsafe.Pointer(over)))
	}
	if ok != 0 {
		return nil
	}
	if errors.Is(err, syscall.ERROR_IO_PENDING) {
		// Current callers open synchronous handles (see docs/lanes/L63.md),
		// but still wait on our event if Windows reports pending I/O. Keep
		// OVERLAPPED and the event alive until the result is known.
		var transferred uint32
		ok, _, err = procOverlappedResult.Call(fd, uintptr(unsafe.Pointer(over)), uintptr(unsafe.Pointer(&transferred)), 1)
		if ok != 0 {
			return nil
		}
	}
	return err
}

// OpenNoFollow is os.OpenFile that refuses a reparse point as the final component.
func OpenNoFollow(path string, flag int, perm os.FileMode) (*os.File, error) {
	fail := func(err error) (*os.File, error) {
		return nil, &os.PathError{Op: "open", Path: path, Err: err}
	}
	name, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return fail(err)
	}
	var access uint32
	switch flag & (os.O_WRONLY | os.O_RDWR) {
	case os.O_RDONLY:
		access = syscall.GENERIC_READ
	case os.O_WRONLY:
		access = syscall.GENERIC_WRITE
	case os.O_RDWR:
		access = syscall.GENERIC_READ | syscall.GENERIC_WRITE
	default:
		return fail(syscall.EINVAL)
	}
	if flag&os.O_CREATE != 0 {
		access |= syscall.GENERIC_WRITE
	}
	if flag&os.O_APPEND != 0 {
		// NewFile cannot set os.File's append mode. Enforce append in the
		// kernel by granting GENERIC_WRITE's rights except FILE_WRITE_DATA.
		const fileWriteEA = 0x00000010
		access &^= syscall.GENERIC_WRITE
		access |= syscall.FILE_APPEND_DATA | syscall.FILE_WRITE_ATTRIBUTES | fileWriteEA | syscall.STANDARD_RIGHTS_WRITE | syscall.SYNCHRONIZE
	}
	mode := uint32(syscall.OPEN_EXISTING)
	switch {
	case flag&(os.O_CREATE|os.O_EXCL) == os.O_CREATE|os.O_EXCL:
		mode = syscall.CREATE_NEW
	case flag&os.O_CREATE != 0:
		mode = syscall.OPEN_ALWAYS
	}
	// No caller opens a directory, so do not request BACKUP_SEMANTICS.
	attrs := uint32(syscall.FILE_ATTRIBUTE_NORMAL | syscall.FILE_FLAG_OPEN_REPARSE_POINT)
	if flag&os.O_CREATE != 0 && perm&0o200 == 0 {
		attrs = syscall.FILE_ATTRIBUTE_READONLY | syscall.FILE_FLAG_OPEN_REPARSE_POINT
	}
	if flag&os.O_SYNC != 0 {
		const fileFlagWriteThrough = 0x80000000
		attrs |= fileFlagWriteThrough
	}
	openAccess := access
	if flag&(os.O_TRUNC|os.O_APPEND) == os.O_TRUNC|os.O_APPEND {
		openAccess |= syscall.GENERIC_WRITE // needed only until truncation
	}
	h, err := syscall.CreateFile(name, openAccess,
		syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE|syscall.FILE_SHARE_DELETE,
		nil, mode, attrs, 0)
	if err != nil {
		// Directory reparse points (junctions) cannot be opened without
		// BACKUP_SEMANTICS. Refine the failed open's error, never retry it
		// following the link. Successful opens are checked by handle below.
		if errors.Is(err, syscall.ERROR_ACCESS_DENIED) && mode != syscall.CREATE_NEW {
			if a, e := syscall.GetFileAttributes(name); e == nil && a&syscall.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
				err = syscall.ELOOP
			}
		}
		return fail(err)
	}
	closeFail := func(err error) (*os.File, error) {
		_ = syscall.CloseHandle(h)
		return fail(err)
	}
	var info syscall.ByHandleFileInformation
	if err := syscall.GetFileInformationByHandle(h, &info); err != nil {
		return closeFail(err)
	}
	if info.FileAttributes&syscall.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		return closeFail(syscall.ELOOP)
	}
	// CREATE_ALWAYS is incompatible with OPEN_REPARSE_POINT. Truncate only
	// after checking the opened handle, so a refused link is never modified.
	if flag&os.O_TRUNC != 0 {
		if err := syscall.Ftruncate(h, 0); err != nil {
			return closeFail(err)
		}
	}
	if openAccess != access {
		// Reduce the same file object's access after O_TRUNC|O_APPEND; do
		// not reopen the path, which could have been replaced in between.
		process, err := syscall.GetCurrentProcess()
		if err != nil {
			return closeFail(err)
		}
		var appendHandle syscall.Handle
		if err := syscall.DuplicateHandle(process, h, process, &appendHandle, access, false, 0); err != nil {
			return closeFail(err)
		}
		_ = syscall.CloseHandle(h)
		h = appendHandle
	}
	return os.NewFile(uintptr(h), path), nil
}
