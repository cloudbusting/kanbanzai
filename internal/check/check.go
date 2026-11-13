// Package check validates the board: every brief is well formed and sits in the folder for its stage.
//
// Each check is a function over a project that returns its problems, so a test can prove every check can fail.
// The checks are the ones every project shares: the column folders exist and nothing else is a board folder, each
// brief's first line names its number and its file name, its head has the fields, its stage is known and its folder
// matches, no two share a number, a Held line says blocked or waiting, every relative link resolves, and each brief
// and the template end in ## Comments.
package check

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/cloudbusting/kanbanzai/internal/board"
	"github.com/cloudbusting/kanbanzai/internal/brief"
	"github.com/cloudbusting/kanbanzai/internal/config"
	"github.com/cloudbusting/kanbanzai/internal/gate"
	"github.com/cloudbusting/kanbanzai/internal/question"
	"github.com/cloudbusting/kanbanzai/internal/workspace"
)

// HeadFields are the fields every brief and the template must carry.
var HeadFields = []string{"Stage", "Updated", "Next action", "Where"}

var (
	updatedRE = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	datedRE   = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}\b`)
	linkRE    = regexp.MustCompile(`\[[^\]]+\]\(([^)\s]+)\)`)
)

// Checks are the board's checks, in the order the command reports them.
var Checks = []func(*config.Project) []string{
	Folders, TypedFiles, Titles, Headers, Stages, Unique, Held, Gates, Links, Questions, Comments, Template, BriefSchemas,
}

// Questions validates optional question records and the hold on an open question.
func Questions(project *config.Project) []string {
	var problems []string
	for _, path := range board.Briefs(project) {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		text := string(data)
		entries, err := question.Parse(text)
		if err != nil {
			problems = append(problems, fmt.Sprintf("%s: Questions: %v", project.Rel(path), err))
			continue
		}
		_, fields := brief.Header(text)
		for _, entry := range entries {
			if (entry.Answer == "" || entry.Answer == "—") && brief.StageWord(fields["Held"]) != "waiting" {
				problems = append(problems, fmt.Sprintf("%s: an open question needs a Held: waiting line", project.Rel(path)))
				break
			}
		}
	}
	return problems
}

// TypedFiles checks that every brief in a board folder names a configured type. A legacy project ignores files
// that are not its prefix, as it always has; a typed project cannot guess what an unknown prefix means.
func TypedFiles(project *config.Project) []string {
	if project.TypeSet == nil {
		return nil
	}
	problems := []string{}
	for _, folder := range project.Folders {
		paths, _ := filepath.Glob(filepath.Join(project.Work, folder, "*.md"))
		sort.Strings(paths)
		for _, path := range paths {
			if project.CardFileRe.MatchString(filepath.Base(path)) {
				continue
			}
			problems = append(problems, fmt.Sprintf("%s: the file name is not a configured type (valid prefixes: %s)",
				project.Rel(path), strings.Join(project.CardPrefixes(), ", ")))
		}
	}
	return problems
}

// Folders checks that every column folder exists and the work folder holds no other folder.
func Folders(project *config.Project) []string {
	problems := []string{}
	rel := project.Rel(project.Work)
	for _, folder := range project.Folders {
		info, err := os.Stat(filepath.Join(project.Work, folder))
		if err != nil || !info.IsDir() {
			problems = append(problems, fmt.Sprintf("%s/%s/ is missing", rel, folder))
		}
	}
	entries, _ := os.ReadDir(project.Work)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		path := filepath.Join(project.Work, entry.Name())
		if contains(project.Folders, entry.Name()) || filepath.Clean(path) == filepath.Clean(project.Batches) {
			continue
		}
		problems = append(problems, fmt.Sprintf("%s/%s/ is not a board folder (%s)", rel, entry.Name(),
			strings.Join(project.Folders, ", ")))
	}
	if info, err := os.Stat(project.Work); err == nil && info.IsDir() {
		_ = filepath.WalkDir(project.Work, func(path string, entry fs.DirEntry, err error) error {
			if err != nil || entry.IsDir() {
				return nil
			}
			name := entry.Name()
			if !project.CardFileRe.MatchString(name) {
				return nil
			}
			if !contains(project.Folders, filepath.Base(filepath.Dir(path))) {
				problems = append(problems, fmt.Sprintf("%s: a brief must sit in %s", project.Rel(path),
					strings.Join(project.Folders, ", ")))
			}
			return nil
		})
	}
	return problems
}

// Titles checks that the first line is "# ID: <the work>" and names the number in the file name.
func Titles(project *config.Project) []string {
	problems := []string{}
	for _, path := range board.Briefs(project) {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		first := strings.SplitN(string(data), "\n", 2)[0]
		found := project.TitleRe.FindStringSubmatch(first)
		if found == nil {
			problems = append(problems, fmt.Sprintf("%s: the first line must be '# %s: <the work>'",
				project.Rel(path), strings.Join(project.IDExamples(), " or ")))
			continue
		}
		if !strings.HasPrefix(filepath.Base(path), found[1]+"-") {
			problems = append(problems, fmt.Sprintf("%s: the title names %s, the file name does not",
				project.Rel(path), found[1]))
		}
	}
	return problems
}

// Headers checks Stage, Updated, Next action and Where on every brief; a Done date on a done card.
func Headers(project *config.Project) []string {
	problems := []string{}
	doneWords := map[string]bool{}
	for _, column := range project.Columns {
		if column.Kind == "done" {
			for _, word := range column.Words {
				doneWords[word] = true
			}
		}
	}
	for _, path := range board.Briefs(project) {
		data, _ := os.ReadFile(path)
		_, fields := brief.Header(string(data))
		name := project.Rel(path)
		for _, field := range HeadFields {
			if strings.TrimSpace(fields[field]) == "" {
				problems = append(problems, fmt.Sprintf("%s: no '- **%s:**' line in the head", name, field))
			}
		}
		if value := fields["Updated"]; value != "" && !updatedRE.MatchString(value) {
			problems = append(problems, fmt.Sprintf("%s: Updated must be YYYY-MM-DD, not '%s'", name, value))
		}
		kind := ""
		if column, ok := project.ByKey[project.StageColumn[brief.StageWord(fields["Stage"])]]; ok {
			kind = column.Kind
		}
		if value := fields["Where"]; value != "" && !workspace.WhereValid(project, value, kind) {
			short := value
			if len(short) > 30 {
				short = short[:30]
			}
			problems = append(problems, fmt.Sprintf("%s: Where must be %s, not '%s'", name, whereWords(project), short))
		}
		if doneWords[brief.StageWord(fields["Stage"])] && !datedRE.MatchString(fields["Done"]) {
			problems = append(problems, fmt.Sprintf("%s: a done card's Done line starts with its date", name))
		}
	}
	return problems
}

// Stages checks that the Stage word is known and the brief sits in the folder for it.
func Stages(project *config.Project) []string {
	problems := []string{}
	for _, path := range board.Briefs(project) {
		data, _ := os.ReadFile(path)
		_, fields := brief.Header(string(data))
		word := brief.StageWord(fields["Stage"])
		key := project.StageColumn[word]
		name := project.Rel(path)
		if key == "" && (word == "blocked" || word == "waiting") {
			problems = append(problems, fmt.Sprintf("%s: a %s card stays in the column it finished; its Stage "+
				"is that column and a Held line says '%s: …'", name, word, word))
			continue
		}
		if key == "" {
			short := fields["Stage"]
			if len(short) > 40 {
				short = short[:40]
			}
			problems = append(problems, fmt.Sprintf("%s: unknown stage '%s'", name, short))
			continue
		}
		if filepath.Base(filepath.Dir(path)) != project.ByKey[key].Folder {
			problems = append(problems, fmt.Sprintf("%s: a %s card belongs in %s/%s/", name, word,
				project.Rel(project.Work), project.ByKey[key].Folder))
		}
	}
	return problems
}

// Unique checks that no two briefs share a number, and that a typed board's numbers are global across types.
func Unique(project *config.Project) []string {
	problems := []string{}
	seen := map[string]string{}
	numbers := map[int]string{}
	for _, path := range board.Briefs(project) {
		named := project.CardFileRe.FindStringSubmatch(filepath.Base(path))
		card := filepath.Base(path)
		if named != nil {
			card = named[1]
		}
		if other, ok := seen[card]; ok {
			problems = append(problems, fmt.Sprintf("%s: %s is also %s", project.Rel(path), card, other))
			continue
		}
		seen[card] = project.Rel(path)
		if project.TypeSet == nil {
			continue
		}
		number, ok := project.CardNumber(card)
		if !ok {
			continue
		}
		if other, exists := numbers[number]; exists {
			problems = append(problems, fmt.Sprintf("%s: %s and %s share the global number %d", project.Rel(path),
				card, other, number))
			continue
		}
		numbers[number] = card
	}
	return problems
}

// Held checks that a held card says blocked or waiting, and what it is held on.
func Held(project *config.Project) []string {
	problems := []string{}
	for _, path := range board.Briefs(project) {
		data, _ := os.ReadFile(path)
		_, fields := brief.Header(string(data))
		held := brief.HeldOf(fields["Held"])
		if held == nil {
			continue
		}
		name := project.Rel(path)
		column := project.ByKey[project.StageColumn[brief.StageWord(fields["Stage"])]]
		word := brief.StageWord(*held)
		if word != "blocked" && word != "waiting" {
			short := *held
			if len(short) > 30 {
				short = short[:30]
			}
			problems = append(problems, fmt.Sprintf("%s: a Held line starts 'blocked' or 'waiting', not '%s'",
				name, short))
		} else if heldRest(*held, word) == "" {
			problems = append(problems, fmt.Sprintf("%s: a Held line says what it is held on ('%s: <what>')",
				name, word))
		}
		if column.Key != "" && column.Kind != "open" {
			problems = append(problems, fmt.Sprintf("%s: a %s card is not held; remove its Held line", name,
				brief.StageWord(fields["Stage"])))
		}
	}
	return problems
}

var gateLineRE = regexp.MustCompile(`^(passed|failed) by \S+, \d{4}-\d{2}-\d{2}( — .+)?$`)

// Gates checks the Gate and Needs lines: a Gate line sits only on a card in a column with a recorded gate and
// says who passed or failed it and when; a Needs line names cards that exist, never the card itself, and the cards
// never wait on each other in a circle.
func Gates(project *config.Project) []string {
	problems := []string{}
	needs := map[string][]string{}
	known := map[string]bool{}
	paths := map[string]string{}
	for _, path := range board.Briefs(project) {
		data, _ := os.ReadFile(path)
		title, fields := brief.Header(string(data))
		if found := project.TitleRe.FindStringSubmatch(title); found != nil {
			known[found[1]] = true
			paths[found[1]] = project.Rel(path)
			needs[found[1]] = gate.Needs(fields["Needs"])
		}
	}
	for _, path := range board.Briefs(project) {
		data, _ := os.ReadFile(path)
		title, fields := brief.Header(string(data))
		name := project.Rel(path)
		id := ""
		if found := project.TitleRe.FindStringSubmatch(title); found != nil {
			id = found[1]
		}
		if record := fields["Gate"]; record != "" {
			column := project.ByKey[project.StageColumn[brief.StageWord(fields["Stage"])]]
			switch {
			case !column.Gate.Recorded():
				problems = append(problems, fmt.Sprintf("%s: a Gate line belongs on a card in a column with an owner, "+
					"ci or external gate, not '%s'", name, fields["Stage"]))
			case !gateLineRE.MatchString(record):
				problems = append(problems, fmt.Sprintf("%s: a Gate line reads 'passed|failed by <who>, YYYY-MM-DD "+
					"[— <note>]', not '%s'", name, shorten(record, 40)))
			}
		}
		for _, other := range needs[id] {
			switch {
			case other == id:
				problems = append(problems, fmt.Sprintf("%s: a card cannot need itself", name))
			case !known[other]:
				problems = append(problems, fmt.Sprintf("%s: Needs names %s, which is not on the board", name, other))
			}
		}
	}
	for _, cycle := range needCycles(needs) {
		problems = append(problems, fmt.Sprintf("%s: the cards wait on each other: %s", paths[cycle[0]],
			strings.Join(cycle, " -> ")))
	}
	return problems
}

// needCycles is one cycle for each card that waits on itself through other cards, e.g. [W-1 W-2 W-1]; a card
// that needs only itself is reported by Gates on its own.
func needCycles(needs map[string][]string) [][]string {
	ids := make([]string, 0, len(needs))
	for id := range needs {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	var cycles [][]string
	reported := map[string]bool{}
	state := map[string]int{} // 1 on the path, 2 finished
	var path []string
	var visit func(string)
	visit = func(id string) {
		state[id] = 1
		path = append(path, id)
		for _, other := range needs[id] {
			if other == id {
				continue
			}
			switch state[other] {
			case 0:
				if _, exists := needs[other]; exists {
					visit(other)
				}
			case 1:
				start := 0
				for i, item := range path {
					if item == other {
						start = i
					}
				}
				cycle := append(append([]string{}, path[start:]...), other)
				key := strings.Join(sortedCopy(cycle[:len(cycle)-1]), ",")
				if !reported[key] {
					reported[key] = true
					cycles = append(cycles, cycle)
				}
			}
		}
		path = path[:len(path)-1]
		state[id] = 2
	}
	for _, id := range ids {
		if state[id] == 0 {
			visit(id)
		}
	}
	return cycles
}

func sortedCopy(values []string) []string {
	out := append([]string{}, values...)
	sort.Strings(out)
	return out
}

func shorten(value string, limit int) string {
	if len(value) > limit {
		return value[:limit]
	}
	return value
}

// Links checks that every relative link in a brief resolves.
func Links(project *config.Project) []string {
	problems := []string{}
	for _, path := range board.Briefs(project) {
		data, _ := os.ReadFile(path)
		for _, match := range linkRE.FindAllStringSubmatch(string(data), -1) {
			link := match[1]
			if strings.HasPrefix(link, "http://") || strings.HasPrefix(link, "https://") ||
				strings.HasPrefix(link, "#") || strings.HasPrefix(link, "mailto:") {
				continue
			}
			target := filepath.Join(filepath.Dir(path), strings.Split(link, "#")[0])
			if _, err := os.Stat(target); err != nil {
				problems = append(problems, fmt.Sprintf("%s: broken link %s", project.Rel(path), link))
			}
		}
	}
	return problems
}

// Comments checks that every brief ends in ## Comments.
func Comments(project *config.Project) []string {
	problems := []string{}
	for _, path := range board.Briefs(project) {
		data, _ := os.ReadFile(path)
		sections := headings(string(data))
		if len(sections) == 0 || sections[len(sections)-1] != "## Comments" {
			problems = append(problems, fmt.Sprintf("%s: the last section must be '## Comments'", project.Rel(path)))
		}
	}
	return problems
}

// Template checks that the card template exists, has the head's fields and ends in ## Comments.
func Template(project *config.Project) []string {
	problems := []string{}
	for _, item := range project.Types {
		if item.Template == "" {
			continue
		}
		data, err := os.ReadFile(item.Template)
		if err != nil {
			problems = append(problems, fmt.Sprintf("%s: template for type %s cannot be read: %v", project.Rel(item.Template), item.Key, err))
			continue
		}
		if schema, ok := project.BriefSchemas[item.Schema]; ok {
			problems = append(problems, checkBriefSchema(project.Rel(item.Template), string(data), schema, true)...)
		}
	}
	shipped := project.TemplateIsShipped
	name := "templates/card.md"
	if !shipped {
		name = project.Rel(project.Template)
		if _, err := os.Stat(project.Template); err != nil {
			return append(problems, fmt.Sprintf("%s: the card template is missing", name))
		}
	}
	text, err := project.TemplateText()
	if err != nil {
		if shipped {
			return problems
		}
		return append(problems, fmt.Sprintf("%s: the card template is missing", name))
	}
	for _, item := range project.Types {
		if item.Template != "" || item.Schema == "" {
			continue
		}
		if schema, ok := project.BriefSchemas[item.Schema]; ok {
			problems = append(problems, checkBriefSchema(name+" (type "+item.Key+")", text, schema, true)...)
		}
	}
	_, fields := brief.Header(text)
	for _, field := range HeadFields {
		if _, ok := fields[field]; !ok {
			problems = append(problems, fmt.Sprintf("%s: the template has no '- **%s:**' line in its head", name, field))
		}
	}
	headings := headings(text)
	if len(headings) == 0 || headings[len(headings)-1] != "## Comments" {
		problems = append(problems, fmt.Sprintf("%s: the last section must be '## Comments'", name))
	}
	return problems
}

// headings is the `## ` section lines of a text, without a CRLF's carriage return, so a checkout or an editor on
// Windows reads the same as one on Unix.
func headings(text string) []string {
	sections := []string{}
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "## ") {
			sections = append(sections, strings.TrimSuffix(line, "\r"))
		}
	}
	return sections
}

// ProblemsOf is every problem on the board, in check order.
func ProblemsOf(project *config.Project) []string {
	problems := []string{}
	for _, check := range Checks {
		problems = append(problems, check(project)...)
	}
	return problems
}

// Run prints the problems, or that the board is sound; an unsound board is exit 1.
func Run(project *config.Project, out io.Writer) int {
	if out == nil {
		out = io.Discard
	}
	problems := ProblemsOf(project)
	if len(problems) == 0 {
		count := 0
		for _, column := range board.Load(project, map[string]string{}, nil).Columns {
			count += len(column.Cards)
		}
		fmt.Fprintf(out, "board ok (%d cards)\n", count)
		return 0
	}
	for _, problem := range problems {
		fmt.Fprintf(out, "  ! %s\n", problem)
	}
	fmt.Fprintf(out, "%d problem(s)\n", len(problems))
	return 1
}

func heldRest(held, word string) string {
	rest := strings.TrimLeft(held, " \t")
	if len(rest) >= len(word) {
		rest = rest[len(word):]
	}
	return strings.Trim(rest, " :—,")
}

func contains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

// whereWords names the Where lines an open card may carry. The default mode's wording is the one it has always had.
func whereWords(project *config.Project) string {
	vocabulary := workspace.Vocabulary(project)
	switch workspace.Of(project) {
	case workspace.Worktree:
		examples := project.IDExamples()
		if len(examples) == 1 {
			return "'—', 'main' or 'worktree " + examples[0]
		}
		parts := []string{"'—'", "'main'"}
		for index, example := range examples {
			if index == len(examples)-1 {
				parts = append(parts, "'worktree "+example)
				continue
			}
			parts = append(parts, "'worktree "+example+"'")
		}
		return strings.Join(parts, ", ")
	case workspace.Shared:
		return "'—' or 'main'"
	}
	return fmt.Sprintf("'—', 'main' or '%s'", vocabulary[len(vocabulary)-1])
}
