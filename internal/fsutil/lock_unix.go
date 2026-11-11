//go:build !windows

package fsutil

import (
	"errors"
	"os"
	"syscall"
)

func lockFile(file *os.File, try bool) error {
	how := syscall.LOCK_EX
	if try {
		how |= syscall.LOCK_NB
	}
	err := syscall.Flock(int(file.Fd()), how)
	if err != nil {
		if try && (errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN)) {
			return ErrLocked
		}
		return err
	}
	return nil
}

func unlockFile(file *os.File) error {
	return syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
}
