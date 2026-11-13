package wip

import (
	"github.com/cloudbusting/kanbanzai/internal/claim"
	"github.com/cloudbusting/kanbanzai/internal/config"
	"github.com/cloudbusting/kanbanzai/internal/workspace"
)

// recoveryEligible checks only facts needed to classify unclaimed work. Reconcile still checks every other finding.
// An open card qualifies in a Doing column (interrupted) or a column that needs a workspace (a stopped step whose
// work was preserved and whose claim was released); the recorded Where and the one matching workspace are the guard.
func recoveryEligible(project *config.Project, cards CardsByColumn, wts []Worktree, held map[string]map[string]any, card string, onHold map[string]bool) bool {
	if card == "" || onHold[card] {
		return false
	}
	if live, _, owner := ClaimState(held[card]); live && owner != claim.OwnerLabel(project) {
		return false
	}
	mode := workspace.Of(project)
	if mode != workspace.Worktree && mode != workspace.Branch {
		return false
	}
	if mode == workspace.Branch {
		for other, record := range held {
			if other != card && isLive(record) {
				return false
			}
		}
	}
	where := ""
	found := false
	for key, rows := range cards {
		for _, row := range rows {
			if row.Item == card {
				column, ok := project.ByKey[key]
				if found || !ok || column.Kind != "open" || (column.Doing == "" && !column.Worktree) {
					return false
				}
				found, where = true, row.Where
			}
		}
	}
	if !found {
		return false
	}
	if mode == workspace.Branch {
		return where == "branch "+card
	}
	if where != "worktree "+card {
		return false
	}
	count := 0
	for _, wt := range wts {
		if !samePath(wt.Path, project.Root) && wt.Card == card {
			count++
		}
	}
	return count == 1
}
