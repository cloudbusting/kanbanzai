package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/cloudbusting/kanbanzai/internal/config"
	"github.com/cloudbusting/kanbanzai/internal/flow"
	"github.com/cloudbusting/kanbanzai/internal/fsutil"
	"github.com/cloudbusting/kanbanzai/internal/preset"
)

const workflowUsage = `usage: kanbanzai workflow <verb> [options]

verbs:
  show [--json]                     the resolved flow: columns, steps, transitions, gates, WIP, isolation, pull order
  validate --file FILE|--preset NAME|DIR [--json]
                                    every problem with a proposed flow (TOML or JSON, or a preset's); writes nothing
  plan --file FILE|--preset NAME|DIR [--step-skill STEP=SKILL[,...]] [--step-extend STEP=SKILL[+SKILL][,...]]
       [--map OLD=NEW]... [--out PLAN] [--json]
                                    the card migration the flow implies; writes only --out
  presets [NAME] [--json]           the shipped delivery presets, or one explained (needs no board)
  apply --plan PLAN                 back up, migrate the briefs, write the config, run check and doctor; undo on failure
  apply --rollback                  undo an apply that did not finish
  export [--format toml|json|mermaid] [--out FILE]
                                    the resolved flow as a reviewable file
`

// extractRepeated pulls every --name VALUE and --name=VALUE out of args.
func extractRepeated(args []string, name string) (values, rest []string, err error) {
	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch {
		case arg == "--"+name:
			if index+1 >= len(args) {
				return nil, nil, fmt.Errorf("--%s needs a value", name)
			}
			values = append(values, args[index+1])
			index++
		case strings.HasPrefix(arg, "--"+name+"="):
			values = append(values, strings.TrimPrefix(arg, "--"+name+"="))
		default:
			rest = append(rest, arg)
		}
	}
	return values, rest, nil
}

// flowSource reads the flow a verb works on: --file FILE, or --preset NAME|DIR (composed with --step-skill and
// --step-extend). It is the only place a preset is turned into a flow for the workflow command.
func flowSource(flags map[string]string, verb string) (*flow.Doc, error) {
	file, named := flags["file"], flags["preset"]
	if (file == "") == (named == "") {
		return nil, errors.New("give --file FILE or --preset NAME, not both and not neither")
	}
	if file != "" {
		if flags["step-skill"] != "" || flags["step-extend"] != "" {
			return nil, errors.New("--step-skill and --step-extend compose a preset: they need --preset")
		}
		return flow.ReadFile(file)
	}
	p, err := preset.Load(named)
	if err != nil {
		return nil, err
	}
	var compose preset.Compose
	if list := flags["step-skill"]; list != "" {
		compose.StepSkills = strings.Split(list, ",")
	}
	if list := flags["step-extend"]; list != "" {
		compose.StepExtends = strings.Split(list, ",")
	}
	if err := p.Compose(compose); err != nil {
		return nil, err
	}
	return p.FlowDoc(), nil
}

func runWorkflow(project *config.Project, args []string, out, errOut io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(errOut, workflowUsage)
		return 2
	}
	verb, rest := args[0], args[1:]
	switch verb {
	case "show":
		positionals, _, set, err := parse(rest, map[string]bool{"json": true}, nil)
		if err != nil || len(positionals) != 0 {
			return usageError(errOut, "workflow show", err)
		}
		model := flow.Build(project)
		model.Preset.Flow = preset.Compare(project)
		if set["json"] {
			writeCompact(out, model)
			return 0
		}
		fmt.Fprint(out, showText(model))
		return 0
	case "validate":
		positionals, flags, set, err := parse(rest, map[string]bool{"json": true}, map[string]bool{"file": true, "preset": true,
			"step-skill": true, "step-extend": true})
		if err != nil || len(positionals) != 0 {
			return usageError(errOut, "workflow validate", err)
		}
		doc, err := flowSource(flags, "validate")
		if err != nil {
			fmt.Fprintf(errOut, "kanbanzai: workflow validate: %v\n", err)
			return 2
		}
		report := flow.Validate(project, doc, currentConfig(project))
		if set["json"] {
			writeCompact(out, report)
		} else {
			for _, f := range report.Findings {
				fmt.Fprintf(out, "%s %s: %s\n", f.Severity, f.ID, f.Message)
			}
			if report.OK {
				fmt.Fprintf(out, "flow ok (%d warning(s))\n", len(report.Findings))
			} else {
				fmt.Fprintf(out, "%d error(s)\n", report.Errors())
			}
		}
		if !report.OK {
			return 1
		}
		return 0
	case "plan":
		maps, rest, err := extractRepeated(rest, "map")
		if err != nil {
			return usageError(errOut, "workflow plan", err)
		}
		positionals, flags, set, err := parse(rest, map[string]bool{"json": true}, map[string]bool{"file": true, "out": true,
			"preset": true, "step-skill": true, "step-extend": true})
		if err != nil || len(positionals) != 0 {
			return usageError(errOut, "workflow plan", err)
		}
		mapping := map[string]string{}
		for _, item := range maps {
			from, to, ok := strings.Cut(item, "=")
			if !ok || strings.TrimSpace(from) == "" || strings.TrimSpace(to) == "" {
				return usageError(errOut, "workflow plan", fmt.Errorf("--map %q: use OLD=NEW", item))
			}
			mapping[from] = to
		}
		doc, err := flowSource(flags, "plan")
		if err != nil {
			fmt.Fprintf(errOut, "kanbanzai: workflow plan: %v\n", err)
			return 2
		}
		plan, err := flow.BuildPlan(project, doc, mapping, time.Now())
		if err != nil {
			var invalid *flow.Invalid
			if errors.As(err, &invalid) {
				for _, f := range invalid.Report.Findings {
					fmt.Fprintf(errOut, "%s %s: %s\n", f.Severity, f.ID, f.Message)
				}
				fmt.Fprintln(errOut, "kanbanzai: workflow plan: the flow file is not valid; there is no plan")
				return 1
			}
			fmt.Fprintf(errOut, "kanbanzai: workflow plan: %v\n", err)
			return 1
		}
		data, err := plan.JSON()
		if err != nil {
			fmt.Fprintf(errOut, "kanbanzai: workflow plan: %v\n", err)
			return 1
		}
		if file := flags["out"]; file != "" {
			if err := fsutil.WriteFileAtomic(file, data, 0o644); err != nil {
				fmt.Fprintf(errOut, "kanbanzai: workflow plan: --out: %v\n", err)
				return 2
			}
		}
		if set["json"] {
			_, _ = out.Write(data)
		} else {
			fmt.Fprint(out, planText(plan, flags["out"]))
		}
		if len(plan.Blockers) > 0 {
			return 1
		}
		return 0
	case "apply":
		positionals, flags, set, err := parse(rest, map[string]bool{"rollback": true, "json": true}, map[string]bool{"plan": true})
		if err != nil || len(positionals) != 0 || (flags["plan"] == "") == !set["rollback"] || (set["rollback"] && flags["plan"] != "") {
			if err == nil {
				err = errors.New("give --plan FILE, or --rollback")
			}
			return usageError(errOut, "workflow apply", err)
		}
		var result *flow.Result
		if set["rollback"] {
			result, err = flow.Rollback(project)
		} else {
			var plan *flow.Plan
			if plan, err = flow.LoadPlan(flags["plan"]); err == nil {
				result, err = flow.Apply(project, plan, flow.ApplyOptions{})
			}
		}
		if err != nil {
			var refusal *flow.Refusal
			if errors.As(err, &refusal) {
				fmt.Fprintf(errOut, "kanbanzai: workflow apply: %v\n", err)
				return 2
			}
			fmt.Fprintf(errOut, "kanbanzai: workflow %v\n", err)
			return 1
		}
		if set["json"] {
			writeCompact(out, result)
			return 0
		}
		for _, line := range result.Changed {
			fmt.Fprintln(out, "  "+line)
		}
		for _, folder := range result.Unused {
			fmt.Fprintf(out, "  now unused (left in place, remove it when you like): %s\n", folder)
		}
		if set["rollback"] {
			fmt.Fprintln(out, "rolled back: the board is as it was")
		} else {
			fmt.Fprintln(out, "applied; check and doctor pass. Nothing is committed: review `git status` and commit the board.")
		}
		return 0
	case "export":
		positionals, flags, _, err := parse(rest, nil, map[string]bool{"format": true, "out": true})
		if err != nil || len(positionals) != 0 {
			return usageError(errOut, "workflow export", err)
		}
		format := flags["format"]
		if format == "" {
			format = "toml"
		}
		doc := flow.FromProject(project)
		var data []byte
		switch format {
		case "toml":
			data = []byte("# The flow of this project, as `kanbanzai workflow export` wrote it.\n" +
				"# Edit it, then `kanbanzai workflow validate --file FILE` and `kanbanzai workflow plan --file FILE`.\n" +
				"# A flow file replaces the project's [flow], [[columns]] and [steps.*] tables.\n\n" + doc.Text)
		case "json":
			if data, err = doc.JSON(); err != nil {
				fmt.Fprintf(errOut, "kanbanzai: workflow export: %v\n", err)
				return 1
			}
		case "mermaid":
			data = []byte(flow.Mermaid(flow.Build(project)))
		default:
			return usageError(errOut, "workflow export", fmt.Errorf("--format must be toml, json or mermaid, not %q", format))
		}
		if file := flags["out"]; file != "" {
			if err := fsutil.WriteFileAtomic(file, data, 0o644); err != nil {
				fmt.Fprintf(errOut, "kanbanzai: workflow export: --out: %v\n", err)
				return 2
			}
			return 0
		}
		_, _ = out.Write(data)
		return 0
	}
	fmt.Fprintf(errOut, "kanbanzai: workflow: unknown verb %q\n%s", verb, workflowUsage)
	return 2
}

// runWorkflowPresets is `workflow presets [NAME] [--json]`: it reads only what is embedded in the command, so it
// works in any folder, before a project has a config.
func runWorkflowPresets(args []string, out, errOut io.Writer) int {
	positionals, _, set, err := parse(args, map[string]bool{"json": true}, nil)
	if err != nil || len(positionals) > 1 {
		return usageError(errOut, "workflow presets", err)
	}
	if len(positionals) == 1 {
		p, err := preset.Load(positionals[0])
		if err == nil {
			var info *preset.Info
			if info, err = p.Describe(); err == nil {
				if set["json"] {
					writeCompact(out, info)
				} else {
					fmt.Fprint(out, info.Text())
				}
				return 0
			}
		}
		fmt.Fprintf(errOut, "kanbanzai: workflow presets: %v\n", err)
		return 2
	}
	names, err := preset.List()
	if err != nil {
		fmt.Fprintf(errOut, "kanbanzai: workflow presets: %v\n", err)
		return 1
	}
	var list []preset.Info
	for _, name := range names {
		p, err := preset.Load(name)
		if err != nil {
			fmt.Fprintf(errOut, "kanbanzai: workflow presets: %v\n", err)
			return 1
		}
		list = append(list, p.Brief())
	}
	if set["json"] {
		writeCompact(out, map[string]any{"schema_version": 1, "presets": list})
		return 0
	}
	for _, info := range list {
		fmt.Fprintf(out, "%-14s %-14s v%d  %s\n", info.Name, info.DeliveryStyle, info.Version, info.Summary)
	}
	return 0
}

func orMissing(err error, flag string) error {
	if err != nil {
		return err
	}
	return fmt.Errorf("%s is needed", flag)
}

func currentConfig(project *config.Project) string {
	if project.ConfigFile == "" {
		return ""
	}
	data, err := os.ReadFile(project.ConfigFile)
	if err != nil {
		return ""
	}
	return string(data)
}

func showText(m *flow.Model) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Columns (%d)\n", len(m.Columns))
	for _, c := range m.Columns {
		extra := []string{}
		if c.Lane != "" {
			extra = append(extra, "lane "+c.Lane)
		}
		if c.Doing != "" {
			extra = append(extra, "doing "+c.Doing)
		}
		if c.Next != "" {
			extra = append(extra, "next "+c.Next)
		}
		if c.SettledNext != "" {
			extra = append(extra, "settled "+c.SettledNext)
		}
		if c.Worktree {
			extra = append(extra, "worktree")
		}
		if c.Gate != "" {
			text := "gate " + c.Gate
			switch {
			case c.GateName != "":
				text += " (" + c.GateName + ")"
			case c.GateAfter != "":
				text += " (after " + c.GateAfter + ")"
			case c.GateUntil != "":
				text += " (until " + c.GateUntil + ")"
			}
			if c.Gate == "owner" {
				text += ", waits for owner"
			}
			extra = append(extra, text)
		}
		fmt.Fprintf(&b, "  %-14s %-14s %-9s %-8s %s\n", c.Key, c.Label, c.Folder, c.Kind, strings.Join(extra, ", "))
	}
	fmt.Fprintf(&b, "Steps (%d)\n", len(m.Steps))
	for _, s := range m.Steps {
		skill := s.Skill
		if skill == "" {
			skill = "(no skill: skipped)"
		}
		line := fmt.Sprintf("  %-10s into %-12s during %-12s %s", s.Name, s.Into, orDash(s.During), skill)
		if len(s.Extend) > 0 {
			line += "  extend " + strings.Join(s.Extend, ", ")
		}
		if len(s.Checks) > 0 {
			line += "  checks " + strings.Join(s.Checks, ", ")
		}
		if s.SkipField != "" {
			line += fmt.Sprintf("  skipped when %s starts with %s", s.SkipField, strings.Join(s.SkipValues, " or "))
		}
		fmt.Fprintln(&b, line)
	}
	fmt.Fprintf(&b, "Transitions (%d)\n", len(m.Transitions))
	for _, t := range m.Transitions {
		fmt.Fprintf(&b, "  %-14s -> %-14s %s\n", t.From, t.To, t.Kind)
	}
	fmt.Fprintln(&b, "Gates")
	if len(m.Gates) == 0 {
		fmt.Fprintln(&b, "  none")
	}
	for _, g := range m.Gates {
		fmt.Fprintf(&b, "  %s: %s before %s\n", g.Column, g.Kind, orDash(g.Step))
	}
	fmt.Fprintf(&b, "WIP\n  limit %d started cards (columns: %s; with a claim: %s); stale after %d days; claimed-not-started after %g minutes\n",
		m.WIP.Limit, strings.Join(m.WIP.Started, ", "), strings.Join(m.WIP.Claimed, ", "), m.WIP.StaleDays, m.WIP.MoveGraceMinutes)
	columns := strings.Join(m.Isolation.WorktreeColumns, ", ")
	switch m.Isolation.Setting {
	case "branch":
		fmt.Fprintf(&b, "Isolation\n  %s: a branch %s in the one checkout from %s, one card at a time, required in: %s\n",
			m.Isolation.Mode, m.Isolation.BranchPattern, m.Isolation.MainBranch, columns)
	case "shared":
		fmt.Fprintf(&b, "Isolation\n  %s: the one checkout, no workspace record; parallel sessions share one tree; work "+
			"columns: %s\n", m.Isolation.Mode, columns)
	case "external":
		fmt.Fprintf(&b, "Isolation\n  %s: the host owns the workspace; a card records it as Where: external <ref>; required "+
			"in: %s\n", m.Isolation.Mode, columns)
	default:
		fmt.Fprintf(&b, "Isolation\n  %s: a worktree %s on branch %s (batches %s) from %s, required in: %s\n", m.Isolation.Mode,
			m.Isolation.WorktreePattern, m.Isolation.BranchPattern, m.Isolation.BatchBranch, m.Isolation.MainBranch, columns)
	}
	fmt.Fprintf(&b, "Pull order\n  %s\n", strings.Join(m.PullOrder, " > "))
	if m.Preset.Name != "" {
		line := fmt.Sprintf("Preset\n  %s", m.Preset.Name)
		if m.Preset.DeliveryStyle != "" || m.Preset.Version > 0 {
			var parts []string
			if m.Preset.DeliveryStyle != "" {
				parts = append(parts, m.Preset.DeliveryStyle)
			}
			if m.Preset.Version > 0 {
				parts = append(parts, fmt.Sprintf("v%d", m.Preset.Version))
			}
			line += " (" + strings.Join(parts, ", ") + ")"
		}
		switch m.Preset.Flow {
		case "matches":
			line += ", flow matches the shipped preset"
		case "differs":
			line += ", flow differs from the shipped preset"
		}
		fmt.Fprintln(&b, line)
	}
	return b.String()
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func planText(plan *flow.Plan, out string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Config:   %s (%s)\n", plan.Config.Path, map[bool]string{true: "flow tables replaced", false: "created"}[plan.Config.Existed])
	if plan.Template != nil {
		fmt.Fprintf(&b, "Template: %s (first Stage word)\n", plan.Template.Path)
	}
	for _, template := range plan.TypeTemplates {
		fmt.Fprintf(&b, "Type template: %s (first Stage word)\n", template.Path)
	}
	for _, folder := range plan.Folders {
		fmt.Fprintf(&b, "Folder:   create %s\n", folder)
	}
	for _, folder := range plan.Unused {
		fmt.Fprintf(&b, "Folder:   %s is no longer used (left in place)\n", folder)
	}
	fmt.Fprintf(&b, "Cards:    %d to change\n", len(plan.Cards))
	for _, c := range plan.Cards {
		move := ""
		if c.NewPath != c.Path {
			move = fmt.Sprintf("  %s -> %s", c.Path, c.NewPath)
		}
		fmt.Fprintf(&b, "  %-6s stage %s -> %s%s\n", c.ID, c.StageBefore, c.StageAfter, move)
	}
	for _, w := range plan.Warnings {
		fmt.Fprintf(&b, "warning:  %s\n", w)
	}
	for _, x := range plan.Blockers {
		fmt.Fprintf(&b, "BLOCKER:  %s\n", x)
	}
	switch {
	case len(plan.Blockers) > 0:
		fmt.Fprintf(&b, "%d blocker(s): the plan cannot be applied\n", len(plan.Blockers))
	case out != "":
		fmt.Fprintf(&b, "plan written to %s: review it, then `kanbanzai workflow apply --plan %s`\n", out, out)
	default:
		fmt.Fprintln(&b, "no plan file written: add --out FILE to keep it for `kanbanzai workflow apply --plan FILE`")
	}
	return b.String()
}
