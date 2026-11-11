//go:build linux

package sessions

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"syscall"
)

func processAlive(pid int, procStart any) bool {
	if _, err := os.Stat("/proc/self/stat"); err != nil {
		return pidAlive(pid)
	}
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return false
	}
	text := string(data)
	end := strings.LastIndex(text, ")")
	if end < 0 {
		return false
	}
	fields := strings.Fields(text[end+1:])
	if len(fields) < 20 {
		return false
	}
	return fmt.Sprint(procStart) == fields[19]
}

func pidAlive(pid int) bool {
	err := syscall.Kill(pid, 0)
	if err == nil {
		return true
	}
	if errors.Is(err, syscall.ESRCH) {
		return false
	}
	return true
}
