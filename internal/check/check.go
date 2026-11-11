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
	"strings"

	"github.com/cloudbusting/kanbanzai/internal/board"
	"github.com/cloudbusting/kanbanzai/internal/brief"
	"github.com/cloudbusting/kanbanzai/internal/config"
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
	Folders, Titles, Headers, Stages, Unique, Held, Links, Comments, Template,
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
			if !strings.HasPrefix(name, project.Prefix+"-") || !strings.HasSuffix(name, ".md") {
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
			problems = append(problems, fmt.Sprintf("%s: the first line must be '# %s-NN: <the work>'",
				project.Rel(path), project.Prefix))
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
	whereRE := regexp.MustCompile(`^(—|main|worktree ` + project.IDPattern + `)(\s.*)?$`)
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
		if value := fields["Where"]; value != "" && !whereRE.MatchString(value) {
			short := value
			if len(short) > 30 {
				short = short[:30]
			}
			problems = append(problems, fmt.Sprintf("%s: Where must be '—', 'main' or 'worktree %s-NN, not '%s'",
				name, project.Prefix, short))
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

// Unique checks that no two briefs share a number.
func Unique(project *config.Project) []string {
	problems := []string{}
	seen := map[string]string{}
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
		headings := []string{}
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(line, "## ") {
				headings = append(headings, line)
			}
		}
		if len(headings) == 0 || headings[len(headings)-1] != "## Comments" {
			problems = append(problems, fmt.Sprintf("%s: the last section must be '## Comments'", project.Rel(path)))
		}
	}
	return problems
}

// Template checks that the card template exists, has the head's fields and ends in ## Comments.
func Template(project *config.Project) []string {
	shipped := project.TemplateIsShipped
	name := "templates/card.md"
	if !shipped {
		name = project.Rel(project.Template)
		if _, err := os.Stat(project.Template); err != nil {
			return []string{fmt.Sprintf("%s: the card template is missing", name)}
		}
	}
	text, err := project.TemplateText()
	if err != nil {
		if shipped {
			return nil
		}
		return []string{fmt.Sprintf("%s: the card template is missing", name)}
	}
	problems := []string{}
	_, fields := brief.Header(text)
	for _, field := range HeadFields {
		if _, ok := fields[field]; !ok {
			problems = append(problems, fmt.Sprintf("%s: the template has no '- **%s:**' line in its head", name, field))
		}
	}
	headings := []string{}
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "## ") {
			headings = append(headings, line)
		}
	}
	if len(headings) == 0 || headings[len(headings)-1] != "## Comments" {
		problems = append(problems, fmt.Sprintf("%s: the last section must be '## Comments'", name))
	}
	return problems
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
