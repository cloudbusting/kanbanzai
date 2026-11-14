package main

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Board records can travel with a marketplace checkout, even though the curated
// source bundle deliberately omits the board. Do not require git in an export.
var privateRecordValue = regexp.MustCompile(`(?i)(^|[^a-z0-9_./\\-])((?:/home/|/users/)[^/\s\x60]+/[^\s\x60]+|[a-z]:\\users\\[^\\\s\x60]+\\[^\s\x60]+|ses_[a-z0-9]{10,})`)

func checkBoardRecords(root string) error {
	dir := filepath.Join(root, ".kanbanzai", "work")
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil // The curated source bundle does not contain the board.
	} else if err != nil {
		return err
	}
	return filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || filepath.Ext(path) != ".md" {
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("board record %s is a symlink", path)
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()
		scanner := bufio.NewScanner(file)
		scanner.Buffer(make([]byte, 4096), 1024*1024)
		for line := 1; scanner.Scan(); line++ {
			for _, match := range privateRecordValue.FindAllStringSubmatch(scanner.Text(), -1) {
				value := strings.ToLower(match[2])
				if strings.HasPrefix(value, "/home/example/") || strings.HasPrefix(value, "/users/example/") || strings.HasPrefix(value, `c:\users\example\`) {
					continue
				}
				rel, err := filepath.Rel(root, path)
				if err != nil {
					return err
				}
				return fmt.Errorf("%s:%d: machine-specific path or session ID in board record", filepath.ToSlash(rel), line)
			}
		}
		return scanner.Err()
	})
}
