package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/cloudbusting/kanbanzai/internal/testutil"
)

func TestDefaultsMatchPythonTomllib(t *testing.T) {
	root, err := testutil.RepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(root, "internal", "config", "testdata", "defaults.json"))
	if err != nil {
		t.Fatal(err)
	}
	var want any
	if err := json.Unmarshal(raw, &want); err != nil {
		t.Fatal(err)
	}

	fromDisk, err := parseTOML(string(mustRead(t, filepath.Join(root, "defaults", "config.toml"))))
	if err != nil {
		t.Fatalf("defaults/config.toml: %v", err)
	}
	if got := normalize(fromDisk); !reflect.DeepEqual(got, want) {
		t.Fatalf("the parser's defaults differ from Python's tomllib:\n got: %#v\nwant: %#v", got, want)
	}

	embedded, err := readDefaults()
	if err != nil {
		t.Fatalf("embedded defaults: %v", err)
	}
	if got := normalize(embedded); !reflect.DeepEqual(got, want) {
		t.Fatalf("the embedded defaults differ from Python's tomllib:\n got: %#v\nwant: %#v", got, want)
	}
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// normalize makes TOML values and JSON-decoded values comparable: every number is a float64.
func normalize(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, item := range typed {
			out[key] = normalize(item)
		}
		return out
	case []any:
		out := make([]any, len(typed))
		for index, item := range typed {
			out[index] = normalize(item)
		}
		return out
	case int64:
		return float64(typed)
	case int:
		return float64(typed)
	default:
		return value
	}
}

func TestTOMLSubset(t *testing.T) {
	doc, err := parseTOML(`
# a comment
title = "a \u0041 \n \"quoted\""
literal = 'C:\path'
multi = """
line one
line two \
  continued
"""
arr = [1, 2, 3,]
inline = { a = 1, b = "two" }

[table]
x = true
y = 1.5
dotted.key = "value"

[[items]]
name = "one"

[[items]]
name = "two"
`)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := doc["title"], "a A \n \"quoted\""; got != want {
		t.Fatalf("title = %q, want %q", got, want)
	}
	if got, want := doc["literal"], `C:\path`; got != want {
		t.Fatalf("literal = %q, want %q", got, want)
	}
	if got, want := doc["multi"], "line one\nline two continued\n"; got != want {
		t.Fatalf("multi = %q, want %q", got, want)
	}
	if got, want := doc["arr"], []any{int64(1), int64(2), int64(3)}; !reflect.DeepEqual(got, want) {
		t.Fatalf("arr = %#v, want %#v", got, want)
	}
	inline, ok := doc["inline"].(map[string]any)
	if !ok || inline["a"] != int64(1) || inline["b"] != "two" {
		t.Fatalf("inline = %#v", doc["inline"])
	}
	table, ok := doc["table"].(map[string]any)
	if !ok || table["x"] != true || table["y"] != 1.5 {
		t.Fatalf("table = %#v", doc["table"])
	}
	dotted, ok := table["dotted"].(map[string]any)
	if !ok || dotted["key"] != "value" {
		t.Fatalf("dotted = %#v", table["dotted"])
	}
	items, ok := doc["items"].([]any)
	if !ok || len(items) != 2 {
		t.Fatalf("items = %#v", doc["items"])
	}
	if got, want := items[1].(map[string]any)["name"], "two"; got != want {
		t.Fatalf("items[1].name = %q, want %q", got, want)
	}
}

func TestTOMLErrors(t *testing.T) {
	cases := []struct {
		name string
		text string
		want string
	}{
		{"duplicate key", "a = 1\na = 2\n", "defined twice"},
		{"duplicate table", "[a]\nx = 1\n[a]\ny = 2\n", "defined twice"},
		{"bad value", "a = nope\n", "invalid value"},
		{"unterminated string", "a = \"x", "unterminated"},
		{"date", "a = 2026-11-11T00:00:00Z\n", "date and time"},
		{"missing equals", "a\n", "expected '='"},
		{"unterminated array", "a = [1, 2\n", "unterminated array"},
		{"unterminated inline table", "a = {b = 1\n", "unterminated inline table"},
		{"single-line newline", "a = \"one\ntwo\"\n", "cannot span lines"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseTOML(tc.text)
			if err == nil {
				t.Fatalf("parsed %q without an error", tc.text)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error %q does not contain %q", err, tc.want)
			}
		})
	}
}
