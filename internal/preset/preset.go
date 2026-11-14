// Package preset loads delivery presets: a named, complete starting configuration for a kind of work.
//
// A preset is a folder and nothing in it is a new format:
//
//	flow.toml       a flow file (the [flow], [[columns]] and [steps.<name>] tables `workflow` already speaks), in the
//	                exact form `workflow export` writes it
//	config.toml     a fragment of the project config: [preset] (name, delivery_style, version and the descriptions
//	                summary, best_for, not_for), a few [project] keys, [[types]] and [schemas.*]
//	templates/*.md  the brief templates the types name (as .kanbanzai/templates/<file>)
//
// Shipped presets are embedded in the command (presets/<name>); a path to a folder of the same shape is a local
// preset. `init --preset` and `workflow plan --preset` both call Load, and nothing else reads a preset.
package preset

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/cloudbusting/kanbanzai"
	"github.com/cloudbusting/kanbanzai/internal/check"
	"github.com/cloudbusting/kanbanzai/internal/config"
	"github.com/cloudbusting/kanbanzai/internal/flow"
)

// maxFileBytes bounds every file of a local preset; a real one is a few kilobytes.
const maxFileBytes = 1 << 20

// TemplatesDir is where the project keeps a preset's templates; a type's `template` names a file in it.
const TemplatesDir = ".kanbanzai/templates/"

// projectKeys are the only [project] keys a preset may set. Everything else (the work and state folders, the main
// branch, the owner, the claims) belongs to the project.
var projectKeys = []string{"default_type", "worktree", "branch", "batch_branch", "template"}

var typeKeys = []string{"key", "label", "prefix", "template", "schema", "colour", "icon", "description"}
var schemaKeys = []string{"required_fields", "optional_fields", "required_sections", "optional_sections",
	"repeatable_sections", "section_order", "final_section"}
var presetKeys = []string{"name", "delivery_style", "version", "summary", "best_for", "not_for"}

var nameRE = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// Preset is a loaded, validated preset.
type Preset struct {
	Name          string
	DeliveryStyle string
	Version       int
	Summary       string
	BestFor       string
	NotFor        string
	// Source is "shipped" for an embedded preset, else the folder it was read from.
	Source string

	Flow      *flow.Doc
	Project   map[string]any // the allow-listed [project] keys
	Types     []map[string]any
	Schemas   map[string]map[string]any
	Templates map[string]string // file name under TemplatesDir -> text
}

// Settings are what the project, not the preset, decides.
type Settings struct {
	Owner      string
	MainBranch string
	Context    []string
}

// Problem is a preset that cannot be used.
type Problem struct{ Message string }

func (p *Problem) Error() string { return p.Message }

func problem(format string, args ...any) error {
	return &Problem{Message: fmt.Sprintf(format, args...)}
}

// List is the names of the shipped presets, sorted.
func List() ([]string, error) {
	entries, err := kanbanzai.Files.ReadDir("presets")
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names, nil
}

// IsPath says whether ref names a folder rather than a shipped preset.
func IsPath(ref string) bool {
	return strings.ContainsAny(ref, `/\`) || strings.HasPrefix(ref, ".")
}

// Load reads and validates a preset: a shipped name, or the path of a folder.
func Load(ref string) (*Preset, error) {
	var root fs.FS
	source := "shipped"
	if IsPath(ref) {
		info, err := os.Stat(ref)
		if err != nil || !info.IsDir() {
			return nil, problem("--preset %s: not a folder of a preset", ref)
		}
		root, source = os.DirFS(ref), ref
	} else {
		sub, err := fs.Sub(kanbanzai.Files, "presets/"+ref)
		if err != nil {
			return nil, problem("--preset %s: no such preset", ref)
		}
		if _, err := fs.Stat(sub, "flow.toml"); err != nil {
			names, _ := List()
			return nil, problem("--preset %s: no such preset (shipped: %s)", ref, strings.Join(names, ", "))
		}
		root = sub
	}
	p, err := read(root, source)
	if err != nil {
		return nil, err
	}
	if source == "shipped" && p.Name != ref {
		return nil, problem("preset %s: its config.toml names it %q", ref, p.Name)
	}
	if err := p.checkCanonicalFlow(); err != nil {
		return nil, err
	}
	if err := p.validate(); err != nil {
		return nil, err
	}
	return p, nil
}

func readFile(root fs.FS, source, name string) ([]byte, error) {
	info, err := fs.Stat(root, name)
	if err != nil {
		return nil, problem("preset %s: %s is missing", source, name)
	}
	if !info.Mode().IsRegular() || info.Size() > maxFileBytes {
		return nil, problem("preset %s: %s must be a regular file of at most %d bytes", source, name, maxFileBytes)
	}
	data, err := fs.ReadFile(root, name)
	if err != nil {
		return nil, problem("preset %s: %s: %v", source, name, err)
	}
	return data, nil
}

func read(root fs.FS, source string) (*Preset, error) {
	p := &Preset{Source: source, Schemas: map[string]map[string]any{}, Templates: map[string]string{}}
	data, err := readFile(root, source, "flow.toml")
	if err != nil {
		return nil, err
	}
	if p.Flow, err = flow.Parse(data, false); err != nil {
		return nil, problem("preset %s: flow.toml: %v", source, err)
	}
	data, err = readFile(root, source, "config.toml")
	if err != nil {
		return nil, err
	}
	raw, err := config.ParseTOML(string(data))
	if err != nil {
		return nil, problem("preset %s: config.toml: %v", source, err)
	}
	for key := range raw {
		switch key {
		case "preset", "project", "types", "schemas":
		default:
			return nil, problem("preset %s: config.toml may hold only [preset], [project], [[types]] and [schemas.*], not %q "+
				"(the flow is in flow.toml)", source, key)
		}
	}
	if err := p.readRecord(raw["preset"], source); err != nil {
		return nil, err
	}
	if project, ok := raw["project"].(map[string]any); ok {
		for key, value := range project {
			if !contains(projectKeys, key) {
				return nil, problem("preset %s: config.toml [project] may set only %s, not %q: the rest is the project's own",
					source, strings.Join(projectKeys, ", "), key)
			}
			if _, ok := value.(string); !ok {
				return nil, problem("preset %s: config.toml [project] %s must be a string", source, key)
			}
		}
		p.Project = project
	} else if raw["project"] != nil {
		return nil, problem("preset %s: config.toml [project] must be a table", source)
	}
	if items, ok := raw["types"].([]any); ok {
		for _, item := range items {
			entry, ok := item.(map[string]any)
			if !ok {
				return nil, problem("preset %s: every [[types]] entry must be a table", source)
			}
			for key := range entry {
				if !contains(typeKeys, key) {
					return nil, problem("preset %s: [[types]] has an unknown key %q", source, key)
				}
			}
			p.Types = append(p.Types, entry)
		}
	} else if raw["types"] != nil {
		return nil, problem("preset %s: types must be a list of [[types]] tables", source)
	}
	if schemas, ok := raw["schemas"].(map[string]any); ok {
		for name, item := range schemas {
			entry, ok := item.(map[string]any)
			if !ok {
				return nil, problem("preset %s: [schemas.%s] must be a table", source, name)
			}
			p.Schemas[name] = entry
		}
	} else if raw["schemas"] != nil {
		return nil, problem("preset %s: schemas must be [schemas.<name>] tables", source)
	}
	// Every template a type names must be in the folder, as templates/<file>; every file there must be named.
	named := map[string]bool{}
	for _, item := range p.Types {
		template, _ := item["template"].(string)
		if template == "" {
			continue
		}
		file := strings.TrimPrefix(template, TemplatesDir)
		if file == template || file == "" || strings.ContainsAny(file, `/\`) || strings.HasPrefix(file, ".") {
			return nil, problem("preset %s: type %v: template %q must be %s<file>.md", source, item["key"], template, TemplatesDir)
		}
		named[file] = true
		text, err := readFile(root, source, "templates/"+file)
		if err != nil {
			return nil, err
		}
		p.Templates[file] = string(text)
	}
	if entries, err := fs.ReadDir(root, "templates"); err == nil {
		for _, e := range entries {
			if !named[e.Name()] {
				return nil, problem("preset %s: templates/%s is not named by any type", source, e.Name())
			}
		}
	}
	return p, nil
}

func (p *Preset) readRecord(value any, source string) error {
	record, ok := value.(map[string]any)
	if !ok {
		return problem("preset %s: config.toml needs a [preset] table (%s)", source, strings.Join(presetKeys, ", "))
	}
	for key := range record {
		if !contains(presetKeys, key) {
			return problem("preset %s: [preset] has an unknown key %q", source, key)
		}
	}
	text := func(key string) (string, error) {
		s, _ := record[key].(string)
		if strings.TrimSpace(s) == "" || strings.ContainsAny(s, "\r\n") {
			return "", problem("preset %s: [preset] %s must be one line of text", source, key)
		}
		return s, nil
	}
	var err error
	if p.Name, err = text("name"); err != nil {
		return err
	}
	if !nameRE.MatchString(p.Name) {
		return problem("preset %s: [preset] name %q must be lower-case letters, digits and hyphens", source, p.Name)
	}
	if p.DeliveryStyle, err = text("delivery_style"); err != nil {
		return err
	}
	if p.Summary, err = text("summary"); err != nil {
		return err
	}
	if p.BestFor, err = text("best_for"); err != nil {
		return err
	}
	if p.NotFor, err = text("not_for"); err != nil {
		return err
	}
	version, ok := record["version"].(int64)
	if !ok || version < 1 || version > 1<<31 {
		return problem("preset %s: [preset] version must be a positive integer", source)
	}
	p.Version = int(version)
	return nil
}

func contains(list []string, want string) bool {
	for _, item := range list {
		if item == want {
			return true
		}
	}
	return false
}

// leadingComments drops the comment lines at the top of a flow file.
func leadingComments(text string) string {
	lines := strings.Split(text, "\n")
	i := 0
	for i < len(lines) && (strings.HasPrefix(lines[i], "#") || strings.TrimSpace(lines[i]) == "") {
		i++
	}
	return strings.Join(lines[i:], "\n")
}

// resolve loads the project this preset would make, over a scratch root that holds its templates.
func (p *Preset) resolve(settings Settings) (*config.Project, func(), error) {
	root, err := os.MkdirTemp("", "kanbanzai-preset-")
	if err != nil {
		return nil, nil, err
	}
	cleanup := func() { _ = os.RemoveAll(root) }
	if len(p.Templates) > 0 {
		dir := filepath.Join(root, filepath.FromSlash(TemplatesDir))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			cleanup()
			return nil, nil, err
		}
		for file, text := range p.Templates {
			if err := os.WriteFile(filepath.Join(dir, file), []byte(text), 0o644); err != nil {
				cleanup()
				return nil, nil, err
			}
		}
	}
	project, err := config.ValidateText(root, root, "", p.ConfigText(settings))
	if err != nil {
		cleanup()
		return nil, nil, problem("preset %s: %v", p.label(), err)
	}
	return project, cleanup, nil
}

func (p *Preset) label() string {
	if p.Source == "shipped" || p.Source == "" {
		return p.Name
	}
	return p.Name + " (" + p.Source + ")"
}

// checkCanonicalFlow requires flow.toml to be what `workflow export` writes for the project it resolves to, apart
// from its leading comments, so a preset round-trips through export and cannot rot unnoticed.
func (p *Preset) checkCanonicalFlow() error {
	project, cleanup, err := p.resolve(Settings{Owner: "Preset Check"})
	if err != nil {
		return err
	}
	defer cleanup()
	want := flow.FromProject(project).Text
	if got := leadingComments(p.Flow.Text); got != want {
		return problem("preset %s: flow.toml is not what `workflow export` writes for the flow it makes: replace it with that "+
			"text (leading comments may stay)", p.label())
	}
	return nil
}

// validate runs every check a preset must pass before anything uses it.
func (p *Preset) validate() error {
	project, cleanup, err := p.resolve(Settings{Owner: "Preset Check"})
	if err != nil {
		return err
	}
	defer cleanup()
	if report := flow.Validate(project, p.Flow, ""); !report.OK {
		var messages []string
		for _, f := range report.Findings {
			if f.Severity == flow.Error {
				messages = append(messages, f.Message)
			}
		}
		return problem("preset %s: its flow is not valid: %s", p.label(), strings.Join(messages, "; "))
	}
	if problems := check.TypeTemplates(project); len(problems) > 0 {
		return problem("preset %s: a type template does not fit its schema: %s", p.label(), strings.Join(problems, "; "))
	}
	return nil
}
