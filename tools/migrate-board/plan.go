package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/cloudbusting/kanbanzai/internal/brief"
	"github.com/cloudbusting/kanbanzai/internal/claim"
	"github.com/cloudbusting/kanbanzai/internal/config"
	"github.com/cloudbusting/kanbanzai/internal/timeutil"
)

// bodyLimit is GitHub's longest issue body, in characters.
const bodyLimit = 65536

// card is one brief of the old board, as it was read.
type card struct {
	OldID  string
	Number int
	Type   string // the type key the new id carries
	Legacy bool   // the old id had no type (W-NN): it takes the legacy type
	Title  string
	Stage  string // the column key
	Folder string // the column's board folder
	Path   string
	Text   string
}

// plan is the old board: every card in the order they are created, and what was found beside them.
type plan struct {
	project *config.Project
	cards   []*card
	strays  []string // .md files in a board folder that are not a card's brief
	refRe   *regexp.Regexp
}

// readPlan reads every brief under from (the old board's work folder, which holds the project's board folders) and
// touches nothing there. A legacy id (W-NN) takes legacyType.
func readPlan(project *config.Project, from, legacyType string) (*plan, error) {
	if project.TypeSet == nil || !project.Typed {
		return nil, fmt.Errorf("this project has no card types: the new ids are type prefixes, so it must be a typed project")
	}
	if _, ok := project.TypeSet.ByKey(legacyType); !ok {
		return nil, fmt.Errorf("--legacy-type %q is not one of the project's types (%s)", legacyType, strings.Join(project.TypeSet.Keys(), ", "))
	}
	p := &plan{project: project}
	found := false
	for _, folder := range project.Folders {
		dir := filepath.Join(from, folder)
		entries, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		found = true
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
				continue
			}
			path := filepath.Join(dir, entry.Name())
			match := project.CardFileRe.FindStringSubmatch(entry.Name())
			if match == nil {
				p.strays = append(p.strays, path)
				continue
			}
			c, err := readCard(project, path, match[1], legacyType)
			if err != nil {
				return nil, err
			}
			p.cards = append(p.cards, c)
		}
	}
	if !found {
		return nil, fmt.Errorf("%s holds none of the board folders (%s): --from is the old board's work folder", from,
			strings.Join(project.Folders, ", "))
	}
	sort.Slice(p.cards, func(a, b int) bool {
		if p.cards[a].Number != p.cards[b].Number {
			return p.cards[a].Number < p.cards[b].Number
		}
		return p.cards[a].OldID < p.cards[b].OldID
	})
	seen := map[string]bool{}
	for _, c := range p.cards {
		if seen[c.OldID] {
			return nil, fmt.Errorf("%s is on the board twice (%s)", c.OldID, c.Path)
		}
		seen[c.OldID] = true
	}
	p.refRe = referencePattern(project)
	return p, nil
}

func readCard(project *config.Project, path, id, legacyType string) (*card, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	text := string(data)
	first, _, _ := strings.Cut(text, "\n")
	heading := project.TitleRe.FindStringSubmatch(first)
	if heading == nil || heading[1] != id {
		return nil, fmt.Errorf("%s: the heading %q does not name %s", path, first, id)
	}
	_, fields := brief.Header(text)
	column, ok := project.StageColumn[brief.StageWord(fields["Stage"])]
	if !ok {
		return nil, fmt.Errorf("%s: the Stage %q is not a column of this project", path, fields["Stage"])
	}
	number, _ := project.CardNumber(id)
	c := &card{OldID: id, Number: number, Title: strings.TrimSpace(heading[2]), Stage: column,
		Folder: project.ByKey[column].Folder, Path: path, Text: text}
	if item, ok := project.TypeOf(id); ok {
		c.Type = item.Key
	} else {
		c.Type, c.Legacy = legacyType, true
	}
	return c, nil
}

// referencePattern finds a card id in prose: a configured type prefix or the legacy prefix, upper-case, a hyphen and
// the number, standing alone (so a word that merely ends in one is not an id).
func referencePattern(project *config.Project) *regexp.Regexp {
	prefixes := append([]string{}, project.TypeSet.Prefixes()...)
	if project.Prefix != "" {
		prefixes = append(prefixes, project.Prefix)
	}
	for n := range prefixes {
		prefixes[n] = regexp.QuoteMeta(prefixes[n])
	}
	return regexp.MustCompile(`\b(?:` + strings.Join(prefixes, "|") + `)-\d+\b`)
}

// remap is text with every reference to a migrated card rewritten to its new id, in one pass over the map (never one
// card after another, so an id that is somebody else's old id is not rewritten twice). Fenced code blocks are left as
// they are, and so is an id that has no card; those are returned, with the number of references rewritten.
func (p *plan) remap(text string, ids map[string]string) (out string, rewritten int, dangling []string) {
	seen := map[string]bool{}
	lines := strings.Split(text, "\n")
	fenced := false
	for n, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			fenced = !fenced
			continue
		}
		if fenced {
			continue
		}
		lines[n] = p.refRe.ReplaceAllStringFunc(line, func(found string) string {
			canonical, ok := p.project.CanonicalCardID(found)
			if !ok {
				return found
			}
			if next, ok := ids[canonical]; ok {
				rewritten++
				return next
			}
			if !seen[canonical] {
				seen[canonical] = true
				dangling = append(dangling, found)
			}
			return found
		})
	}
	return strings.Join(lines, "\n"), rewritten, dangling
}

// expected is the brief the new card must hold: the old one remapped, with a Was line saying its old id.
func (p *plan) expected(c *card, ids map[string]string) string {
	text, _, _ := p.remap(c.Text, ids)
	was := c.OldID
	return brief.SetFields(text, map[string]*string{"Was": &was})
}

// liveClaims is the cards of the old board that a session holds now: their claims stay per clone and would not
// follow a card to its new id.
func liveClaims(project *config.Project, from string) []string {
	old := *project
	old.Claims = filepath.Join(from, ".claims")
	now := timeutil.Now()
	var held []string
	for id, record := range claim.ReadRecords(&old) {
		if claim.Live(record, now) {
			held = append(held, id)
		}
	}
	sort.Strings(held)
	return held
}

// report says what a run would do, without a word to the store.
func (p *plan) report(w func(format string, args ...any), claims []string) (problems int) {
	byStage, byType := map[string]int{}, map[string]int{}
	var legacy []string
	known := map[string]string{}
	for _, c := range p.cards {
		known[c.OldID] = c.OldID
	}
	refs, danglingAt := 0, map[string][]string{}
	for _, c := range p.cards {
		byStage[c.Stage]++
		byType[c.Type]++
		if c.Legacy {
			legacy = append(legacy, c.OldID)
		}
		_, n, dangling := p.remap(c.Text, known)
		refs += n
		for _, d := range dangling {
			danglingAt[d] = append(danglingAt[d], c.OldID)
		}
	}
	w("%d card(s) to migrate, created in ascending old-number order", len(p.cards))
	w("by stage: %s", counts(byStage))
	w("by type:  %s", counts(byType))
	if len(legacy) > 0 {
		w("legacy ids (no type), given the legacy type: %s", strings.Join(legacy, " "))
	}
	w("%d reference(s) to migrated cards will be rewritten, in every line outside fenced code blocks", refs)
	if len(danglingAt) > 0 {
		names := make([]string, 0, len(danglingAt))
		for name := range danglingAt {
			names = append(names, name)
		}
		sort.Strings(names)
		w("%d id(s) named in the briefs have no card and are left as written:", len(names))
		for _, name := range names {
			w("  %s (in %s)", name, strings.Join(danglingAt[name], ", "))
		}
	}
	for _, c := range p.cards {
		if size := len([]rune(p.expected(c, known))); size > bodyLimit {
			w("PROBLEM %s is %d characters; GitHub keeps at most %d in an issue body", c.OldID, size, bodyLimit)
			problems++
		}
	}
	for _, path := range p.strays {
		w("note: %s is in a board folder but is not a card's brief; it is not migrated", path)
	}
	if len(claims) > 0 {
		w("PROBLEM the old board has live claims on %s: finish or release them first", strings.Join(claims, ", "))
		problems++
	}
	return problems
}

func counts(values map[string]int) string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s %d", key, values[key]))
	}
	return strings.Join(parts, ", ")
}
