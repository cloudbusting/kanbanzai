package config

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/cloudbusting/kanbanzai/internal/testutil"
)

const typedConfig = `
[project]
owner = "Sam"
default_type = "task"
project_key = "KANZ"
digits = 2

[[types]]
key = "task"
label = "Task"
prefix = "TASK"

[[types]]
key = "bug"
label = "Bug"
prefix = "BUG"
colour = "#C43B3B"
icon = "bug"
description = "Unexpected behaviour"

[[types]]
key = "change"
label = "Change"
prefix = "CHANGE"
`

func TestTypedConfigResolves(t *testing.T) {
	project := loadProject(t, testutil.Repo(t, typedConfig))
	if !project.Typed {
		t.Fatal("typed config did not enable typed ids")
	}
	if project.Prefix != "W" {
		t.Fatalf("prefix = %q, want the inherited legacy prefix W", project.Prefix)
	}
	if project.DefaultType != "task" || project.ProjectKey != "KANZ" {
		t.Fatalf("default = %q, project key = %q", project.DefaultType, project.ProjectKey)
	}
	if len(project.Types) != 3 {
		t.Fatalf("types = %d, want 3", len(project.Types))
	}
	if got := project.IDsExample(); got != "KANZ-TASK-NN" {
		t.Fatalf("IDsExample = %q", got)
	}
	if got := project.FormatID(5); got != "KANZ-TASK-05" {
		t.Fatalf("FormatID = %q", got)
	}
	if got, ok := project.CanonicalCardID("kanz-bug-7"); !ok || got != "KANZ-BUG-07" {
		t.Fatalf("CanonicalCardID = %q, %v", got, ok)
	}
	if number, ok := project.CardNumber("KANZ-BUG-42"); !ok || number != 42 {
		t.Fatalf("CardNumber = %d, %v", number, ok)
	}
	for _, id := range []string{"KANZ-TASK-01", "KANZ-BUG-42", "KANZ-CHANGE-123", "W-01"} {
		if !project.IsCardID(id) {
			t.Fatalf("IsCardID(%q) = false", id)
		}
	}
	for _, id := range []string{"TASK-01", "KANZ-DEF-01", "KANZ-BUG-1"} {
		if project.IsCardID(id) {
			t.Fatalf("IsCardID(%q) = true", id)
		}
	}
	if got, ok := project.CanonicalCardID("w-7"); !ok || got != "W-07" {
		t.Fatalf("legacy CanonicalCardID = %q, %v", got, ok)
	}
	if project.CardTypeInfo("W-07") != nil {
		t.Fatal("a legacy card got a type")
	}
	info := project.CardTypeInfo("KANZ-BUG-42")
	if info == nil || info.Key != "bug" || info.Colour != "#C43B3B" || info.Icon != "bug" {
		t.Fatalf("CardTypeInfo = %+v", info)
	}
	if project.CardTypeInfo("KANZ-DEF-42") != nil {
		t.Fatal("an unknown prefix got a type")
	}
}

func TestTypedConfigJSONExposesTypes(t *testing.T) {
	project := loadProject(t, testutil.Repo(t, typedConfig))
	dict := project.AsDict()
	section, ok := dict["project"].(map[string]any)
	if !ok {
		t.Fatalf("project section = %T", dict["project"])
	}
	if section["typed"] != true || section["default_type"] != "task" || section["project_key"] != "KANZ" {
		t.Fatalf("project JSON = %v", section)
	}
	types, ok := section["types"].([]any)
	if !ok || len(types) != 3 {
		t.Fatalf("types JSON = %T %v", section["types"], section["types"])
	}
	first, _ := types[0].(map[string]any)
	if first["key"] != "task" || first["prefix"] != "TASK" || first["label"] != "Task" {
		t.Fatalf("first type = %v", first)
	}
}

func TestLegacyConfigStaysSinglePrefix(t *testing.T) {
	project := loadProject(t, testutil.Repo(t, "types = []\n[project]\nowner = \"Sam\"\n"))
	if project.Typed {
		t.Fatal("a config with types = [] became typed")
	}
	if project.Prefix != "W" {
		t.Fatalf("prefix = %q, want W", project.Prefix)
	}
	if got := project.IDsExample(); got != "W-NN" {
		t.Fatalf("IDsExample = %q", got)
	}
	if got, ok := project.CanonicalCardID("w-7"); !ok || got != "W-07" {
		t.Fatalf("CanonicalCardID = %q, %v", got, ok)
	}
	if project.CardTypeInfo("W-07") != nil {
		t.Fatal("a legacy card got a type")
	}
	section := project.AsDict()["project"].(map[string]any)
	if section["typed"] != false {
		t.Fatalf("typed = %v", section["typed"])
	}
	if types, ok := section["types"].([]any); !ok || len(types) != 0 {
		t.Fatalf("legacy types = %v", section["types"])
	}
}

func TestTypedConfigErrors(t *testing.T) {
	cases := []struct {
		name string
		text string
		want string
	}{
		{"duplicate prefix", typedConfig + "\n[[types]]\nkey = \"defect\"\nlabel = \"Defect\"\nprefix = \"BUG\"\n", "share the prefix"},
		{"bad default", strings.Replace(typedConfig, `default_type = "task"`, `default_type = "nope"`, 1), "default_type names"},
		{"bad project key", strings.Replace(typedConfig, `project_key = "KANZ"`, `project_key = "kanz"`, 1), "project_key"},
		{"bad colour", typedConfig + "\n[[types]]\nkey = \"chore\"\nlabel = \"Chore\"\nprefix = \"CHORE\"\ncolour = \"red\"\n", "colour must be"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := loadError(t, testutil.Repo(t, tc.text))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestTheShippedDefaultsAreTyped(t *testing.T) {
	project := loadProject(t, testutil.Repo(t, ""))
	if !project.Typed || project.DefaultType != "task" || len(project.Types) != 6 {
		t.Fatalf("typed = %v, default = %q, types = %d", project.Typed, project.DefaultType, len(project.Types))
	}
	if project.Prefix != "W" {
		t.Fatalf("legacy prefix = %q, want W", project.Prefix)
	}
	if got, ok := project.CanonicalCardID("w-7"); !ok || got != "W-07" {
		t.Fatalf("legacy id = %q, %v", got, ok)
	}
}

const branchPrefixConfig = `
[project]
owner = "Sam"
default_type = "task"
branch = "{branch_prefix}{id}"
worktree = "../{type}/{id}"

[[types]]
key = "task"
label = "Task"
prefix = "TASK"

[[types]]
key = "change"
label = "Change"
prefix = "CHANGE"
branch_prefix = "feat/"
`

// A card's branch and worktree can start with its type: {type} is the type key and {branch_prefix} the type's own
// prefix (empty when it has none), both worked out from the card id.
func TestBranchAndWorktreePatternsTakeTheType(t *testing.T) {
	project := loadProject(t, testutil.Repo(t, branchPrefixConfig))
	if got := project.Branch("CHANGE-03", false); got != "feat/CHANGE-03" {
		t.Fatalf("branch of a change = %q, want feat/CHANGE-03", got)
	}
	if got := project.Branch("change-03", false); got != "feat/change-03" {
		t.Fatalf("branch of a lower-case id = %q", got)
	}
	if got := project.Branch("TASK-04", false); got != "TASK-04" {
		t.Fatalf("a type with no branch_prefix gives the plain id, got %q", got)
	}
	if got := project.WorktreeHint("CHANGE-03"); got != "../change/CHANGE-03" {
		t.Fatalf("worktree hint = %q, want ../change/CHANGE-03", got)
	}
	if got := project.WorktreeHint("W-03"); got != "../W-03" {
		t.Fatalf("a legacy card has no type, so {type} is empty; got %q", got)
	}
	if got := project.Branch("batch-1", true); got != "batch/batch-1" {
		t.Fatalf("batch branch = %q", got)
	}
	if got, _ := project.Expand("{branch_prefix}{id}", ""); got != "{branch_prefix}{id}" {
		t.Fatalf("with no card the placeholders stay as written, got %q", got)
	}
}

func TestBranchPrefixIsInTheTypeJSON(t *testing.T) {
	project := loadProject(t, testutil.Repo(t, branchPrefixConfig))
	section := project.AsDict()["project"].(map[string]any)
	found := false
	for _, entry := range section["types"].([]any) {
		item := entry.(map[string]any)
		if item["key"] == "change" {
			found = item["branch_prefix"] == "feat/"
		} else if _, has := item["branch_prefix"]; has {
			t.Fatalf("a type with no branch_prefix shows one: %v", item)
		}
	}
	if !found {
		t.Fatalf("the change type's branch_prefix is not in the JSON: %v", section["types"])
	}
}

func TestBranchAndWorktreePlaceholdersAreChecked(t *testing.T) {
	typed := func(project string, prefix string) string {
		return "[project]\nowner = \"Sam\"\ndefault_type = \"task\"\n" + project +
			"\n[[types]]\nkey = \"task\"\nlabel = \"Task\"\nprefix = \"TASK\"\n" + prefix + "\n"
	}
	cases := []struct {
		name, text, want string
	}{
		{"a type in the batch branch", typed(`batch_branch = "{type}/{id}"`, ""), "batch_branch"},
		{"a branch prefix in the batch branch", typed(`batch_branch = "{branch_prefix}{id}"`, ""), "batch_branch"},
		{"a branch prefix in the worktree", typed(`worktree = "../{branch_prefix}{id}"`, ""), "worktree"},
		{"an unknown placeholder in the branch", typed(`branch = "{tpye}/{id}"`, ""), "{tpye}"},
		{"an unknown placeholder in the worktree", typed(`worktree = "../{slug}/{id}"`, ""), "{slug}"},
		{"a type in the last part of the worktree", typed(`worktree = "../{type}-{id}"`, ""), "folder"},
		{"a branch prefix with a space", typed("", "branch_prefix = \"feat x/\""), "branch_prefix"},
		{"a branch prefix with two dots", typed("", "branch_prefix = \"a..b/\""), "branch_prefix"},
		{"a branch prefix with a colon", typed("", "branch_prefix = \"a:/\""), "branch_prefix"},
		{"a branch prefix that starts with a slash", typed("", "branch_prefix = \"/feat/\""), "branch_prefix"},
		{"a branch prefix that starts with a dash", typed("", "branch_prefix = \"-feat/\""), "branch_prefix"},
		{"a branch prefix that is not text", typed("", "branch_prefix = 4"), "branch_prefix"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := loadError(t, testutil.Repo(t, tc.text))
			if err == nil {
				t.Fatal("the config loaded without an error")
			}
			requireContains(t, err.Error(), tc.want)
		})
	}
}

func TestPlaceholdersAreUnchangedForAProjectThatUsesNone(t *testing.T) {
	project := loadProject(t, testutil.Repo(t, typedConfig))
	if got := project.Branch("KANZ-BUG-07", false); got != "KANZ-BUG-07" {
		t.Fatalf("branch = %q", got)
	}
	if got := project.WorktreeHint("KANZ-BUG-07"); got != "../worktrees/"+filepath.Base(project.Root)+"/KANZ-BUG-07" {
		t.Fatalf("worktree hint = %q", got)
	}
}
