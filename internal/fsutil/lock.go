package fsutil

import (
	"os"
	"path/filepath"
)

// Lock is an advisory exclusive lock on a file. Opening it blocks until it is free; Close releases it.
type Lock struct {
	file *os.File
	path string
}

// OpenLock takes the exclusive lock at path, creating the file and its folder when needed. It waits for another
// holder, as a session waits for a number or a claim.
func OpenLock(path string) (*Lock, error) {
	return openLock(path, false)
}

// TryLock takes the exclusive lock at path or returns ErrLocked when another holder has it.
func TryLock(path string) (*Lock, error) {
	return openLock(path, true)
}

func openLock(path string, try bool) (*Lock, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, err
	}
	if err := lockFile(file, try); err != nil {
		file.Close()
		return nil, err
	}
	return &Lock{file: file, path: path}, nil
}

// Close releases the lock and closes its file.
func (l *Lock) Close() error {
	if l == nil || l.file == nil {
		return nil
	}
	unlockErr := unlockFile(l.file)
	closeErr := l.file.Close()
	l.file = nil
	if unlockErr != nil {
		return unlockErr
	}
	return closeErr
}
