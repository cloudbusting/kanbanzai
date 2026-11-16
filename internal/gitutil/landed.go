package gitutil

import (
	"fmt"
	"strconv"
	"strings"
)

// Unlanded counts the commits of ref that exist nowhere else: not on the main branch, not on any remote-tracking
// branch (so not pushed), and not already in the main branch's content. It is what a project whose pull requests
// are merged on the host (`[flow] integration = "remote"`) means by "not merged".
//
// A host's squash merge puts a branch's change into main as one new commit, so the branch's own commits are never
// ancestors of main and a plain count would call a landed branch unmerged for ever. When commits remain, the last
// test is whether merging ref into main would change main at all: if the merged tree is main's own tree, every
// change of the branch is already there. That needs `git merge-tree --write-tree` (git 2.38 or later); without it,
// or when the merge conflicts, the commits stay counted, so a branch is kept for the owner rather than dropped. The
// check writes tree objects into the object store, which git collects later, and changes no ref or file.
func Unlanded(dir, main, ref string) (int, error) {
	out, err := Git(dir, "rev-list", "--count", ref, "--not", main, "--remotes")
	if err != nil {
		return 0, err
	}
	count, err := strconv.Atoi(out)
	if err != nil {
		return 0, fmt.Errorf("git rev-list --count gave %q", out)
	}
	if count == 0 {
		return 0, nil
	}
	merged, err := Git(dir, "merge-tree", "--write-tree", main, ref)
	if err != nil {
		return count, nil
	}
	mainTree, err := Git(dir, "rev-parse", main+"^{tree}")
	if err != nil {
		return count, nil
	}
	if first := strings.SplitN(merged, "\n", 2)[0]; first == mainTree {
		return 0, nil
	}
	return count, nil
}
