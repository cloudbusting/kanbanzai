package migrate

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/cloudbusting/kanbanzai/internal/config"
	"github.com/cloudbusting/kanbanzai/internal/gitutil"
)

// SchemaVersion is the version of the plan's JSON.
const SchemaVersion = 1

// Item is one migration in a plan.
type Item struct {
	ID      string   `json:"id"`
	Kind    string   `json:"kind"`
	Summary string   `json:"summary"`
	Files   []string `json:"files"`
}

// Plan is everything a migration would do, as one reviewable thing. SHA covers each file's before and after
// bytes, so approving a plan approves exactly those writes.
type Plan struct {
	SchemaVersion int      `json:"schema_version"`
	Pending       bool     `json:"pending"`
	SHA           string   `json:"plan_sha256"`
	Migrations    []Item   `json:"migrations"`
	Recovery      string   `json:"recovery"`
	Warnings      []string `json:"warnings"`

	root    string
	project *config.Project
	steps   []step
}

// step is a change with the migration it belongs to.
type step struct {
	Change
	Kind string
}

func digest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func (c Change) beforeSHA() string {
	if c.Before == nil {
		return ""
	}
	return digest(c.Before)
}

// Preview works out what a migration of the project at start would do. It reads only, and works when the
// project's config is too old to load.
func Preview(start string, registry []Migration) (*Plan, error) {
	if registry == nil {
		registry = Registry
	}
	upgrade, err := config.PlanUpgrade(start)
	if err != nil {
		return nil, err
	}
	plan := &Plan{SchemaVersion: SchemaVersion, Migrations: []Item{}, Recovery: "none", Warnings: []string{}}
	if upgrade.Project == nil {
		return plan, nil
	}
	plan.project = upgrade.Project
	plan.root = upgrade.Project.Root
	if journal, err := readJournal(journalPath(plan.project)); err == nil && journal.Status == StatusApplying {
		plan.Recovery = "rollback-needed"
	}
	env := Env{Root: plan.root, Project: upgrade.Project, Config: upgrade}
	ordered := append([]Migration{}, registry...)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].ID < ordered[j].ID })
	owner := map[string]string{}
	var configSteps []step
	var sum bytes.Buffer
	sum.WriteString("kanbanzai-migrate-plan-v1\n")
	for _, m := range ordered {
		changes, err := m.Detect(env)
		if err != nil {
			return nil, fmt.Errorf("migration %s: %v", m.ID, err)
		}
		if len(changes) == 0 {
			continue
		}
		item := Item{ID: m.ID, Kind: m.Kind, Summary: m.Summary, Files: []string{}}
		sort.SliceStable(changes, func(i, j int) bool { return changes[i].Path < changes[j].Path })
		for _, c := range changes {
			if c.Path == "" || filepath.IsAbs(c.Path) || c.Path == ".." || strings.HasPrefix(c.Path, "../") || strings.Contains(c.Path, "\\") {
				return nil, fmt.Errorf("migration %s: %q is not a path inside the project", m.ID, c.Path)
			}
			if other, taken := owner[c.Path]; taken {
				return nil, fmt.Errorf("migrations %s and %s both write %s", other, m.ID, c.Path)
			}
			owner[c.Path] = m.ID
			item.Files = append(item.Files, c.Path)
			fmt.Fprintf(&sum, "%s\x00%s\x00%s\x00%s\n", m.ID, c.Path, c.beforeSHA(), digest(c.After))
			s := step{Change: c, Kind: m.Kind}
			if m.Kind == "config" {
				configSteps = append(configSteps, s)
			} else {
				plan.steps = append(plan.steps, s)
			}
		}
		plan.Migrations = append(plan.Migrations, item)
	}
	plan.steps = append(plan.steps, configSteps...) // the config goes last, so a half-done apply still reads the old one
	if len(plan.steps) > 0 {
		plan.Pending = true
		plan.SHA = digest(sum.Bytes())
		plan.Warnings = append(plan.Warnings, worktreeWarnings(plan, upgrade)...)
	}
	return plan, nil
}

// worktreeWarnings names a linked worktree that holds its own tracked copy of the config. The command reads the
// main checkout's first, so such a copy is not changed and not used; merging main into its branch brings the new one.
func worktreeWarnings(plan *Plan, upgrade *config.UpgradePlan) []string {
	if upgrade.Path == "" || upgrade.Current {
		return nil
	}
	listing, err := gitutil.Git(plan.root, "worktree", "list", "--porcelain")
	if err != nil {
		return nil
	}
	rel, err := filepath.Rel(plan.root, upgrade.Path)
	if err != nil {
		return nil
	}
	var warnings []string
	for _, line := range strings.Split(listing, "\n") {
		path, ok := strings.CutPrefix(strings.TrimSpace(line), "worktree ")
		if !ok || sameDir(path, plan.root) {
			continue
		}
		data, err := os.ReadFile(filepath.Join(path, rel))
		if err != nil || bytes.Equal(data, upgrade.New) {
			continue
		}
		warnings = append(warnings, fmt.Sprintf("%s holds its own copy of %s; it is not changed and the command reads the main checkout's, so merging main into that branch brings the new one",
			path, filepath.ToSlash(rel)))
	}
	return warnings
}

func sameDir(a, b string) bool {
	x, errA := filepath.EvalSymlinks(a)
	y, errB := filepath.EvalSymlinks(b)
	if errA != nil || errB != nil {
		return filepath.Clean(a) == filepath.Clean(b)
	}
	return x == y
}

// JSON is the plan as the versioned contract.
func (p *Plan) JSON() ([]byte, error) {
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(p); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Text is the plan for the owner to read.
func (p *Plan) Text() string {
	var b strings.Builder
	switch {
	case p.Recovery == "rollback-needed":
		b.WriteString("An earlier migration did not finish. Run `kanbanzai migrate --rollback` to put the project back before anything else.\n")
	case !p.Pending:
		b.WriteString("Nothing to migrate: this project's files already match this release.\n")
	}
	if p.Pending {
		fmt.Fprintf(&b, "This project needs migrating to this release. Nothing is written until you apply this plan.\n\n")
		for _, m := range p.Migrations {
			fmt.Fprintf(&b, "- %s (%s): %s\n", m.ID, m.Kind, m.Summary)
			for _, f := range m.Files {
				fmt.Fprintf(&b, "    writes %s\n", f)
			}
		}
		fmt.Fprintf(&b, "\nEach file is backed up first under the project's state folder, written atomically, then checked; any failure puts\nthe old files back. Nothing is committed.\n")
		for _, w := range p.Warnings {
			fmt.Fprintf(&b, "\nNote: %s\n", w)
		}
		fmt.Fprintf(&b, "\nPlan %s\nTo apply it: kanbanzai migrate --apply --plan %s\n", p.SHA, p.SHA)
	}
	return b.String()
}
