package preset

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/cloudbusting/kanbanzai/internal/config"
	"github.com/cloudbusting/kanbanzai/internal/flow"
)

// Compare says whether a project's flow is still the flow of the preset its [preset] record names: "matches",
// "differs", or "" when the project has no record or names a preset this command does not ship.
func Compare(project *config.Project) string {
	if project.Preset.Name == "" || IsPath(project.Preset.Name) {
		return ""
	}
	p, err := Load(project.Preset.Name)
	if err != nil {
		return ""
	}
	return p.Compare(project)
}

// Compare says whether the project's flow equals this preset's.
func (p *Preset) Compare(project *config.Project) string {
	if reflect.DeepEqual(flow.FromProject(project).Raw, p.Flow.Raw) {
		return "matches"
	}
	return "differs"
}

// TypeInfo is one issue type of a preset.
type TypeInfo struct {
	Key      string `json:"key"`
	Label    string `json:"label"`
	Prefix   string `json:"prefix"`
	Template string `json:"template"`
	Schema   string `json:"schema"`
}

// Info is what a preset says about itself: the record, the flow it makes and the files it writes.
type Info struct {
	Name          string            `json:"name"`
	DeliveryStyle string            `json:"delivery_style"`
	Version       int               `json:"version"`
	Summary       string            `json:"summary"`
	BestFor       string            `json:"best_for"`
	NotFor        string            `json:"not_for"`
	Source        string            `json:"source"`
	Isolation     string            `json:"isolation"`
	PullOrder     []string          `json:"pull_order"`
	Columns       []flow.ColumnInfo `json:"columns"`
	Steps         []flow.StepInfo   `json:"steps"`
	Gates         []flow.Gate       `json:"gates"`
	Types         []TypeInfo        `json:"types"`
	Paths         map[string]string `json:"paths"`
	Files         []string          `json:"files"`
}

// Brief is the part of Info that `workflow presets` lists.
func (p *Preset) Brief() Info {
	return Info{Name: p.Name, DeliveryStyle: p.DeliveryStyle, Version: p.Version, Summary: p.Summary, BestFor: p.BestFor,
		NotFor: p.NotFor, Source: p.Source}
}

// Files are the files the preset adds to a project, as project-relative paths, in name order.
func (p *Preset) Files() []string {
	var files []string
	for _, item := range p.Types {
		if template, _ := item["template"].(string); template != "" && !contains(files, template) {
			files = append(files, template)
		}
	}
	return files
}

// Describe is the whole explanation: what `init --preset NAME --dry-run` and `workflow presets NAME` print.
func (p *Preset) Describe() (*Info, error) {
	project, cleanup, err := p.resolve(Settings{Owner: "Preset Check"})
	if err != nil {
		return nil, err
	}
	defer cleanup()
	model := flow.Build(project)
	info := p.Brief()
	info.Isolation = model.Isolation.Mode
	info.PullOrder = model.PullOrder
	info.Columns, info.Steps, info.Gates = model.Columns, model.Steps, model.Gates
	info.Paths = map[string]string{"worktree": project.WorktreePattern, "branch": project.BranchPattern,
		"batch_branch": project.BatchBranchPattern, "work": project.Rel(project.Work)}
	for _, item := range project.Types {
		info.Types = append(info.Types, TypeInfo{Key: item.Key, Label: item.Label, Prefix: item.Prefix,
			Template: project.Rel(item.Template), Schema: item.Schema})
	}
	info.Files = p.Files()
	return &info, nil
}

// Text is the explanation for a person.
func (i *Info) Text() string {
	var b strings.Builder
	fmt.Fprintf(&b, "Preset %s (delivery style %s, v%d)\n  %s\n  best for: %s\n  does not: %s\n", i.Name, i.DeliveryStyle, i.Version,
		i.Summary, i.BestFor, i.NotFor)
	fmt.Fprintf(&b, "Columns (%d)\n", len(i.Columns))
	for _, c := range i.Columns {
		var extra []string
		if c.Doing != "" {
			extra = append(extra, "doing "+c.Doing)
		}
		if c.Next != "" {
			extra = append(extra, "next "+c.Next)
		}
		if c.Gate != "" {
			gate := "gate " + c.Gate
			if c.GateName != "" {
				gate += " (" + c.GateName + ")"
			}
			extra = append(extra, gate)
		}
		fmt.Fprintf(&b, "  %-14s %-14s %-9s %s\n", c.Key, c.Label, c.Kind, strings.Join(extra, ", "))
	}
	fmt.Fprintf(&b, "Steps (%d)\n", len(i.Steps))
	for _, s := range i.Steps {
		skill := s.Skill
		if skill == "" {
			skill = "(no skill: skipped)"
		}
		line := fmt.Sprintf("  %-10s into %-12s %s", s.Name, s.Into, skill)
		if s.SkipField != "" {
			line += fmt.Sprintf("  skipped when %s starts with %s", s.SkipField, strings.Join(s.SkipValues, " or "))
		}
		fmt.Fprintln(&b, line)
	}
	fmt.Fprintf(&b, "Isolation\n  %s (worktree %s, branch %s)\n", i.Isolation, i.Paths["worktree"], i.Paths["branch"])
	fmt.Fprintf(&b, "Pull order\n  %s\n", strings.Join(i.PullOrder, " > "))
	fmt.Fprintf(&b, "Types (%d)\n", len(i.Types))
	for _, t := range i.Types {
		line := fmt.Sprintf("  %-10s %-12s %s", t.Key, t.Label, t.Prefix)
		if t.Template != "" {
			line += "  template " + t.Template
		}
		if t.Schema != "" {
			line += "  schema " + t.Schema
		}
		fmt.Fprintln(&b, line)
	}
	if len(i.Files) > 0 {
		fmt.Fprintf(&b, "Files it creates\n")
		for _, f := range i.Files {
			fmt.Fprintf(&b, "  %s\n", f)
		}
	}
	return b.String()
}
