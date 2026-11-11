package sessions

// alive reports whether a record's process exists here and is the one the file recorded. A pid that has been
// reused has another start time, which is why the platform readers compare it where they can.
func alive(pid int, procStart any) bool {
	if pid <= 0 {
		return false
	}
	return processAlive(pid, procStart)
}
