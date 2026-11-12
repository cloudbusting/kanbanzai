package config

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/cloudbusting/kanbanzai/internal/testutil"
)

func TestIsolationDefaultsToWorktree(t *testing.T) {
	project := loadProject(t, testutil.Repo(t, defaultConfig))
	if project.Isolation != "worktree" {
		t.Fatalf("isolation = %q, want worktree", project.Isolation)
	}
}

func TestIsolationModesLoad(t *testing.T) {
	for _, mode := range []string{"worktree", "branch", "shared", "external"} {
		project := loadProject(t, testutil.Repo(t, defaultConfig+"\n[flow]\nisolation = \""+mode+"\"\n"))
		if project.Isolation != mode {
			t.Fatalf("isolation = %q, want %q", project.Isolation, mode)
		}
	}
}

func TestIsolationRejectsAnythingElse(t *testing.T) {
	for _, value := range []string{`"cloud"`, `""`, `"Worktree"`, `3`, `true`} {
		err := loadError(t, testutil.Repo(t, defaultConfig+"\n[flow]\nisolation = "+value+"\n"))
		if err == nil {
			t.Fatalf("isolation = %s loaded", value)
		}
		for _, name := range []string{"worktree", "branch", "shared", "external", "flow.isolation"} {
			if !strings.Contains(err.Error(), name) {
				t.Fatalf("isolation = %s: error %q does not name %q", value, err, name)
			}
		}
	}
}

func TestIsolationIsInTheConfigDictionary(t *testing.T) {
	project := loadProject(t, testutil.Repo(t, defaultConfig+"\n[flow]\nisolation = \"shared\"\n"))
	raw, err := json.Marshal(project.AsDict())
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	flow := decoded["flow"].(map[string]any)
	if flow["isolation"] != "shared" {
		t.Fatalf("flow = %#v", flow)
	}
}
