//go:build !linux

package claim

// visibleParentProcess has no /proc to read outside Linux. A shell with no session id falls back to user@host; the
// harnesses set a session id in normal use.
func visibleParentProcess() string { return "" }
