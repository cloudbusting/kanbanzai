// Package flow is the workflow command's core: the resolved flow as one model, the flow file a project proposes,
// its validation, the plan that migrates the board to it, and the apply that carries the plan out and can undo it.
//
// The flow is three tables of .kanbanzai/config.toml: [flow], [[columns]] and [steps.<name>]. Everything else in
// the config is not the command's to read or write.
package flow

import (
	"github.com/cloudbusting/kanbanzai/internal/board"
	"github.com/cloudbusting/kanbanzai/internal/config"
	"github.com/cloudbusting/kanbanzai/internal/move"
)

// ModelVersion is the schema_version of `workflow show --json`. New fields are added without raising it.
const ModelVersion = 1

// ColumnInfo is one column, as the config resolved it.
type ColumnInfo struct {
	Key           string   `json:"key"`
	Label         string   `json:"label"`
	Words         []string `json:"words"`
	Folder        string   `json:"folder"`
	Kind          string   `json:"kind"`
	Started       any      `json:"started"`
	Lane          string   `json:"lane"`
	Doing         string   `json:"doing"`
	Next          string   `json:"next"`
	SettledNext   string   `json:"settled_next"`
	Title         string   `json:"title"`
	SettledTitle  string   `json:"settled_title"`
	Worktree      bool     `json:"worktree"`
	WaitsForOwner bool     `json:"waits_for_owner"`
}

// StepInfo is one step.
type StepInfo struct {
	Name    string   `json:"name"`
	Into    string   `json:"into"`
	During  string   `json:"during"`
	Skill   string   `json:"skill"`
	Extend  []string `json:"extend"`
	Context []string `json:"context"`
	Checks  []string `json:"checks"`
	Active  bool     `json:"active"`
}

// Transition is a move a card may make without --force: From and To are column keys, Kind says why it is legal
// (start, finish, back, pause or close).
type Transition struct {
	From string `json:"from"`
	To   string `json:"to"`
	Kind string `json:"kind"`
}

// Gate is a point where the flow waits for the owner.
type Gate struct {
	Column string `json:"column"`
	Kind   string `json:"kind"`
	Step   string `json:"step"`
}

// WIP is the work-in-progress rules.
type WIP struct {
	Limit            int      `json:"limit"`
	StaleDays        int      `json:"stale_days"`
	MoveGraceMinutes float64  `json:"move_grace_minutes"`
	Started          []string `json:"started"`
	Claimed          []string `json:"claimed"`
}

// Isolation is how a card's change is kept apart from the board: today always a worktree and a branch per card.
type Isolation struct {
	Mode            string   `json:"mode"`
	WorktreeColumns []string `json:"worktree_columns"`
	WorktreePattern string   `json:"worktree_pattern"`
	BranchPattern   string   `json:"branch_pattern"`
	BatchBranch     string   `json:"batch_branch_pattern"`
	MainBranch      string   `json:"main_branch"`
}

// Model is the whole flow as one value: what `workflow show` prints.
type Model struct {
	SchemaVersion int          `json:"schema_version"`
	Columns       []ColumnInfo `json:"columns"`
	Steps         []StepInfo   `json:"steps"`
	Transitions   []Transition `json:"transitions"`
	Gates         []Gate       `json:"gates"`
	WIP           WIP          `json:"wip"`
	Isolation     Isolation    `json:"isolation"`
	PullOrder     []string     `json:"pull_order"`
}

func list(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

// Build is the model of a loaded project.
func Build(p *config.Project) *Model {
	m := &Model{SchemaVersion: ModelVersion, PullOrder: list(p.PullOrder)}
	for _, c := range p.Columns {
		m.Columns = append(m.Columns, ColumnInfo{Key: c.Key, Label: c.Label, Words: list(c.Words), Folder: c.Folder,
			Kind: c.Kind, Started: c.Started, Title: c.Title, SettledTitle: c.SettledTitle, Lane: c.Lane, Doing: c.Doing, Next: c.Next, SettledNext: c.SettledNext,
			Worktree: c.Worktree, WaitsForOwner: c.WaitsForOwner})
	}
	m.Steps = []StepInfo{}
	for _, name := range p.StepOrder {
		s := p.Steps[name]
		m.Steps = append(m.Steps, StepInfo{Name: s.Name, Into: s.Into, During: s.During, Skill: s.Skill,
			Extend: list(s.Extend), Context: list(s.Context), Checks: list(s.Checks), Active: s.Active})
	}
	m.Transitions = Transitions(p)
	m.Gates = []Gate{}
	m.WIP = WIP{Limit: p.WIPLimit, StaleDays: p.StaleDays, MoveGraceMinutes: p.MoveGraceMinutes, Started: []string{},
		Claimed: []string{}}
	m.Isolation = Isolation{Mode: "worktree-per-card", WorktreeColumns: []string{}, WorktreePattern: p.WorktreePattern,
		BranchPattern: p.BranchPattern, BatchBranch: p.BatchBranchPattern, MainBranch: p.MainBranch}
	for _, c := range p.Columns {
		if c.WaitsForOwner {
			m.Gates = append(m.Gates, Gate{Column: c.Key, Kind: "owner-approval", Step: c.Next})
		}
		if c.StartedBool() {
			m.WIP.Started = append(m.WIP.Started, c.Key)
		}
		if c.CountsWithClaim() {
			m.WIP.Claimed = append(m.WIP.Claimed, c.Key)
		}
		if c.Worktree {
			m.Isolation.WorktreeColumns = append(m.Isolation.WorktreeColumns, c.Key)
		}
	}
	return m
}

// Transitions is every move a card can make without --force, as the move command allows it: the union of
// move.LegalTargets over each column, for a card that is settled or not and whose deploy is None or not. The
// rules live in the move package; this only asks it.
func Transitions(p *config.Project) []Transition {
	found := []Transition{}
	seen := map[Transition]bool{}
	for _, c := range p.Columns {
		for _, settled := range []bool{false, true} {
			for _, nothing := range []bool{false, true} {
				card := &board.Card{}
				if settled {
					card.Next = "settled"
				}
				targets := move.LegalTargets(p, c.Key, card, nothing)
				for _, to := range p.Columns {
					why, ok := targets[to.Key]
					if !ok || why == "stay" {
						continue
					}
					t := Transition{From: c.Key, To: to.Key, Kind: why}
					if !seen[t] {
						seen[t] = true
						found = append(found, t)
					}
				}
			}
		}
	}
	return found
}
