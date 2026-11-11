//go:build windows

package sessions

import "syscall"

const (
	processQueryLimitedInformation = 0x1000
	stillActive                    = 259
)

func processAlive(pid int, procStart any) bool {
	handle, err := syscall.OpenProcess(processQueryLimitedInformation, false, uint32(pid))
	if err != nil {
		return false
	}
	defer syscall.CloseHandle(handle)
	var code uint32
	if err := syscall.GetExitCodeProcess(handle, &code); err != nil {
		return false
	}
	return code == stillActive
}
