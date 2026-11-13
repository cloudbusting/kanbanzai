package wip

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/cloudbusting/kanbanzai/internal/config"
	"github.com/cloudbusting/kanbanzai/internal/gitutil"
	"github.com/cloudbusting/kanbanzai/internal/workspace"
)

// IsolationLine is the one report line that names a mode other than the default, and what that mode gives up.
func IsolationLine(project *config.Project) string {
	switch workspace.Of(project) {
	case workspace.Branch:
		return "Isolation: branch — a branch per card in one checkout; one card at a time (one HEAD); " +
			"parallel work needs worktree mode"
	case workspace.Shared:
		return "Isolation: shared — one checkout, no workspace records; parallel sessions share one tree " +
			"(parallel work needs worktree mode)"
	case workspace.External:
		return "Isolation: external — the host owns each card's workspace; the audit trusts the recorded ref"
	}
	return ""
}

// isolationFindings are the audit rules of the modes other than worktree: the vocabulary of Where, the workspace
// record of a claimed card, and (per mode) the checkout, the branches and the extra worktrees.
func isolationFindings(project *config.Project, cards CardsByColumn, wts []Worktree, held map[string]map[string]any,
	only string, recovery bool) (hard, soft, cleanup []string) {
	mode := workspace.Of(project)
	var liveWork []string
	rowWhere := map[string]string{}
	cancelled := map[string]bool{}
	closed := closedCards(project, cards)
	for _, column := range project.Columns {
		if column.Kind == "done" {
			for _, row := range cards[column.Key] {
				rowWhere[row.Item] = row.Where
				cancelled[row.Item] = row.Cancelled
			}
			continue
		}
		for _, row := range cards[column.Key] {
			rowWhere[row.Item] = row.Where
			if !workspace.WhereValid(project, row.Where, column.Kind) {
				hard = append(hard, fmt.Sprintf("%s (%s): Where is '%s', which isolation %s does not use; set it to %s",
					row.Item, column.Label, row.Where, mode, strings.Join(workspace.Vocabulary(project), ", ")))
				continue
			}
			if column.Kind != "open" {
				continue
			}
			if row.Where == "main" {
				switch mode {
				case workspace.Branch:
					hard = append(hard, fmt.Sprintf("%s (%s): Where says main — the change is made on branch %s; record "+
						"'branch %s'", row.Item, column.Label, project.Branch(row.Item, false), row.Item))
				case workspace.External:
					hard = append(hard, fmt.Sprintf("%s (%s): Where says main — the change is made in the host's "+
						"workspace; record 'external <ref>'", row.Item, column.Label))
				}
			}
			record, hasRecord := held[row.Item]
			if !column.Worktree || !hasRecord || !isLive(record) {
				continue
			}
			liveWork = append(liveWork, row.Item)
			if !workspace.Recorded(project, row.Where) {
				_, _, owner := ClaimState(record)
				switch mode {
				case workspace.Branch:
					hard = append(hard, fmt.Sprintf("%s (%s): claimed by %s but no branch; make it (%s) and set Where "+
						"to 'branch %s'", row.Item, column.Label, owner, workspace.MakeSteps(project, row.Item)[0], row.Item))
				case workspace.External:
					hard = append(hard, fmt.Sprintf("%s (%s): claimed by %s but no external workspace; record it and set "+
						"Where to 'external <ref>'", row.Item, column.Label, owner))
				}
			}
		}
	}
	sort.Strings(liveWork)

	var checkout *Worktree
	for index := range wts {
		if samePath(wts[index].Path, project.Root) {
			checkout = &wts[index]
		}
	}
	switch mode {
	case workspace.Shared:
		soft = append(soft, parallelFinding(project, liveWork, "share one tree; collisions are yours to avoid")...)
		soft = append(soft, extraWorktrees(project, wts)...)
	case workspace.Branch:
		soft = append(soft, extraWorktrees(project, wts)...)
		h, s, c := branchFindings(project, checkout, liveWork, rowWhere, cancelled, closed, held, only, recovery)
		hard, soft, cleanup = append(hard, h...), append(soft, s...), append(cleanup, c...)
	}
	return hard, soft, cleanup
}

func parallelFinding(project *config.Project, liveWork []string, what string) []string {
	if len(liveWork) < 2 {
		return nil
	}
	return []string{fmt.Sprintf("%d live claims in work columns (%s) %s — parallel work needs worktree mode",
		len(liveWork), strings.Join(liveWork, ", "), what)}
}

func extraWorktrees(project *config.Project, wts []Worktree) []string {
	var out []string
	for _, wt := range wts {
		if !samePath(wt.Path, project.Root) {
			out = append(out, fmt.Sprintf("%s: %s mode, but this is a git worktree (branch %s); work in the one checkout "+
				"or use worktree mode", wt.Path, workspace.Of(project), wt.Branch))
		}
	}
	return out
}

// branchFindings are the rules that tie the one checkout to the claimed card's branch, and the card branches to the
// board.
func branchFindings(project *config.Project, checkout *Worktree, liveWork []string, where map[string]string,
	cancelled, closed map[string]bool, held map[string]map[string]any, only string, recovery bool) (hard, soft, cleanup []string) {
	exists := map[string]bool{}
	var names []string
	if listing, err := gitutil.Git(project.Root, "for-each-ref", "--format=%(refname:short)", "refs/heads"); err == nil {
		for _, name := range strings.Split(listing, "\n") {
			if name = strings.TrimSpace(name); name != "" {
				exists[name] = true
				names = append(names, name)
			}
		}
	}
	items := make([]string, 0, len(where))
	for item := range where {
		items = append(items, item)
	}
	sort.Strings(items)
	for _, item := range items {
		kind, ref := workspace.Parse(where[item])
		if kind == workspace.Branch && ref != "" && !exists[project.Branch(ref, false)] {
			hard = append(hard, fmt.Sprintf("%s: the board says branch %s, but no such branch", item, ref))
		}
	}
	head := ""
	if checkout != nil {
		head = checkout.Branch
		if only == "" || len(liveWork) > 0 {
			switch {
			case len(liveWork) > 1:
				hard = append(hard, parallelFinding(project, liveWork, "share one checkout: one HEAD cannot hold both")...)
			case len(liveWork) == 1 && head != project.Branch(liveWork[0], false):
				hard = append(hard, fmt.Sprintf("the checkout is on '%s'; %s is claimed and its change goes on branch %s "+
					"(git switch %s)", head, liveWork[0], project.Branch(liveWork[0], false), project.Branch(liveWork[0], false)))
			}
		}
		if len(liveWork) > 0 && head == project.MainBranch && checkout.Dirty > 0 {
			hard = append(hard, fmt.Sprintf("the checkout has %d uncommitted file(s) on %s while %s is claimed; the "+
				"change goes on its branch %s — %s", checkout.Dirty, project.MainBranch, liveWork[0],
				project.Branch(liveWork[0], false), strings.Join(checkout.DirtyFiles, ", ")))
		}
	}
	for _, name := range names {
		card := CardOf(project, Worktree{Branch: name}, []string{})
		if card == "" || card == "batch" || (only != "" && card != only) {
			continue
		}
		if _, onBoard := where[card]; !onBoard {
			continue
		}
		ahead := 0
		if count, err := gitutil.Git(project.Root, "rev-list", "--count", project.MainBranch+".."+name); err == nil {
			ahead, _ = strconv.Atoi(count)
		}
		dirty := 0
		if checkout != nil && name == head {
			dirty = checkout.Dirty
		}
		record, hasRecord := held[card]
		switch {
		case ahead == 0 && dirty == 0:
			if name != head && closed[card] && !cancelled[card] && !(hasRecord && isLive(record)) {
				cleanup = append(cleanup, fmt.Sprintf("branch %s (clean and merged)", name))
			}
			continue
		case !hasRecord && recovery && card == only:
			soft = append(soft, fmt.Sprintf("branch %s holds unclaimed work for %s; recovery requires a claim", name, card))
		case !hasRecord:
			hard = append(hard, fmt.Sprintf("branch %s holds unclaimed work for %s; claim it or remove the branch", name, card))
		case !isLive(record):
			soft = append(soft, fmt.Sprintf("branch %s holds %s past its claim; renew it or take it over", name, card))
		}
		if ahead > 0 {
			soft = append(soft, fmt.Sprintf("branch %s: %d commit(s) not on %s (push when %s asks)", name, ahead,
				project.MainBranch, project.Owner))
		}
	}
	return hard, soft, cleanup
}
