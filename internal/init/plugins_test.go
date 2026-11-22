package initcmd

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/cloudbusting/kanbanzai/internal/testutil"
)

// W-17: `init --plugins` puts the project's Claude Code and OpenCode plugin settings in the plan, as `merge`
// actions that carry only the JSON that is missing, guarded by a digest of the file they merge into.

func dryPlan(t *testing.T, root string, opts Options) *Plan {
	t.Helper()
	opts.DryRun, opts.JSON = true, true
	code, out, errOut := initRun(root, opts)
	if code != 0 {
		t.Fatalf("dry run = %d\n%s%s", code, out, errOut)
	}
	var report Report
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatal(err)
	}
	return report.Plan
}

func actionAt(plan *Plan, path string) *Action {
	for i := range plan.Actions {
		if plan.Actions[i].Path == path {
			return &plan.Actions[i]
		}
	}
	return nil
}

func readJSON(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var value map[string]any
	if err := json.Unmarshal(data, &value); err != nil {
		t.Fatalf("%s is not JSON: %v\n%s", path, err, data)
	}
	return value
}

// pluginCheckout is a directory that looks like a Kanbanzai checkout, for --plugin-source.
func pluginCheckout(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	testutil.Write(t, filepath.Join(dir, ".claude-plugin", "plugin.json"), "{}")
	testutil.Write(t, filepath.Join(dir, "opencode", "server.js"), "export default {}\n")
	return dir
}

func installedBundle(t *testing.T) {
	t.Helper()
	source, err := bundledPluginSource()
	if err != nil {
		t.Fatal(err)
	}
	testutil.Write(t, filepath.Join(source, ".claude-plugin", "plugin.json"), "{}")
	testutil.Write(t, filepath.Join(source, "opencode", "server.js"), "export default {}\n")
}

func TestSelectedPluginsDefaultToBundled(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "data"))
	t.Setenv("LOCALAPPDATA", filepath.Join(home, "data"))
	t.Setenv("USERPROFILE", home)
	for _, agents := range [][]string{{"opencode"}, {"claude"}, {"claude", "opencode"}} {
		t.Run(strings.Join(agents, ","), func(t *testing.T) {
			root := repo(t, "main")
			before := tree(t, root)
			implicit := dryPlan(t, root, Options{Plugins: agents})
			explicit := dryPlan(t, root, Options{Plugins: agents, PluginSource: "@bundled"})
			if !reflect.DeepEqual(implicit, explicit) {
				t.Fatalf("default plan differs from bundled plan:\nimplicit: %+v\nexplicit: %+v", implicit, explicit)
			}
			if !reflect.DeepEqual(before, tree(t, root)) {
				t.Fatal("dry run changed the project")
			}
		})
	}
}

func TestDefaultBundleMustBeInstalledBeforeApply(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "data"))
	t.Setenv("LOCALAPPDATA", filepath.Join(home, "data"))
	t.Setenv("USERPROFILE", home)
	root := repo(t, "main")
	before := tree(t, root)
	plan := dryPlan(t, root, Options{Plugins: []string{"opencode"}})
	data, _ := json.Marshal(&Report{SchemaVersion: 1, Mode: "dry-run", Plan: plan})
	file := filepath.Join(t.TempDir(), "plan.json")
	if err := os.WriteFile(file, data, 0o600); err != nil {
		t.Fatal(err)
	}
	code, _, errOut := initRun(root, Options{Plugins: []string{"opencode"}})
	if code != 2 || !strings.Contains(errOut, "kanbanzai install") {
		t.Fatalf("missing bundle: code=%d error=%q", code, errOut)
	}
	if !reflect.DeepEqual(before, tree(t, root)) {
		t.Fatal("missing bundle changed the project")
	}
	if code, _, errOut := initRun(root, Options{PlanFile: file}); code != 2 || !strings.Contains(errOut, "kanbanzai install") {
		t.Fatalf("saved plan with missing bundle: code=%d error=%q", code, errOut)
	}
	if !reflect.DeepEqual(before, tree(t, root)) {
		t.Fatal("missing bundle saved plan changed the project")
	}
}

func TestDefaultOpenCodeInitIsIdempotent(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "data"))
	t.Setenv("LOCALAPPDATA", filepath.Join(home, "data"))
	t.Setenv("USERPROFILE", home)
	installedBundle(t)
	root := repo(t, "main")
	opts := Options{Plugins: []string{"opencode"}}
	if code, out, errOut := initRun(root, opts); code != 0 {
		t.Fatalf("init = %d\n%s%s", code, out, errOut)
	}
	commitAll(t, root)
	before := tree(t, root)
	if code, out, errOut := initRun(root, opts); code != 0 {
		t.Fatalf("second init = %d\n%s%s", code, out, errOut)
	}
	if !reflect.DeepEqual(before, tree(t, root)) {
		t.Fatal("a second init changed the project")
	}
}

func TestPluginsAddTheClaudeSettingsToAFreshProject(t *testing.T) {
	root := repo(t, "main")
	checkout := pluginCheckout(t)
	before := tree(t, root)
	plan := dryPlan(t, root, Options{Plugins: []string{"claude"}, PluginSource: checkout})
	if !reflect.DeepEqual(before, tree(t, root)) {
		t.Fatal("the dry run changed the tree")
	}
	action := actionAt(plan, claudeLocalSettingsRel)
	if action == nil || action.Op != OpMerge || action.What != WhatPlugin || action.ExpectSHA256 != "" {
		t.Fatalf("the plan has no merge for %s: %+v", claudeLocalSettingsRel, action)
	}
	for _, want := range []string{`"extraKnownMarketplaces"`, checkout, `"directory"`, `"kanbanzai@kanbanzai"`} {
		if !strings.Contains(action.Content, want) {
			t.Errorf("the fragment lacks %s:\n%s", want, action.Content)
		}
	}
	if actionAt(plan, ".opencode/opencode.json") != nil {
		t.Error("claude only was asked for, but the plan touches OpenCode")
	}
	if code, out, errOut := initRun(root, Options{Plugins: []string{"claude"}, PluginSource: checkout}); code != 0 {
		t.Fatalf("init = %d\n%s%s", code, out, errOut)
	}
	settings := readJSON(t, filepath.Join(root, filepath.FromSlash(claudeLocalSettingsRel)))
	enabled, _ := settings["enabledPlugins"].(map[string]any)
	if enabled["kanbanzai@kanbanzai"] != true {
		t.Fatalf("settings = %v", settings)
	}
	source := settings["extraKnownMarketplaces"].(map[string]any)["kanbanzai"].(map[string]any)["source"].(map[string]any)
	if source["source"] != "directory" {
		t.Fatalf("source = %v", source)
	}
}

func TestBundledPluginsArePlannedWithoutCheckout(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "data"))
	t.Setenv("LOCALAPPDATA", filepath.Join(home, "data"))
	t.Setenv("USERPROFILE", home)
	root := repo(t, "main")
	installedBundle(t)
	plan := dryPlan(t, root, Options{Plugins: []string{"claude", "opencode"}, PluginSource: "@bundled"})
	serialized, _ := json.Marshal(plan)
	var record map[string]any
	if err := json.Unmarshal(serialized, &record); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(serialized), `"bundled_plugins":["claude","opencode"]`) {
		t.Fatalf("plan does not identify selected bundled plugins: %s", serialized)
	}
	var claude, opencode, localReference bool
	for _, action := range plan.Actions {
		switch action.Path {
		case ".claude/settings.local.json":
			claude = strings.Contains(action.Content, `"directory"`)
		case ".opencode/opencode.json":
			opencode = strings.Contains(action.Content, "{file:../.kanbanzai/state/kanbanzai-opencode-path}")
		case ".kanbanzai/state/kanbanzai-opencode-path":
			localReference = strings.Contains(filepath.ToSlash(action.Content), "/kanbanzai/plugins/current/opencode")
		}
	}
	if !claude || !opencode || !localReference {
		t.Fatalf("bundled plan missing local settings: claude=%v opencode=%v reference=%v\n%+v", claude, opencode, localReference, plan.Actions)
	}
	detected, _, err := Detect(root)
	if err != nil {
		t.Fatal(err)
	}
	original, _ := json.Marshal(plan)
	for i := range plan.Actions {
		if plan.Actions[i].Path == claudeLocalSettingsRel && plan.Actions[i].Op == OpMerge {
			var settings map[string]any
			if err := json.Unmarshal([]byte(plan.Actions[i].Content), &settings); err != nil {
				t.Fatal(err)
			}
			market := settings["extraKnownMarketplaces"].(map[string]any)["kanbanzai"].(map[string]any)
			market["source"].(map[string]any)["path"] = filepath.Join(home, "attacker")
			content, _ := json.Marshal(settings)
			plan.Actions[i].Content = string(content)
			plan.Actions[i].SHA256 = digest(string(content))
			break
		}
	}
	if err := ValidatePlan(plan, detected); err == nil {
		t.Fatal("tampered bundled Claude source was accepted")
	}
	if err := json.Unmarshal(original, plan); err != nil {
		t.Fatal(err)
	}
	for i := range plan.Actions {
		if plan.Actions[i].What == WhatPluginReference {
			plan.Actions[i].Content = filepath.Join(home, "attacker", "opencode")
			plan.Actions[i].SHA256 = digest(plan.Actions[i].Content)
			break
		}
	}
	if err := ValidatePlan(plan, detected); err == nil {
		t.Fatal("tampered bundled plugin reference was accepted")
	}
	opts := Options{Plugins: []string{"claude", "opencode"}, PluginSource: "@bundled"}
	if code, out, errOut := initRun(root, opts); code != 0 {
		t.Fatalf("init failed %d: %s %s", code, out, errOut)
	}
	for _, file := range []string{pluginReferenceRel, claudeLocalSettingsRel} {
		cmd := exec.Command("git", "-C", root, "check-ignore", "-q", file)
		if err := cmd.Run(); err != nil {
			t.Errorf("machine-local file %s is not gitignored: %v", file, err)
		}
	}
}

func TestBundledPlanRefusesTrackedMachineLocalPaths(t *testing.T) {
	for _, tracked := range []string{pluginReferenceRel, claudeLocalSettingsRel} {
		t.Run(tracked, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("XDG_DATA_HOME", filepath.Join(home, "data"))
			t.Setenv("LOCALAPPDATA", filepath.Join(home, "data"))
			t.Setenv("USERPROFILE", home)
			root := repo(t, "main")
			installedBundle(t)
			opts := Options{Plugins: []string{"claude", "opencode"}, PluginSource: "@bundled"}
			if code, _, errOut := initRun(root, opts); code != 0 {
				t.Fatalf("initial init %d: %s", code, errOut)
			}
			cmd := exec.Command("git", "-C", root, "add", "-f", "--", tracked)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("track local file: %v %s", err, out)
			}
			if code, _, _ := initRun(root, opts); code == 0 {
				t.Fatal("accepted a tracked machine-local file")
			}
		})
	}
}

func TestBundledPlanDoesNotInstallDisabledClaudePlugin(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "data"))
	t.Setenv("LOCALAPPDATA", filepath.Join(home, "data"))
	t.Setenv("USERPROFILE", home)
	root := repo(t, "main")
	testutil.Write(t, filepath.Join(root, ".claude", "settings.json"), `{"enabledPlugins":{"kanbanzai@kanbanzai":false}}`)
	plan := dryPlan(t, root, Options{Plugins: []string{"claude", "opencode"}, PluginSource: "@bundled"})
	data, _ := json.Marshal(plan)
	if strings.Contains(string(data), `"bundled_plugins":["claude"`) {
		t.Fatal("disabled Claude plugin was selected for installation")
	}
	for _, action := range plan.Actions {
		if action.Path == claudeLocalSettingsRel && action.Op == OpMerge {
			t.Fatal("plan would override a disabled Claude plugin")
		}
	}
}

func TestBundledPlanRefusesLocalPathTrackedAfterPreview(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "data"))
	t.Setenv("LOCALAPPDATA", filepath.Join(home, "data"))
	t.Setenv("USERPROFILE", home)
	installedBundle(t)
	root := repo(t, "main")
	opts := Options{Plugins: []string{"claude", "opencode"}, PluginSource: "@bundled"}
	if code, _, errOut := initRun(root, opts); code != 0 {
		t.Fatalf("initial init %d: %s", code, errOut)
	}
	plan := dryPlan(t, root, opts)
	data, _ := json.Marshal(plan)
	file := filepath.Join(t.TempDir(), "plan.json")
	if err := os.WriteFile(file, data, 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "-C", root, "add", "-f", "--", pluginReferenceRel)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("track local file: %v %s", err, out)
	}
	if code, _, _ := initRun(root, Options{PlanFile: file}); code == 0 {
		t.Fatal("applied a plan after its machine-local file became tracked")
	}
}

func TestPluginsMergeIntoExistingSettingsAndKeepEverythingElse(t *testing.T) {
	root := repo(t, "main")
	source := pluginCheckout(t)
	mine := `{"permissions":{"allow":["Bash(ls)"]},"enabledPlugins":{"other@market":true},"model":"x","n":12345678901234567890}`
	testutil.Write(t, filepath.Join(root, filepath.FromSlash(claudeLocalSettingsRel)), mine)
	commitAll(t, root)
	plan := dryPlan(t, root, Options{Plugins: []string{"claude"}, PluginSource: source})
	action := actionAt(plan, claudeLocalSettingsRel)
	if action == nil || action.Op != OpMerge || action.ExpectSHA256 != digest(mine) {
		t.Fatalf("merge = %+v", action)
	}
	if strings.Contains(action.Content, "permissions") || strings.Contains(action.Content, "other@market") {
		t.Errorf("the fragment must carry only what is missing:\n%s", action.Content)
	}
	if code, out, errOut := initRun(root, Options{Plugins: []string{"claude"}, PluginSource: source}); code != 0 {
		t.Fatalf("init = %d\n%s%s", code, out, errOut)
	}
	settings := readJSON(t, filepath.Join(root, filepath.FromSlash(claudeLocalSettingsRel)))
	if settings["model"] != "x" || settings["permissions"] == nil {
		t.Fatalf("another setting was lost: %v", settings)
	}
	enabled := settings["enabledPlugins"].(map[string]any)
	if enabled["other@market"] != true || enabled["kanbanzai@kanbanzai"] != true {
		t.Fatalf("enabledPlugins = %v", enabled)
	}
	text, _ := os.ReadFile(filepath.Join(root, filepath.FromSlash(claudeLocalSettingsRel)))
	if !strings.Contains(string(text), "12345678901234567890") {
		t.Errorf("a large number was rewritten:\n%s", text)
	}
}

func TestPluginsAreIdempotent(t *testing.T) {
	root := repo(t, "main")
	opts := Options{Plugins: []string{"claude", "opencode"}, PluginSource: pluginCheckout(t)}
	if code, out, errOut := initRun(root, opts); code != 0 {
		t.Fatalf("init = %d\n%s%s", code, out, errOut)
	}
	commitAll(t, root)
	before := tree(t, root)
	plan := dryPlan(t, root, opts)
	for _, action := range plan.Actions {
		if action.What == WhatPlugin && action.Op != OpKeep {
			t.Errorf("a second plan still wants to change %s (%s)", action.Path, action.Op)
		}
	}
	if code, out, errOut := initRun(root, opts); code != 0 {
		t.Fatalf("second init = %d\n%s%s", code, out, errOut)
	}
	if !reflect.DeepEqual(before, tree(t, root)) {
		t.Fatal("a second run changed the tree")
	}
}

func TestPluginSourceUsesALocalDirectoryAndTheLocalSettingsFile(t *testing.T) {
	root := repo(t, "main")
	source := pluginCheckout(t)
	plan := dryPlan(t, root, Options{Plugins: []string{"claude", "opencode"}, PluginSource: source})
	if actionAt(plan, ".claude/settings.json") != nil {
		t.Error("a machine path must not go into the committed .claude/settings.json")
	}
	local := actionAt(plan, ".claude/settings.local.json")
	if local == nil || !strings.Contains(local.Content, `"directory"`) {
		t.Fatalf("local settings = %+v", local)
	}
	var fragment struct {
		Extra map[string]struct {
			Source struct{ Path string }
		} `json:"extraKnownMarketplaces"`
	}
	if err := json.Unmarshal([]byte(local.Content), &fragment); err != nil || fragment.Extra["kanbanzai"].Source.Path != source {
		t.Fatalf("the directory source is not the checkout (%q): %v\n%s", source, err, local.Content)
	}
	opencode := actionAt(plan, ".opencode/opencode.json")
	if opencode == nil || !strings.Contains(opencode.Content, "plugins") {
		t.Fatalf("opencode = %+v", opencode)
	}
	warned := strings.Join(plan.Warnings, "\n")
	if !strings.Contains(warned, ".opencode/opencode.json") || !strings.Contains(warned, "machine path") {
		t.Errorf("no warning that the OpenCode file holds a machine path:\n%s", warned)
	}
	if code, out, errOut := initRun(root, Options{Plugins: []string{"claude", "opencode"}, PluginSource: source}); code != 0 {
		t.Fatalf("init = %d\n%s%s", code, out, errOut)
	}
	entries := readJSON(t, filepath.Join(root, ".opencode", "opencode.json"))["plugins"].([]any)
	if len(entries) != 1 || entries[0] != filepath.Join(source, "opencode") {
		t.Fatalf("plugins = %v", entries)
	}
}

func TestOpenCodeDefaultsToBundledSource(t *testing.T) {
	root := repo(t, "main")
	plan := dryPlan(t, root, Options{Plugins: []string{"opencode"}})
	if actionAt(plan, ".opencode/opencode.json") == nil || !reflect.DeepEqual(plan.BundledPlugins, []string{"opencode"}) {
		t.Fatal("OpenCode was not planned with bundled source")
	}
	if strings.Contains(strings.Join(plan.Warnings, "\n"), "not published") {
		t.Errorf("obsolete warning: %v", plan.Warnings)
	}
}

func TestOpenCodePluginsAreAUnion(t *testing.T) {
	root := repo(t, "main")
	source := pluginCheckout(t)
	testutil.Write(t, filepath.Join(root, ".opencode", "opencode.json"), `{"plugins":["/elsewhere/x"],"theme":"dark"}`)
	commitAll(t, root)
	if code, out, errOut := initRun(root, Options{Plugins: []string{"opencode"}, PluginSource: source}); code != 0 {
		t.Fatalf("init = %d\n%s%s", code, out, errOut)
	}
	file := readJSON(t, filepath.Join(root, ".opencode", "opencode.json"))
	entries := file["plugins"].([]any)
	if len(entries) != 2 || entries[0] != "/elsewhere/x" || entries[1] != filepath.Join(source, "opencode") || file["theme"] != "dark" {
		t.Fatalf("opencode.json = %v", file)
	}
}

func TestASettingsFileThatIsNotPlainJSONIsLeftAloneWithTheFragmentToAdd(t *testing.T) {
	root := repo(t, "main")
	source := pluginCheckout(t)
	mine := "{\n  // my settings\n  \"model\": \"x\",\n}\n"
	testutil.Write(t, filepath.Join(root, filepath.FromSlash(claudeLocalSettingsRel)), mine)
	commitAll(t, root)
	plan := dryPlan(t, root, Options{Plugins: []string{"claude"}, PluginSource: source})
	if actionAt(plan, claudeLocalSettingsRel) != nil {
		t.Fatal("a file with comments must not get a merge")
	}
	warned := strings.Join(plan.Warnings, "\n")
	if !strings.Contains(warned, claudeLocalSettingsRel) || !strings.Contains(warned, "kanbanzai@kanbanzai") {
		t.Errorf("the warning must name the file and give the fragment to add by hand:\n%s", warned)
	}
	if code, out, errOut := initRun(root, Options{Plugins: []string{"claude"}, PluginSource: source}); code != 0 {
		t.Fatalf("init = %d\n%s%s", code, out, errOut)
	}
	if tree(t, root)[claudeLocalSettingsRel] != mine {
		t.Fatal("the file was rewritten")
	}
	if _, ok := tree(t, root)[ConfigRel]; !ok {
		t.Fatal("the rest of the setup was not done")
	}
}

func TestADisabledPluginIsNeverReEnabled(t *testing.T) {
	root := repo(t, "main")
	mine := `{"enabledPlugins":{"kanbanzai@kanbanzai":false}}`
	testutil.Write(t, filepath.Join(root, ".claude", "settings.json"), mine)
	commitAll(t, root)
	plan := dryPlan(t, root, Options{Plugins: []string{"claude"}})
	if action := actionAt(plan, ".claude/settings.json"); action != nil && action.Op != OpKeep {
		t.Fatalf("a disabled plugin was planned to change: %+v", action)
	}
	if !strings.Contains(strings.Join(plan.Warnings, "\n"), "disables") {
		t.Errorf("no warning that the plugin is disabled:\n%v", plan.Warnings)
	}
	if code, _, _ := initRun(root, Options{Plugins: []string{"claude"}}); code != 0 || tree(t, root)[".claude/settings.json"] != mine {
		t.Fatal("the disabled setting was changed")
	}
}

func TestPluginsWorkOnAnExistingBoardWithoutTouchingTheConfig(t *testing.T) {
	root := repo(t, "main")
	source := pluginCheckout(t)
	mine := "config_version = 1\n[project]\nowner = \"Me\"\n"
	testutil.Write(t, filepath.Join(root, ".kanbanzai", "config.toml"), mine)
	commitAll(t, root)
	if code, out, errOut := initRun(root, Options{Plugins: []string{"claude"}, PluginSource: source}); code != 0 {
		t.Fatalf("init = %d\n%s%s", code, out, errOut)
	}
	if tree(t, root)[ConfigRel] != mine {
		t.Fatal("the config was changed")
	}
	if _, ok := tree(t, root)[claudeLocalSettingsRel]; !ok {
		t.Fatal("the plugin settings were not written")
	}
}

func TestPluginOptionsAreRefusedWhenWrong(t *testing.T) {
	root := repo(t, "main")
	before := tree(t, root)
	missing := filepath.Join(t.TempDir(), "nowhere")
	notCheckout := t.TempDir()
	for _, opts := range []Options{
		{Plugins: []string{"vim"}}, {Plugins: []string{""}}, {Plugins: []string{"claude", "claude"}},
		{PluginSource: pluginCheckout(t)}, // a source with no --plugins
		{Plugins: []string{"claude"}, PluginSource: missing}, {Plugins: []string{"claude"}, PluginSource: notCheckout},
		{Plugins: []string{"claude"}, PluginSource: "relative/path"},
	} {
		if code, _, errOut := initRun(root, opts); code != 2 {
			t.Errorf("init %+v = %d (%q), want 2", opts, code, errOut)
		}
	}
	if !reflect.DeepEqual(before, tree(t, root)) {
		t.Fatal("a refusal changed the tree")
	}
}

func TestAPlanFileMayOnlyMergeIntoTheKnownSettingsFiles(t *testing.T) {
	root := repo(t, "main")
	d, _, err := Detect(root)
	if err != nil {
		t.Fatal(err)
	}
	good := dryPlan(t, root, Options{Plugins: []string{"claude"}})
	if err := ValidatePlan(good, d); err != nil {
		t.Fatalf("a real plan does not validate: %v", err)
	}
	clone := func() *Plan {
		data, _ := json.Marshal(good)
		var plan Plan
		_ = json.Unmarshal(data, &plan)
		return &plan
	}
	index := func(plan *Plan) int {
		for i, a := range plan.Actions {
			if a.What == WhatPlugin {
				return i
			}
		}
		t.Fatal("no plugin action")
		return -1
	}
	for name, edit := range map[string]func(*Action){
		"another path":    func(a *Action) { a.Path = "src/evil.json" },
		"the config path": func(a *Action) { a.Path = ConfigRel },
		"not JSON":        func(a *Action) { a.Content = "{ // no"; a.SHA256 = digest(a.Content) },
		"an array":        func(a *Action) { a.Content = "[1]"; a.SHA256 = digest(a.Content) },
		"no content":      func(a *Action) { a.Content, a.SHA256 = "", "" },
		"a bad digest":    func(a *Action) { a.SHA256 = digest("other") },
		"unrelated executable settings": func(a *Action) {
			a.Content = `{"hooks":{"SessionStart":[{"command":"malicious"}]}}`
			a.SHA256 = digest(a.Content)
		},
		"another marketplace": func(a *Action) {
			a.Content = `{"extraKnownMarketplaces":{"attacker":{"source":{"source":"github","repo":"evil/repo"}}}}`
			a.SHA256 = digest(a.Content)
		},
		"another plugin": func(a *Action) {
			a.Content = `{"enabledPlugins":{"attacker@elsewhere":true}}`
			a.SHA256 = digest(a.Content)
		},
	} {
		plan := clone()
		edit(&plan.Actions[index(plan)])
		if err := ValidatePlan(plan, d); err == nil {
			t.Errorf("%s: the plan was accepted", name)
		}
	}
}

func TestAMergeIsStaleWhenTheFileChangedAfterThePlan(t *testing.T) {
	root := repo(t, "main")
	source := pluginCheckout(t)
	testutil.Write(t, filepath.Join(root, filepath.FromSlash(claudeLocalSettingsRel)), `{"a":1}`)
	commitAll(t, root)
	plan := dryPlan(t, root, Options{Plugins: []string{"claude"}, PluginSource: source})
	data, _ := json.Marshal(&Report{SchemaVersion: 1, Mode: "dry-run", Plan: plan})
	file := filepath.Join(t.TempDir(), "plan.json")
	if err := os.WriteFile(file, data, 0o600); err != nil {
		t.Fatal(err)
	}
	testutil.Write(t, filepath.Join(root, filepath.FromSlash(claudeLocalSettingsRel)), `{"a":2}`)
	before := tree(t, root)
	if code, _, errOut := initRun(root, Options{PlanFile: file}); code != 2 || !strings.Contains(errOut, "run init --dry-run again") {
		t.Fatalf("a stale merge = %d %q, want 2", code, errOut)
	}
	if !reflect.DeepEqual(before, tree(t, root)) {
		t.Fatal("a stale plan wrote something")
	}
}
