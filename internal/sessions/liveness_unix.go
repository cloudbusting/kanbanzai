//go:build !linux && !windows

package sessions

import (
	"errors"
	"syscall"
)

func processAlive(pid int, procStart any) bool {
	err := syscall.Kill(pid, 0)
	if err == nil {
		return true
	}
	if errors.Is(err, syscall.ESRCH) {
		return false
	}
	return true
}
