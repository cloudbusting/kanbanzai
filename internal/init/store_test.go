package initcmd

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cloudbusting/kanbanzai/internal/config"
)

// githubStore is the store flags the interview passes for the shipped GitHub plugin.
func githubStore(opts Options) Options {
	opts.Store = "github"
	opts.StoreSettings = []string{"repo=owner/name", "label=kanbanzai"}
	return opts
}

type storeReport struct {
	Plan struct {
		Settings struct {
			BoardHome   string `json:"board_home"`
			BoardReason string `json:"board_reason"`
			Store       *struct {
				Plugin    string            `json:"plugin"`
				Settings  map[string]string `json:"settings"`
				Query     string            `json:"query"`
				Source    string            `json:"source"`
				NextSteps []string          `json:"next_steps"`
			} `json:"store"`
		} `json:"settings"`
		Actions  []Action `json:"actions"`
		Warnings []string `json:"warnings"`
	} `json:"plan"`
}

func storePlan(t *testing.T, root string, opts Options) (storeReport, string) {
	t.Helper()
	opts.DryRun, opts.JSON = true, true
	code, out, errOut := initRun(root, opts)
	if code != 0 {
		t.Fatalf("dry run exit %d: %s", code, errOut)
	}
	var report storeReport
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatal(err)
	}
	return report, out
}

func configContent(t *testing.T, report storeReport) string {
	t.Helper()
	for _, action := range report.Plan.Actions {
		if action.What == WhatConfig {
			return action.Content
		}
	}
	t.Fatal("the plan writes no config")
	return ""
}

func TestStoreFlagsWriteTheStoreTablesAndKeepTheBoardOnMain(t *testing.T) {
	presets := append([]string{""}, shippedPresets(t)...)
	for _, name := range presets {
		root := repo(t, "main")
		// BoardMain is not passed: a store forces the board to main whatever the preset would have chosen.
		report, _ := storePlan(t, root, githubStore(Options{Preset: name, Owner: "Sam", BoardMain: false, BoardBranch: false}))
		text := configContent(t, report)
		for _, want := range []string{"[store]", `plugin = "github"`, "[store.settings]", `repo = "owner/name"`, `label = "kanbanzai"`} {
			if !strings.Contains(text, want) {
				t.Errorf("preset %q: the config does not say %q:\n%s", name, want, text)
			}
		}
		if strings.Contains(text, `home = "branch"`) {
			t.Errorf("preset %q: a store plan puts the board on a branch:\n%s", name, text)
		}
		settings := report.Plan.Settings
		if settings.BoardHome != "main" || !strings.Contains(settings.BoardReason, "github") {
			t.Errorf("preset %q: board home %q (%q): want main, naming the store", name, settings.BoardHome, settings.BoardReason)
		}
		if settings.Store == nil || settings.Store.Plugin != "github" || settings.Store.Settings["repo"] != "owner/name" ||
			settings.Store.Source != "--store" {
			t.Errorf("preset %q: plan.settings.store = %+v", name, settings.Store)
		}
	}
}

func TestStorePlanLoadsAndNamesTheOwnersNextSteps(t *testing.T) {
	root := repo(t, "main")
	report, _ := storePlan(t, root, githubStore(Options{Owner: "Sam"}))
	text := configContent(t, report)
	project, err := config.ValidateText(root, root, filepath.Join(root, ".git"), text)
	if err != nil {
		t.Fatalf("the config does not load: %v\n%s", err, text)
	}
	if project.StorePlugin != "github" || project.StoreSettings["repo"] != "owner/name" || project.BoardOnBranch() {
		t.Fatalf("loaded store = %q %v, board on branch %v", project.StorePlugin, project.StoreSettings, project.BoardOnBranch())
	}
	if report.Plan.Settings.Store == nil {
		t.Fatal("the plan has no store section")
	}
	steps := strings.Join(report.Plan.Settings.Store.NextSteps, "\n")
	for _, want := range []string{"gh auth login", "kanbanzai store trust", "kanbanzai sync"} {
		if !strings.Contains(steps, want) {
			t.Errorf("the next steps do not say %q:\n%s", want, steps)
		}
	}
	if strings.Index(steps, "kanbanzai store trust") > strings.Index(steps, "kanbanzai sync") {
		t.Errorf("trust must come before sync:\n%s", steps)
	}
	if !strings.Contains(strings.Join(report.Plan.Warnings, "\n"), "public") {
		t.Errorf("no warning that cards in a public repository are public: %v", report.Plan.Warnings)
	}
}

func TestStoreFilesIsTheDefaultPlanByteForByte(t *testing.T) {
	rootA, rootB := repo(t, "main"), repo(t, "main")
	_, plain := storePlan(t, rootA, Options{Owner: "Sam"})
	_, files := storePlan(t, rootB, Options{Owner: "Sam", Store: "files"})
	// the two roots differ only by the temporary folder's name
	plain = strings.ReplaceAll(plain, rootA, "ROOT")
	files = strings.ReplaceAll(files, rootB, "ROOT")
	if plain != files {
		t.Errorf("--store files changes the plan:\n%s\n---\n%s", plain, files)
	}
	if strings.Contains(plain, "[store]") {
		t.Errorf("the default plan writes [store]:\n%s", plain)
	}
}

func TestStoreQueryIsWrittenVerbatim(t *testing.T) {
	root := repo(t, "main")
	opts := githubStore(Options{Owner: "Sam", StoreQuery: `label:agent is:open, "x"`})
	report, _ := storePlan(t, root, opts)
	if text := configContent(t, report); !strings.Contains(text, `query = "label:agent is:open, \"x\""`) {
		t.Errorf("the query is not written as one TOML string:\n%s", text)
	}
}

func TestStoreRefusals(t *testing.T) {
	cases := []struct {
		name string
		opts Options
		want string
	}{
		{"github needs a repository", Options{Store: "github"}, "repo"},
		{"github repository must be owner/name", Options{Store: "github", StoreSettings: []string{"repo=nope"}}, "owner/name"},
		{"a path is not a plugin name", Options{Store: "../evil"}, "plugin name"},
		{"an argument is not a plugin name", Options{Store: "x --y"}, "plugin name"},
		{"an empty setting value is no setting", Options{Store: "other", StoreSettings: []string{"host="}}, "KEY=VALUE"},
		{"a credential key", Options{Store: "github", StoreSettings: []string{"repo=o/n", "token=abc"}}, "credential"},
		{"a password key in any case", Options{Store: "github", StoreSettings: []string{"repo=o/n", "Password=abc"}}, "credential"},
		{"settings without a store", Options{StoreSettings: []string{"repo=o/n"}}, "--store"},
		{"a query without a store", Options{StoreQuery: "x"}, "--store"},
		{"settings with the files store", Options{Store: "files", StoreSettings: []string{"repo=o/n"}}, "files store"},
		{"a board branch with a store", githubStore(Options{BoardBranch: true}), "board branch"},
	}
	for _, c := range cases {
		root := repo(t, "main")
		before := tree(t, root)
		c.opts.Owner = "Sam"
		c.opts.DryRun = true
		code, _, errOut := initRun(root, c.opts)
		if code != 2 || !strings.Contains(errOut, c.want) {
			t.Errorf("%s: exit %d, stderr %q: want exit 2 naming %q", c.name, code, errOut, c.want)
		}
		if after := tree(t, root); len(after) != len(before) {
			t.Errorf("%s: a refusal wrote files", c.name)
		}
	}
}

func TestStoreHasNoEffectOnAnExistingConfig(t *testing.T) {
	root := repo(t, "main")
	if code, _, errOut := initRun(root, Options{Owner: "Sam"}); code != 0 {
		t.Fatalf("first init: %s", errOut)
	}
	code, _, errOut := initRun(root, githubStore(Options{DryRun: true}))
	if code != 2 || !strings.Contains(errOut, "has no effect on the existing") || !strings.Contains(errOut, "docs/store-plugins.md") {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
}

func TestStorePlanAppliesAndVerifyNamesTheOwnersSteps(t *testing.T) {
	root := repo(t, "main")
	code, out, errOut := initRun(root, githubStore(Options{Owner: "Sam", JSON: true}))
	if code != 0 {
		t.Fatalf("init exit %d\n%s%s", code, out, errOut)
	}
	var report struct {
		Verify struct {
			Check struct {
				OK     bool   `json:"ok"`
				Output string `json:"output"`
			} `json:"check"`
			NextActions []string `json:"next_actions"`
		} `json:"verify"`
	}
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatal(err)
	}
	// check reads the working copy, so it passes before the plugin is trusted; the owner's steps are what is left
	if !report.Verify.Check.OK {
		t.Errorf("verify check = %+v: want ok for a store that is not trusted yet", report.Verify.Check)
	}
	actions := strings.Join(report.Verify.NextActions, "\n")
	for _, want := range []string{"gh auth login", "kanbanzai store trust", "kanbanzai sync"} {
		if !strings.Contains(actions, want) {
			t.Errorf("next actions do not say %q:\n%s", want, actions)
		}
	}
	data, err := os.ReadFile(filepath.Join(root, ".kanbanzai", ".gitignore"))
	if err != nil || !strings.Contains(string(data), "state") {
		t.Errorf("the working copy is not git-ignored: %q %v", data, err)
	}
	testutilIgnored(t, root, ".kanbanzai/state/store/work/1-open/X.md")
}

// testutilIgnored fails the test unless git ignores the path.
func testutilIgnored(t *testing.T, root, path string) {
	t.Helper()
	command := exec.Command("git", "check-ignore", "-q", path)
	command.Dir = root
	if err := command.Run(); err != nil {
		t.Errorf("git does not ignore %s: %v", path, err)
	}
}

func TestStoreIsInTheTextPlanToo(t *testing.T) {
	root := repo(t, "main")
	code, out, errOut := initRun(root, githubStore(Options{Owner: "Sam", DryRun: true}))
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	for _, want := range []string{"store:       github keeps the cards", "kanbanzai store trust", "note: GitHub issues"} {
		if !strings.Contains(out, want) {
			t.Errorf("the plan does not say %q:\n%s", want, out)
		}
	}
}
