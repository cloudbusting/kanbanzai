package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	kanbanzai "github.com/cloudbusting/kanbanzai"
	"github.com/cloudbusting/kanbanzai/internal/config"
	"github.com/cloudbusting/kanbanzai/internal/testutil"
)

func example(t *testing.T, file string) string {
	t.Helper()
	data, err := kanbanzai.Files.ReadFile("templates/" + file)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// schemaExampleProject opts a project into the shipped example: its templates and schemas are copied in as a user would.
func schemaExampleProject(t *testing.T) *config.Project {
	t.Helper()
	project := newProject(t, "[project]\ndefault_type = \"task\"\n[flow]\nisolation = \"shared\"\n"+example(t, "schemas.example.toml"))
	for _, name := range []string{"task", "bug", "change"} {
		testutil.Write(t, filepath.Join(project.Root, "templates", name+".md"), example(t, name+".md"))
	}
	return project
}

func card(t *testing.T, project *config.Project, id, template string, edit func(string) string) {
	t.Helper()
	text := strings.Replace(example(t, template), "<ID>", id, 1)
	text = strings.Replace(text, "YYYY-MM-DD", "2026-11-13", 1)
	if edit != nil {
		text = edit(text)
	}
	testutil.Write(t, filepath.Join(project.Work, "1-open", id+"-card.md"), text)
}

func TestShippedTypeExamplesPassTheirOwnChecks(t *testing.T) {
	project := schemaExampleProject(t)
	if code, out, errOut := runCLI(t, "check"); code != 0 {
		t.Fatalf("the examples fail check with no cards: %d %s %s", code, out, errOut)
	}
	card(t, project, "TASK-01", "task.md", nil)
	card(t, project, "BUG-02", "bug.md", nil)
	card(t, project, "CHANGE-03", "change.md", nil)
	if code, out, errOut := runCLI(t, "check"); code != 0 {
		t.Fatalf("cards written from the shipped templates fail check: %d %s %s", code, out, errOut)
	}
}

func TestTypeSchemasAcceptABugWithReproductionAndATaskWithout(t *testing.T) {
	project := schemaExampleProject(t)
	card(t, project, "TASK-01", "task.md", nil)
	card(t, project, "BUG-02", "bug.md", nil)
	if code, out, errOut := runCLI(t, "check"); code != 0 {
		t.Fatalf("check = %d %s %s", code, out, errOut)
	}
	cases := []struct {
		name, id, template, want string
		edit                     func(string) string
	}{
		{"bug without reproduction", "BUG-05", "bug.md", `missing required section "Reproduction"`,
			func(s string) string { return strings.Replace(s, "## Reproduction", "## Steps", 1) }},
		{"task with a bug section", "TASK-05", "task.md", `undeclared section "Reproduction"`,
			func(s string) string { return strings.Replace(s, "## Docs", "## Reproduction\n\n## Docs", 1) }},
		{"missing required field", "TASK-05", "task.md", `missing required field "Next action"`,
			func(s string) string { return strings.Replace(s, "- **Next action:**", "- **Next step:**", 1) }},
		{"undeclared head field", "TASK-05", "task.md", `undeclared field "Priority"`,
			func(s string) string {
				return strings.Replace(s, "- **Where:**", "- **Priority:** high\n- **Where:**", 1)
			}},
		{"order", "BUG-05", "bug.md", `section "Expected" out of order`,
			func(s string) string {
				s = strings.Replace(s, "## Expected", "## \x00", 1)
				return strings.Replace(strings.Replace(s, "## Actual", "## Expected", 1), "## \x00", "## Actual", 1)
			}},
		{"illegal repeat", "TASK-05", "task.md", `duplicate section "Goal"`,
			func(s string) string { return strings.Replace(s, "## Docs", "## Goal\n\n## Docs", 1) }},
		{"wrong final section", "TASK-05", "task.md", "Comments",
			func(s string) string { return s + "\n## Risk\n" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			card(t, project, tc.id, tc.template, tc.edit)
			defer os.Remove(filepath.Join(project.Work, "1-open", tc.id+"-card.md"))
			code, out, errOut := runCLI(t, "check")
			if code == 0 || !strings.Contains(out+errOut, tc.want) || !strings.Contains(out+errOut, tc.id+"-card.md") {
				t.Fatalf("check = %d, want %q naming %s-card.md:\n%s%s", code, tc.want, tc.id, out, errOut)
			}
		})
	}
}

func TestTemplateCommandNamesTheTemplateAForTheTypeOrCard(t *testing.T) {
	project := schemaExampleProject(t)
	for _, arg := range []string{"bug", "BUG", "BUG-12", "bug-12"} {
		code, out, errOut := runCLI(t, "template", arg)
		if code != 0 || out != example(t, "bug.md") {
			t.Fatalf("template %s = %d %q %s", arg, code, out, errOut)
		}
	}
	code, out, _ := runCLI(t, "template", "BUG-NN", "--where")
	if code != 0 || out != "template: templates/bug.md\nschema: bug\n" {
		t.Fatalf("template --where = %d %q", code, out)
	}
	if code, out, errOut := runCLI(t, "template", "nope"); code != 2 || !strings.Contains(errOut, "types: task, bug, change") {
		t.Fatalf("unknown type = %d %s %s", code, out, errOut)
	}
	if err := os.Remove(filepath.Join(project.Root, "templates", "bug.md")); err != nil {
		t.Fatal(err)
	}
	code, out, errOut := runCLI(t, "template", "bug")
	if code != 1 || out != "" || !strings.Contains(errOut, "templates/bug.md") {
		t.Fatalf("an unreadable type template must fail, not fall back: %d %q %s", code, out, errOut)
	}
}

func TestTemplateCommandFallsBackOnlyWhenTheTypeNamesNoTemplate(t *testing.T) {
	newProject(t, typedCLIConfig)
	for _, arg := range []string{"task", "BUG-04", ""} {
		args := []string{"template"}
		if arg != "" {
			args = append(args, arg)
		}
		code, out, errOut := runCLI(t, args...)
		if code != 0 || out != example(t, "card.md") {
			t.Fatalf("template %q = %d %q %s", arg, code, out, errOut)
		}
	}
	if code, out, _ := runCLI(t, "template", "bug", "--where"); code != 0 ||
		out != "template: templates/card.md (shipped)\nschema: none (the project's current brief rules)\n" {
		t.Fatalf("--where = %d %q", code, out)
	}
	for _, name := range []string{"task", "bug", "change"} {
		code, out, _ := runCLI(t, "template", "--example", name)
		if code != 0 || out != example(t, name+".md") {
			t.Fatalf("--example %s = %d", name, code)
		}
	}
	if code, _, errOut := runCLI(t, "template", "--example", "nope"); code != 2 {
		t.Fatalf("unknown example = %d %s", code, errOut)
	}
}

func TestAProjectWithNoSchemaKeepsTheLegacyCheckerAndJSON(t *testing.T) {
	project := newProject(t, typedCLIConfig)
	custom := "# BUG-02: Custom\n\n- **Stage:** todo\n- **Updated:** 2026-11-13\n- **Priority:** high\n- **Next action:** go\n- **Where:** —\n\n## Goal\n\n## Whatever I like\n\n## Comments\n"
	testutil.Write(t, filepath.Join(project.Work, "1-open", "BUG-02-custom.md"), custom)
	writeCard(t, project, "W-01", "Legacy", "todo", "")
	if code, out, errOut := runCLI(t, "check"); code != 0 {
		t.Fatalf("custom fields and sections without a schema = %d %s %s", code, out, errOut)
	}
	code, out, _ := runCLI(t, "board", "--json")
	if code != 0 {
		t.Fatal("board --json failed")
	}
	if strings.Contains(out, `"schema"`) {
		t.Fatalf("board --json grew a schema field for a project with none: %s", out)
	}
}

func TestSchemaIsAnAdditiveFieldInBoardAndConfigJSON(t *testing.T) {
	project := schemaExampleProject(t)
	card(t, project, "BUG-02", "bug.md", nil)
	writeCard(t, project, "W-01", "Legacy", "todo", "")
	code, out, _ := runCLI(t, "board", "--json")
	if code != 0 {
		t.Fatal("board --json failed")
	}
	type cardJSON struct {
		ID   string         `json:"id"`
		Type map[string]any `json:"type"`
	}
	var board struct {
		Types   []map[string]any `json:"types"`
		Columns []struct {
			Cards []cardJSON `json:"cards"`
		} `json:"columns"`
	}
	if err := json.Unmarshal([]byte(out), &board); err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	var cards []cardJSON
	for _, column := range board.Columns {
		cards = append(cards, column.Cards...)
	}
	for _, c := range cards {
		seen[c.ID] = true
		switch c.ID {
		case "BUG-02":
			if c.Type["key"] != "bug" || c.Type["schema"] != "bug" || c.Type["prefix"] != "BUG" {
				t.Fatalf("BUG-02 type = %v", c.Type)
			}
		case "W-01":
			if c.Type != nil {
				t.Fatalf("a legacy card has a type: %v", c.Type)
			}
		}
	}
	if !seen["BUG-02"] || !seen["W-01"] || len(board.Types) != 3 || board.Types[1]["schema"] != "bug" {
		t.Fatalf("board JSON = %s", out)
	}
	code, out, _ = runCLI(t, "config", "--json")
	if code != 0 || !strings.Contains(out, `"schema": "bug"`) {
		t.Fatalf("config --json lacks the schema: %d %s", code, out)
	}
}

func TestShippedSchemaSurvivesMoveAskAndAnswer(t *testing.T) {
	project := schemaExampleProject(t)
	card(t, project, "BUG-02", "bug.md", func(s string) string {
		s = strings.Replace(s, "<one concrete step; the next session starts here>", "Reproduce it", 1)
		s = strings.Replace(s, "- **Where:** — before work starts, then the record from `kanbanzai workspace <ID>` when required; follow the configured close instructions.", "- **Where:** —", 1)
		return strings.Replace(s, "- **Stage:** todo (the shipped example initial column; replace it with the project's initial column key from `kanbanzai workflow show`; use\n  `kanbanzai step --for <ID>` and `kanbanzai move` for later transitions, never edit the Stage by hand. A card that cannot move keeps its column and\n  gets a line `- **Held:** blocked: <another card, an account, a key>` or `- **Held:** waiting: <the owner, a\n  service, a date>` after Where; delete the line when it clears)", "- **Stage:** todo", 1)
	})
	step := func(args ...string) {
		t.Helper()
		if code, out, errOut := runCLI(t, args...); code != 0 {
			t.Fatalf("%s = %d %s %s", strings.Join(args, " "), code, out, errOut)
		}
		if code, out, errOut := runCLI(t, "check"); code != 0 {
			t.Fatalf("check after %s = %d %s %s", strings.Join(args, " "), code, out, errOut)
		}
	}
	step("check")
	step("claim", "claim", "BUG-02", "--no-move")
	step("move", "BUG-02", "start")
	step("ask", "BUG-02", "--text", "Which build?", "--option", "A", "--option", "B", "--recommend", "A")
	step("answer", "BUG-02", "--text", "A")
	step("claim", "claim", "BUG-02", "--no-move")
	step("move", "BUG-02", "start")
	step("move", "BUG-02", "finish", "--next", "Check it", "--progress", "Fixed")
	data, err := os.ReadFile(filepath.Join(project.Work, "1-open", "BUG-02-card.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	order := []string{"## Reproduction", "## Expected", "## Actual", "## Progress", "## Questions", "## Comments"}
	last := -1
	for _, heading := range order {
		at := strings.Index(text, heading+"\n")
		if at < 0 || at < last {
			t.Fatalf("%s missing or out of the schema's order:\n%s", heading, text)
		}
		last = at
	}
}

func TestALegacyProjectKeepsItsTemplateAndChecker(t *testing.T) {
	project := newProject(t, "types = []\n[project]\ntemplate = \"my-card.md\"\n[flow]\nisolation = \"shared\"\n")
	mine := strings.Replace(example(t, "card.md"), "## Risk", "## My own section\n\n## Risk", 1)
	testutil.Write(t, filepath.Join(project.Root, "my-card.md"), mine)
	writeCard(t, project, "W-01", "Legacy", "todo", "")
	if code, out, errOut := runCLI(t, "check"); code != 0 {
		t.Fatalf("legacy check = %d %s %s", code, out, errOut)
	}
	for _, args := range [][]string{{"template"}, {"template", "W-01"}, {"template", "W-NN"}} {
		if code, out, errOut := runCLI(t, args...); code != 0 || out != mine {
			t.Fatalf("%v = %d %q %s", args, code, out, errOut)
		}
	}
	if code, _, _ := runCLI(t, "template", "bug"); code != 2 {
		t.Fatalf("a legacy project has no bug type: %d", code)
	}
}

// fenced returns the body of the first ``` block whose opening line is exactly open.
func fenced(t *testing.T, file, open string) string {
	t.Helper()
	root, err := testutil.RepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, file))
	if err != nil {
		t.Fatal(err)
	}
	_, after, ok := strings.Cut(string(data), "\n"+open+"\n")
	if !ok {
		t.Fatalf("%s has no %s block", file, open)
	}
	body, _, _ := strings.Cut(after, "\n```")
	return body + "\n"
}

func TestTheDocumentedIncidentExampleWorksWithTheRealChecker(t *testing.T) {
	configText := fenced(t, "docs/types.md", "```toml title=\"incident-config\"")
	template := fenced(t, "docs/types.md", "```markdown title=\"incident-template\"")
	project := newProject(t, configText+"\n[flow]\nisolation = \"shared\"\n")
	testutil.Write(t, filepath.Join(project.Root, "templates", "incident.md"), template)
	write := func(edit func(string) string) {
		text := strings.Replace(strings.Replace(template, "INC-NN", "INC-07", 1), "YYYY-MM-DD", "2026-11-13", 1)
		text = strings.Replace(text, "<sev1, sev2 or sev3>", "sev2", 1)
		if edit != nil {
			text = edit(text)
		}
		testutil.Write(t, filepath.Join(project.Work, "1-open", "INC-07-db-down.md"), text)
	}
	write(nil)
	if code, out, errOut := runCLI(t, "check"); code != 0 {
		t.Fatalf("the documented example fails its own check: %d %s %s", code, out, errOut)
	}
	if code, out, _ := runCLI(t, "template", "INC"); code != 0 || out != template {
		t.Fatalf("template INC = %d %q", code, out)
	}
	write(func(s string) string { return strings.Replace(s, "## Timeline\n", "## Timeline\n\n## Timeline\n", 1) })
	if code, out, errOut := runCLI(t, "check"); code != 0 {
		t.Fatalf("a repeated Timeline is allowed: %d %s %s", code, out, errOut)
	}
	cases := []struct {
		want string
		edit func(string) string
	}{
		{`missing required field "Severity"`, func(s string) string { return strings.Replace(s, "- **Severity:** sev2\n", "", 1) }},
		{`missing required section "Timeline"`, func(s string) string { return strings.Replace(s, "## Timeline", "## Notes", 1) }},
		{`undeclared section "Impact"`, func(s string) string { return strings.Replace(s, "## Progress", "## Impact\n\n## Progress", 1) }},
		{`section "Goal" out of order`, func(s string) string { return strings.Replace(s, "## Goal", "## Timeline\n\n## Goal", 1) }},
		{`duplicate section "Goal"`, func(s string) string { return strings.Replace(s, "## Timeline", "## Goal\n\n## Timeline", 1) }},
		{`undeclared field "Priority"`, func(s string) string {
			return strings.Replace(s, "- **Where:**", "- **Priority:** p1\n- **Where:**", 1)
		}},
	}
	for _, tc := range cases {
		write(tc.edit)
		code, out, errOut := runCLI(t, "check")
		if code == 0 || !strings.Contains(out+errOut, "INC-07-db-down.md: "+tc.want) {
			t.Errorf("want %q in the check output (the docs list it):\n%s%s", tc.want, out, errOut)
		}
		if !strings.Contains(fencedAll(t, "docs/types.md"), "INC-07-db-down.md: "+tc.want) {
			t.Errorf("docs/types.md does not show the error %q", tc.want)
		}
	}
}

func fencedAll(t *testing.T, file string) string {
	t.Helper()
	root, _ := testutil.RepoRoot()
	data, err := os.ReadFile(filepath.Join(root, file))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestTheReadmeSchemaExampleLoadsAndItsBugTemplateChecks(t *testing.T) {
	readme := fencedAll(t, "README.md")
	at := strings.Index(readme, "[schemas.bug]")
	if at < 0 {
		t.Fatal("README.md has no [schemas.bug] example")
	}
	start := strings.LastIndex(readme[:at], "\n```toml\n") + len("\n```toml\n")
	configText := readme[start : at+strings.Index(readme[at:], "\n```")]
	project := newProject(t, configText+"\n[flow]\nisolation = \"shared\"\n")
	testutil.Write(t, filepath.Join(project.Root, "templates", "bug.md"), example(t, "bug.md"))
	if code, out, errOut := runCLI(t, "check"); code != 0 {
		t.Fatalf("the README example fails check: %d %s %s", code, out, errOut)
	}
}
