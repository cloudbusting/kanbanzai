package initcmd

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	kanbanzai "github.com/cloudbusting/kanbanzai"
	"github.com/cloudbusting/kanbanzai/internal/config"
)

// PlanKind and PlanVersion identify the plan format: the one JSON value that dry run shows, apply executes and
// --plan reads back.
const (
	PlanKind    = "kanbanzai-init-plan"
	PlanVersion = 1
)

// The operations and kinds of thing a plan may hold.
const (
	OpMkdir   = "mkdir"
	OpCreate  = "create"
	OpKeep    = "keep"
	OpAppend  = "append"
	OpBackup  = "backup"
	OpReplace = "replace"

	WhatFolder    = "folder"
	WhatBatches   = "batches"
	WhatGitkeep   = "gitkeep"
	WhatGitignore = "gitignore"
	WhatTemplate  = "template"
	WhatConfig    = "config"
	WhatBackup    = "backup"
)

// ConfigRel and TemplateRel are where init puts the config (the only place the loader reads) and a project
// template when the config names none.
const (
	ConfigRel          = ".kanbanzai/config.toml"
	DefaultTemplateRel = ".kanbanzai/templates/card.md"
)

// Action is one write, or one thing init saw and leaves alone. Paths are relative to the plan's root and use
// forward slashes.
type Action struct {
	Op           string `json:"op"`
	Path         string `json:"path"`
	What         string `json:"what"`
	Content      string `json:"content,omitempty"`
	SHA256       string `json:"sha256,omitempty"`
	ExpectSHA256 string `json:"expect_sha256,omitempty"`
}

// Settings are the values init chose for a config it writes. They are empty when the existing config is kept.
type Settings struct {
	MainBranch       string `json:"main_branch"`
	MainBranchSource string `json:"main_branch_source"`
	Owner            string `json:"owner"`
	OwnerSource      string `json:"owner_source"`
}

// Plan is everything init will do. Building it writes nothing.
type Plan struct {
	SchemaVersion int       `json:"schema_version"`
	Kind          string    `json:"kind"`
	Root          string    `json:"root"`
	Detected      Detected  `json:"detected"`
	Settings      *Settings `json:"settings,omitempty"`
	Actions       []Action  `json:"actions"`
	Warnings      []string  `json:"warnings"`
}

// Refusal is a plan init will not make; it exits 2 and says why.
type Refusal struct{ Message string }

func (r *Refusal) Error() string { return r.Message }

func refuse(format string, args ...any) error { return &Refusal{Message: fmt.Sprintf(format, args...)} }

// IsRefusal reports whether err is a Refusal.
func IsRefusal(err error) bool {
	var refusal *Refusal
	return errors.As(err, &refusal)
}

func digest(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

// Build makes the plan for the detected checkout. existing is the project's loaded configuration, or nil.
func Build(d *Detected, existing *config.Project, opts Options) (*Plan, error) {
	plan := &Plan{SchemaVersion: PlanVersion, Kind: PlanKind, Root: d.Root, Detected: *d, Actions: []Action{},
		Warnings: []string{}}
	root := d.Root

	var project *config.Project
	var configActions []Action
	switch {
	case d.Config.Exists && !d.Config.Regular:
		return nil, refuse("%s exists and is not a regular file: replace it by hand, init will not", ConfigRel)
	case d.Config.Exists && !opts.Force:
		if !d.Config.Valid {
			return nil, refuse("%s cannot be loaded: %s\nuse --force to back it up and replace it with a fresh "+
				"config", ConfigRel, d.Config.Error)
		}
		if opts.MainBranch != "" && opts.MainBranch != d.Config.MainBranch {
			return nil, refuse("--main-branch %s disagrees with the existing config (main_branch = \"%s\"): edit "+
				"the config, or use --force to replace it", opts.MainBranch, d.Config.MainBranch)
		}
		if opts.Owner != "" {
			return nil, refuse("--owner has no effect on the existing %s: edit it, or use --force to replace it",
				ConfigRel)
		}
		if opts.Prefix != "" || len(opts.Context) > 0 {
			return nil, refuse("--prefix and --context have no effect on the existing %s: edit it, or use --force to "+
				"replace it", ConfigRel)
		}
		project = existing
		configActions = []Action{{Op: OpKeep, Path: ConfigRel, What: WhatConfig}}
		if detected, _, err := chooseMainBranch(d, ""); err == nil && detected != project.MainBranch {
			plan.Warnings = append(plan.Warnings, fmt.Sprintf("the config says main_branch = \"%s\" but this "+
				"checkout looks like \"%s\": wip --audit will flag it until project.main_branch is corrected",
				project.MainBranch, detected))
		}
	default:
		branch, source, err := chooseMainBranch(d, opts.MainBranch)
		if err != nil {
			return nil, refuse("%v", err)
		}
		settings := &Settings{MainBranch: branch, MainBranchSource: source}
		switch {
		case opts.Owner != "":
			settings.Owner, settings.OwnerSource = opts.Owner, "--owner"
		case d.GitUser != "":
			settings.Owner, settings.OwnerSource = d.GitUser, "git config user.name"
		default:
			settings.Owner, settings.OwnerSource = "the user", "default"
		}
		if err := checkOwner(settings.Owner); err != nil {
			if opts.Owner != "" {
				return nil, refuse("--owner: %v", err)
			}
			plan.Warnings = append(plan.Warnings, fmt.Sprintf("git user.name is not usable as the owner (%v)", err))
			settings.Owner, settings.OwnerSource = "the user", "default"
		}
		prefix, err := checkPrefix(opts.Prefix)
		if err != nil {
			return nil, err
		}
		context, err := contextFiles(root, opts.Context)
		if err != nil {
			return nil, err
		}
		template := ""
		if opts.WithTemplate {
			template = DefaultTemplateRel
		}
		text := ConfigText(settings.Owner, branch, template, prefix, context)
		loaded, err := config.ValidateText(root, d.Toplevel, d.GitCommon, text)
		if err != nil {
			return nil, fmt.Errorf("init built a config the loader rejects (a bug): %w", err)
		}
		project = loaded
		plan.Settings = settings
		if d.Config.Exists {
			old, err := os.ReadFile(config.BootstrapPath(root))
			if err != nil {
				return nil, fmt.Errorf("%s: %v", ConfigRel, err)
			}
			if !utf8Valid(old) {
				return nil, refuse("%s is not valid UTF-8, so a plan cannot carry its backup: back it up by hand "+
					"and remove it", ConfigRel)
			}
			backup := backupName(root, opts.now())
			configActions = []Action{
				{Op: OpBackup, Path: backup, What: WhatBackup, Content: string(old), SHA256: digest(string(old))},
				{Op: OpReplace, Path: ConfigRel, What: WhatConfig, Content: text, SHA256: digest(text),
					ExpectSHA256: digest(string(old))},
			}
		} else {
			configActions = []Action{{Op: OpCreate, Path: ConfigRel, What: WhatConfig, Content: text,
				SHA256: digest(text)}}
		}
	}

	if filepath.Clean(project.Home) != filepath.Join(root, ".kanbanzai") {
		plan.Warnings = append(plan.Warnings, fmt.Sprintf("project.home is %s: the config stays at %s, the only "+
			"place it is read, and the home folder is used for its .gitignore only", project.Rel(project.Home),
			ConfigRel))
	}

	seen := map[string]bool{}
	folder := func(path, what string) error {
		rel, err := insideRoot(root, path, what)
		if err != nil {
			return err
		}
		if seen[rel] {
			return nil
		}
		seen[rel] = true
		abs := filepath.Join(root, filepath.FromSlash(rel))
		info, statErr := os.Stat(abs)
		switch {
		case statErr == nil && !info.IsDir():
			return refuse("%s exists and is not a folder", rel)
		case statErr == nil:
			plan.Actions = append(plan.Actions, Action{Op: OpKeep, Path: rel, What: what})
		default:
			plan.Actions = append(plan.Actions, Action{Op: OpMkdir, Path: rel, What: what})
		}
		plan.Actions = append(plan.Actions, createOrKeep(root, rel+"/.gitkeep", WhatGitkeep, ""))
		return nil
	}
	for _, name := range project.Folders {
		if err := folder(filepath.Join(project.Work, name), WhatFolder); err != nil {
			return nil, err
		}
	}
	if err := folder(project.Batches, WhatBatches); err != nil {
		return nil, err
	}

	ignores, warnings, err := ignoreActions(d, project)
	if err != nil {
		return nil, err
	}
	plan.Actions = append(plan.Actions, ignores...)
	plan.Warnings = append(plan.Warnings, warnings...)

	if !project.TemplateIsShipped {
		rel, err := insideRoot(root, project.Template, "project.template")
		if err != nil {
			return nil, err
		}
		if info, statErr := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); statErr == nil && !info.Mode().IsRegular() {
			return nil, refuse("%s exists and is not a file", rel)
		}
		shipped, err := kanbanzai.Files.ReadFile("templates/card.md")
		if err != nil {
			return nil, fmt.Errorf("templates/card.md: %v", err)
		}
		action := createOrKeep(root, rel, WhatTemplate, string(shipped))
		if action.Op == OpCreate && !opts.WithTemplate {
			plan.Warnings = append(plan.Warnings, fmt.Sprintf("project.template names %s, which does not exist: pass "+
				"--with-template to create it from the shipped template", rel))
		} else {
			plan.Actions = append(plan.Actions, action)
		}
	} else if opts.WithTemplate {
		return nil, refuse("--with-template: the existing config sets no project.template; add one, or use " +
			"--force to replace the config")
	}

	plan.Actions = append(plan.Actions, configActions...)
	if len(d.SuggestedContext) > 0 && len(project.Context) == 0 {
		plan.Warnings = append(plan.Warnings, "host rule files found: "+strings.Join(d.SuggestedContext, ", ")+
			" (init does not edit them; list the ones the skills should read in project.context)")
	}
	return plan, nil
}

// createOrKeep is a create when the file is absent and a keep when it exists, whatever it holds.
func createOrKeep(root, rel, what, content string) Action {
	if _, err := os.Lstat(filepath.Join(root, filepath.FromSlash(rel))); err == nil {
		return Action{Op: OpKeep, Path: rel, What: what}
	}
	action := Action{Op: OpCreate, Path: rel, What: what, Content: content}
	if content != "" {
		action.SHA256 = digest(content)
	}
	return action
}

// insideRoot is path relative to root with forward slashes, or a refusal naming what when it is not inside.
func insideRoot(root, path, what string) (string, error) {
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) ||
		filepath.IsAbs(rel) {
		return "", refuse("%s resolves outside the main checkout (%s): init only creates files inside it; "+
			"correct the setting or make the folder yourself", what, path)
	}
	return filepath.ToSlash(rel), nil
}

func backupName(root string, now time.Time) string {
	base := ConfigRel + ".bak-" + now.UTC().Format("20060102T150405Z")
	name := base
	for n := 2; ; n++ {
		if _, err := os.Lstat(filepath.Join(root, filepath.FromSlash(name))); err != nil {
			return name
		}
		name = fmt.Sprintf("%s-%d", base, n)
	}
}

// ignoreActions makes sure the state, claims and numbers folders are never committed. Folders inside home get a
// line in <home>/.gitignore; others inside the checkout get a marked block in the root .gitignore; a folder
// outside the checkout needs nothing.
func ignoreActions(d *Detected, project *config.Project) ([]Action, []string, error) {
	root := d.Root
	var warnings []string
	chosen := []string{}
	for _, entry := range []struct{ key, path string }{{"project.state", project.State},
		{"project.claims", project.Claims}, {"project.numbers", project.Numbers}} {
		if _, err := insideRoot(root, entry.path, entry.key); err != nil {
			warnings = append(warnings, fmt.Sprintf("%s is outside the main checkout (%s): nothing to ignore",
				entry.key, entry.path))
			continue
		}
		covered := false
		for _, other := range chosen {
			if within(other, entry.path) {
				covered = true
			}
		}
		if !covered {
			chosen = append(chosen, entry.path)
		}
	}
	homeRel, homeErr := insideRoot(root, project.Home, "project.home")
	homeLines, rootLines := []string{}, []string{}
	for _, path := range chosen {
		if homeErr == nil && within(project.Home, path) {
			rel, _ := filepath.Rel(project.Home, path)
			homeLines = append(homeLines, filepath.ToSlash(rel)+"/")
		} else {
			rel, _ := insideRoot(root, path, "")
			rootLines = append(rootLines, "/"+rel+"/")
		}
	}
	var actions []Action
	if len(homeLines) > 0 {
		actions = append(actions, gitignoreAction(root, homeRel+"/.gitignore", "", homeLines))
	}
	if len(rootLines) > 0 {
		actions = append(actions, gitignoreAction(root, ".gitignore",
			"# kanbanzai: local state, never committed", rootLines))
	}
	return actions, warnings, nil
}

// gitignoreAction creates the ignore file, appends the lines it lacks, or keeps it when it has them all.
func gitignoreAction(root, rel, marker string, lines []string) Action {
	existing, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	block := func(missing []string, withMarker bool) string {
		text := ""
		if withMarker && marker != "" {
			text = marker + "\n"
		}
		return text + strings.Join(missing, "\n") + "\n"
	}
	if err != nil {
		content := block(lines, true)
		return Action{Op: OpCreate, Path: rel, What: WhatGitignore, Content: content, SHA256: digest(content)}
	}
	have := map[string]bool{}
	for _, line := range strings.Split(string(existing), "\n") {
		have[strings.Trim(strings.TrimSpace(line), "/")] = true
	}
	missing := []string{}
	for _, line := range lines {
		if !have[strings.Trim(line, "/")] {
			missing = append(missing, line)
		}
	}
	if len(missing) == 0 {
		return Action{Op: OpKeep, Path: rel, What: WhatGitignore}
	}
	content := block(missing, marker != "" && !strings.Contains(string(existing), marker))
	return Action{Op: OpAppend, Path: rel, What: WhatGitignore, Content: content, SHA256: digest(content),
		ExpectSHA256: digest(string(existing))}
}

// within reports whether path is dir or below it.
func within(dir, path string) bool {
	rel, err := filepath.Rel(dir, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) &&
		!filepath.IsAbs(rel)
}

func utf8Valid(data []byte) bool { return strings.ToValidUTF8(string(data), "\x00") == string(data) }

// ConfigText is the short config a new project starts from; the shipped defaults fill the rest. main_branch and
// template are written only when they differ from the defaults.
func ConfigText(owner, mainBranch, template, prefix string, context []string) string {
	var b strings.Builder
	b.WriteString(`# Kanbanzai config for this project. Only what differs from Kanbanzai's shipped defaults belongs here:
# the columns, the cap of five started cards, the two-hour lease and the pull order are the defaults.
# ` + "`kanbanzai config`" + ` prints the settings in force. Tables merge key by key; a list (such as [[columns]])
# replaces the default one whole.

config_version = 1

[project]
`)
	fmt.Fprintf(&b, "owner = %s      # the person the skills address and whose decisions they ask for\n", tomlString(owner))
	if mainBranch != "" && mainBranch != "main" {
		fmt.Fprintf(&b, "main_branch = %s      # the branch the board and finished work live on\n", tomlString(mainBranch))
	}
	if template != "" {
		fmt.Fprintf(&b, "template = %s      # the card template; edit it freely, init never overwrites it\n",
			tomlString(template))
	}
	if prefix != "" {
		fmt.Fprintf(&b, "prefix = %s      # card ids are <prefix>-<number>\n", tomlString(prefix))
	}
	if len(context) > 0 {
		quoted := make([]string, len(context))
		for i, item := range context {
			quoted[i] = tomlString(item)
		}
		fmt.Fprintf(&b, "context = [%s]      # files every skill reads first\n", strings.Join(quoted, ", "))
	}
	b.WriteString(`
# More you can set (see Kanbanzai's defaults/config.toml for every key):
#   [project]  prefix, work, claims, numbers, worktree, context = ["rules file", ...]
#   [flow]     wip_limit, pull_order
#   [steps.check]  skill = "your-skill", extend = ["your-extension"], checks = ["your-test-skill"]
`)
	return b.String()
}

func tomlString(text string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range text {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

// checkPrefix validates --prefix the way the loader will (letters only), so a bad key is a refusal before any plan.
func checkPrefix(prefix string) (string, error) {
	if prefix == "" {
		return "", nil
	}
	for _, r := range prefix {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z') {
			return "", refuse("--prefix '%s': a card id prefix is letters only (for example W or ACME)", prefix)
		}
	}
	return prefix, nil
}

// contextFiles turns --context into the config's list: each entry a regular file inside the checkout, written
// relative to it with forward slashes, once.
func contextFiles(root string, given []string) ([]string, error) {
	out := []string{}
	seen := map[string]bool{}
	for _, item := range given {
		if strings.TrimSpace(item) == "" || filepath.IsAbs(item) {
			return nil, refuse("--context '%s': name a file inside the checkout, relative to its root", item)
		}
		rel, err := insideRoot(root, filepath.Join(root, filepath.FromSlash(item)), "--context "+item)
		if err != nil {
			return nil, err
		}
		info, statErr := os.Stat(filepath.Join(root, filepath.FromSlash(rel)))
		if statErr != nil || !info.Mode().IsRegular() {
			return nil, refuse("--context %s is not a file in the checkout", rel)
		}
		if !seen[rel] {
			seen[rel] = true
			out = append(out, rel)
		}
	}
	return out, nil
}

// checkOwner refuses a name that cannot be a single-line TOML string.
func checkOwner(name string) error {
	if strings.TrimSpace(name) == "" {
		return errors.New("it is empty")
	}
	for _, r := range name {
		if r < 0x20 || r == 0x7f {
			return errors.New("it contains a control character")
		}
	}
	if len(name) > 200 {
		return errors.New("it is longer than 200 bytes")
	}
	return nil
}
