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

func TestWorktreeCommandIsChecked(t *testing.T) {
	cases := []struct{ name, text, want string }{
		{"an unknown placeholder", "[project]\nworktree_command = \"start {id} {slug}\"\n", "{slug}"},
		{"a second line", "[project]\nworktree_command = \"start {id}\\nrm -rf x\"\n", "one line"},
		{"another isolation", "[project]\nworktree_command = \"start {id}\"\n[flow]\nisolation = \"branch\"\n", "isolation"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := loadError(t, testutil.Repo(t, tc.text))
			if err == nil {
				t.Fatal("the config loaded without an error")
			}
			requireContains(t, err.Error(), "worktree_command")
			requireContains(t, err.Error(), tc.want)
		})
	}
	project := loadProject(t, testutil.Repo(t, "[project]\nworktree_command = \"  start {id} {branch} {path} {base} {main}  \"\n"))
	if project.WorktreeCommand != "start {id} {branch} {path} {base} {main}" {
		t.Fatalf("WorktreeCommand = %q", project.WorktreeCommand)
	}
	if section := project.AsDict()["project"].(map[string]any); section["worktree_command"] != project.WorktreeCommand {
		t.Fatalf("config JSON has worktree_command = %v", section["worktree_command"])
	}
	if unset := loadProject(t, testutil.Repo(t, defaultConfig)); unset.WorktreeCommand != "" {
		t.Fatalf("an unset command = %q", unset.WorktreeCommand)
	}
}
