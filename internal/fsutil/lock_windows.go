//go:build windows

package fsutil

import (
	"os"
	"syscall"
	"unsafe"
)

// The standard library's syscall package does not export LockFileEx, so call it from kernel32 directly. This keeps
// the Windows build dependency-free.
var (
	kernel32         = syscall.NewLazyDLL("kernel32.dll")
	procLockFileEx   = kernel32.NewProc("LockFileEx")
	procUnlockFileEx = kernel32.NewProc("UnlockFileEx")
)

const (
	lockfileExclusiveLock   = 0x00000002
	lockfileFailImmediately = 0x00000001
	errorLockViolation      = syscall.Errno(33)
)

func lockFile(file *os.File, try bool) error {
	var overlapped syscall.Overlapped
	flags := uintptr(lockfileExclusiveLock)
	if try {
		flags |= lockfileFailImmediately
	}
	ok, _, err := procLockFileEx.Call(file.Fd(), flags, 0, 1, 0, uintptr(unsafe.Pointer(&overlapped)))
	if ok == 0 {
		if try && (err == errorLockViolation || err == syscall.ERROR_IO_PENDING) {
			return ErrLocked
		}
		return err
	}
	return nil
}

func unlockFile(file *os.File) error {
	var overlapped syscall.Overlapped
	ok, _, err := procUnlockFileEx.Call(file.Fd(), 0, 1, 0, uintptr(unsafe.Pointer(&overlapped)))
	if ok == 0 {
		return err
	}
	return nil
}
