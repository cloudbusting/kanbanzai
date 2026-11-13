// Package tree answers one question for a session about to merge, package or release: what is in the main
// checkout, and does any of it stand in the way?
//
// Several sessions keep the board's briefs changing in the main checkout while one of them integrates. A brief that
// someone else is editing is not a reason to stop, but a source file is, and so is anything this package cannot
// verify. The answer is a deterministic classification of every changed path, so the skills and tools do not each
// read `git status` their own way. It only reads: it never stages, stashes, commits, resets or edits a file.
package tree

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/cloudbusting/kanbanzai/internal/config"
	"github.com/cloudbusting/kanbanzai/internal/gitutil"
)

// The verdicts. A session goes ahead on Clean and BoardOnly and stops on Blocked.
const (
	Clean     = "clean"
	BoardOnly = "board-only"
	Blocked   = "blocked"
)

// Report is the answer, and the JSON `kanbanzai tree --json` prints. Every list is present and never null.
type Report struct {
	Verdict    string   `json:"verdict"`
	Branch     string   `json:"branch"`
	Head       string   `json:"head"`
	Board      []string `json:"board"`       // changed card briefs and batch records, verified
	Other      []string `json:"other"`       // changed paths that are not board files
	Uncertain  []string `json:"uncertain"`   // changed paths that could not be verified either way
	Staged     []string `json:"staged"`      // paths with a change in the index (any class)
	InProgress string   `json:"in_progress"` // merge, rebase, cherry-pick, revert or bisect under way, or ""
	Reasons    []string `json:"reasons"`     // why the verdict is blocked, one line each
	Merge      *Merge   `json:"merge,omitempty"`

	dirty []string // every changed path, including a rename's source: what a merge must not overwrite
}

// Merge is the preflight of one card's merge into the integration branch.
type Merge struct {
	Card    string   `json:"card"`
	Branch  string   `json:"branch"`
	Plan    string   `json:"plan"`    // fast-forward, merge, up-to-date, not-applicable or none
	Overlap []string `json:"overlap"` // files the merge brings that are changed or untracked in the checkout
	Command string   `json:"command"` // the merge to run; empty unless the verdict allows it
	Args    []string `json:"-"`       // the same, as git arguments after `-C <root>`
}

// OK reports whether a session may go ahead.
func (r Report) OK() bool { return r.Verdict != Blocked }

func newReport() Report {
	return Report{Board: []string{}, Other: []string{}, Uncertain: []string{}, Staged: []string{}, Reasons: []string{}}
}

var (
	batchFile    = regexp.MustCompile(`^B-\d+\.md$`)
	batchHeading = regexp.MustCompile(`^# B-\d+\b`)
)

// Classify reads the main checkout and sorts its changes.
func Classify(p *config.Project) Report {
	r := newReport()
	top, err := gitutil.Git(p.Root, "rev-parse", "--show-toplevel")
	if err != nil || top == "" {
		r.Uncertain = append(r.Uncertain, "(git could not read the main checkout)")
		r.Reasons = append(r.Reasons, fmt.Sprintf("git could not read the main checkout %s: %v", p.Root, err))
		r.Verdict = Blocked
		return r
	}
	if branch, err := gitutil.Git(p.Root, "rev-parse", "--abbrev-ref", "HEAD"); err == nil {
		r.Branch = branch
	}
	if head, err := gitutil.Git(p.Root, "rev-parse", "HEAD"); err == nil {
		r.Head = head
	} else {
		r.Reasons = append(r.Reasons, "the main checkout has no readable HEAD commit")
	}
	r.InProgress = inProgress(p.Root)

	status, err := gitutil.GitRaw(p.Root, "status", "--porcelain=v1", "-z", "--untracked-files=all")
	if err != nil {
		r.Uncertain = append(r.Uncertain, "(git status failed)")
		r.Reasons = append(r.Reasons, fmt.Sprintf("git status failed: %v", err))
		r.Verdict = Blocked
		return r
	}
	for _, change := range parseStatus(status) {
		r.dirty = append(r.dirty, change.paths...)
		if change.staged {
			r.Staged = append(r.Staged, change.paths...)
		}
		for _, name := range change.paths {
			switch {
			case change.unmerged:
				r.Uncertain = append(r.Uncertain, name)
			default:
				switch kind := boardFile(p, top, name); kind {
				case boardOK:
					r.Board = append(r.Board, name)
				case boardNot:
					r.Other = append(r.Other, name)
				default:
					r.Uncertain = append(r.Uncertain, name)
				}
			}
		}
	}
	r.dirty, r.Board, r.Other, r.Uncertain, r.Staged = uniq(r.dirty), uniq(r.Board), uniq(r.Other), uniq(r.Uncertain), uniq(r.Staged)

	switch {
	case len(r.Board)+len(r.Other)+len(r.Uncertain) == 0:
		r.Verdict = Clean
	case len(r.Other)+len(r.Uncertain) == 0:
		r.Verdict = BoardOnly
	default:
		r.Verdict = Blocked
	}
	if len(r.Other) > 0 {
		r.Reasons = append(r.Reasons, "changes outside the board, which belong to another session: "+list(r.Other))
	}
	if len(r.Uncertain) > 0 {
		r.Reasons = append(r.Reasons, "changes that cannot be verified as board files: "+list(r.Uncertain))
	}
	if r.InProgress != "" {
		r.Reasons = append(r.Reasons, "a "+r.InProgress+" is under way in the main checkout")
		r.Verdict = Blocked
	}
	if p.MainBranch != "" && r.Branch != p.MainBranch {
		r.Reasons = append(r.Reasons, fmt.Sprintf("the main checkout is on %q, not on the integration branch %q", r.Branch, p.MainBranch))
		r.Verdict = Blocked
	}
	if r.Head == "" {
		r.Verdict = Blocked
	}
	return r
}

type change struct {
	paths    []string
	staged   bool
	unmerged bool
}

// parseStatus reads `git status --porcelain=v1 -z`: "XY path\0", and for a rename or copy "XY new\0old\0".
func parseStatus(raw string) []change {
	var out []change
	tokens := strings.Split(raw, "\x00")
	for i := 0; i < len(tokens); i++ {
		token := tokens[i]
		if len(token) < 4 {
			continue
		}
		x, y := token[0], token[1]
		entry := change{paths: []string{token[3:]}}
		if x == 'R' || x == 'C' || y == 'R' || y == 'C' {
			if i+1 < len(tokens) && tokens[i+1] != "" {
				i++
				entry.paths = append(entry.paths, tokens[i])
			}
		}
		entry.staged = x != ' ' && x != '?' && x != '!'
		entry.unmerged = x == 'U' || y == 'U' || (x == 'A' && y == 'A') || (x == 'D' && y == 'D')
		out = append(out, entry)
	}
	return out
}

const (
	boardOK = iota
	boardNot
	boardUnsure
)

// boardFile says whether a changed path is a card brief or batch record: named like one, in a configured folder, a
// regular file whose first heading is one. A path that is only shaped like one but cannot be shown to be one is
// unsure; a path that is not shaped like one is not a board file.
func boardFile(p *config.Project, top, name string) int {
	dir, file := path.Dir(name), path.Base(name)
	card := false
	switch {
	case under(top, p.Batches, dir) && batchFile.MatchString(file):
	case cardFolder(p, top, dir) && p.CardFileRe.MatchString(file):
		card = true
	default:
		return boardNot
	}
	full := filepath.Join(top, filepath.FromSlash(name))
	info, err := os.Lstat(full)
	var heading string
	switch {
	case err == nil && !info.Mode().IsRegular():
		return boardUnsure
	case err == nil:
		heading, err = firstHeading(full)
		if err != nil {
			return boardUnsure
		}
	default:
		// Deleted, or moved away: it must have been a brief in the committed tree.
		blob, gitErr := gitutil.GitRaw(top, "show", "HEAD:"+name)
		if gitErr != nil {
			return boardUnsure
		}
		heading = headingOf(strings.NewReader(blob))
	}
	if card {
		match := p.TitleRe.FindStringSubmatch(heading)
		if match == nil || !strings.HasPrefix(file, match[1]+"-") {
			return boardUnsure
		}
		return boardOK
	}
	if !batchHeading.MatchString(heading) {
		return boardUnsure
	}
	return boardOK
}

// under reports whether the repository-relative folder is the configured folder.
func under(top, folder, dir string) bool {
	rel, err := filepath.Rel(top, folder)
	return err == nil && !strings.HasPrefix(rel, "..") && filepath.ToSlash(rel) == dir
}

// cardFolder reports whether dir is <work>/<column folder>.
func cardFolder(p *config.Project, top, dir string) bool {
	for _, folder := range p.Folders {
		if under(top, filepath.Join(p.Work, filepath.FromSlash(folder)), dir) {
			return true
		}
	}
	return false
}

func firstHeading(file string) (string, error) {
	f, err := os.Open(file)
	if err != nil {
		return "", err
	}
	defer f.Close()
	return headingOf(f), nil
}

// headingOf is the first non-blank line, read from at most 64 KiB.
func headingOf(r io.Reader) string {
	scanner := bufio.NewScanner(io.LimitReader(r, 64<<10))
	for scanner.Scan() {
		line := strings.TrimPrefix(strings.TrimRight(scanner.Text(), "\r"), "\ufeff")
		if strings.TrimSpace(line) != "" {
			return line
		}
	}
	return ""
}

// inProgress names the operation git has half done in the checkout, or "".
func inProgress(root string) string {
	for _, probe := range []struct{ file, name string }{
		{"MERGE_HEAD", "merge"}, {"rebase-merge", "rebase"}, {"rebase-apply", "rebase"},
		{"CHERRY_PICK_HEAD", "cherry-pick"}, {"REVERT_HEAD", "revert"}, {"BISECT_LOG", "bisect"},
	} {
		located, err := gitutil.Git(root, "rev-parse", "--path-format=absolute", "--git-path", probe.file)
		if err != nil {
			continue
		}
		if _, err := os.Lstat(located); err == nil {
			return probe.name
		}
	}
	return ""
}

func uniq(in []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, item := range in {
		if !seen[item] {
			seen[item] = true
			out = append(out, item)
		}
	}
	sort.Strings(out)
	return out
}

// list names up to five paths and counts the rest.
func list(paths []string) string {
	if len(paths) <= 5 {
		return strings.Join(paths, ", ")
	}
	return fmt.Sprintf("%s and %d more", strings.Join(paths[:5], ", "), len(paths)-5)
}

// Text is the report for a person or a session reading the terminal.
func Text(r Report) string {
	var b strings.Builder
	fmt.Fprintf(&b, "tree: %s (branch %s, head %s)\n", r.Verdict, r.Branch, short(r.Head))
	for _, group := range []struct {
		name  string
		paths []string
	}{{"board", r.Board}, {"other", r.Other}, {"uncertain", r.Uncertain}, {"staged", r.Staged}} {
		if len(group.paths) > 0 {
			fmt.Fprintf(&b, "  %-9s %s\n", group.name, strings.Join(group.paths, ", "))
		}
	}
	if r.InProgress != "" {
		fmt.Fprintf(&b, "  under way %s\n", r.InProgress)
	}
	if m := r.Merge; m != nil {
		fmt.Fprintf(&b, "merge %s: %s", m.Card, m.Plan)
		if m.Branch != "" {
			fmt.Fprintf(&b, " (branch %s)", m.Branch)
		}
		b.WriteString("\n")
		if len(m.Overlap) > 0 {
			fmt.Fprintf(&b, "  overlap   %s\n", strings.Join(m.Overlap, ", "))
		}
		if m.Command != "" {
			fmt.Fprintf(&b, "  run       %s\n", m.Command)
		}
	}
	for _, reason := range r.Reasons {
		fmt.Fprintf(&b, "blocked: %s\n", reason)
	}
	return b.String()
}

func short(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}
