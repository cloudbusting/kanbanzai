package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cloudbusting/kanbanzai/internal/fsutil"
)

// Upgrade previews the supported v0→v1 migration; only apply writes. It never serializes TOML back out,
// so comments, unknown settings and custom paths cannot be silently discarded.
func Upgrade(start string, apply bool) (string, error) {
	root, top, common, err := Locate(start)
	if err != nil {
		return "", err
	}
	path := ""
	for _, base := range uniqueStrings([]string{root, top}) {
		candidate := configCandidate(base, ".kanbanzai")
		if _, err := os.Lstat(candidate); err == nil {
			path = candidate
			break
		} else if !os.IsNotExist(err) {
			return "", err
		}
	}
	if path == "" {
		return "No config file: embedded defaults already use config_version = 1. Run `kanbanzai init` to create one.", nil
	}
	info, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", errf("%s: upgrade requires a regular, non-symlink config file", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	raw, err := parseTOML(string(data))
	if err != nil {
		return "", qualifyError(path, err)
	}
	value, exists := raw["config_version"]
	current := false
	if exists {
		n, ok := value.(int64)
		if !ok || n < 0 || n > CurrentVersion {
			return "", validateVersion(raw, path)
		}
		if n == CurrentVersion {
			current = true
		}
	}
	prospective := data
	if !current {
		prospective, err = versionOneBytes(data, exists)
		if err != nil {
			return "", qualifyError(path, err)
		}
	}
	raw, err = parseTOML(string(prospective))
	if err != nil {
		return "", qualifyError(path, err)
	}
	if err := validateVersion(raw, path); err != nil {
		return "", err
	}
	if _, err := validateSchema(raw, path); err != nil {
		return "", err
	}
	defaults, err := readDefaults()
	if err != nil {
		return "", err
	}
	p := &Project{Root: root, Toplevel: top, GitCommon: common, ConfigFile: path, cfg: merge(defaults, raw)}
	if err := p.build(); err != nil {
		return "", qualifyError(path, err)
	}
	if current {
		return fmt.Sprintf("%s: config_version = %d; no upgrade needed.", path, CurrentVersion), nil
	}
	backup := path + ".v0.bak"
	if !apply {
		return fmt.Sprintf("%s: upgrade config_version 0 -> 1; preserve all settings and comments; backup %s. Run `kanbanzai config upgrade --apply` to write.", path, backup), nil
	}
	// An exclusive backup is also the migration's single-writer gate. Never replace an earlier backup.
	file, err := os.OpenFile(backup, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return "", errf("%s: cannot create exclusive backup (%v); keep the backup safe and resolve this before retrying", backup, err)
	}
	_, writeErr := file.Write(data)
	if writeErr == nil {
		writeErr = file.Sync()
	}
	closeErr := file.Close()
	if writeErr != nil {
		return "", writeErr
	}
	if closeErr != nil {
		return "", closeErr
	}
	// Refuse a source replaced or edited while the backup was being made.
	currentInfo, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	now, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	if !currentInfo.Mode().IsRegular() || !os.SameFile(info, currentInfo) || !bytes.Equal(now, data) {
		return "", errf("%s: changed during upgrade; no config was written; original backup is %s", path, backup)
	}
	if err := fsutil.WriteFileAtomic(path, prospective, info.Mode().Perm()); err != nil {
		return "", err
	}
	return fmt.Sprintf("%s: upgraded config_version 0 -> 1; backup %s", path, backup), nil
}

func versionOneBytes(data []byte, hasVersion bool) ([]byte, error) {
	if !hasVersion {
		return append([]byte("config_version = 1\n"), data...), nil
	}
	// Use the TOML lexer rather than a line regex: quoted keys, hex zero and multiline values are valid.
	// Root keys precede table declarations. parseTOML has already proved the whole document valid.
	p := &tomlParser{src: strings.ReplaceAll(string(data), "\r\n", "\n"), line: 1}
	for {
		p.skipBlank()
		if p.eof() || p.peek() == '[' {
			return nil, fmt.Errorf("cannot locate root config_version for upgrade")
		}
		path, err := p.keyPath()
		if err != nil {
			return nil, err
		}
		p.skipSpace()
		if !p.take('=') {
			return nil, p.errorf("expected '='")
		}
		p.skipSpace()
		start := p.pos
		if _, err := p.value(); err != nil {
			return nil, err
		}
		end := p.pos
		if len(path) == 1 && path[0] == "config_version" {
			start, end = originalOffset(data, start), originalOffset(data, end)
			out := append([]byte{}, data[:start]...)
			out = append(out, '1')
			out = append(out, data[end:]...)
			return out, nil
		}
		if err := p.endOfLine(); err != nil {
			return nil, err
		}
	}
}

// The parser normalizes CRLF; map its byte offsets back without rewriting the source's line endings.
func originalOffset(data []byte, normalized int) int {
	i := 0
	for n := 0; n < normalized; n++ {
		if data[i] == '\r' && i+1 < len(data) && data[i+1] == '\n' {
			i++
		}
		i++
	}
	return i
}

// BootstrapPath is the fixed discovery location, independent of project.home.
func BootstrapPath(root string) string { return filepath.Join(root, ".kanbanzai", "config.toml") }
