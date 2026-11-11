// Package fsutil holds the file and lock primitives every part of the core shares: atomic state writes, JSON files
// and the advisory lock around a read-modify-write.
package fsutil

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// ErrLocked means a non-blocking lock is already held.
var ErrLocked = errors.New("locked")

// WriteFileAtomic writes data to path through a hidden temporary file in the same folder, then renames it into
// place. A reader sees the old file or the new one, never half of either.
func WriteFileAtomic(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(dir, "."+filepath.Base(path)+".*")
	if err != nil {
		return err
	}
	name := temporary.Name()
	cleanup := func() {
		temporary.Close()
		os.Remove(name)
	}
	if err := temporary.Chmod(0o600); err != nil {
		cleanup()
		return err
	}
	if _, err := temporary.Write(data); err != nil {
		cleanup()
		return err
	}
	if err := temporary.Sync(); err != nil {
		cleanup()
		return err
	}
	if err := temporary.Close(); err != nil {
		os.Remove(name)
		return err
	}
	if err := os.Chmod(name, mode); err != nil {
		os.Remove(name)
		return err
	}
	if err := os.Rename(name, path); err != nil {
		os.Remove(name)
		return err
	}
	return nil
}

// ReadJSON reads a JSON object, or returns false when the path is missing, unreadable, invalid or not an object.
func ReadJSON(path string) (map[string]any, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return nil, false
	}
	object, ok := value.(map[string]any)
	if !ok {
		return nil, false
	}
	return object, true
}

// WriteJSON writes a JSON object the way the state files are written: two-space indent, sorted keys, one trailing
// newline, and mode 0600.
func WriteJSON(path string, value map[string]any) error {
	data, err := MarshalIndent(value)
	if err != nil {
		return err
	}
	return WriteFileAtomic(path, append(data, '\n'), 0o600)
}

// MarshalIndent is the state files' JSON encoding, without the trailing newline.
func MarshalIndent(value any) ([]byte, error) {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(buffer.Bytes(), []byte("\n")), nil
}

// AppendJSONLine appends one JSON object to a line-delimited log, creating it 0600.
func AppendJSONLine(path string, value map[string]any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()
	if _, err := file.Write(append(data, '\n')); err != nil {
		return err
	}
	return os.Chmod(path, 0o600)
}
