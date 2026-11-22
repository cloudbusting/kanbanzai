package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestOpenCodeNamesWaitsForRequestedSkill(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fake host uses /bin/sh; the real host test runs on Linux")
	}
	dir := t.TempDir()
	counter := filepath.Join(dir, "called")
	host := filepath.Join(dir, "fake-opencode")
	script := "#!/bin/sh\nif test -f '" + counter + "'; then\n" +
		"  echo '{\"data\":[{\"name\":\"Report\"},{\"name\":\"build\"}]}'\n" +
		"else\n  touch '" + counter + "'\n  echo '{\"data\":[{\"name\":\"Report\"}]}'\nfi\n"
	if err := os.WriteFile(host, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	names := openCodeNames(t, dir, os.Environ(), host, "skill", "build")
	if !has(names, "build") {
		t.Fatalf("returned before the requested skill loaded: %v", names)
	}
	if _, err := os.Stat(counter); err != nil || !strings.Contains(strings.Join(names, ","), "Report") {
		t.Fatalf("fake host was not queried: %v %v", names, err)
	}
}
