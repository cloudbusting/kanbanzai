package testutil

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// ClaudeSession writes the status file Claude Code keeps for a live session into a fresh CLAUDE_CONFIG_DIR, for the
// running test process (so it counts as alive),. The session is waiting for the owner
// unless over says otherwise; a nil value in over drops the field.
func ClaudeSession(t *testing.T, id, name, cwd string, over map[string]any) {
	t.Helper()
	dir := os.Getenv("CLAUDE_CONFIG_DIR")
	if dir == "" || !strings.HasPrefix(dir, os.TempDir()) {
		dir = t.TempDir()
		t.Setenv("CLAUDE_CONFIG_DIR", dir)
	}
	if err := os.MkdirAll(filepath.Join(dir, "sessions"), 0o755); err != nil {
		t.Fatal(err)
	}
	record := map[string]any{
		"pid": os.Getpid(), "sessionId": id, "cwd": cwd, "startedAt": 1794343666976, "procStart": ownStart(t),
		"name": name, "status": "waiting", "waitingFor": "input needed", "updatedAt": 1794343821264,
		"statusUpdatedAt": 1794343821000,
	}
	for key, value := range over {
		record[key] = value
	}
	for key, value := range record {
		if value == nil {
			delete(record, key)
		}
	}
	data, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sessions", name+".json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// ownStart is this process's start time as the status file records it (the stat field the liveness check compares),
// or nil where the check does not use one.
func ownStart(t *testing.T) any {
	t.Helper()
	if runtime.GOOS != "linux" {
		return nil
	}
	data, err := os.ReadFile("/proc/self/stat")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	end := strings.LastIndex(text, ")")
	if end < 0 {
		t.Fatal("no stat fields")
	}
	fields := strings.Fields(text[end+1:])
	if len(fields) < 20 {
		t.Fatalf("stat fields = %d", len(fields))
	}
	return fields[19]
}
