// Package detangle reads the loaded board and works out what holds each card up, chain by chain, and which cards to do
// first to free the most of them (TASK-41). It is read-only: it takes no claim, writes nothing and makes no network
// request. The map is for a skill's own resolution (`/detangle`), printed by `kanbanzai wip --blocked`.
package detangle

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/cloudbusting/kanbanzai/internal/board"
	"github.com/cloudbusting/kanbanzai/internal/config"
	"github.com/cloudbusting/kanbanzai/internal/gate"
)

// Wait kinds. needs and held are edges to another card; the rest end a chain (a leaf).
const (
	KindNeeds    = "needs"
	KindHeld     = "held"
	KindMissing  = "missing"   // a Needs line names a card that is not on the board
	KindHeldText = "held-text" // a Held line that names no open card on the board; shown as written
)

// Wait is one reason a card cannot move on.
type Wait struct {
	Kind        string `json:"kind"`
	Card        string `json:"card,omitempty"` // the card waited on, for needs and held
	Column      string `json:"column,omitempty"`
	ColumnLabel string `json:"column_label,omitempty"`
	Wants       string `json:"wants,omitempty"` // for needs: the column the gate wants the card to reach
	Text        string `json:"text"`
}

// Blocked is a card that cannot move on, and every reason it cannot.
type Blocked struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Column      string `json:"column"`
	ColumnLabel string `json:"column_label"`
	Waits       []Wait `json:"waits"`
}

// Unblocker is a card other cards wait on, directly or through a chain.
type Unblocker struct {
	Rank        int      `json:"rank"`
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Column      string   `json:"column"`
	ColumnLabel string   `json:"column_label"`
	Actionable  bool     `json:"actionable"`
	WhyNot      string   `json:"why_not,omitempty"`
	Directly    []string `json:"directly"` // the cards whose Needs or Held line names it
	Through     []string `json:"through"`  // those and every card that waits on them in turn
}

// Leaf is where a chain ends: a person, a date, a system, a card that is not on the board or a Held line in words.
type Leaf struct {
	Kind    string   `json:"kind"` // owner, ci, external, date, missing or held-text
	Card    string   `json:"card"` // the card that carries it
	Text    string   `json:"text"`
	Through []string `json:"through"` // that card first, then every card that waits on it
}

// Map is the whole answer. Every list is present and never null.
type Map struct {
	Blocked    []Blocked   `json:"blocked"`
	Unblockers []Unblocker `json:"unblockers"`
	Leaves     []Leaf      `json:"leaves"`
	Cycles     [][]string  `json:"cycles"`
}

type node struct {
	at     board.CardAt
	order  int
	parked bool
	done   bool
	waits  []Wait
	edges  []string // the open cards this one waits on, in the order of its waits
}

// Build is the map of a loaded board: every open card with its reasons, the cards that free the most of them, where
// the chains end and any cycle. An unblocker that cannot be started (held, claimed, parked, itself blocked) is listed
// after every card that can.
func Build(project *config.Project, loaded *board.Board) *Map {
	m := &Map{Blocked: []Blocked{}, Unblockers: []Unblocker{}, Leaves: []Leaf{}, Cycles: [][]string{}}
	nodes := map[string]*node{}
	var inOrder []*node
	for _, at := range board.Cards(loaded) {
		column := project.ByKey[at.Column]
		n := &node{at: at, order: len(inOrder), parked: column.Kind == "parked", done: column.Kind == "done"}
		nodes[at.Card.ID] = n
		inOrder = append(inOrder, n)
	}
	open := func(id string) bool { n, ok := nodes[id]; return ok && !n.done }
	for _, n := range inOrder {
		if n.done || n.parked {
			continue
		}
		n.waits, n.edges = waitsOf(n.at.Card, open)
		if len(n.waits) > 0 {
			m.Blocked = append(m.Blocked, Blocked{ID: n.at.Card.ID, Title: n.at.Card.Title, Column: n.at.Column,
				ColumnLabel: project.Label(n.at.Column), Waits: n.waits})
		}
	}
	dependents := map[string][]string{}
	for _, n := range inOrder {
		for _, target := range n.edges {
			dependents[target] = append(dependents[target], n.at.Card.ID)
		}
	}
	downstream := func(id string) []string {
		seen := map[string]bool{id: true}
		queue := []string{id}
		for len(queue) > 0 {
			current := queue[0]
			queue = queue[1:]
			for _, next := range dependents[current] {
				if !seen[next] {
					seen[next] = true
					queue = append(queue, next)
				}
			}
		}
		delete(seen, id)
		return inBoardOrder(nodes, seen)
	}
	pull := map[string]int{}
	for index, key := range project.PullOrder {
		pull[key] = index
	}
	for _, n := range inOrder {
		through := downstream(n.at.Card.ID)
		if len(through) == 0 {
			continue
		}
		directly := map[string]bool{}
		for _, id := range dependents[n.at.Card.ID] {
			directly[id] = true
		}
		why := whyNot(n)
		m.Unblockers = append(m.Unblockers, Unblocker{ID: n.at.Card.ID, Title: n.at.Card.Title, Column: n.at.Column,
			ColumnLabel: project.Label(n.at.Column), Actionable: why == "", WhyNot: why,
			Directly: inBoardOrder(nodes, directly), Through: through})
	}
	sort.SliceStable(m.Unblockers, func(i, j int) bool {
		a, b := m.Unblockers[i], m.Unblockers[j]
		switch {
		case a.Actionable != b.Actionable:
			return a.Actionable
		case len(a.Through) != len(b.Through):
			return len(a.Through) > len(b.Through)
		case len(a.Directly) != len(b.Directly):
			return len(a.Directly) > len(b.Directly)
		}
		if pa, pb := pullIndex(pull, a.Column), pullIndex(pull, b.Column); pa != pb {
			return pa < pb
		}
		return nodes[a.ID].order < nodes[b.ID].order
	})
	for i := range m.Unblockers {
		m.Unblockers[i].Rank = i + 1
	}
	for _, n := range inOrder {
		for _, w := range n.waits {
			if w.Kind == KindNeeds || w.Kind == KindHeld {
				continue
			}
			m.Leaves = append(m.Leaves, Leaf{Kind: w.Kind, Card: n.at.Card.ID, Text: w.Text,
				Through: append([]string{n.at.Card.ID}, downstream(n.at.Card.ID)...)})
		}
	}
	sort.SliceStable(m.Leaves, func(i, j int) bool { return len(m.Leaves[i].Through) > len(m.Leaves[j].Through) })
	m.Cycles = cycles(inOrder)
	return m
}

func pullIndex(pull map[string]int, column string) int {
	if index, ok := pull[column]; ok {
		return index
	}
	return len(pull)
}

func inBoardOrder(nodes map[string]*node, ids map[string]bool) []string {
	found := make([]*node, 0, len(ids))
	for id := range ids {
		found = append(found, nodes[id])
	}
	sort.Slice(found, func(i, j int) bool { return found[i].order < found[j].order })
	list := make([]string, 0, len(found))
	for _, n := range found {
		list = append(list, n.at.Card.ID)
	}
	return list
}

// waitsOf is every reason an open card cannot move on, and the open cards among them.
func waitsOf(card *board.Card, open func(string) bool) ([]Wait, []string) {
	waits := []Wait{}
	edges := []string{}
	if g := card.Gate; g != nil && g.State != gate.Open {
		if g.Type == "card" {
			for _, need := range g.Needs {
				if need.Met {
					continue
				}
				if !need.OnBoard {
					waits = append(waits, Wait{Kind: KindMissing, Card: need.Card,
						Text: fmt.Sprintf("needs %s, which is not on the board", need.Card)})
					continue
				}
				waits = append(waits, Wait{Kind: KindNeeds, Card: need.Card, Column: need.Column,
					ColumnLabel: need.ColumnLabel, Wants: need.Wants,
					Text: fmt.Sprintf("needs %s, which is in %s and must reach %s or later", need.Card, need.ColumnLabel,
						need.WantsLabel)})
				edges = append(edges, need.Card)
			}
		} else {
			text := strings.ReplaceAll(g.OpensWhen, "<ID>", card.ID)
			if g.State == gate.Failed {
				text = "its " + g.Type + " gate failed: " + g.Why
			}
			waits = append(waits, Wait{Kind: g.Type, Text: text})
		}
	}
	if card.Held != nil {
		named := []string{}
		for _, id := range gate.Needs(*card.Held) { // the same card-id reading as a Needs line
			if id != card.ID && open(id) {
				named = append(named, id)
			}
		}
		for _, id := range named {
			waits = append(waits, Wait{Kind: KindHeld, Card: id, Text: *card.Held})
			edges = append(edges, id)
		}
		if len(named) == 0 {
			waits = append(waits, Wait{Kind: KindHeldText, Text: *card.Held})
		}
	}
	return waits, edges
}

// whyNot is why a session cannot start a card now, or "" when it can.
func whyNot(n *node) string {
	card := n.at.Card
	switch {
	case n.parked:
		return "parked: only the owner un-parks it"
	case card.Owner != nil:
		return "claimed by " + *card.Owner
	case card.Held != nil:
		return "held: " + *card.Held
	case len(n.waits) > 0:
		first := n.waits[0]
		switch first.Kind {
		case KindNeeds, KindHeld, KindMissing:
			return "itself waiting: " + first.Text
		}
		return "waiting at its " + first.Kind + " gate: " + first.Text
	}
	return ""
}

// cycles is each loop of cards waiting on one another, once, starting from its lowest id and following the waits.
func cycles(inOrder []*node) [][]string {
	edges := map[string][]string{}
	for _, n := range inOrder {
		edges[n.at.Card.ID] = n.edges
	}
	found := [][]string{}
	reported := map[string]bool{}
	state := map[string]int{} // 1 on the path, 2 finished
	var path []string
	var visit func(id string)
	visit = func(id string) {
		state[id] = 1
		path = append(path, id)
		for _, next := range edges[id] {
			switch state[next] {
			case 0:
				visit(next)
			case 1:
				start := 0
				for index, member := range path {
					if member == next {
						start = index
					}
				}
				loop := rotate(append([]string{}, path[start:]...))
				if key := strings.Join(loop, ">"); !reported[key] {
					reported[key] = true
					found = append(found, loop)
				}
			}
		}
		path = path[:len(path)-1]
		state[id] = 2
	}
	for _, n := range inOrder {
		if state[n.at.Card.ID] == 0 {
			visit(n.at.Card.ID)
		}
	}
	return found
}

func rotate(loop []string) []string {
	low := 0
	for index, id := range loop {
		if id < loop[low] {
			low = index
		}
	}
	return append(loop[low:], loop[:low]...)
}

// JSON is the map as one compact JSON object.
func (m *Map) JSON() []byte {
	var out bytes.Buffer
	encoder := json.NewEncoder(&out)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(m); err != nil {
		return []byte("{}")
	}
	return bytes.TrimRight(out.Bytes(), "\n")
}

// Text is the map in a few lines, for whoever runs the flag by hand.
func (m *Map) Text() string {
	if len(m.Blocked) == 0 {
		return "Blocked cards: nothing is blocked."
	}
	lines := []string{fmt.Sprintf("Blocked cards: %d", len(m.Blocked))}
	for _, b := range m.Blocked {
		for _, w := range b.Waits {
			lines = append(lines, fmt.Sprintf("  %-9s %s", b.ID, w.Text))
		}
	}
	if len(m.Unblockers) > 0 {
		lines = append(lines, "", "Cards to do first (most blocked cards freed first):")
		for _, u := range m.Unblockers {
			status := "can start"
			if !u.Actionable {
				status = "cannot start: " + u.WhyNot
			}
			lines = append(lines, fmt.Sprintf("  %d. %s (%s) holds up %d: %s — %s", u.Rank, u.ID, u.ColumnLabel,
				len(u.Through), strings.Join(u.Through, ", "), status))
		}
	}
	if len(m.Leaves) > 0 {
		lines = append(lines, "", "Where the chains end:")
		for _, l := range m.Leaves {
			lines = append(lines, fmt.Sprintf("  %s on %s (holds up %s): %s", l.Kind, l.Card,
				strings.Join(l.Through, ", "), l.Text))
		}
	}
	for _, loop := range m.Cycles {
		lines = append(lines, "", "Cycle: "+strings.Join(loop, " -> ")+" -> "+loop[0])
	}
	return strings.Join(lines, "\n")
}
