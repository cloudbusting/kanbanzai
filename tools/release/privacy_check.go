package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

// TASK-71: nothing a release packages or publishes may carry a path, a session id or a settings file from one
// person's machine. The check has two parts: the repository's shipped files (checkPrivateTree, part of -check) and
// the unpacked artifacts, binaries included (scanArtifacts, -scan). Both use the same rules, and both allow the
// fictional forms the docs name: a user called "example", the all-zero Claude Code session id and a ses_example… id.

type privateRule struct {
	kind    string
	pattern *regexp.Regexp
	// fictional reports whether the first capture group is one of the allowed made-up values.
	fictional func(captured string) bool
}

const (
	// The character before a value must not make it part of a longer word, a URL or a relative path.
	valueBoundary = `(?:^|[^a-z0-9_./\\-])`
	valueEnd      = `[^\s"'\x60]+`
)

func startsWithExample(captured string) bool {
	return strings.HasPrefix(strings.ToLower(captured), "example")
}

var privateRules = []privateRule{
	{"home path", regexp.MustCompile(`(?i)` + valueBoundary + `(?:/` + `home/|/` + `users/)([^/\s"'\x60]+)/` + valueEnd), startsWithExample},
	{"home path", regexp.MustCompile(`(?i)` + valueBoundary + `[a-z]:\\{1,2}users\\{1,2}([^\\\s"'\x60]+)\\{1,2}` + valueEnd), startsWithExample},
	{"OpenCode session id", regexp.MustCompile(`(?i)` + valueBoundary + `ses_([a-z0-9]{10,})`), startsWithExample},
	{"Claude Code session id", regexp.MustCompile(`(?i)claude:([0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12})`),
		func(captured string) bool { return strings.HasPrefix(captured, "00000000-") }},
}

// privateKind names the first machine-specific value in text, or "" when it holds none.
func privateKind(text []byte) (kind string, at int) {
	for _, rule := range privateRules {
		for _, loc := range rule.pattern.FindAllSubmatchIndex(text, -1) {
			if rule.fictional(string(text[loc[2]:loc[3]])) {
				continue
			}
			if kind == "" || loc[2] < at {
				kind, at = rule.kind, loc[2]
			}
			break
		}
	}
	return kind, at
}

// personalFile names a file that belongs to one machine's setup, never to a release.
func personalFile(name string) bool {
	return name == "settings.local.json" || name == "scheduled_tasks.lock" || strings.HasSuffix(name, ".openchamber.backup")
}

var absoluteEntry = regexp.MustCompile(`^(?:[/\\]|[A-Za-z]:[/\\])`)

// absolutePlugin reports the first absolute entry in an OpenCode settings file's plugins list.
func absolutePlugin(data []byte) string {
	var settings struct {
		Plugins []any `json:"plugins"`
	}
	if json.Unmarshal(data, &settings) != nil {
		return ""
	}
	for _, entry := range settings.Plugins {
		if text, ok := entry.(string); ok && absoluteEntry.MatchString(text) {
			return text
		}
	}
	return ""
}

const maxFindings = 20

var textExtensions = map[string]bool{
	".go": true, ".mjs": true, ".js": true, ".json": true, ".md": true, ".toml": true, ".yml": true, ".yaml": true,
	".sh": true, ".txt": true, ".html": true, ".css": true, ".xml": true,
}

// checkPrivateTree checks the files a repository ships: the tracked files when root is a git checkout (a developer's
// own ignored files are not shipped), otherwise every file under root, so an exported tree with no .git is checked
// the same way. The board's briefs are checkBoardRecords' business.
func checkPrivateTree(root string) error {
	files, err := shippedFiles(root)
	if err != nil {
		return err
	}
	var findings []string
	add := func(format string, args ...any) {
		if len(findings) < maxFindings {
			findings = append(findings, fmt.Sprintf(format, args...))
		}
	}
	for _, relative := range files {
		if strings.HasPrefix(relative, ".kanbanzai/work/") {
			continue
		}
		base := path.Base(relative)
		if personalFile(base) {
			add("%s: a file from one machine's setup; ignore it instead of tracking it", relative)
			continue
		}
		full := filepath.Join(root, filepath.FromSlash(relative))
		info, err := os.Lstat(full)
		if errors.Is(err, fs.ErrNotExist) {
			continue // tracked but deleted in this working tree
		} else if err != nil {
			return err
		}
		if !info.Mode().IsRegular() || !textExtensions[strings.ToLower(path.Ext(base))] {
			continue
		}
		if relative == ".opencode/opencode.json" {
			data, err := os.ReadFile(full)
			if err != nil {
				return err
			}
			if entry := absolutePlugin(data); entry != "" {
				add("%s: a plugin entry is an absolute path; use a relative one such as ./opencode", relative)
			}
		}
		if err := scanLines(full, func(line int, text []byte) {
			if kind, _ := privateKind(text); kind != "" {
				add("%s:%d: machine-specific %s", relative, line, kind)
			}
		}); err != nil {
			return err
		}
	}
	if len(findings) == 0 {
		return nil
	}
	return errors.New(strings.Join(findings, "\n"))
}

func scanLines(full string, visit func(line int, text []byte)) error {
	file, err := os.Open(full)
	if err != nil {
		return err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 4096), 4*1024*1024)
	for line := 1; scanner.Scan(); line++ {
		visit(line, scanner.Bytes())
	}
	return scanner.Err()
}

// shippedFiles lists the files under root as slash paths: git's tracked files in a checkout, otherwise a walk that
// leaves out folders that are never shipped.
func shippedFiles(root string) ([]string, error) {
	if _, err := os.Stat(filepath.Join(root, ".git")); err == nil {
		if _, err := exec.LookPath("git"); err == nil {
			out, err := exec.Command("git", "-C", root, "ls-files", "-z").Output()
			if err == nil {
				var files []string
				for _, name := range strings.Split(string(out), "\x00") {
					if name != "" {
						files = append(files, name)
					}
				}
				return files, nil
			}
		}
	}
	var files []string
	err := filepath.WalkDir(root, func(current string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, current)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		if entry.IsDir() {
			switch entry.Name() {
			case ".git", "node_modules", "dist":
				return filepath.SkipDir
			}
			if relative == "bin" {
				return filepath.SkipDir
			}
			return nil
		}
		files = append(files, relative)
		return nil
	})
	return files, err
}

// scanArtifacts checks every file under dir, binaries too, for the same values: the way to prove an unpacked
// source tarball, .vsix or command binary carries no build path or session id.
func scanArtifacts(dir string) error {
	var findings []string
	err := filepath.WalkDir(dir, func(current string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !entry.Type().IsRegular() {
			return nil
		}
		relative, err := filepath.Rel(dir, current)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		if personalFile(entry.Name()) && len(findings) < maxFindings {
			findings = append(findings, fmt.Sprintf("%s: a file from one machine's setup", relative))
		}
		data, err := os.ReadFile(current)
		if err != nil {
			return err
		}
		if kind, at := privateKind(data); kind != "" && len(findings) < maxFindings {
			findings = append(findings, fmt.Sprintf("%s: offset %d: machine-specific %s", relative, at, kind))
		}
		return nil
	})
	if err != nil {
		return err
	}
	if len(findings) == 0 {
		return nil
	}
	return errors.New(strings.Join(findings, "\n"))
}
