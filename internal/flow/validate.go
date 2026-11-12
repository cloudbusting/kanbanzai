package flow

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	kanbanzai "github.com/cloudbusting/kanbanzai"
	"github.com/cloudbusting/kanbanzai/internal/config"
)

// Severities of a finding.
const (
	Error   = "error"
	Warning = "warning"
)

// Finding is one problem with a proposed flow: a stable id, a severity and one message.
type Finding struct {
	ID       string `json:"id"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
}

// Report is what `workflow validate` prints.
type Report struct {
	SchemaVersion int       `json:"schema_version"`
	OK            bool      `json:"ok"`
	Findings      []Finding `json:"findings"`
}

// Errors is how many findings are errors.
func (r *Report) Errors() int {
	n := 0
	for _, f := range r.Findings {
		if f.Severity == Error {
			n++
		}
	}
	return n
}

func (r *Report) add(severity, id, format string, args ...any) {
	r.Findings = append(r.Findings, Finding{ID: id, Severity: severity, Message: fmt.Sprintf(format, args...)})
}

// col, stp are the tables as the validation reads them: the file's, over the shipped defaults.
type col struct {
	key, folder, kind, next, settled, doing string
	words                                   []string
}

type stp struct {
	name, into, during, skill string
	refs                      []string // extend and checks
	inFile                    bool
}

// Validate reports every problem with a proposed flow, in the order a person would fix them, without writing
// anything. The graph checks read the flow as the config would merge it over the shipped defaults; the config's
// own loader then has the last word, and its message is reported only when nothing above explained the refusal.
func Validate(p *config.Project, doc *Doc, oldConfigText string) *Report {
	r := &Report{SchemaVersion: 1, Findings: []Finding{}}
	defaults, derr := defaultsRaw()
	if derr != nil {
		r.add(Error, "defaults", "%v", derr)
		return r
	}
	columns, steps, pull := effective(defaults, doc.Raw)
	checkColumns(r, columns)
	checkSteps(r, columns, steps)
	checkPull(r, columns, pull)
	checkGraph(r, columns, steps)
	checkSkills(r, p.Root, steps)
	if r.Errors() == 0 {
		if _, err := Resolve(p, doc, oldConfigText); err != nil {
			r.add(Error, "config.invalid", "%v", err)
		}
	}
	r.OK = r.Errors() == 0
	return r
}

func defaultsRaw() (map[string]any, error) {
	data, err := kanbanzai.Files.ReadFile("defaults/config.toml")
	if err != nil {
		return nil, err
	}
	return config.ParseTOML(string(data))
}

func str(m map[string]any, key string) string {
	s, _ := m[key].(string)
	return s
}

func strList(m map[string]any, key string) []string {
	list, _ := m[key].([]any)
	var out []string
	for _, v := range list {
		if s, ok := v.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

// effective is the columns, steps and pull order the config would have: the file's columns replace the default
// list, the file's [flow] and each [steps.<name>] merge key by key over the defaults.
func effective(defaults, file map[string]any) ([]col, []stp, []string) {
	items, ok := file["columns"].([]any)
	if !ok {
		items, _ = defaults["columns"].([]any)
	}
	var columns []col
	for _, item := range items {
		m, _ := item.(map[string]any)
		c := col{key: str(m, "key"), folder: str(m, "folder"), kind: str(m, "kind"), next: str(m, "next"),
			settled: str(m, "settled_next"), doing: str(m, "doing"), words: strList(m, "words")}
		if c.kind == "" {
			c.kind = "open"
		}
		if len(c.words) == 0 {
			c.words = []string{c.key}
		}
		columns = append(columns, c)
	}
	merged := map[string]map[string]any{}
	inFile := map[string]bool{}
	for _, source := range []map[string]any{defaults, file} {
		tables, _ := source["steps"].(map[string]any)
		for name, v := range tables {
			m, _ := v.(map[string]any)
			if merged[name] == nil {
				merged[name] = map[string]any{}
			}
			for k, value := range m {
				merged[name][k] = value
			}
		}
	}
	fileSteps, _ := file["steps"].(map[string]any)
	for name := range fileSteps {
		inFile[name] = true
	}
	names := make([]string, 0, len(merged))
	for name := range merged {
		names = append(names, name)
	}
	sort.Strings(names)
	var steps []stp
	for _, name := range names {
		m := merged[name]
		s := stp{name: name, into: str(m, "into"), during: str(m, "during"), skill: str(m, "skill"), inFile: inFile[name]}
		s.refs = append(append(s.refs, strList(m, "extend")...), strList(m, "checks")...)
		steps = append(steps, s)
	}
	flow := map[string]any{}
	for _, source := range []map[string]any{defaults, file} {
		t, _ := source["flow"].(map[string]any)
		for k, v := range t {
			flow[k] = v
		}
	}
	return columns, steps, strList(flow, "pull_order")
}

func checkColumns(r *Report, columns []col) {
	if len(columns) == 0 {
		r.add(Error, "flow.no-columns", "the flow has no [[columns]]")
		return
	}
	keys := map[string]bool{}
	words := map[string]string{}
	hasDone := false
	for i, c := range columns {
		where := fmt.Sprintf("column #%d", i+1)
		if c.key != "" {
			where = "column '" + c.key + "'"
		}
		if c.key == "" {
			r.add(Error, "column.key", "%s has no key", where)
		} else if keys[c.key] {
			r.add(Error, "column.duplicate-key", "two columns share the key '%s'", c.key)
		}
		keys[c.key] = true
		if c.folder == "" {
			r.add(Error, "column.folder", "%s has no folder", where)
		} else if !safeFolder(c.folder) {
			r.add(Error, "column.folder-unsafe", "%s: folder '%s' must be one plain name under the cards folder "+
				"(no '..', no path separator, not absolute)", where, c.folder)
		}
		switch c.kind {
		case "open", "parked", "done":
		default:
			r.add(Error, "column.kind", "%s: kind must be open, parked or done, not '%s'", where, c.kind)
		}
		if c.kind == "done" {
			hasDone = true
		}
		for _, w := range c.words {
			w = strings.ToLower(w)
			if other, dup := words[w]; dup && other != c.key {
				r.add(Error, "column.word-duplicate", "the stage word '%s' belongs to two columns ('%s' and '%s')", w, other, c.key)
			}
			words[w] = c.key
		}
	}
	if !hasDone {
		r.add(Error, "flow.no-done-column", "the flow has no column of kind \"done\": a card could never finish")
	}
}

func safeFolder(folder string) bool {
	if folder == "." || folder == ".." || filepath.IsAbs(folder) || strings.ContainsAny(folder, `/\:`) {
		return false
	}
	return !strings.Contains(folder, "..")
}

func checkSteps(r *Report, columns []col, steps []stp) {
	byKey := map[string]col{}
	for _, c := range columns {
		byKey[c.key] = c
	}
	byName := map[string]stp{}
	for _, s := range steps {
		byName[s.name] = s
	}
	used := map[string]bool{}
	for _, c := range columns {
		for _, name := range []string{c.next, c.settled} {
			if name == "" {
				continue
			}
			used[name] = true
			s, ok := byName[name]
			switch {
			case !ok:
				r.add(Error, "column.next-undefined", "column '%s': its next step '%s' is not defined in [steps]", c.key, name)
			case byKey[s.into].key == "":
				r.add(Error, "column.next-inactive", "column '%s': step '%s' moves a card into '%s', which is not a column",
					c.key, name, s.into)
			}
		}
		if c.doing == "" {
			continue
		}
		used[c.doing] = true
		s, ok := byName[c.doing]
		switch {
		case !ok:
			r.add(Error, "column.doing-undefined", "column '%s': its `doing` names '%s', a step that is not defined in [steps]", c.key, c.doing)
		case s.during != c.key:
			r.add(Error, "column.doing-mismatch", "column '%s' is the Doing column of step '%s', but that step's `during` is '%s'",
				c.key, c.doing, s.during)
		case byKey[s.into].key == "":
			r.add(Error, "column.next-inactive", "column '%s': step '%s' moves a card into '%s', which is not a column",
				c.key, c.doing, s.into)
		}
	}
	for _, s := range steps {
		if s.during != "" {
			if c, ok := byKey[s.during]; !ok {
				if s.inFile || used[s.name] {
					r.add(Error, "step.during-missing", "step '%s': its `during` column '%s' is not a column", s.name, s.during)
				}
			} else if c.doing != s.name {
				r.add(Error, "step.during-mismatch", "step '%s': its `during` column '%s' must say `doing = \"%s\"`", s.name, s.during, s.name)
			}
		}
		if s.inFile && !used[s.name] {
			r.add(Warning, "step.unused", "step '%s' is not the next or Doing step of any column", s.name)
		}
	}
}

func checkPull(r *Report, columns []col, pull []string) {
	byKey := map[string]col{}
	for _, c := range columns {
		byKey[c.key] = c
	}
	seen := map[string]bool{}
	for _, key := range pull {
		c, ok := byKey[key]
		switch {
		case !ok:
			r.add(Error, "pull.unknown", "flow.pull_order names '%s', which is not a column: set flow.pull_order when you set [[columns]]", key)
		case c.next == "":
			r.add(Error, "pull.no-next", "flow.pull_order names '%s', which has no next step", key)
		case seen[key]:
			r.add(Error, "pull.duplicate", "flow.pull_order names '%s' twice", key)
		}
		seen[key] = true
	}
}

// edges is the forward flow: where a card goes by doing its next step, finishing a step, or closing.
func edges(columns []col, steps []stp) map[string][]string {
	byName := map[string]stp{}
	for _, s := range steps {
		byName[s.name] = s
	}
	known := map[string]bool{}
	firstDone := ""
	for _, c := range columns {
		known[c.key] = true
		if c.kind == "done" && firstDone == "" {
			firstDone = c.key
		}
	}
	out := map[string][]string{}
	add := func(from, to string) {
		if known[to] && to != from {
			out[from] = append(out[from], to)
		}
	}
	for _, c := range columns {
		if c.kind != "open" {
			continue
		}
		for _, name := range []string{c.next, c.settled} {
			s, ok := byName[name]
			if name == "" || !ok {
				continue
			}
			if s.skill == "" {
				// No skill: a card skips the step or closes. The Doing column stays connected, though dormant.
				add(c.key, firstDone)
				add(c.key, s.into)
				add(c.key, s.during)
				continue
			}
			if s.during != "" {
				add(c.key, s.during)
			} else {
				add(c.key, s.into)
			}
		}
		if c.doing != "" {
			if s, ok := byName[c.doing]; ok {
				add(c.key, s.into)
			}
		}
	}
	return out
}

func checkGraph(r *Report, columns []col, steps []stp) {
	if len(columns) == 0 {
		return
	}
	forward := edges(columns, steps)
	reach := walk(forward, columns[0].key)
	for _, c := range columns {
		if (c.kind == "open" || c.kind == "done") && !reach[c.key] && c.key != "" {
			r.add(Error, "flow.unreachable", "column '%s' cannot be reached from the first column '%s'", c.key, columns[0].key)
		}
	}
	if cycle := findCycle(forward, columns); cycle != nil {
		r.add(Error, "flow.cycle", "the forward flow loops: %s (a card going forward would never finish)", strings.Join(cycle, " -> "))
	}
	done := map[string]bool{}
	for _, c := range columns {
		if c.kind == "done" {
			done[c.key] = true
		}
	}
	for _, c := range columns {
		if c.kind != "open" || c.key == "" {
			continue
		}
		reaches := false
		for key := range walk(forward, c.key) {
			if done[key] {
				reaches = true
			}
		}
		if !reaches && len(done) > 0 {
			r.add(Error, "flow.dead-end", "a card in column '%s' cannot reach a done column", c.key)
		}
	}
}

func walk(forward map[string][]string, from string) map[string]bool {
	seen := map[string]bool{from: true}
	queue := []string{from}
	for len(queue) > 0 {
		at := queue[0]
		queue = queue[1:]
		for _, to := range forward[at] {
			if !seen[to] {
				seen[to] = true
				queue = append(queue, to)
			}
		}
	}
	return seen
}

func findCycle(forward map[string][]string, columns []col) []string {
	state := map[string]int{}
	var path []string
	var visit func(string) []string
	visit = func(at string) []string {
		state[at] = 1
		path = append(path, at)
		for _, to := range forward[at] {
			if state[to] == 1 {
				start := 0
				for i, key := range path {
					if key == to {
						start = i
					}
				}
				return append(append([]string{}, path[start:]...), to)
			}
			if state[to] == 0 {
				if cycle := visit(to); cycle != nil {
					return cycle
				}
			}
		}
		path = path[:len(path)-1]
		state[at] = 2
		return nil
	}
	for _, c := range columns {
		if state[c.key] == 0 {
			if cycle := visit(c.key); cycle != nil {
				return cycle
			}
		}
	}
	return nil
}

// ShippedSkills are the skills the plugin ships: `kanbanzai:<name>`. A test fails when skills/ and this list
// disagree.
var ShippedSkills = []string{"audit", "build", "check", "cleanup", "design", "do", "next", "setup", "triage", "wip"}

func checkSkills(r *Report, root string, steps []stp) {
	seen := map[string]bool{}
	for _, s := range steps {
		if !s.inFile {
			continue
		}
		names := append([]string{s.skill}, s.refs...)
		for _, name := range names {
			if name == "" || seen[name] {
				continue
			}
			seen[name] = true
			if rest, ok := strings.CutPrefix(name, "kanbanzai:"); ok {
				if !contains(ShippedSkills, rest) {
					r.add(Error, "skill.unknown", "step '%s': '%s' is not a skill Kanbanzai ships (%s)", s.name, name,
						strings.Join(ShippedSkills, ", "))
				}
				continue
			}
			if strings.Contains(name, ":") {
				r.add(Warning, "skill.unchecked", "step '%s': skill '%s' belongs to another plugin; it cannot be checked here", s.name, name)
				continue
			}
			if !projectSkill(root, name) {
				r.add(Warning, "skill.missing", "step '%s': no project skill '%s' (looked for .claude/skills/%s/SKILL.md and "+
					".opencode/skills/%s/SKILL.md); a user-level skill is fine", s.name, name, name, name)
			}
		}
	}
}

func projectSkill(root, name string) bool {
	if name == "" || strings.ContainsAny(name, `/\`) || strings.Contains(name, "..") {
		return false
	}
	for _, dir := range []string{".claude", ".opencode"} {
		if info, err := os.Stat(filepath.Join(root, dir, "skills", name, "SKILL.md")); err == nil && info.Mode().IsRegular() {
			return true
		}
	}
	return false
}

func contains(values []string, wanted string) bool {
	for _, v := range values {
		if v == wanted {
			return true
		}
	}
	return false
}
