package main

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// Board records can travel with a marketplace checkout, even though the curated
// source bundle deliberately omits the board. Do not require git in an export.
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
			if kind, _ := privateKind(scanner.Bytes()); kind != "" {
				rel, err := filepath.Rel(root, path)
				if err != nil {
					return err
				}
				return fmt.Errorf("%s:%d: machine-specific %s in board record", filepath.ToSlash(rel), line, kind)
			}
		}
		return scanner.Err()
	})
}
