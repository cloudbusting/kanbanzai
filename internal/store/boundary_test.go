package store_test

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// The core reaches the cards only through a store. Outside internal/store and internal/config (which defines the
// path), a reference to the cards folder is allowed only for what is not a brief: the folder layout (folders that
// must exist, a file in the wrong one), setup, git paths and the path shown to a person. Each file's count is
// pinned with its reason, so a new direct read, glob or walk of the briefs fails here.
var workAllowed = map[string]struct {
	count  int
	reason string
}{
	"internal/board/board.go":     {1, "the folder a misplaced brief belongs in, in a problem message"},
	"internal/check/check.go":     {8, "the folder layout: column folders exist, no other folder, file names are a type, a misplaced brief"},
	"internal/cli/store.go":       {1, "store status shows where the working copy is"},
	"internal/cli/cli.go":         {1, "config --json and config print the path"},
	"internal/context/context.go": {2, "the session context names the path"},
	"internal/doctor/board.go":    {3, "doctor says where the board branch is checked out"},
	"internal/doctor/doctor.go":   {2, "doctor checks the folders exist"},
	"internal/flow/apply.go":      {1, "git status over the cards folder before a workflow change"},
	"internal/flow/plan.go":       {4, "a workflow change plans new and unused folders and where each brief goes"},
	"internal/flow/planfile.go":   {1, "the plan's file list names the cards folder"},
	"internal/init/detect.go":     {1, "setup keeps an existing project's cards path"},
	"internal/init/plan.go":       {2, "setup makes the folders and the ignore rules"},
	"internal/preset/describe.go": {1, "a preset's description shows the path"},
	"internal/tree/tree.go":       {1, "a changed file under the cards folder is board-only in git"},
	"internal/webboard/server.go": {1, "the browser board reports a missing cards folder"},
	"internal/wip/wip.go":         {1, "the board branch's worktree is not a card's worktree"},
}

// pathAllowed is the one line that turns a board card into a store entry, so the store reads it.
var pathAllowed = map[string]string{"internal/board/board.go": "path := project.Abs(card.Path)"}

var (
	workRef  = regexp.MustCompile(`\b(project|p|next|existing)\.Work\b`)
	pathRead = regexp.MustCompile(`\.Abs\(\w+\.Path\)|Join\(\w+\.Root, card\.Path\)`)
)

func TestTheCoreReachesTheCardsOnlyThroughAStore(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	var reads []string
	for _, top := range []string{"internal", "cmd"} {
		err := filepath.WalkDir(filepath.Join(root, top), func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			rel := filepath.ToSlash(strings.TrimPrefix(path, root+string(filepath.Separator)))
			if entry.IsDir() {
				if rel == "internal/store" || rel == "internal/config" || rel == "internal/testutil" {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(rel, ".go") || strings.HasSuffix(rel, "_test.go") {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			for number, line := range strings.Split(string(data), "\n") {
				if strings.HasPrefix(strings.TrimSpace(line), "//") {
					continue
				}
				counts[rel] += len(workRef.FindAllString(line, -1))
				if pathRead.MatchString(line) && pathAllowed[rel] != strings.TrimSpace(line) {
					reads = append(reads, rel+":"+strconv.Itoa(number+1)+": "+strings.TrimSpace(line))
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	var wrong []string
	for file, count := range counts {
		if count != workAllowed[file].count {
			wrong = append(wrong, file+": "+strconv.Itoa(count)+" references to the cards folder, "+strconv.Itoa(workAllowed[file].count)+" allowed")
		}
	}
	for file, allowed := range workAllowed {
		if _, seen := counts[file]; !seen && allowed.count > 0 {
			wrong = append(wrong, file+": allowed but gone; drop it from workAllowed")
		}
	}
	sort.Strings(wrong)
	for _, line := range wrong {
		t.Error(line)
	}
	for _, line := range reads {
		t.Errorf("a brief read by its path outside the store: %s", line)
	}
}
