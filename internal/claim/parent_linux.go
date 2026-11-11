//go:build linux

package claim

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// visibleParentProcess walks up from this shell to the Claude Code process that wraps it, so a shell without a
// session id still labels itself like the session's other shells. Linux only: other systems have no /proc.
func visibleParentProcess() string {
	pid := os.Getppid()
	for step := 0; step < 10; step++ {
		if pid <= 1 {
			return ""
		}
		comm, err := os.ReadFile(fmt.Sprintf("/proc/%d/comm", pid))
		if err != nil {
			return ""
		}
		cmdline, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
		if err != nil {
			return ""
		}
		if strings.TrimSpace(string(comm)) == "claude" ||
			strings.Contains(strings.ReplaceAll(string(cmdline), "\x00", " "), "claude-code") {
			return fmt.Sprintf("claude:%d", pid)
		}
		stat, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
		if err != nil {
			return ""
		}
		end := strings.LastIndex(string(stat), ")")
		if end < 0 {
			return ""
		}
		fields := strings.Fields(string(stat)[end+1:])
		if len(fields) < 2 {
			return ""
		}
		next, err := strconv.Atoi(fields[1])
		if err != nil {
			return ""
		}
		pid = next
	}
	return ""
}
