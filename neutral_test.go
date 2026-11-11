package kanbanzai

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/cloudbusting/kanbanzai/internal/testutil"
)

// Kanbanzai names no project: nothing here is twisty's, so any project can use it.
var forbidden = regexp.MustCompile("(?i)" + strings.Join([]string{
	"twis" + "ty", "mo" + "to", "ri" + "der", "a" + "ws", "an" + "dy",
}, "|"))

var skippedDirs = map[string]bool{".git": true, ".kanbanzai": true, "__pycache__": true, "node_modules": true}

func TestNoProjectIsNamed(t *testing.T) {
	root, err := testutil.RepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	found := []string{}
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		parts := strings.Split(filepath.ToSlash(relative), "/")
		if entry.IsDir() {
			if relative != "." && skippedDirs[entry.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Name() == "neutral_test.go" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil || !utf8.Valid(data) {
			return nil
		}
		for number, line := range strings.Split(string(data), "\n") {
			if forbidden.MatchString(line) {
				found = append(found, relative+":"+itoa(number+1)+": "+strings.TrimSpace(line))
			}
		}
		_ = parts
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(found) > 0 {
		t.Fatalf("a project is named:\n%s", strings.Join(found, "\n"))
	}
}

func TestTheNeutralCheckCanFail(t *testing.T) {
	for _, line := range []string{"the Twis" + "ty planner", "AW" + "S account", "it dr" + "aws a map"} {
		if !forbidden.MatchString(line) {
			t.Fatalf("the check misses %q", line)
		}
	}
	if forbidden.MatchString("a board of cards") {
		t.Fatal("the check flags a neutral line")
	}
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	digits := []byte{}
	for value > 0 {
		digits = append([]byte{byte('0' + value%10)}, digits...)
		value /= 10
	}
	return string(digits)
}
