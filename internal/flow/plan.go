package flow

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/cloudbusting/kanbanzai/internal/board"
	"github.com/cloudbusting/kanbanzai/internal/brief"
	"github.com/cloudbusting/kanbanzai/internal/config"
)

// PlanKind and PlanVersion identify the plan format: the one JSON value that `plan` writes, a person reviews and
// `apply` executes.
const (
	PlanKind    = "kanbanzai-workflow-plan"
	PlanVersion = 1
)

// ConfigChange is the config file as it will be written.
type ConfigChange struct {
	Path         string `json:"path"`
	Existed      bool   `json:"existed"`
	ExpectSHA256 string `json:"expect_sha256"`
	AfterSHA256  string `json:"after_sha256"`
	Content      string `json:"content"`
}

// TemplateChange is a project's own card template with its first Stage word rewritten.
type TemplateChange struct {
	Path         string `json:"path"`
	ExpectSHA256 string `json:"expect_sha256"`
	AfterSHA256  string `json:"after_sha256"`
	Content      string `json:"content"`
}

// CardChange is one brief the migration changes. The new text is not stored: it is the old text with the Stage
// and Updated lines set (and Done / Held cleared where the new column asks), and the Progress line added, and
// apply refuses unless it comes out as AfterSHA256.
type CardChange struct {
	ID           string `json:"id"`
	Path         string `json:"path"`
	NewPath      string `json:"new_path"`
	StageBefore  string `json:"stage_before"`
	StageAfter   string `json:"stage_after"`
	ColumnBefore string `json:"column_before"`
	ColumnAfter  string `json:"column_after"`
	FolderBefore string `json:"folder_before"`
	FolderAfter  string `json:"folder_after"`
	Updated      string `json:"updated"`
	Progress     string `json:"progress"`
	ClearDone    bool   `json:"clear_done,omitempty"`
	ClearHeld    bool   `json:"clear_held,omitempty"`
	ExpectSHA256 string `json:"expect_sha256"`
	AfterSHA256  string `json:"after_sha256"`
}

// Plan is everything apply will do. Building it writes nothing.
type Plan struct {
	SchemaVersion int               `json:"schema_version"`
	Kind          string            `json:"kind"`
	Root          string            `json:"root"`
	Source        string            `json:"source"`
	Config        ConfigChange      `json:"config"`
	Template      *TemplateChange   `json:"template,omitempty"`
	Folders       []string          `json:"folders"`
	Cards         []CardChange      `json:"cards"`
	Map           map[string]string `json:"map"`
	Unused        []string          `json:"unused_folders"`
	Blockers      []string          `json:"blockers"`
	Warnings      []string          `json:"warnings"`
	Before        *Model            `json:"before"`
	After         *Model            `json:"after"`
}

// Invalid is a flow file that does not validate: there is no plan to make.
type Invalid struct{ Report *Report }

func (e *Invalid) Error() string {
	var messages []string
	for _, f := range e.Report.Findings {
		if f.Severity == Error {
			messages = append(messages, f.Message)
		}
	}
	return "the flow file is not valid (run `kanbanzai workflow validate`): " + strings.Join(messages, "; ")
}

// Digest is the sha256 of a text, in hex.
func Digest(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

func slash(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(rel)
}

var linkRE = regexp.MustCompile(`\[[^\]]+\]\(([^)\s]+)\)`)

// ConfigPath is where the project's config is, or would be.
func ConfigPath(p *config.Project) string {
	if p.ConfigFile != "" {
		return p.ConfigFile
	}
	return config.BootstrapPath(p.Root)
}

// BuildPlan makes the plan to move the board to a proposed flow. mapping says where the cards of a stage word the
// new flow does not know go: old word or column key -> new column key. now dates the migration.
func BuildPlan(p *config.Project, doc *Doc, mapping map[string]string, now time.Time) (*Plan, error) {
	oldText := ""
	if p.ConfigFile != "" {
		data, err := os.ReadFile(p.ConfigFile)
		if err != nil {
			return nil, err
		}
		oldText = string(data)
	}
	report := Validate(p, doc, oldText)
	if !report.OK {
		return nil, &Invalid{Report: report}
	}
	resolved, err := Resolve(p, doc, oldText)
	if err != nil {
		return nil, &Invalid{Report: &Report{Findings: []Finding{{ID: "config.invalid", Severity: Error, Message: err.Error()}}}}
	}
	next := resolved.Project
	plan := &Plan{SchemaVersion: PlanVersion, Kind: PlanKind, Root: p.Root, Source: doc.Format, Folders: []string{},
		Cards: []CardChange{}, Map: map[string]string{}, Unused: []string{}, Blockers: []string{}, Warnings: []string{},
		Before: Build(p), After: Build(next)}
	for _, f := range report.Findings {
		plan.Warnings = append(plan.Warnings, f.Message)
	}
	path := ConfigPath(p)
	plan.Config = ConfigChange{Path: slash(p.Root, path), Existed: p.ConfigFile != "", AfterSHA256: Digest(resolved.ConfigText),
		Content: resolved.ConfigText}
	if p.ConfigFile != "" {
		plan.Config.ExpectSHA256 = Digest(oldText)
	}

	// The mapping, normalised to lower case and checked against the new flow.
	for from, to := range mapping {
		key, ok := next.StageColumn[strings.ToLower(strings.TrimSpace(to))]
		if !ok {
			plan.Blockers = append(plan.Blockers, fmt.Sprintf("--map %s=%s: '%s' is not a column of the new flow", from, to, to))
			continue
		}
		plan.Map[strings.ToLower(strings.TrimSpace(from))] = key
	}
	used := map[string]bool{}

	folderSet := map[string]bool{}
	for _, folder := range next.Folders {
		dir := filepath.Join(next.Work, folder)
		if info, err := os.Stat(dir); err != nil || !info.IsDir() {
			if !folderSet[folder] {
				folderSet[folder] = true
				plan.Folders = append(plan.Folders, slash(p.Root, dir))
			}
		}
	}
	for _, folder := range p.Folders {
		if !contains(next.Folders, folder) {
			if _, err := os.Stat(filepath.Join(p.Work, folder)); err == nil {
				plan.Unused = append(plan.Unused, slash(p.Root, filepath.Join(p.Work, folder)))
			}
		}
	}

	claims := board.LiveClaims(p)
	type moving struct{ old, new string }
	var moves []moving
	var texts = map[string]string{}
	for _, briefPath := range board.Briefs(p) {
		data, err := os.ReadFile(briefPath)
		if err != nil {
			return nil, err
		}
		text := string(data)
		title, fields := brief.Header(text)
		id := p.CardFileRe.FindStringSubmatch(filepath.Base(briefPath))
		if id == nil {
			continue
		}
		card := id[1]
		_ = title
		word := brief.StageWord(fields["Stage"])
		oldKey := p.StageColumn[word]
		folderBefore := filepath.Base(filepath.Dir(briefPath))
		change := CardChange{ID: card, Path: slash(p.Root, briefPath), StageBefore: word, StageAfter: word,
			ColumnBefore: oldKey, FolderBefore: folderBefore, Updated: now.Format("2006-01-02"),
			ExpectSHA256: Digest(text)}
		target := next.StageColumn[word]
		if target == "" {
			dest, ok := plan.Map[word]
			if ok {
				used[word] = true
			} else if oldKey != "" {
				dest, ok = plan.Map[strings.ToLower(oldKey)]
				if ok {
					used[strings.ToLower(oldKey)] = true
				}
			}
			if !ok {
				plan.Blockers = append(plan.Blockers, fmt.Sprintf("%s: its stage '%s' is not a word of any column in the new flow; "+
					"say where it goes with --map %s=<column>", card, word, word))
				continue
			}
			target = dest
			change.StageAfter = dest
		}
		goal := next.ByKey[target]
		change.ColumnAfter = target
		change.FolderAfter = goal.Folder
		if change.StageAfter == change.StageBefore && change.FolderAfter == change.FolderBefore {
			continue
		}
		if change.StageAfter != change.StageBefore && goal.Kind == "done" {
			plan.Blockers = append(plan.Blockers, fmt.Sprintf("%s: --map into the done column '%s' is not allowed: a card is closed with "+
				"`kanbanzai move %s done --done \"…\"`", card, goal.Key, card))
			continue
		}
		if owner, claimed := claims[card]; claimed {
			plan.Blockers = append(plan.Blockers, fmt.Sprintf("%s: claimed by %s; wait for the claim to end, then plan again", card, owner))
			continue
		}
		if oldCol, ok := p.ByKey[oldKey]; ok && oldCol.Kind == "done" && goal.Kind != "done" {
			change.ClearDone = true
		}
		if fields["Held"] != "" && goal.Kind != "open" {
			change.ClearHeld = true
			plan.Warnings = append(plan.Warnings, fmt.Sprintf("%s: its Held line (%s) is removed: a %s column holds no held card", card, fields["Held"], goal.Kind))
		}
		change.NewPath = slash(p.Root, filepath.Join(next.Work, goal.Folder, filepath.Base(briefPath)))
		if change.StageAfter != change.StageBefore {
			change.Progress = fmt.Sprintf("- %s: workflow: stage %s -> %s", change.Updated, change.StageBefore, change.StageAfter)
		} else {
			change.Progress = fmt.Sprintf("- %s: workflow: folder %s -> %s", change.Updated, change.FolderBefore, change.FolderAfter)
		}
		newText := Migrate(text, change)
		change.AfterSHA256 = Digest(newText)
		if _, err := os.Lstat(filepath.Join(p.Root, filepath.FromSlash(change.NewPath))); err == nil && change.NewPath != change.Path {
			plan.Blockers = append(plan.Blockers, fmt.Sprintf("%s: %s already exists", card, change.NewPath))
			continue
		}
		moves = append(moves, moving{old: briefPath, new: filepath.Join(p.Root, filepath.FromSlash(change.NewPath))})
		texts[briefPath] = text
		plan.Cards = append(plan.Cards, change)
	}
	for from := range plan.Map {
		if !used[from] {
			plan.Warnings = append(plan.Warnings, fmt.Sprintf("--map %s=…: no card has that stage word", from))
		}
	}

	// A link that resolved from the old folder must still resolve from the new one.
	away := map[string]bool{}
	arriving := map[string]bool{}
	for _, m := range moves {
		away[m.old] = true
		arriving[m.new] = true
	}
	for _, m := range moves {
		for _, match := range linkRE.FindAllStringSubmatch(texts[m.old], -1) {
			link := match[1]
			if strings.Contains(link, "://") || strings.HasPrefix(link, "#") || strings.HasPrefix(link, "mailto:") {
				continue
			}
			link = strings.Split(link, "#")[0]
			if link == "" {
				continue
			}
			if _, err := os.Stat(filepath.Join(filepath.Dir(m.old), link)); err != nil {
				continue // already broken: not this migration's doing
			}
			target := filepath.Join(filepath.Dir(m.new), link)
			if arriving[target] {
				continue
			}
			if _, err := os.Stat(target); err == nil && !away[target] {
				continue
			}
			plan.Blockers = append(plan.Blockers, fmt.Sprintf("%s: its link %s would not resolve from %s", slash(p.Root, m.old), link, filepath.ToSlash(filepath.Dir(slash(p.Root, m.new)))))
		}
	}

	plan.templateChange(p, next)
	sort.Strings(plan.Blockers)
	return plan, nil
}

// Migrate is the brief after its change.
func Migrate(text string, c CardChange) string {
	fields := map[string]*string{"Stage": &c.StageAfter, "Updated": &c.Updated}
	if c.ClearDone {
		fields["Done"] = nil
	}
	if c.ClearHeld {
		fields["Held"] = nil
	}
	return brief.AddProgress(brief.SetFields(text, fields), c.Progress)
}

// templateChange keeps new cards in the first column: a card is written from the template with its Stage word.
func (plan *Plan) templateChange(p, next *config.Project) {
	first := next.Columns[0]
	if p.TemplateIsShipped {
		if next.StageColumn["todo"] != first.Key {
			plan.Warnings = append(plan.Warnings, fmt.Sprintf("new cards are written from the shipped template with the stage "+
				"'todo', which is not a word of the first column '%s': add \"todo\" to that column's `words`, or give the "+
				"project a template (project.template) whose Stage starts with '%s'", first.Key, first.Key))
		}
		return
	}
	data, err := os.ReadFile(p.Template)
	if err != nil {
		return
	}
	text := string(data)
	_, fields := brief.Header(text)
	word := brief.StageWord(fields["Stage"])
	if next.StageColumn[word] == first.Key {
		return
	}
	if p.StageColumn[word] != p.Columns[0].Key {
		plan.Warnings = append(plan.Warnings, fmt.Sprintf("the project template's Stage starts with '%s', which is not a word of the first "+
			"column '%s' of the new flow: edit it by hand", word, first.Key))
		return
	}
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, "- **Stage:**") {
			lines[i] = strings.Replace(line, "- **Stage:** "+word, "- **Stage:** "+first.Key, 1)
			break
		}
	}
	after := strings.Join(lines, "\n")
	if after == text {
		return
	}
	plan.Template = &TemplateChange{Path: slash(p.Root, p.Template), ExpectSHA256: Digest(text), AfterSHA256: Digest(after), Content: after}
}
