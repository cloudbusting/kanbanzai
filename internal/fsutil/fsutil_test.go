package fsutil

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestWriteFileAtomic(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "file.json")
	if err := WriteFileAtomic(path, []byte("one\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "one\n" {
		t.Fatalf("read = %q, %v", data, err)
	}
	if err := WriteFileAtomic(path, []byte("two\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(path)
	if string(data) != "two\n" {
		t.Fatalf("read = %q", data)
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("temporary files left behind: %v", entries)
	}
	if runtime.GOOS != "windows" {
		info, _ := os.Stat(path)
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("mode = %v", info.Mode().Perm())
		}
	}
}

func TestJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "record.json")
	if _, ok := ReadJSON(path); ok {
		t.Fatal("a missing file read as JSON")
	}
	if err := WriteJSON(path, map[string]any{"b": 2, "a": "x"}); err != nil {
		t.Fatal(err)
	}
	record, ok := ReadJSON(path)
	if !ok || record["a"] != "x" || record["b"] != float64(2) {
		t.Fatalf("record = %#v, %v", record, ok)
	}
	data, _ := os.ReadFile(path)
	if !strings.HasPrefix(string(data), "{\n  \"a\": \"x\",") || !strings.HasSuffix(string(data), "}\n") {
		t.Fatalf("format = %q", data)
	}
	if err := os.WriteFile(path, []byte("[1, 2]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, ok := ReadJSON(path); ok {
		t.Fatal("an array read as a JSON object")
	}
}

func TestAppendJSONLine(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.jsonl")
	for _, value := range []string{"one", "two"} {
		if err := AppendJSONLine(path, map[string]any{"event": value}); err != nil {
			t.Fatal(err)
		}
	}
	data, _ := os.ReadFile(path)
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 || !strings.Contains(lines[0], `"one"`) || !strings.Contains(lines[1], `"two"`) {
		t.Fatalf("history = %q", data)
	}
}

func TestTryLockAndTheBlockingLock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lock")
	helper := startHelper(t, path)

	if _, err := TryLock(path); !errors.Is(err, ErrLocked) {
		t.Fatalf("TryLock error = %v, want ErrLocked", err)
	}

	acquired := make(chan *Lock, 1)
	go func() {
		lock, err := OpenLock(path)
		if err == nil {
			acquired <- lock
		}
	}()
	select {
	case <-acquired:
		t.Fatal("the blocking lock was acquired while the helper held it")
	case <-time.After(150 * time.Millisecond):
	}

	stopHelper(t, helper)
	select {
	case lock := <-acquired:
		if err := lock.Close(); err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the blocking lock never arrived after the helper left")
	}

	lock, err := TryLock(path)
	if err != nil {
		t.Fatalf("TryLock after the helper: %v", err)
	}
	if err := lock.Close(); err != nil {
		t.Fatal(err)
	}
}

type helperProcess struct {
	cmd   *exec.Cmd
	stdin io.WriteCloser
}

func startHelper(t *testing.T, path string) *helperProcess {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=TestLockHelper")
	cmd.Env = append(os.Environ(), "GO_WANT_LOCK_HELPER=1", "KANBANZAI_LOCK_PATH="+path)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	reader := bufio.NewReader(stdout)
	line, err := reader.ReadString('\n')
	if err != nil || strings.TrimSpace(line) != "locked" {
		_ = cmd.Process.Kill()
		t.Fatalf("helper did not lock: %q, %v", line, err)
	}
	return &helperProcess{cmd: cmd, stdin: stdin}
}

func stopHelper(t *testing.T, helper *helperProcess) {
	t.Helper()
	_, _ = helper.stdin.Write([]byte("go\n"))
	_ = helper.stdin.Close()
	done := make(chan error, 1)
	go func() { done <- helper.cmd.Wait() }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		_ = helper.cmd.Process.Kill()
		t.Fatal("helper did not stop")
	}
}

func TestLockHelper(t *testing.T) {
	if os.Getenv("GO_WANT_LOCK_HELPER") != "1" {
		return
	}
	lock, err := OpenLock(os.Getenv("KANBANZAI_LOCK_PATH"))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	fmt.Println("locked")
	_, _ = bufio.NewReader(os.Stdin).ReadString('\n')
	_ = lock.Close()
	os.Exit(0)
}
