package detangle

import (
	"reflect"
	"strings"
	"testing"

	"github.com/cloudbusting/kanbanzai/internal/board"
	"github.com/cloudbusting/kanbanzai/internal/config"
	"github.com/cloudbusting/kanbanzai/internal/gate"
	"github.com/cloudbusting/kanbanzai/internal/testutil"
)

func testProject(t *testing.T) *config.Project {
	t.Helper()
	t.Setenv("KANBANZAI_PROJECT", "")
	root := testutil.Repo(t, "[project]\nowner = \"Sam\"\n")
	testutil.Folders(t, root, "1-open", "x-paused", "10-done")
	project, err := config.Find(root)
	if err != nil {
		t.Fatalf("config.Find: %v", err)
	}
	return project
}

// spec is one card of a fixture board.
type spec struct {
	id, column, held, owner string
	gate                    *gate.Status
}

// needs is a closed card gate on a card, waiting on each pair: card id and the column it sits in ("" is not on the board).
func needs(pairs ...string) *gate.Status {
	status := &gate.Status{Column: "todo", Type: "card", State: gate.Closed,
		OpensWhen: "every card in its Needs line is in Designed or later", Why: "waiting"}
	for i := 0; i < len(pairs); i += 2 {
		need := gate.Need{Card: pairs[i], Column: pairs[i+1], Wants: "designed", WantsLabel: "Designed"}
		need.ColumnLabel, need.OnBoard = need.Column, need.Column != ""
		status.Needs = append(status.Needs, need)
	}
	return status
}

func ownerGate(id string) *gate.Status {
	return &gate.Status{Column: "qa-approved", Type: "owner", State: gate.Closed,
		OpensWhen: "Sam's approval is recorded: kanbanzai gate pass " + id + " --note \"<their words>\"",
		Why:       "waiting: Sam's approval"}
}

func fixture(t *testing.T, cards ...spec) (*config.Project, *board.Board) {
	t.Helper()
	project := testProject(t)
	byColumn := map[string][]*board.Card{}
	for _, c := range cards {
		card := &board.Card{ID: c.id, Title: "Title " + c.id, Stage: c.column, Gate: c.gate}
		if c.held != "" {
			held := c.held
			card.Held = &held
		}
		if c.owner != "" {
			owner := c.owner
			card.Owner = &owner
		}
		byColumn[c.column] = append(byColumn[c.column], card)
	}
	loaded := &board.Board{}
	for _, column := range project.Columns {
		loaded.Columns = append(loaded.Columns, board.Column{Key: column.Key, Cards: byColumn[column.Key]})
	}
	return project, loaded
}

func ids(list []string) []string {
	if list == nil {
		return []string{}
	}
	return list
}

func TestAChainRanksTheCardAtItsFootFirst(t *testing.T) {
	project, loaded := fixture(t,
		spec{id: "TASK-1", column: "todo"},
		spec{id: "TASK-2", column: "todo", gate: needs("TASK-1", "todo")},
		spec{id: "TASK-3", column: "todo", gate: needs("TASK-2", "todo")})
	m := Build(project, loaded)
	if got := len(m.Blocked); got != 2 || m.Blocked[0].ID != "TASK-2" || m.Blocked[1].ID != "TASK-3" {
		t.Fatalf("blocked = %+v, want TASK-2 and TASK-3 in board order", m.Blocked)
	}
	if len(m.Leaves) != 0 || len(m.Cycles) != 0 {
		t.Errorf("leaves %+v cycles %+v, want none", m.Leaves, m.Cycles)
	}
	first := m.Unblockers[0]
	if first.Rank != 1 || first.ID != "TASK-1" || !first.Actionable {
		t.Fatalf("first unblocker = %+v, want actionable TASK-1 ranked 1", first)
	}
	if !reflect.DeepEqual(first.Through, []string{"TASK-2", "TASK-3"}) || !reflect.DeepEqual(first.Directly, []string{"TASK-2"}) {
		t.Errorf("TASK-1 through %v directly %v, want [TASK-2 TASK-3] and [TASK-2]", first.Through, first.Directly)
	}
	second := m.Unblockers[1]
	if second.ID != "TASK-2" || second.Actionable || second.WhyNot == "" {
		t.Errorf("second unblocker = %+v, want TASK-2, not actionable, with a reason", second)
	}
	if w := m.Blocked[1].Waits; len(w) != 1 || w[0].Kind != "needs" || w[0].Card != "TASK-2" ||
		!strings.Contains(w[0].Text, "TASK-2") {
		t.Errorf("TASK-3 waits = %+v, want one needs wait on TASK-2", w)
	}
}

func TestADiamondCountsTheSharedCardOnce(t *testing.T) {
	project, loaded := fixture(t,
		spec{id: "TASK-1", column: "todo"},
		spec{id: "TASK-2", column: "todo", gate: needs("TASK-1", "todo")},
		spec{id: "TASK-3", column: "todo", gate: needs("TASK-1", "todo")},
		spec{id: "TASK-4", column: "todo", gate: needs("TASK-2", "todo", "TASK-3", "todo")})
	m := Build(project, loaded)
	first := m.Unblockers[0]
	if first.ID != "TASK-1" || !reflect.DeepEqual(first.Through, []string{"TASK-2", "TASK-3", "TASK-4"}) ||
		!reflect.DeepEqual(first.Directly, []string{"TASK-2", "TASK-3"}) {
		t.Errorf("first = %+v, want TASK-1 through [2 3 4], directly [2 3]", first)
	}
	count := 0
	for _, u := range m.Unblockers {
		if u.ID == "TASK-1" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("TASK-1 listed %d times", count)
	}
}

func TestAHeldLineNamingACardIsAnEdgeAndOneNamingNoneIsALeaf(t *testing.T) {
	project, loaded := fixture(t,
		spec{id: "TASK-1", column: "todo"},
		spec{id: "TASK-2", column: "todo", held: "blocked: TASK-1 and the staging key"},
		spec{id: "TASK-3", column: "todo", held: "waiting: the vendor's reply"},
		spec{id: "TASK-4", column: "todo", held: "blocked: TASK-99 is not on the board"})
	m := Build(project, loaded)
	if first := m.Unblockers[0]; first.ID != "TASK-1" || !reflect.DeepEqual(first.Directly, []string{"TASK-2"}) {
		t.Errorf("first = %+v, want TASK-1 holding up TASK-2", first)
	}
	if w := m.Blocked[0].Waits; len(w) != 1 || w[0].Kind != "held" || w[0].Card != "TASK-1" ||
		w[0].Text != "blocked: TASK-1 and the staging key" {
		t.Errorf("TASK-2 waits = %+v, want a held wait on TASK-1 with the line verbatim", w)
	}
	leaves := map[string]Leaf{}
	for _, leaf := range m.Leaves {
		leaves[leaf.Card] = leaf
	}
	if l := leaves["TASK-3"]; l.Kind != "held-text" || l.Text != "waiting: the vendor's reply" ||
		!reflect.DeepEqual(l.Through, []string{"TASK-3"}) {
		t.Errorf("TASK-3 leaf = %+v, want held-text, verbatim, through itself", l)
	}
	if l := leaves["TASK-4"]; l.Kind != "held-text" {
		t.Errorf("TASK-4 leaf = %+v, want held-text (the named card is not on the board)", l)
	}
	if _, ok := leaves["TASK-2"]; ok {
		t.Error("TASK-2 has an edge to a card, so it is not itself a leaf")
	}
}

func TestAnOwnerGateIsALeafThatNamesTheCardsItHoldsUp(t *testing.T) {
	project, loaded := fixture(t,
		spec{id: "TASK-1", column: "qa-approved", gate: ownerGate("TASK-1")},
		spec{id: "TASK-2", column: "todo", gate: needs("TASK-1", "qa-approved")})
	m := Build(project, loaded)
	if len(m.Leaves) != 1 {
		t.Fatalf("leaves = %+v, want one", m.Leaves)
	}
	leaf := m.Leaves[0]
	if leaf.Kind != "owner" || leaf.Card != "TASK-1" || !strings.Contains(leaf.Text, "gate pass TASK-1") ||
		!reflect.DeepEqual(leaf.Through, []string{"TASK-1", "TASK-2"}) {
		t.Errorf("leaf = %+v, want an owner leaf on TASK-1 through [TASK-1 TASK-2] naming the pass command", leaf)
	}
	if u := m.Unblockers[0]; u.ID != "TASK-1" || u.Actionable || !strings.Contains(u.WhyNot, "owner") {
		t.Errorf("unblocker = %+v, want TASK-1 not actionable because of the owner's gate", u)
	}
}

func TestANeedNotOnTheBoardIsAMissingLeaf(t *testing.T) {
	project, loaded := fixture(t, spec{id: "TASK-2", column: "todo", gate: needs("TASK-77", "")})
	m := Build(project, loaded)
	if len(m.Leaves) != 1 || m.Leaves[0].Kind != "missing" || m.Leaves[0].Card != "TASK-2" ||
		!strings.Contains(m.Leaves[0].Text, "TASK-77") {
		t.Errorf("leaves = %+v, want a missing leaf on TASK-2 naming TASK-77", m.Leaves)
	}
	if len(m.Unblockers) != 0 {
		t.Errorf("unblockers = %+v, a card that is not on the board cannot be one", m.Unblockers)
	}
	if w := m.Blocked[0].Waits[0]; w.Kind != KindMissing || w.Card != "TASK-77" {
		t.Errorf("wait = %+v, want a missing wait on TASK-77", w)
	}
}

func TestACycleIsReportedOnceAndTerminates(t *testing.T) {
	project, loaded := fixture(t,
		spec{id: "TASK-3", column: "todo", gate: needs("TASK-2", "todo")},
		spec{id: "TASK-2", column: "todo", gate: needs("TASK-3", "todo", "TASK-4", "todo")},
		spec{id: "TASK-4", column: "todo"})
	m := Build(project, loaded)
	if !reflect.DeepEqual(m.Cycles, [][]string{{"TASK-2", "TASK-3"}}) {
		t.Errorf("cycles = %v, want [[TASK-2 TASK-3]] once", m.Cycles)
	}
	if u := m.Unblockers[0]; u.ID != "TASK-4" || !u.Actionable {
		t.Errorf("first unblocker = %+v, want the actionable TASK-4 that frees the loop's entry", u)
	}
}

func TestADoneCardIsNeverABlockerAndAParkedOneIsNotActionable(t *testing.T) {
	project, loaded := fixture(t,
		spec{id: "TASK-1", column: "done", held: "blocked: TASK-2"},
		spec{id: "TASK-2", column: "paused"},
		spec{id: "TASK-3", column: "todo", held: "blocked: TASK-1"},
		spec{id: "TASK-4", column: "todo", gate: needs("TASK-2", "paused")})
	m := Build(project, loaded)
	for _, b := range m.Blocked {
		if b.ID == "TASK-1" || b.ID == "TASK-2" {
			t.Errorf("%s is listed as blocked; a done or parked card is not", b.ID)
		}
	}
	for _, u := range m.Unblockers {
		if u.ID == "TASK-1" {
			t.Error("a done card is listed as an unblocker")
		}
	}
	if u := m.Unblockers[0]; u.ID != "TASK-2" || u.Actionable || !strings.Contains(u.WhyNot, "parked") {
		t.Errorf("unblocker = %+v, want the parked TASK-2, not actionable, parked", u)
	}
	if len(m.Leaves) != 1 || m.Leaves[0].Card != "TASK-3" || m.Leaves[0].Kind != "held-text" {
		t.Errorf("leaves = %+v, want TASK-3's Held line (its named card is done) as held-text", m.Leaves)
	}
}

func TestActionableCardsComeFirstThenMostHeldUpThenPullOrderThenID(t *testing.T) {
	project, loaded := fixture(t,
		spec{id: "TASK-1", column: "todo", held: "waiting: the owner"},
		spec{id: "TASK-2", column: "todo", owner: "claude:other"},
		spec{id: "TASK-5", column: "todo"},
		spec{id: "TASK-6", column: "todo"},
		spec{id: "TASK-7", column: "designed"},
		spec{id: "TASK-10", column: "todo", gate: needs("TASK-1", "todo", "TASK-2", "todo")},
		spec{id: "TASK-11", column: "todo", gate: needs("TASK-1", "todo")},
		spec{id: "TASK-12", column: "todo", gate: needs("TASK-5", "todo", "TASK-6", "todo", "TASK-7", "designed")},
		spec{id: "TASK-13", column: "todo", gate: needs("TASK-6", "todo", "TASK-7", "designed")},
		spec{id: "TASK-14", column: "todo", gate: needs("TASK-7", "designed")})
	m := Build(project, loaded)
	var order []string
	for _, u := range m.Unblockers {
		order = append(order, u.ID)
	}
	// TASK-7 holds up 3 (designed is further along than todo), TASK-6 two, TASK-5 one; TASK-1 (held, 2) and TASK-2
	// (claimed, 1) are listed after every actionable card.
	want := []string{"TASK-7", "TASK-6", "TASK-5", "TASK-1", "TASK-2"}
	if !reflect.DeepEqual(order, want) {
		t.Fatalf("order = %v, want %v", order, want)
	}
	for i, u := range m.Unblockers {
		if u.Rank != i+1 {
			t.Errorf("%s rank = %d, want %d", u.ID, u.Rank, i+1)
		}
	}
	if u := m.Unblockers[4]; !strings.Contains(u.WhyNot, "claude:other") {
		t.Errorf("claimed card why_not = %q, want the claim's owner", u.WhyNot)
	}
}

func TestANothingBlockedBoardGivesEmptyListsNotNull(t *testing.T) {
	project, loaded := fixture(t, spec{id: "TASK-1", column: "todo"})
	m := Build(project, loaded)
	data := string(m.JSON())
	for _, want := range []string{`"blocked":[]`, `"unblockers":[]`, `"leaves":[]`, `"cycles":[]`} {
		if !strings.Contains(data, want) {
			t.Errorf("json %s lacks %s", data, want)
		}
	}
	if !strings.Contains(m.Text(), "nothing is blocked") {
		t.Errorf("text = %q, want it to say nothing is blocked", m.Text())
	}
}
