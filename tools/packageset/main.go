// tools/packageset chooses which Checked cards one package run includes. It is read-only: it never claims, moves
// or writes a card. Run it from the main checkout:
//
//	go run ./tools/packageset            # JSON on stdout
//	go run ./tools/packageset -main main -column checked
//
// A card is included only when the board and `kanbanzai step --for ID --json` say it is ready for the deploy-qa
// step, nobody holds or owns it, and its branch is merged into HEAD. Anything else, and any card it cannot read,
// is left out with a reason; the tool never asks. It exits non-zero, with nothing included, when the board cannot
// be read or the checkout is not on the main branch. It runs only `kanbanzai` and `git`, with no network.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Report is the selector's answer. Tree is the core's reading of the main checkout (`kanbanzai tree --json`) and Clean
// is true only when that verdict is "clean"; both are information for the package skill, which owns the rule.
// InHead on an excluded card means its branch is already inside HEAD, so the artifact carries its code although
// the card is not packaged.
type Report struct {
	Head    string     `json:"head"`
	Branch  string     `json:"branch"`
	Clean   bool       `json:"clean"`
	Tree    Tree       `json:"tree"`
	Include []string   `json:"include"`
	Exclude []Excluded `json:"exclude"`
}

// MarshalJSON writes empty lists, never null, so a consumer can always iterate.
func (r Report) MarshalJSON() ([]byte, error) {
	type plain Report
	r.Tree = r.Tree.lists()
	if r.Include == nil {
		r.Include = []string{}
	}
	if r.Exclude == nil {
		r.Exclude = []Excluded{}
	}
	return json.Marshal(plain(r))
}

// Tree is what `kanbanzai tree --json` said. A verdict of "blocked" with a reason is also what an unreadable answer
// becomes: the selector never reports a tree it could not read as clean.
type Tree struct {
	Verdict    string   `json:"verdict"`
	Board      []string `json:"board"`
	Other      []string `json:"other"`
	Uncertain  []string `json:"uncertain"`
	InProgress string   `json:"in_progress"`
	Reasons    []string `json:"reasons"`
}

func (t Tree) lists() Tree {
	for _, list := range []*[]string{&t.Board, &t.Other, &t.Uncertain, &t.Reasons} {
		if *list == nil {
			*list = []string{}
		}
	}
	return t
}

func readTree(r Runner) Tree {
	out, code, err := r.Run("kanbanzai", "tree", "--json")
	var tree Tree
	// Exit 0 is go ahead and 1 is blocked, and both print the report; anything else is not an answer.
	if err != nil || (code != 0 && code != 1) || json.Unmarshal(out, &tree) != nil || tree.Verdict == "" {
		return Tree{Verdict: "blocked", Reasons: []string{fmt.Sprintf("kanbanzai tree --json gave no answer (exit %d, %v)", code, err)}}.lists()
	}
	return tree.lists()
}

type Excluded struct {
	ID     string `json:"id"`
	Reason string `json:"reason"`
	InHead bool   `json:"in_head"`
}

// Runner runs a command and returns its output and exit code. The error is for a command that did not run.
type Runner interface {
	Run(name string, args ...string) ([]byte, int, error)
}

type execRunner struct{ dir string }

func (r execRunner) Run(name string, args ...string) ([]byte, int, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = r.dir
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return out.Bytes(), exit.ExitCode(), nil
	}
	return out.Bytes(), 0, err
}

type boardJSON struct {
	Columns []struct {
		Key   string `json:"key"`
		Cards []struct {
			ID    string  `json:"id"`
			Stage string  `json:"stage"`
			Where string  `json:"where"`
			Owner *string `json:"owner"`
			Held  *string `json:"held"`
		} `json:"cards"`
	} `json:"columns"`
}

type stepJSON struct {
	Column string `json:"column"`
	Route  struct {
		Action  string `json:"action"`
		Step    string `json:"step"`
		Blocked bool   `json:"blocked"`
		Reason  string `json:"reason"`
	} `json:"route"`
	SkippedSteps []string `json:"skipped_steps"`
}

// Select reads the cards in the column and sorts them into include and exclude.
func Select(r Runner, column, main string) (Report, error) {
	report := Report{Include: []string{}, Exclude: []Excluded{}}
	out, code, err := r.Run("kanbanzai", "board", "--json")
	if err != nil || code != 0 {
		return report, fmt.Errorf("kanbanzai board --json failed (exit %d, %v)", code, err)
	}
	var board boardJSON
	if err := json.Unmarshal(out, &board); err != nil {
		return report, fmt.Errorf("kanbanzai board --json: %v", err)
	}
	if report.Head, err = gitLine(r, "rev-parse", "HEAD"); err != nil {
		return report, err
	}
	if report.Branch, err = gitLine(r, "rev-parse", "--abbrev-ref", "HEAD"); err != nil {
		return report, err
	}
	report.Tree = readTree(r)
	report.Clean = report.Tree.Verdict == "clean"
	if report.Branch != main {
		return report, fmt.Errorf("the checkout is on %q, not the main branch %q", report.Branch, main)
	}
	for _, col := range board.Columns {
		if col.Key != column {
			continue
		}
		for _, card := range col.Cards {
			c := Candidate{ID: card.ID, Stage: card.Stage}
			if card.Owner != nil {
				c.Owner = *card.Owner
			}
			if card.Held != nil {
				c.Held = *card.Held
			}
			out, code, err := r.Run("kanbanzai", "step", "--for", card.ID, "--json")
			var step stepJSON
			switch {
			case err != nil || code != 0:
				c.Err = fmt.Sprintf("step --for failed (exit %d, %v)", code, err)
			case json.Unmarshal(out, &step) != nil:
				c.Err = "step --for did not return JSON"
			default:
				c.Column = step.Column
				c.Route = Route{Action: step.Route.Action, Step: step.Route.Step, Blocked: step.Route.Blocked, Reason: step.Route.Reason}
				c.SkippedSteps = step.SkippedSteps
			}
			c.Merge = merged(r, card.Where)
			d := Classify(c)
			if d.Include {
				report.Include = append(report.Include, d.ID)
				continue
			}
			report.Exclude = append(report.Exclude, Excluded{ID: d.ID, Reason: d.Reason, InHead: c.Merge == MergeYes})
		}
	}
	return report, nil
}

func gitLine(r Runner, args ...string) (string, error) {
	out, code, err := r.Run("git", args...)
	if err != nil || code != 0 {
		return "", fmt.Errorf("git %s failed (exit %d, %v)", strings.Join(args, " "), code, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// merged proves a card's branch is inside HEAD. The branch is the one a "worktree NAME" record names; any other
// record, a missing branch or an odd git answer is Unknown, never Yes.
func merged(r Runner, where string) Merge {
	fields := strings.Fields(where)
	if len(fields) != 2 || fields[0] != "worktree" {
		return MergeUnknown
	}
	ref := "refs/heads/" + fields[1]
	if _, code, err := r.Run("git", "rev-parse", "--verify", "--quiet", ref); err != nil || code != 0 {
		return MergeUnknown
	}
	_, code, err := r.Run("git", "merge-base", "--is-ancestor", ref, "HEAD")
	switch {
	case err != nil:
		return MergeUnknown
	case code == 0:
		return MergeYes
	case code == 1:
		return MergeNo
	}
	return MergeUnknown
}

func main() {
	dir := flag.String("dir", ".", "the main checkout")
	column := flag.String("column", "checked", "the board column key whose cards are candidates")
	branch := flag.String("main", "main", "the main branch the checkout must be on")
	flag.Parse()
	report, err := Select(execRunner{dir: *dir}, *column, *branch)
	if err != nil {
		fmt.Fprintln(os.Stderr, "packageset:", err)
		os.Exit(1)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(report); err != nil {
		fmt.Fprintln(os.Stderr, "packageset:", err)
		os.Exit(1)
	}
}
