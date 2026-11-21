// Package workspace holds every rule that depends on the flow's isolation mode: how a card's change is kept apart
// from the board, what its Where line records, how to make and remove its workspace, and whether the record is real.
//
// The modes (`[flow] isolation`): worktree (a git worktree and a branch per card, the default), branch (a branch per
// card in the one checkout), shared (the one checkout, whatever branch it is on) and external (a workspace the host
// owns, recorded as `Where: external <ref>`). Nothing else in the command knows a mode's words.
package workspace

import (
	"fmt"
	"os"
	"strings"

	"github.com/cloudbusting/kanbanzai/internal/config"
	"github.com/cloudbusting/kanbanzai/internal/gitutil"
	"github.com/cloudbusting/kanbanzai/internal/pluginlink"
)

const (
	Worktree = "worktree"
	Branch   = "branch"
	Shared   = "shared"
	External = "external"
)

// Of is the project's isolation mode; an unset one is the default, worktree.
func Of(p *config.Project) string {
	if p.Isolation == "" {
		return Worktree
	}
	return p.Isolation
}

// ModeName is the mode as `workflow show` has always named it, and its siblings.
func ModeName(p *config.Project) string {
	switch Of(p) {
	case Branch:
		return "branch-per-card"
	case Shared:
		return "shared-checkout"
	case External:
		return "external-workspace"
	}
	return "worktree-per-card"
}

// Needs is true when a claimed card in a workspace column must carry a record of its workspace in its Where line.
func Needs(p *config.Project) bool { return Of(p) != Shared }

// Parallel is true when the mode lets several cards be worked at once.
func Parallel(p *config.Project) bool { return Of(p) == Worktree || Of(p) == External }

// word is the Where word that records a card's workspace in this mode ("" when it records none).
func word(p *config.Project) string {
	switch Of(p) {
	case Worktree, Branch, External:
		return Of(p)
	}
	return ""
}

// Parse splits a Where line into its word and reference: "worktree W-07 (branch W-07)" is ("worktree", "W-07").
// "—" and "main" have no reference; an unrecognised first word is returned as it is.
func Parse(where string) (kind, ref string) {
	words := strings.Fields(strings.ReplaceAll(where, "`", ""))
	if len(words) == 0 {
		return "—", ""
	}
	switch words[0] {
	case Worktree, Branch, External:
		if len(words) >= 2 {
			return words[0], words[1]
		}
		return words[0], ""
	}
	return words[0], ""
}

// Normal is a Where line without its note: the two words of a workspace record, or the one word.
func Normal(where string) string {
	kind, ref := Parse(where)
	if ref != "" {
		return kind + " " + ref
	}
	return kind
}

func idRef(p *config.Project, ref string) bool { return p.CardRe.MatchString(ref) }

// refOK is true when the reference after the mode's word is well-formed: a card id, or any one word for external.
func refOK(p *config.Project, kind, ref string) bool {
	if ref == "" {
		return false
	}
	if kind == External {
		return true
	}
	return idRef(p, ref)
}

// WhereValid is true when a brief's Where line is allowed on a card in a column of this kind (open, parked, done).
// An open card takes "—", "main" and its mode's record; a finished or parked card takes any mode's record, so
// switching mode never invalidates the archive.
func WhereValid(p *config.Project, where, columnKind string) bool {
	kind, ref := Parse(where)
	switch kind {
	case "—", "main":
		return true
	case Worktree, Branch, External:
		if !refOK(p, kind, ref) {
			return false
		}
		return columnKind != "open" || kind == word(p)
	}
	return false
}

// Recorded is true when a claimed workspace-column card's Where line carries the mode's workspace record. In a
// shared checkout there is nothing to record, so any valid line (including "—") counts.
func Recorded(p *config.Project, where string) bool {
	if !Needs(p) {
		kind, _ := Parse(where)
		return kind == "—" || kind == "main"
	}
	kind, ref := Parse(where)
	return kind == word(p) && refOK(p, kind, ref)
}

// Exists is true when the record names a workspace that is really there: the worktree's folder, the branch, or (for
// an external workspace) a non-empty reference the host owns.
func Exists(p *config.Project, where string) bool {
	if !Needs(p) {
		return true
	}
	if !Recorded(p, where) {
		return false
	}
	_, ref := Parse(where)
	switch Of(p) {
	case Worktree:
		info, err := os.Stat(p.WorktreePath(ref))
		return err == nil && info.IsDir()
	case Branch:
		_, err := gitutil.Git(p.Root, "rev-parse", "--verify", "--quiet", "refs/heads/"+p.Branch(ref, false))
		return err == nil
	}
	return true
}

// Record is the Where value to set for a card in this mode.
func Record(p *config.Project, card string) string {
	switch Of(p) {
	case Worktree, Branch:
		return Of(p) + " " + card
	case External:
		return "external <ref>"
	}
	return "main"
}

// Vocabulary is the Where lines an open card may carry, as the check message shows them.
func Vocabulary(p *config.Project) []string {
	out := []string{"—", "main"}
	switch Of(p) {
	case Worktree, Branch:
		for _, example := range p.IDExamples() {
			out = append(out, Of(p)+" "+example)
		}
	case External:
		out = append(out, "external <ref>")
	}
	return out
}

// MakeSteps are the commands that give a card its workspace: for a new card first, for one whose branch exists second.
//
// A project that names its own command (project.worktree_command) gets that one line, with the card's values filled in,
// in place of both git lines: the host's script decides whether the branch exists. It is printed and never run here.
func MakeSteps(p *config.Project, card string) []string {
	branch := p.Branch(card, false)
	switch Of(p) {
	case Worktree:
		path := p.WorktreeHint(card)
		if p.WorktreeCommand != "" {
			return []string{strings.NewReplacer("{id}", card, "{branch}", branch, "{path}", path, "{base}", p.MainBranch,
				"{main}", p.Root).Replace(p.WorktreeCommand)}
		}
		return []string{fmt.Sprintf("git worktree add %s -b %s %s", path, branch, p.MainBranch),
			fmt.Sprintf("git worktree add %s %s", path, branch)}
	case Branch:
		return []string{fmt.Sprintf("git switch -c %s %s", branch, p.MainBranch), fmt.Sprintf("git switch %s", branch)}
	}
	return nil
}

// RemoveSteps are the commands that take a finished card's workspace away, once its work is merged.
//
// When the host merges the work (`[flow] integration = "remote"`), the branch is not merged into the main branch here
// and a squash merge leaves its commits out of it altogether, so `git branch -d` would refuse. The step is `-D`
// then, and is safe only because /cleanup runs it for a workspace `wip --audit` listed as landed.
func RemoveSteps(p *config.Project, card string) []string {
	branch := p.Branch(card, false)
	force := "-d"
	if p.Integration == "remote" {
		force = "-D"
	}
	switch Of(p) {
	case Worktree:
		return []string{fmt.Sprintf("git worktree remove %s", p.WorktreeHint(card)), fmt.Sprintf("git branch %s %s", force, branch)}
	case Branch:
		return []string{fmt.Sprintf("git switch %s", p.MainBranch), fmt.Sprintf("git branch %s %s", force, branch)}
	}
	return nil
}

// Missing is what `move` says when a card's step needs a workspace record it has not got.
func Missing(p *config.Project, label, card string) string {
	switch Of(p) {
	case Branch:
		return fmt.Sprintf("%s needs the card's branch: make it (%s) and pass --where \"branch %s\" in this move",
			label, MakeSteps(p, card)[0], card)
	case External:
		return fmt.Sprintf("%s needs the card's workspace recorded: pass --where \"external <ref>\" in this move", label)
	}
	if p.WorktreeCommand != "" {
		return fmt.Sprintf("%s needs the card's worktree: make it (%s) and pass --where \"worktree %s\" in this move",
			label, MakeSteps(p, card)[0], card)
	}
	return fmt.Sprintf("%s needs the card's worktree: make it (../<repo>-%s) and pass --where \"worktree %s\" in this move",
		label, card, card)
}

// StartHint is what a claim says when it leaves a card where it is because its next step needs a workspace first.
// It is empty in a shared checkout, which needs none.
func StartHint(p *config.Project, card string) string {
	switch Of(p) {
	case Worktree:
		made := p.WorktreeHint(card)
		if p.WorktreeCommand != "" {
			made = MakeSteps(p, card)[0]
		}
		return fmt.Sprintf("now: make the worktree (%s), then run: kanbanzai move %s start --where \"worktree %s\"",
			made, card, card)
	case Branch:
		return fmt.Sprintf("now: make the branch (%s), then run: kanbanzai move %s start --where \"branch %s\"",
			MakeSteps(p, card)[0], card, card)
	case External:
		return fmt.Sprintf("now: record the card's workspace, then run: kanbanzai move %s start --where "+
			"\"external <ref>\"", card)
	}
	return ""
}

// Rule is the one line `kanbanzai context` gives a session about where a card's change is made.
func Rule(p *config.Project) string {
	switch Of(p) {
	case Branch:
		pattern, err := p.Expand(p.BranchPattern, "")
		if err != nil {
			pattern = p.BranchPattern
		}
		return fmt.Sprintf("Every change to a card is made on its own branch (%s) in the one checkout: one card at a "+
			"time, and its brief is committed on its branch. `kanbanzai workspace ID` says how to make and remove it.",
			pattern)
	case Shared:
		return "Every change is made in the one shared checkout; there is no workspace record. Parallel sessions " +
			"share one tree: work that runs in parallel needs worktree mode."
	case External:
		return "Every change to a card is made in the workspace the host owns; record it with `Where: external <ref>` " +
			"(`kanbanzai workspace ID`). The main checkout holds only the board."
	}
	worktree, err := p.Expand(p.WorktreePattern, "")
	if err != nil {
		worktree = p.WorktreePattern
	}
	return fmt.Sprintf("Every change to a card is made in its worktree (%s); the main checkout holds only the board.",
		worktree)
}

// Info is what a card in this project asks of its workspace: `kanbanzai workspace ID` prints it.
type Info struct {
	Card            string   `json:"card"`
	Setting         string   `json:"setting"`
	Mode            string   `json:"mode"`
	NeedsWorkspace  bool     `json:"needs_workspace"`
	Record          string   `json:"record"`
	WhereVocabulary []string `json:"where_vocabulary"`
	Make            []string `json:"make"`
	// MakeCustom is true when Make is the project's own worktree_command (one line) and not git's two.
	MakeCustom bool     `json:"make_custom"`
	Then       string   `json:"then"`
	WorkIn     string   `json:"work_in"`
	Remove     []string `json:"remove"`
	Parallel   bool     `json:"parallel"`
	// Link is the command that gives a new worktree the bundled OpenCode plugin's pointer file (BUG-79), or "" when
	// the project does not need it.
	Link string `json:"link"`
}

// Describe is the card's Info in the project's mode.
func Describe(p *config.Project, card string) Info {
	info := Info{Card: card, Setting: Of(p), Mode: ModeName(p), NeedsWorkspace: Needs(p), Record: Record(p, card),
		WhereVocabulary: Vocabulary(p), Make: MakeSteps(p, card), Remove: RemoveSteps(p, card), Parallel: Parallel(p)}
	if info.Make == nil {
		info.Make = []string{}
	}
	if info.Remove == nil {
		info.Remove = []string{}
	}
	info.MakeCustom = Of(p) == Worktree && p.WorktreeCommand != ""
	info.Then = fmt.Sprintf("kanbanzai move %s start", card)
	if Needs(p) {
		info.Then += fmt.Sprintf(" --where \"%s\"", info.Record)
	}
	info.Link = LinkStep(p, card)
	switch Of(p) {
	case Worktree:
		info.WorkIn = p.WorktreePath(card)
	case External:
		info.WorkIn = "the workspace the host owns"
	default:
		info.WorkIn = p.Root
	}
	return info
}

// LinkStep is the command that gives the card's new worktree the bundled OpenCode plugin's pointer file, which OpenCode
// reads in the checkout it runs in and git does not carry (BUG-79). It is "" unless the project uses worktrees and its
// main checkout holds that file; `move` runs the same link when a card's worktree is recorded.
func LinkStep(p *config.Project, card string) string {
	if Of(p) != Worktree {
		return ""
	}
	if _, err := pluginlink.Read(p.Root); err != nil {
		return ""
	}
	return "kanbanzai plugins link " + p.WorktreePath(card)
}
