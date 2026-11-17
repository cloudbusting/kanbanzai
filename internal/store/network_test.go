package store_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The command makes no network request of its own: a store plugin reaches its tracker with the tools it drives. The
// only network code is the browser board's loopback server (internal/webboard). This fails when any other package
// imports a network package, so a store plugin's client cannot grow one by accident.
var networkImport = regexp.MustCompile(`"(net|net/http|net/http/httputil|net/rpc|net/smtp|net/mail|crypto/tls|net/url/x)"`)

func TestNoPackageButTheBrowserBoardImportsTheNetwork(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, top := range []string{"internal", "cmd"} {
		err := filepath.WalkDir(filepath.Join(root, top), func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			rel := filepath.ToSlash(strings.TrimPrefix(path, root+string(filepath.Separator)))
			if entry.IsDir() {
				if rel == "internal/webboard" {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(rel, ".go") || strings.HasSuffix(rel, "_test.go") {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			checked++
			if match := networkImport.FindString(string(data)); match != "" {
				t.Errorf("%s imports %s: only the browser board's loopback server may use the network", rel, match)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if checked < 50 {
		t.Fatalf("only %d files were checked: the walk lost its way", checked)
	}
}
