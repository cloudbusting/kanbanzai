//go:build !windows

package doctor

import (
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestSpecialFilesDoNotBlockDiagnosis(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pipe")
	if err := syscall.Mkfifo(path, 0o600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name  string
		check func() error
	}{
		{"adapter JSON", func() error { _, err := readObject(path); return err }},
		{"card folder", func() error { return readableDirectory(path) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			done := make(chan error, 1)
			go func() { done <- tc.check() }()
			select {
			case err := <-done:
				if err == nil {
					t.Fatal("special file accepted")
				}
			case <-time.After(time.Second):
				t.Fatal("doctor blocked on a FIFO instead of diagnosing it")
			}
		})
	}
}
