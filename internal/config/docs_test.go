package config

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/cloudbusting/kanbanzai/internal/testutil"
)

// W-18: the documentation's configuration examples use only keys the command accepts, and every key the command
// accepts is documented, so a renamed or removed key cannot rot silently in the docs.

// docFiles are the files a user reads: the README and every guide.
func docFiles(t *testing.T) (string, []string) {
	t.Helper()
	root, err := testutil.RepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	guides, err := filepath.Glob(filepath.Join(root, "docs", "*.md"))
	if err != nil || len(guides) == 0 {
		t.Fatalf("docs/*.md: %v %v", guides, err)
	}
	return root, append([]string{filepath.Join(root, "README.md")}, guides...)
}

var (
	tomlFence    = regexp.MustCompile("(?s)```toml\n(.*?)\n```")
	tomlTable    = regexp.MustCompile(`^\[\[?([A-Za-z0-9_.-]+)\]\]?\s*(#.*)?$`)
	tomlKey      = regexp.MustCompile(`^([A-Za-z0-9_]+)\s*=`)
	illustrative = []string{"future_key", "extensions"} // the config_allow_unknown example, which is about unknown keys
)

// schemaFor maps a TOML table path to the schema its keys belong to; "" means the path is not one the command reads.
func schemaFor(path string) string {
	switch {
	case path == "":
		return ""
	case path == "project" || path == "claims" || path == "flow" || path == "preset" || path == "columns" || path == "types":
		return path
	case strings.HasPrefix(path, "steps."):
		return "step"
	case strings.HasPrefix(path, "schemas."):
		return "brief_schema"
	}
	return "?"
}

func TestEveryConfigKeyInTheDocsIsOneTheCommandAccepts(t *testing.T) {
	_, files := docFiles(t)
	keys := schemaKeys()
	checked := 0
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		for _, block := range tomlFence.FindAllStringSubmatch(string(data), -1) {
			if strings.Contains(block[1], "config_allow_unknown") {
				continue
			}
			schema, root := "", true
			for _, line := range strings.Split(block[1], "\n") {
				line = strings.TrimSpace(line)
				if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "...") || strings.HasPrefix(line, "…") {
					continue
				}
				if m := tomlTable.FindStringSubmatch(line); m != nil {
					root = false
					if schema = schemaFor(m[1]); schema == "?" {
						t.Errorf("%s: [%s] is not a table the command reads", filepath.Base(file), m[1])
					}
					continue
				}
				m := tomlKey.FindStringSubmatch(line)
				if m == nil {
					continue
				}
				key, table := m[1], schema
				if root {
					table = ""
				}
				if table == "?" {
					continue
				}
				if _, ok := keys[table][key]; !ok && !contains(illustrative, key) {
					t.Errorf("%s: %q in a [%s] example is not a key the command accepts (%s)", filepath.Base(file), key, table,
						strings.Join(sortedKeysString(keys[table]), ", "))
				}
				checked++
			}
		}
	}
	if checked < 50 {
		t.Fatalf("only %d documented keys were checked: the examples moved or the check lost its way", checked)
	}
}

func TestEveryConfigKeyTheCommandAcceptsIsDocumented(t *testing.T) {
	root, files := docFiles(t)
	var corpus strings.Builder
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		corpus.Write(data)
	}
	text := corpus.String()
	var missing []string
	for table, keys := range schemaKeys() {
		for key, kind := range keys {
			if kind == "table" || kind == "columns" || kind == "steps" || kind == "types" || kind == "schemas" {
				continue
			}
			if !strings.Contains(text, "`"+key+"`") && !regexp.MustCompile(`(?m)^\s*`+regexp.QuoteMeta(key)+`\s*=`).MatchString(text) {
				missing = append(missing, table+"."+key)
			}
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf("keys the command accepts but README.md and docs/*.md never name (%s): add them to docs/configuration.md\n  %s",
			root, strings.Join(missing, "\n  "))
	}
}
