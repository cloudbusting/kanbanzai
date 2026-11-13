// The project and its configuration: where the board lives, and the workflow as data.
//
// The project is the main checkout: the parent of `git rev-parse --git-common-dir`, the .git every worktree
// shares, so every worktree reaches one board and one set of claims. KANBANZAI_PROJECT names the main checkout
// directly.
//
// The configuration is the shipped defaults with the project's .kanbanzai/config.toml merged over it: tables merge
// key by key, and anything else (a list, so the whole [[columns]] list too) replaces. If the main checkout has no
// config file yet, the current worktree's is used, so a branch that introduces the config can be tried before it
// is merged.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/cloudbusting/kanbanzai"
	"github.com/cloudbusting/kanbanzai/internal/gitutil"
	"github.com/cloudbusting/kanbanzai/internal/typeid"
)

var kinds = []string{"open", "parked", "done"}

// ConfigError is a configuration that cannot be used.
type ConfigError struct{ Message string }

func (e *ConfigError) Error() string { return e.Message }

func errf(format string, args ...any) *ConfigError {
	return &ConfigError{Message: fmt.Sprintf(format, args...)}
}

// NoProject means there is no project here: not a git checkout, and no KANBANZAI_PROJECT.
type NoProject struct{ Message string }

func (e *NoProject) Error() string { return e.Message }

// IsNoProject reports whether err is a NoProject.
func IsNoProject(err error) bool {
	var target *NoProject
	return errors.As(err, &target)
}

// Column is one column of the board.
type Column struct {
	Key           string
	Label         string
	Words         []string
	Folder        string
	Kind          string
	Started       any // true, false, or "claimed"
	Next          string
	SettledNext   string
	Title         string
	SettledTitle  string
	Worktree      bool
	WaitsForOwner bool // true for gate = "owner" (the older `waits_for_owner = true` is the same thing)
	Gate          Gate
	Lane          string
	Doing         string
}

// StartedBool is true when the column counts toward the cap whether or not it has a live claim.
func (c Column) StartedBool() bool {
	value, ok := c.Started.(bool)
	return ok && value
}

// CountsWithClaim is true for the first column, which counts only when a live claim holds a card.
func (c Column) CountsWithClaim() bool {
	value, ok := c.Started.(string)
	return ok && value == "claimed"
}

// Step is one workflow step.
type Step struct {
	Name    string
	Into    string
	Skill   string
	Extend  []string
	Context []string
	Checks  []string
	Active  bool
	During  string

	// SkipField and SkipValues make a card skip the step: its brief's SkipField (a head line or a "## " section)
	// starts with one of SkipValues. Both set, or neither.
	SkipField  string
	SkipValues []string
}

// Project is one project's resolved configuration: its paths, columns and steps.
type Project struct {
	Root      string
	Toplevel  string
	GitCommon string

	ConfigFile         string
	ConfigVersion      int
	ConfigAllowUnknown []string
	IgnoredUnknown     []string

	Prefix             string
	Digits             int
	Typed              bool // true when [[types]] configures typed ids; Prefix is then unused
	Types              []typeid.Type
	TypeSet            *typeid.Set
	DefaultType        string
	ProjectKey         string
	BriefSchemas       map[string]BriefSchema
	Owner              string
	MainBranch         string
	Isolation          string // worktree, branch, shared or external: how a card's change is kept apart
	WorktreePattern    string
	BranchPattern      string
	BatchBranchPattern string
	OwnerEnv           string
	LeaseMinutes       float64

	Home              string
	Work              string
	State             string
	Claims            string
	Numbers           string
	Batches           string
	Template          string
	TemplateIsShipped bool
	Context           []string
	contextPaths      []string

	WIPLimit         int
	StaleDays        int
	MoveGraceMinutes float64
	PullOrder        []string

	Columns     []Column
	ByKey       map[string]Column
	StageColumn map[string]string
	Folders     []string
	OpenKeys    []string

	Steps     map[string]Step
	StepOrder []string

	IDPattern  string
	CardRe     *regexp.Regexp
	TitleRe    *regexp.Regexp
	CardFileRe *regexp.Regexp
	ParseRe    *regexp.Regexp
	IDInRe     *regexp.Regexp

	cfg map[string]any
}

// Find locates the project from start (the current directory when empty) and loads its configuration.
func Find(start string) (*Project, error) {
	root, top, common, err := locate(start)
	if err != nil {
		return nil, err
	}
	return New(root, top, common, nil)
}

// Locate finds the shared main checkout without loading its configuration, for diagnosis and upgrades.
func Locate(start string) (root, top, common string, err error) { return locate(start) }

// New loads the configuration for a project whose main checkout is root.
func New(root, toplevel, gitCommon string, extra map[string]any) (*Project, error) {
	return load(root, toplevel, gitCommon, extra, true)
}

// load is New; readFile false skips the project's config file, for a text that is checked before it is written.
func load(root, toplevel, gitCommon string, extra map[string]any, readFile bool) (*Project, error) {
	root = absolute(root)
	if toplevel == "" {
		toplevel = root
	}
	defaults, err := readDefaults()
	if err != nil {
		return nil, err
	}
	defaultProject, _ := defaults["project"].(map[string]any)
	home := asString(defaultProject["home"], ".kanbanzai")
	merged := defaults
	configFile := ""
	ignored := []string{}
	candidates := []string{}
	if readFile {
		candidates = uniqueStrings([]string{root, toplevel})
	}
	for _, base := range candidates {
		candidate := configCandidate(base, home)
		if _, statErr := os.Lstat(candidate); statErr == nil {
			configFile = candidate
			over, err := ReadTOML(candidate)
			if err != nil {
				return nil, err
			}
			if err := validateVersion(over, candidate); err != nil {
				return nil, err
			}
			ignored, err = validateSchema(over, candidate)
			if err != nil {
				return nil, err
			}
			merged = merge(defaults, over)
			break
		} else if !os.IsNotExist(statErr) {
			return nil, errf("%s: cannot inspect config (%v)", candidate, statErr)
		}
	}
	if extra != nil {
		if _, exists := extra["config_version"]; exists {
			if err := validateVersion(extra, "config overrides"); err != nil {
				return nil, err
			}
		}
		if _, err := validateSchema(extra, "config overrides"); err != nil {
			return nil, err
		}
		merged = merge(merged, extra)
	}
	p := &Project{
		Root:           root,
		Toplevel:       absolute(toplevel),
		GitCommon:      absoluteOrEmpty(gitCommon),
		ConfigFile:     configFile,
		cfg:            merged,
		ConfigVersion:  CurrentVersion,
		IgnoredUnknown: ignored,
	}
	p.ConfigAllowUnknown, _ = stringList(merged["config_allow_unknown"])
	if err := p.build(); err != nil {
		return nil, qualifyError(sourceName(configFile), err)
	}
	return p, nil
}

// ReadTOML reads a config file, naming it in every error.
func ReadTOML(path string) (map[string]any, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, errf("%s: cannot inspect config (%v)", path, err)
	}
	if !info.Mode().IsRegular() {
		return nil, errf("%s: config must be a regular file; replace the directory or special file", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, errf("%s: cannot be read (%v)", path, err)
	}
	parsed, err := parseTOML(string(data))
	if err != nil {
		return nil, errf("%s: %v", path, err)
	}
	return parsed, nil
}

func readDefaults() (map[string]any, error) {
	data, err := kanbanzai.Files.ReadFile("defaults/config.toml")
	if err != nil {
		return nil, errf("defaults/config.toml: cannot be read (%v)", err)
	}
	parsed, err := parseTOML(string(data))
	if err != nil {
		return nil, errf("defaults/config.toml: %v", err)
	}
	if err := validateVersion(parsed, "defaults/config.toml"); err != nil {
		return nil, err
	}
	if _, err := validateSchema(parsed, "defaults/config.toml"); err != nil {
		return nil, err
	}
	return parsed, nil
}

func locate(start string) (string, string, string, error) {
	if start == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return "", "", "", &NoProject{Message: "cannot find the current directory"}
		}
		start = cwd
	}
	top, _ := gitutil.Git(start, "rev-parse", "--show-toplevel")
	common, _ := gitutil.Git(start, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if override := strings.TrimSpace(os.Getenv("KANBANZAI_PROJECT")); override != "" {
		root, err := filepath.Abs(override)
		if err != nil {
			return "", "", "", &NoProject{Message: fmt.Sprintf("KANBANZAI_PROJECT is not a folder: %s", override)}
		}
		info, err := os.Stat(root)
		if err != nil || !info.IsDir() {
			return "", "", "", &NoProject{Message: fmt.Sprintf("KANBANZAI_PROJECT is not a folder: %s", override)}
		}
		if top == "" {
			top = root
		}
		shared, _ := gitutil.Git(root, "rev-parse", "--path-format=absolute", "--git-common-dir")
		return root, absolute(top), absoluteOrEmpty(shared), nil
	}
	if top == "" || common == "" {
		return "", "", "", &NoProject{Message: "not in a git checkout (and KANBANZAI_PROJECT is not set)"}
	}
	shared := absolute(common)
	root := shared
	if filepath.Base(shared) == ".git" {
		root = filepath.Dir(shared)
	} else {
		root = absolute(top)
	}
	return root, absolute(top), shared, nil
}

func configCandidate(base, home string) string {
	if filepath.IsAbs(home) {
		return filepath.Join(home, "config.toml")
	}
	return filepath.Join(base, home, "config.toml")
}

// build turns the merged TOML into the typed configuration.
func (p *Project) build() error {
	project, err := p.section("project")
	if err != nil {
		return err
	}
	claims, err := p.section("claims")
	if err != nil {
		return err
	}
	flow, err := p.section("flow")
	if err != nil {
		return err
	}

	p.Digits = asInt(project["digits"], 2)
	if p.Digits == 0 {
		p.Digits = 2
	}
	types, err := p.types()
	if err != nil {
		return err
	}
	if len(types) == 0 {
		p.Typed = false
		p.Prefix = strings.TrimSpace(asString(project["prefix"], ""))
		if !regexp.MustCompile(`^[A-Za-z]+$`).MatchString(p.Prefix) {
			return errf("project.prefix must be letters only, not '%s'", p.Prefix)
		}
	} else {
		p.Typed = true
		p.Prefix = strings.TrimSpace(asString(project["prefix"], ""))
		if p.Prefix != "" && !regexp.MustCompile(`^[A-Za-z]+$`).MatchString(p.Prefix) {
			return errf("project.prefix is the legacy card id prefix and must be letters only, not '%s'", p.Prefix)
		}
		set, err := typeid.Validate(types, asString(project["default_type"], ""),
			asString(project["project_key"], ""), p.Digits)
		if err != nil {
			return errf("[[types]]: %v", err)
		}
		p.TypeSet = set
		p.Types = set.Types
		p.DefaultType = set.Default
		p.ProjectKey = set.ProjectKey
	}
	if err := p.loadBriefSchemas(); err != nil {
		return err
	}
	p.Owner = asString(project["owner"], "the user")
	p.MainBranch = asString(project["main_branch"], "main")
	p.WorktreePattern = asString(project["worktree"], "../worktrees/{repo}/{id}")
	p.BranchPattern = asString(project["branch"], "{id}")
	p.BatchBranchPattern = asString(project["batch_branch"], "batch/{id}")

	if p.Home, err = p.path(asString(project["home"], ".kanbanzai")); err != nil {
		return err
	}
	if p.Work, err = p.path(asString(project["work"], ".kanbanzai/work")); err != nil {
		return err
	}
	if p.State, err = p.path(asString(project["state"], ".kanbanzai/state")); err != nil {
		return err
	}
	claimsDefault := filepath.Join(p.State, "claims")
	if p.Claims, err = p.path(or(asString(project["claims"], ""), claimsDefault)); err != nil {
		return err
	}
	numbersDefault := filepath.Join(p.State, "numbers")
	if p.Numbers, err = p.path(or(asString(project["numbers"], ""), numbersDefault)); err != nil {
		return err
	}
	batchesDefault := filepath.Join(p.Work, "batches")
	if p.Batches, err = p.path(or(asString(project["batches"], ""), batchesDefault)); err != nil {
		return err
	}
	if template := asString(project["template"], ""); template != "" {
		if p.Template, err = p.path(template); err != nil {
			return err
		}
		p.TemplateIsShipped = false
	} else {
		p.Template = ""
		p.TemplateIsShipped = true
	}
	context, err := stringList(project["context"])
	if err != nil {
		return errf("project.context must be a list of files")
	}
	p.Context = context
	for _, item := range context {
		if _, err := p.path(item); err != nil {
			return err
		}
	}

	p.LeaseMinutes = asFloat(claims["lease_minutes"], 120)
	if p.LeaseMinutes == 0 {
		p.LeaseMinutes = 120
	}
	p.OwnerEnv = asString(claims["owner_env"], "KANBANZAI_OWNER")
	if p.OwnerEnv == "" {
		p.OwnerEnv = "KANBANZAI_OWNER"
	}
	p.Isolation = asString(flow["isolation"], "worktree")
	p.WIPLimit = asInt(flow["wip_limit"], 5)
	if p.WIPLimit == 0 {
		p.WIPLimit = 5
	}
	p.StaleDays = asInt(flow["stale_days"], 7)
	if p.StaleDays == 0 {
		p.StaleDays = 7
	}
	p.MoveGraceMinutes = asFloat(flow["move_grace_minutes"], 10)
	if p.MoveGraceMinutes == 0 {
		p.MoveGraceMinutes = 10
	}
	if err := p.columns(); err != nil {
		return err
	}
	if err := p.steps(); err != nil {
		return err
	}
	pullOrder, err := stringList(flow["pull_order"])
	if err != nil {
		return errf("flow.pull_order must be a list of column keys")
	}
	for _, key := range pullOrder {
		column, ok := p.ByKey[key]
		if !ok {
			return errf("flow.pull_order names '%s', which is not a column: set flow.pull_order when you set "+
				"[[columns]]", key)
		}
		if column.Next == "" {
			return errf("flow.pull_order names '%s', which has no next step", key)
		}
	}
	p.PullOrder = pullOrder

	typedPattern, looseTyped := "", ""
	if p.Typed {
		typedPattern = p.TypeSet.IDPattern()
		looseTyped = p.TypeSet.LooseIDPattern()
	}
	legacyPattern, looseLegacy := "", ""
	if p.Prefix != "" {
		legacyPattern = regexp.QuoteMeta(p.Prefix) + `-\d{` + strconv.Itoa(p.Digits) + `,}`
		looseLegacy = regexp.QuoteMeta(p.Prefix) + `-(\d+)`
	}
	p.IDPattern = alternatives(typedPattern, legacyPattern)
	p.CardRe = regexp.MustCompile(`^` + p.IDPattern + `$`)
	p.TitleRe = regexp.MustCompile(`^# (` + p.IDPattern + `): (.+)$`)
	p.CardFileRe = regexp.MustCompile(`^(` + p.IDPattern + `)-.+\.md$`)
	p.IDInRe = regexp.MustCompile(`(?i)(?:` + alternatives(looseTyped, looseLegacy) + `)`)
	if looseLegacy != "" {
		p.ParseRe = regexp.MustCompile(`(?i)^` + looseLegacy + `$`)
	}
	return nil
}

// alternatives is a non-capturing group of the two patterns, or the one that is set.
func alternatives(first, second string) string {
	switch {
	case first != "" && second != "":
		return "(?:" + first + "|" + second + ")"
	case first != "":
		return first
	default:
		return second
	}
}

// types reads the project's [[types]] list. An empty list keeps the legacy single-prefix ids.
func (p *Project) types() ([]typeid.Type, error) {
	raw, ok := p.cfg["types"]
	if !ok || raw == nil {
		return nil, nil
	}
	items, ok := raw.([]any)
	if !ok {
		return nil, errf("types must be a list of [[types]] tables")
	}
	out := make([]typeid.Type, 0, len(items))
	for index, item := range items {
		entry, ok := item.(map[string]any)
		if !ok {
			return nil, errf("[[types]] #%d must be a table", index+1)
		}
		value := typeid.Type{
			Key:         asString(entry["key"], ""),
			Label:       asString(entry["label"], ""),
			Prefix:      asString(entry["prefix"], ""),
			Schema:      asString(entry["schema"], ""),
			Colour:      asString(entry["colour"], ""),
			Icon:        asString(entry["icon"], ""),
			Description: asString(entry["description"], ""),
		}
		if template := strings.TrimSpace(asString(entry["template"], "")); template != "" {
			path, err := p.path(template)
			if err != nil {
				return nil, err
			}
			value.Template = path
		}
		out = append(out, value)
	}
	return out, nil
}

func (p *Project) section(name string) (map[string]any, error) {
	value, ok := p.cfg[name]
	if !ok || value == nil {
		return map[string]any{}, nil
	}
	table, ok := value.(map[string]any)
	if !ok {
		return nil, errf("[%s] must be a table", name)
	}
	return table, nil
}

func (p *Project) columns() error {
	raw, ok := p.cfg["columns"].([]any)
	if !ok || len(raw) == 0 {
		return errf("there are no [[columns]]")
	}
	columns := make([]Column, 0, len(raw))
	allKeys := make([]string, 0, len(raw))
	for _, item := range raw {
		if entry, ok := item.(map[string]any); ok {
			allKeys = append(allKeys, asString(entry["key"], ""))
		}
	}
	for _, item := range raw {
		entry, ok := item.(map[string]any)
		if !ok || asString(entry["key"], "") == "" || asString(entry["folder"], "") == "" {
			return errf("every [[columns]] entry needs a key and a folder")
		}
		key := asString(entry["key"], "")
		kind := asString(entry["kind"], "open")
		if kind == "held" {
			return errf("column '%s': the 'held' kind is gone (W-92): a blocked or waiting card stays in the "+
				"column it finished, and a '- **Held:**' line in its brief says so; remove this column", key)
		}
		if !contains(kinds, kind) {
			return errf("column '%s': kind must be one of %s", key, strings.Join(kinds, ", "))
		}
		started := entry["started"]
		if started == nil {
			started = false
		}
		switch value := started.(type) {
		case bool:
		case string:
			if value != "claimed" {
				return errf("column '%s': started must be true, false or \"claimed\"", key)
			}
		default:
			return errf("column '%s': started must be true, false or \"claimed\"", key)
		}
		words, err := stringList(entry["words"])
		if err != nil {
			return errf("column '%s': words must be a list", key)
		}
		if len(words) == 0 {
			words = []string{key}
		}
		for i := range words {
			words[i] = strings.ToLower(words[i])
		}
		gate, err := ParseGate(entry, key, kind, asString(entry["doing"], ""), allKeys)
		if err != nil {
			return err
		}
		columns = append(columns, Column{
			Key:           key,
			Label:         asString(entry["label"], key),
			Words:         words,
			Folder:        asString(entry["folder"], ""),
			Kind:          kind,
			Started:       started,
			Next:          asString(entry["next"], ""),
			SettledNext:   asString(entry["settled_next"], ""),
			Title:         asString(entry["title"], ""),
			SettledTitle:  asString(entry["settled_title"], ""),
			Worktree:      asBool(entry["worktree"]),
			WaitsForOwner: gate.Type == "owner" || asBool(entry["waits_for_owner"]),
			Gate:          gate,
			Lane:          asString(entry["lane"], ""),
			Doing:         asString(entry["doing"], ""),
		})
	}
	keys := make([]string, 0, len(columns))
	for _, column := range columns {
		keys = append(keys, column.Key)
	}
	if len(uniqueStrings(keys)) != len(keys) {
		return errf("two columns share a key")
	}
	p.Columns = columns
	p.ByKey = make(map[string]Column, len(columns))
	p.StageColumn = map[string]string{}
	for _, column := range columns {
		p.ByKey[column.Key] = column
		for _, word := range column.Words {
			if _, exists := p.StageColumn[word]; exists {
				return errf("the stage word '%s' belongs to two columns", word)
			}
			p.StageColumn[word] = column.Key
		}
	}
	p.Folders = nil
	for _, column := range columns {
		if !contains(p.Folders, column.Folder) {
			p.Folders = append(p.Folders, column.Folder)
		}
	}
	p.OpenKeys = nil
	for _, column := range columns {
		if column.Kind == "open" {
			p.OpenKeys = append(p.OpenKeys, column.Key)
		}
	}
	return nil
}

func (p *Project) steps() error {
	raw, err := p.section("steps")
	if err != nil {
		return err
	}
	steps := make(map[string]Step, len(raw))
	names := make([]string, 0, len(raw))
	for name, item := range raw {
		entry, ok := item.(map[string]any)
		if !ok {
			return errf("[steps.%s] must be a table", name)
		}
		into := asString(entry["into"], "")
		extend, err := stringList(entry["extend"])
		if err != nil {
			return errf("step '%s': extend must be a list", name)
		}
		context, err := stringList(entry["context"])
		if err != nil {
			return errf("step '%s': context must be a list", name)
		}
		checks, err := stringList(entry["checks"])
		if err != nil {
			return errf("step '%s': checks must be a list", name)
		}
		skipField := asString(entry["skip_field"], "")
		skipValues, err := stringList(entry["skip_values"])
		if err != nil {
			return errf("step '%s': skip_values must be a list", name)
		}
		if (skipField == "") != (len(skipValues) == 0) {
			return errf("step '%s': skip_field and skip_values go together: set both or neither", name)
		}
		for _, value := range skipValues {
			if strings.TrimSpace(value) == "" {
				return errf("step '%s': skip_values must not hold an empty value", name)
			}
		}
		during := asString(entry["during"], "")
		if _, ok := p.ByKey[during]; !ok {
			during = ""
		}
		_, active := p.ByKey[into]
		steps[name] = Step{
			Name:    name,
			Into:    into,
			Skill:   asString(entry["skill"], ""),
			Extend:  extend,
			Context: context,
			Checks:  checks,
			Active:  active,
			During:  during,

			SkipField: skipField, SkipValues: skipValues,
		}
		names = append(names, name)
	}
	sort.Strings(names)
	p.Steps = steps
	p.StepOrder = names

	for _, column := range p.Columns {
		for _, name := range []string{column.Next, column.SettledNext} {
			if name == "" {
				continue
			}
			step, ok := p.Steps[name]
			if !ok {
				return errf("column '%s': its next step '%s' is not defined in [steps]", column.Key, name)
			}
			if !step.Active {
				return errf("column '%s': step '%s' moves a card into '%s', which is not a column",
					column.Key, name, step.Into)
			}
		}
	}
	for _, column := range p.Columns {
		if column.Doing == "" {
			continue
		}
		step, ok := p.Steps[column.Doing]
		if !ok {
			return errf("column '%s': its `doing` names '%s', a step that is not defined in [steps]",
				column.Key, column.Doing)
		}
		if step.During != column.Key {
			return errf("column '%s': it is the Doing column of step '%s', but that step's `during` is '%s', "+
				"not '%s'", column.Key, column.Doing, step.During, column.Key)
		}
	}
	for name, step := range p.Steps {
		if step.During == "" {
			continue
		}
		column, ok := p.ByKey[step.During]
		if !ok {
			continue
		}
		if column.Doing != name {
			return errf("step '%s': its `during` column '%s' must say `doing = \"%s\"`", name, step.During, name)
		}
	}
	return nil
}

// Expand replaces {git_common}, {repo} and {id} in a setting.
func (p *Project) Expand(text, card string) (string, error) {
	var firstErr error
	placeholder := regexp.MustCompile(`\{(git_common|repo|id)\}`)
	out := placeholder.ReplaceAllStringFunc(text, func(found string) string {
		switch found[1 : len(found)-1] {
		case "repo":
			return filepath.Base(p.Root)
		case "id":
			if card != "" {
				return card
			}
			return found
		case "git_common":
			if p.GitCommon == "" {
				firstErr = errf("{git_common} is used, but the project is not a git checkout")
				return found
			}
			return p.GitCommon
		}
		return found
	})
	return out, firstErr
}

// Path is a configured path: expanded, and relative to the main checkout unless absolute.
func (p *Project) Path(value string) (string, error) {
	return p.path(value)
}

func (p *Project) path(value string) (string, error) {
	expanded, err := p.Expand(value, "")
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(expanded) {
		expanded = filepath.Join(p.Root, expanded)
	}
	return filepath.Clean(expanded), nil
}

// Rel is a path relative to the main checkout with forward slashes, or absolute when it lies outside it.
func (p *Project) Rel(path string) string {
	target := filepath.Clean(path)
	rel, err := filepath.Rel(p.Root, target)
	if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return filepath.ToSlash(rel)
	}
	return target
}

// WorktreeHint is where a card's worktree goes, as the setting writes it.
func (p *Project) WorktreeHint(card string) string {
	hint, err := p.Expand(p.WorktreePattern, card)
	if err != nil {
		return p.WorktreePattern
	}
	return hint
}

// WorktreePath is the absolute path of a card's worktree.
func (p *Project) WorktreePath(card string) string {
	path, err := p.path(p.WorktreeHint(card))
	if err != nil {
		return ""
	}
	return path
}

// Branch is a card's branch, or its batch branch.
func (p *Project) Branch(card string, batch bool) string {
	pattern := p.BranchPattern
	if batch {
		pattern = p.BatchBranchPattern
	}
	branch, err := p.Expand(pattern, card)
	if err != nil {
		return pattern
	}
	return branch
}

// FormatID is a number as a card id. In a typed project it uses the default type; callers that know the type use
// TypeSet.Format instead.
func (p *Project) FormatID(number int) string {
	if p.TypeSet != nil {
		if item, ok := p.TypeSet.DefaultType(); ok {
			return p.TypeSet.Format(item, number)
		}
	}
	return p.legacyID(number)
}

// legacyID is a number as the legacy prefix's card id.
func (p *Project) legacyID(number int) string {
	return fmt.Sprintf("%s-%0*d", p.Prefix, p.Digits, number)
}

// IDsExample is an id shape for a message: TASK-NN in a typed project, W-NN in a legacy one.
func (p *Project) IDsExample() string {
	if p.TypeSet != nil {
		return p.TypeSet.Example()
	}
	return p.Prefix + "-NN"
}

// LegacyExample is the legacy prefix's id shape, or empty when there is no legacy prefix.
func (p *Project) LegacyExample() string {
	if p.Prefix == "" {
		return ""
	}
	return p.Prefix + "-NN"
}

// IDExamples are the id shapes a message can offer: the default type and the legacy prefix.
func (p *Project) IDExamples() []string {
	out := []string{p.IDsExample()}
	if legacy := p.LegacyExample(); legacy != "" && legacy != out[0] {
		out = append(out, legacy)
	}
	return out
}

// IsCardID reports whether value is a canonical card id.
func (p *Project) IsCardID(value string) bool {
	return p.CardRe.MatchString(value)
}

// CanonicalCardID returns value as a canonical card id, or false. Input is case-insensitive; a typed id is padded
// to the configured digits, and the legacy prefix is accepted as a legacy card.
func (p *Project) CanonicalCardID(value string) (string, bool) {
	if p.TypeSet != nil {
		if card, ok := p.TypeSet.Canonical(value); ok {
			return card, true
		}
	}
	if p.ParseRe == nil {
		return "", false
	}
	match := p.ParseRe.FindStringSubmatch(value)
	if match == nil {
		return "", false
	}
	number, err := strconv.Atoi(match[1])
	if err != nil || number <= 0 {
		return "", false
	}
	return p.legacyID(number), true
}

// CardIDIn is the first card id inside a branch name or a path, such as batch/W-07 or /x/proj-W-10.
func (p *Project) CardIDIn(value string) (string, bool) {
	match := p.IDInRe.FindString(value)
	if match == "" {
		return "", false
	}
	return p.CanonicalCardID(match)
}

// CardNumber is the numeric part of a card id, typed or legacy.
func (p *Project) CardNumber(value string) (int, bool) {
	if p.TypeSet != nil {
		if _, number, ok := p.TypeSet.Parse(value); ok {
			return number, true
		}
	}
	if p.ParseRe == nil {
		return 0, false
	}
	match := p.ParseRe.FindStringSubmatch(strings.TrimSpace(value))
	if match == nil {
		return 0, false
	}
	number, err := strconv.Atoi(match[1])
	if err != nil || number <= 0 {
		return 0, false
	}
	return number, true
}

// TypeOf is the type of a card id, in a typed project. A legacy id has no type.
func (p *Project) TypeOf(value string) (typeid.Type, bool) {
	if p.TypeSet == nil {
		return typeid.Type{}, false
	}
	item, _, ok := p.TypeSet.Parse(value)
	return item, ok
}

// CardTypeInfo is the card's type for JSON, or nil for a legacy card and in a legacy project.
func (p *Project) CardTypeInfo(value string) *typeid.TypeInfo {
	item, ok := p.TypeOf(value)
	if !ok {
		return nil
	}
	info := item.Info()
	return &info
}

// CardPrefixes are every accepted id prefix: the configured types and the legacy prefix.
func (p *Project) CardPrefixes() []string {
	out := []string{}
	if p.TypeSet != nil {
		out = append(out, p.TypeSet.Prefixes()...)
	}
	if p.Prefix != "" {
		out = append(out, p.Prefix)
	}
	return out
}

// TypePrefixes are the configured type prefixes only.
func (p *Project) TypePrefixes() []string {
	if p.TypeSet != nil {
		return p.TypeSet.Prefixes()
	}
	return nil
}

// TypeInfos are the configured types in their JSON shape.
func (p *Project) TypeInfos() []any {
	out := make([]any, 0, len(p.Types))
	for _, item := range p.Types {
		info := item.Info()
		entry := map[string]any{"key": info.Key, "label": info.Label, "prefix": info.Prefix}
		if info.Template != "" {
			entry["template"] = info.Template
		}
		if info.Colour != "" {
			entry["colour"] = info.Colour
		}
		if info.Icon != "" {
			entry["icon"] = info.Icon
		}
		if info.Description != "" {
			entry["description"] = info.Description
		}
		out = append(out, entry)
	}
	return out
}

// TypeInfoList are the configured types in the board's JSON shape.
func (p *Project) TypeInfoList() []typeid.TypeInfo {
	out := make([]typeid.TypeInfo, 0, len(p.Types))
	for _, item := range p.Types {
		out = append(out, item.Info())
	}
	return out
}

// TemplateText is the shipped card template, or the project's own when it names one. Line endings are Unix: a
// checkout on Windows must not change the template the command tests against.
func (p *Project) TemplateText() (string, error) {
	if p.TemplateIsShipped {
		data, err := kanbanzai.Files.ReadFile("templates/card.md")
		if err != nil {
			return "", err
		}
		return strings.ReplaceAll(string(data), "\r\n", "\n"), nil
	}
	data, err := os.ReadFile(p.Template)
	if err != nil {
		return "", err
	}
	return strings.ReplaceAll(string(data), "\r\n", "\n"), nil
}

// ContextFiles is the project's context files, absolute.
func (p *Project) ContextFiles() []string {
	out := make([]string, 0, len(p.Context))
	for _, item := range p.Context {
		path, err := p.path(item)
		if err != nil {
			out = append(out, item)
			continue
		}
		out = append(out, path)
	}
	return out
}

// Label is a column's label, or its key when there is no such column.
func (p *Project) Label(key string) string {
	if column, ok := p.ByKey[key]; ok {
		return column.Label
	}
	return key
}

// AsDict is the resolved configuration, with absolute paths, in the shape `config --json` prints.
func (p *Project) AsDict() map[string]any {
	project := map[string]any{
		"prefix": p.Prefix, "digits": p.Digits, "owner": p.Owner, "main_branch": p.MainBranch,
		"home": p.Home, "work": p.Work, "state": p.State, "claims": p.Claims, "numbers": p.Numbers,
		"batches": p.Batches, "template": p.Template, "worktree": p.WorktreePattern,
		"branch": p.BranchPattern, "batch_branch": p.BatchBranchPattern, "context": p.ContextFiles(),
		"typed": p.Typed, "default_type": p.DefaultType, "project_key": p.ProjectKey,
		"types": p.TypeInfos(),
	}
	columns := make([]any, 0, len(p.Columns))
	for _, column := range p.Columns {
		columns = append(columns, map[string]any{
			"key": column.Key, "label": column.Label, "words": column.Words, "folder": column.Folder,
			"kind": column.Kind, "started": column.Started, "next": optional(column.Next),
			"settled_next": optional(column.SettledNext), "title": optional(column.Title),
			"settled_title": optional(column.SettledTitle), "worktree": column.Worktree,
			"waits_for_owner": column.WaitsForOwner, "lane": optional(column.Lane),
			"doing": optional(column.Doing), "gate": optional(column.Gate.Type),
			"gate_name": optional(column.Gate.Name), "gate_after": optional(column.Gate.After),
			"gate_until": optional(column.Gate.Until),
		})
	}
	steps := make(map[string]any, len(p.Steps))
	for name, step := range p.Steps {
		steps[name] = map[string]any{
			"into": step.Into, "skill": step.Skill, "extend": step.Extend, "context": step.Context,
			"checks": step.Checks, "active": step.Active, "during": optional(step.During),
			"skip_field": optional(step.SkipField), "skip_values": step.SkipValues,
		}
	}
	return map[string]any{
		"config_version": p.ConfigVersion, "config_allow_unknown": p.ConfigAllowUnknown,
		"root": p.Root, "toplevel": p.Toplevel, "git_common": optional(p.GitCommon),
		"config_file": optional(p.ConfigFile),
		"project":     project,
		"claims":      map[string]any{"lease_minutes": p.LeaseMinutes, "owner_env": p.OwnerEnv},
		"flow": map[string]any{"wip_limit": p.WIPLimit, "stale_days": p.StaleDays,
			"move_grace_minutes": p.MoveGraceMinutes, "pull_order": p.PullOrder, "isolation": p.Isolation},
		"columns": columns,
		"steps":   steps,
	}
}

// merge puts over on top of base: tables merge, every other value (lists included) replaces.
func merge(base, over map[string]any) map[string]any {
	out := make(map[string]any, len(base))
	for key, value := range base {
		out[key] = value
	}
	for key, value := range over {
		if overTable, ok := value.(map[string]any); ok {
			if baseTable, ok := out[key].(map[string]any); ok {
				out[key] = merge(baseTable, overTable)
				continue
			}
		}
		out[key] = value
	}
	return out
}

func asString(value any, fallback string) string {
	if value == nil {
		return fallback
	}
	switch typed := value.(type) {
	case string:
		if typed == "" {
			return fallback
		}
		return typed
	default:
		return fmt.Sprint(typed)
	}
}

func asInt(value any, fallback int) int {
	switch typed := value.(type) {
	case int64:
		if typed == 0 {
			return fallback
		}
		return int(typed)
	case float64:
		if typed == 0 {
			return fallback
		}
		return int(typed)
	case string:
		if typed == "" {
			return fallback
		}
		parsed, err := strconv.Atoi(typed)
		if err != nil {
			return fallback
		}
		if parsed == 0 {
			return fallback
		}
		return parsed
	default:
		return fallback
	}
}

func asFloat(value any, fallback float64) float64 {
	switch typed := value.(type) {
	case int64:
		if typed == 0 {
			return fallback
		}
		return float64(typed)
	case float64:
		if typed == 0 {
			return fallback
		}
		return typed
	case string:
		if typed == "" {
			return fallback
		}
		parsed, err := strconv.ParseFloat(typed, 64)
		if err != nil {
			return fallback
		}
		if parsed == 0 {
			return fallback
		}
		return parsed
	default:
		return fallback
	}
}

func asBool(value any) bool {
	parsed, _ := value.(bool)
	return parsed
}

func stringList(value any) ([]string, error) {
	if value == nil {
		return nil, nil
	}
	list, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("not a list")
	}
	out := make([]string, 0, len(list))
	for _, item := range list {
		out = append(out, fmt.Sprint(item))
	}
	return out, nil
}

func optional(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func or(value, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}

func contains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]bool, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		if !seen[value] {
			seen[value] = true
			out = append(out, value)
		}
	}
	return out
}

func absolute(path string) string {
	value, err := filepath.Abs(path)
	if err != nil {
		return filepath.Clean(path)
	}
	if resolved, err := filepath.EvalSymlinks(value); err == nil {
		return resolved
	}
	return filepath.Clean(value)
}

func absoluteOrEmpty(path string) string {
	if path == "" {
		return ""
	}
	return absolute(path)
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}
