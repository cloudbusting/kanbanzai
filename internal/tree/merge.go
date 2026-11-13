package tree

import (
	"fmt"
	"strings"

	"github.com/cloudbusting/kanbanzai/internal/config"
	"github.com/cloudbusting/kanbanzai/internal/gitutil"
	"github.com/cloudbusting/kanbanzai/internal/workspace"
)

// Preflight classifies the main checkout and then asks whether merging the card's branch into the integration
// branch can go ahead without touching anything another session has changed. It is advisory: git still refuses a
// merge that would overwrite a changed file, and a session that sees that refusal stops.
//
// An error means the card is not a card id; every other problem is a reason in a blocked report.
func Preflight(p *config.Project, card string) (Report, error) {
	canon, ok := p.CanonicalCardID(strings.TrimSpace(card))
	if !ok {
		return Report{}, fmt.Errorf("%q is not a card id", card)
	}
	r := Classify(p)
	merge := &Merge{Card: canon, Plan: "none", Overlap: []string{}}
	r.Merge = merge
	switch workspace.Of(p) {
	case workspace.Shared, workspace.External:
		merge.Plan = "not-applicable"
		return r, nil
	}
	merge.Branch = p.Branch(canon, false)
	if strings.HasPrefix(merge.Branch, "-") || strings.ContainsAny(merge.Branch, " \t\n") {
		return Report{}, fmt.Errorf("the branch pattern gives %q for %s, which is not a usable branch name", merge.Branch, canon)
	}
	if r.Head == "" {
		return r, nil // Classify has already blocked it and said why
	}
	ref := "refs/heads/" + merge.Branch
	if _, err := gitutil.Git(p.Root, "rev-parse", "--verify", "--quiet", ref); err != nil {
		r.block(fmt.Sprintf("the branch %s for %s does not exist", merge.Branch, canon))
		return r, nil
	}
	ahead, err := gitutil.Git(p.Root, "rev-list", "--count", "HEAD.."+ref)
	if err != nil {
		r.block(fmt.Sprintf("git could not compare %s with HEAD: %v", merge.Branch, err))
		return r, nil
	}
	if ahead == "0" {
		merge.Plan = "up-to-date"
		return r, nil
	}
	merge.Plan = "merge"
	if _, err := gitutil.Git(p.Root, "merge-base", "--is-ancestor", "HEAD", ref); err == nil {
		merge.Plan = "fast-forward"
	}
	base, err := gitutil.Git(p.Root, "merge-base", "HEAD", ref)
	if err != nil {
		r.block(fmt.Sprintf("git found no common ancestor of %s and HEAD: %v", merge.Branch, err))
		return r, nil
	}
	changed, err := gitutil.GitRaw(p.Root, "diff", "--name-only", "-z", "--no-renames", base, ref)
	if err != nil {
		r.block(fmt.Sprintf("git could not list what %s changes: %v", merge.Branch, err))
		return r, nil
	}
	merge.Overlap = overlap(r.dirty, strings.Split(strings.TrimRight(changed, "\x00"), "\x00"))
	if len(merge.Overlap) > 0 {
		r.block("the merge changes files that are changed or untracked in the main checkout, which a merge would overwrite: " + list(merge.Overlap))
	}
	if len(r.Staged) > 0 {
		r.block("staged changes in the main checkout (a merge cannot start over them, and they are not this session's to unstage): " + list(r.Staged))
	}
	if r.OK() {
		merge.Args = []string{"merge", "--no-edit", merge.Branch}
		merge.Command = "git -C " + quote(p.Root) + " merge --no-edit " + merge.Branch
	}
	return r, nil
}

func (r *Report) block(reason string) {
	r.Reasons = append(r.Reasons, reason)
	r.Verdict = Blocked
}

// overlap returns the merge's files that collide with a changed path: the same file, a file where the checkout has
// a folder's file, or a folder's file where the checkout has a file.
func overlap(dirty, merge []string) []string {
	exact, parents := map[string]bool{}, map[string]bool{}
	for _, name := range dirty {
		exact[name] = true
		for dir := parent(name); dir != ""; dir = parent(dir) {
			parents[dir] = true
		}
	}
	out := []string{}
	for _, name := range merge {
		if name == "" {
			continue
		}
		hit := exact[name] || parents[name]
		for dir := parent(name); !hit && dir != ""; dir = parent(dir) {
			hit = exact[dir]
		}
		if hit {
			out = append(out, name)
		}
	}
	return uniq(out)
}

func parent(name string) string {
	if i := strings.LastIndex(name, "/"); i > 0 {
		return name[:i]
	}
	return ""
}

// quote is a path as one shell word, only when it needs it.
func quote(value string) string {
	safe := value != ""
	for _, c := range value {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || strings.ContainsRune("_./:@%+=,-", c)) {
			safe = false
		}
	}
	if safe {
		return value
	}
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}
