package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cloudbusting/kanbanzai/internal/testutil"
)

// W-01: a release is published only when the tag, every manifest and the changelog carry the same version.

func TestTheVersionCheckReadsTheRepository(t *testing.T) {
	root, err := testutil.RepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	version, err := Check(root, "")
	if err != nil {
		t.Fatalf("the repository does not pass its own check: %v", err)
	}
	plugin := readFixtureVersion(t, filepath.Join(root, ".claude-plugin", "plugin.json"))
	if version != plugin {
		t.Fatalf("Check = %q, want the manifest's %q", version, plugin)
	}
	if _, err := Check(root, "v"+version); err != nil {
		t.Fatalf("the tag v%s does not pass: %v", version, err)
	}
	if _, err := Check(root, "v0.0.0"); err == nil {
		t.Fatal("a tag the manifests do not carry passed")
	}
}

func TestTheVersionCheckRejectsDisagreeingFiles(t *testing.T) {
	cases := []struct {
		name      string
		plugin    string
		market    string
		entry     string
		packageV  string
		changelog string
		want      string
	}{
		{"marketplace version", "0.6.0", "0.6.1", "0.6.0", "0.6.0", "0.6.0", "marketplace.json"},
		{"marketplace entry", "0.6.0", "0.6.0", "0.6.1", "0.6.0", "0.6.0", "entry"},
		{"extension version", "0.6.0", "0.6.0", "0.6.0", "0.6.1", "0.6.0", "board-view/package.json"},
		{"changelog entry", "0.6.0", "0.6.0", "0.6.0", "0.6.0", "0.5.9", "CHANGELOG.md"},
		{"not semver", "0.6", "0.6", "0.6", "0.6", "0.6", "semver"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			root := writeVersionFixture(t, test.plugin, test.market, test.entry, test.packageV, test.changelog)
			_, err := Check(root, "")
			if err == nil {
				t.Fatal("the disagreeing files passed")
			}
			if !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %q, want it to name %q", err, test.want)
			}
		})
	}
}

func TestTheVersionCheckRejectsAMissingManifest(t *testing.T) {
	root := writeVersionFixture(t, "0.6.0", "0.6.0", "0.6.0", "0.6.0", "0.6.0")
	if err := os.Remove(filepath.Join(root, "board-view", "package.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := Check(root, ""); err == nil {
		t.Fatal("a missing manifest passed")
	}
}

func TestTheReleaseCheckRejectsMachineSpecificBoardRecords(t *testing.T) {
	for _, record := range []string{
		"worktree /home/alice/project/W-01",
		"worktree /Users/alice/project/W-01",
		`worktree C:\Users\alice\project\W-01`,
		"owner opencode:ses_1234567890abcdef",
	} {
		t.Run(record, func(t *testing.T) {
			root := writeVersionFixture(t, "0.6.0", "0.6.0", "0.6.0", "0.6.0", "0.6.0")
			path := filepath.Join(root, ".kanbanzai", "work", "10-done", "W-01.md")
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte("# W-01\n"+record+"\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			_, err := Check(root, "")
			if err == nil || !strings.Contains(err.Error(), "W-01.md") || !strings.Contains(err.Error(), ":2") {
				t.Fatalf("Check = %v, want a finding naming W-01.md:2", err)
			}
		})
	}
}

func TestTheReleaseCheckAllowsFictionalAndRelativeRecords(t *testing.T) {
	root := writeVersionFixture(t, "0.6.0", "0.6.0", "0.6.0", "0.6.0", "0.6.0")
	path := filepath.Join(root, ".kanbanzai", "work", "batches", "B-01.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	text := "worktree W-01\nhttps://example.org/home/alice/project\n/home/example/repo\n/Users/example/repo\nC:\\Users\\example\\repo\nses_example\n"
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Check(root, ""); err != nil {
		t.Fatalf("fictional and relative values should pass: %v", err)
	}
}

func writeVersionFixture(t *testing.T, plugin, market, entry, packageVersion, changelog string) string {
	t.Helper()
	root := t.TempDir()
	write := func(relative, text string) {
		t.Helper()
		path := filepath.Join(root, relative)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(".claude-plugin/plugin.json", `{"name":"kanbanzai","version":"`+plugin+`"}`+"\n")
	write(".claude-plugin/marketplace.json", `{"name":"kanbanzai","version":"`+market+`","owner":{"name":"cloudbusting"},"plugins":[{"name":"kanbanzai","source":"./","version":"`+entry+`"}]}`+"\n")
	write("board-view/package.json", `{"name":"kanbanzai-board","version":"`+packageVersion+`"}`+"\n")
	write("CHANGELOG.md", "# Changelog\n\n## ["+changelog+"] — 2026-11-11\n\n- A change.\n")
	return root
}

func readFixtureVersion(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	return manifest.Version
}
