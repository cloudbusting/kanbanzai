package check

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/cloudbusting/kanbanzai/internal/config"
	"github.com/cloudbusting/kanbanzai/internal/testutil"
)

const checkConfig = "types = []\n[project]\nowner = \"Sam\"\ntemplate = \"template.md\"\n"

const goodTemplate = `# W-NN: <the work>

- **Stage:** todo
- **Updated:** YYYY-MM-DD
- **Next action:** <step>
- **Where:** —

## Goal

## Comments
`

func testProject(t *testing.T) *config.Project {
	t.Helper()
	t.Setenv("KANBANZAI_PROJECT", "")
	root := testutil.Repo(t, checkConfig)
	testutil.Folders(t, root, "1-open", "x-paused", "10-done")
	testutil.Write(t, filepath.Join(root, "template.md"), goodTemplate)
	project, err := config.Find(root)
	if err != nil {
		t.Fatalf("config.Find: %v", err)
	}
	return project
}

func goodBrief(card, stage, extra, held string) string {
	done := ""
	if stage == "done" {
		done = "- **Done:** 2026-11-10 — live\n"
	}
	heldLine := ""
	if held != "" {
		heldLine = "- **Held:** " + held + "\n"
	}
	return "# " + card + ": Some work\n\n- **Stage:** " + stage + "\n- **Updated:** 2026-11-10\n" + done +
		"- **Next action:** Do it\n- **Where:** —\n" + heldLine + "\n## Goal\n\nSee [the guide](../../../README.md)." +
		extra + "\n\n## Comments\n"
}

func put(t *testing.T, project *config.Project, folder, name, text string) {
	t.Helper()
	testutil.Write(t, filepath.Join(project.Work, folder, name), text)
}

func soundProject(t *testing.T) *config.Project {
	t.Helper()
	project := testProject(t)
	cards := []struct {
		card, stage, folder, held string
	}{
		{"W-01", "designed", "1-open", ""},
		{"W-02", "qa-approved", "1-open", ""},
		{"W-03", "designed", "1-open", "waiting: Sam, a decision"},
		{"W-04", "paused", "x-paused", ""},
		{"W-05", "done", "10-done", ""},
		{"W-08", "todo", "1-open", "blocked: W-01"},
	}
	for _, item := range cards {
		put(t, project, item.folder, item.card+"-work.md", goodBrief(item.card, item.stage, "", item.held))
	}
	return project
}

func fails(t *testing.T, fn func(*config.Project) []string, project *config.Project, needle string) {
	t.Helper()
	problems := fn(project)
	for _, problem := range problems {
		if strings.Contains(problem, needle) {
			return
		}
	}
	t.Fatalf("the check missed %q: %v", needle, problems)
}

func TestASoundBoardPasses(t *testing.T) {
	project := soundProject(t)
	for _, check := range Checks {
		if problems := check(project); len(problems) != 0 {
			t.Fatalf("check failed on a sound board: %v", problems)
		}
	}
	var out bytes.Buffer
	if code := Run(project, &out); code != 0 || strings.TrimSpace(out.String()) != "board ok (6 cards)" {
		t.Fatalf("check = %d %q", code, out.String())
	}
}

func TestQuestionSectionValidation(t *testing.T) {
	entry := "\n\n## Questions\n\n### Q1 — 2026-11-13\n- **Asked by:** agent\n- **Question:** Which?\n- **Then:** build\n- **Answer:** —\n- **Answered:** —"
	for _, tc := range []struct {
		name, extra, held string
		bad               bool
	}{
		{"absent", "", "", false},
		{"open held", entry, "waiting: Sam — answer in ## Questions", false},
		{"open without hold", entry, "", true},
		{"missing question", strings.Replace(entry, "- **Question:** Which?\n", "", 1), "waiting: Sam — answer", true},
		{"missing answered", strings.Replace(entry, "- **Answered:** —", "", 1), "waiting: Sam — answer", true},
		{"answered", strings.Replace(strings.Replace(entry, "- **Answer:** —", "- **Answer:** yes", 1), "- **Answered:** —", "- **Answered:** today", 1), "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			project := testProject(t)
			put(t, project, "1-open", "W-01-work.md", goodBrief("W-01", "building", tc.extra, tc.held))
			var output bytes.Buffer
			code := Run(project, &output)
			if (code != 0) != tc.bad {
				t.Fatalf("check = %d %s", code, output.String())
			}
		})
	}
	project := testProject(t)
	put(t, project, "1-open", "W-01-work.md", goodBrief("W-01", "building", "", "waiting: Sam — answer")+entry+"\n")
	var output bytes.Buffer
	if code := Run(project, &output); code == 0 {
		t.Fatalf("accepted Questions after Comments: %s", output.String())
	}
}

func TestTheCommandPrintsTheProblemsAndExitsOne(t *testing.T) {
	project := soundProject(t)
	put(t, project, "1-open", "W-06-x.md", "Some work\n\n## Comments\n")
	var out bytes.Buffer
	if code := Run(project, &out); code != 1 {
		t.Fatalf("check = %d", code)
	}
	if !strings.Contains(out.String(), "W-06-x.md: the first line must be '# W-NN: <the work>'") {
		t.Fatalf("output = %q", out.String())
	}
	if !regexp.MustCompile(`\d+ problem\(s\)`).MatchString(out.String()) {
		t.Fatalf("output = %q", out.String())
	}
}

func TestTitleDoesNotMatchFileName(t *testing.T) {
	project := soundProject(t)
	put(t, project, "1-open", "W-06-x.md", goodBrief("W-07", "designed", "", ""))
	fails(t, Titles, project, "the title names W-07")
}

func TestTitleMissing(t *testing.T) {
	project := soundProject(t)
	put(t, project, "1-open", "W-06-x.md", "Some work\n\n## Comments\n")
	fails(t, Titles, project, "the first line must be")
}

func TestHeaderLineMissing(t *testing.T) {
	project := soundProject(t)
	put(t, project, "1-open", "W-06-x.md", strings.Replace(goodBrief("W-06", "designed", "", ""),
		"- **Where:** —\n", "", 1))
	fails(t, Headers, project, "no '- **Where:**' line")
}

func TestHeaderLineMalformed(t *testing.T) {
	project := soundProject(t)
	put(t, project, "1-open", "W-06-x.md", strings.Replace(goodBrief("W-06", "designed", "", ""),
		"2026-11-10", "10 Nov", 1))
	fails(t, Headers, project, "Updated must be YYYY-MM-DD")
	put(t, project, "1-open", "W-07-x.md", strings.Replace(goodBrief("W-07", "designed", "", ""),
		"**Where:** —", "**Where:** somewhere", 1))
	fails(t, Headers, project, "Where must be")
}

func TestWhereMayNameAWorktreeOrMain(t *testing.T) {
	project := soundProject(t)
	put(t, project, "1-open", "W-06-x.md", strings.Replace(goodBrief("W-06", "designed", "", ""),
		"**Where:** —", "**Where:** worktree W-06 (note)", 1))
	put(t, project, "10-done", "W-07-x.md", strings.Replace(goodBrief("W-07", "done", "", ""),
		"**Where:** —", "**Where:** main", 1))
	if problems := Headers(project); len(problems) != 0 {
		t.Fatalf("problems = %v", problems)
	}
}

func TestDoneCardWithoutItsDate(t *testing.T) {
	project := soundProject(t)
	put(t, project, "10-done", "W-06-x.md", strings.Replace(goodBrief("W-06", "done", "", ""),
		"- **Done:** 2026-11-10 — live", "- **Done:** —", 1))
	fails(t, Headers, project, "Done line starts with its date")
}

func TestUnknownStage(t *testing.T) {
	project := soundProject(t)
	put(t, project, "1-open", "W-06-x.md", goodBrief("W-06", "pondering", "", ""))
	fails(t, Stages, project, "unknown stage")
}

func TestFolderDoesNotMatchStage(t *testing.T) {
	project := soundProject(t)
	put(t, project, "x-paused", "W-06-x.md", goodBrief("W-06", "designed", "", ""))
	fails(t, Stages, project, "belongs in .kanbanzai/work/1-open/")
	put(t, project, "10-done", "W-07-x.md", goodBrief("W-07", "built", "", ""))
	fails(t, Stages, project, "belongs in .kanbanzai/work/1-open/")
}

func TestTwoBriefsShareANumber(t *testing.T) {
	project := soundProject(t)
	put(t, project, "x-paused", "W-01-again.md", goodBrief("W-01", "paused", "", ""))
	fails(t, Unique, project, "W-01 is also")
}

func TestAHeldLineSaysBlockedOrWaiting(t *testing.T) {
	project := soundProject(t)
	put(t, project, "1-open", "W-06-x.md", goodBrief("W-06", "built", "", "stuck on something"))
	fails(t, Held, project, "W-06-x.md: a Held line starts 'blocked' or 'waiting'")
	put(t, project, "1-open", "W-07-x.md", goodBrief("W-07", "built", "", "blocked"))
	fails(t, Held, project, "W-07-x.md: a Held line says what it is held on")
	for _, held := range []string{"blocked: W-01", "waiting: Sam — the key", "—"} {
		put(t, project, "1-open", "W-09-x.md", goodBrief("W-09", "built", "", held))
		for _, problem := range Held(project) {
			if strings.Contains(problem, "W-09") {
				t.Fatalf("valid Held %q was reported: %q", held, problem)
			}
		}
	}
}

func TestAPausedOrDoneCardIsNotHeld(t *testing.T) {
	project := soundProject(t)
	put(t, project, "x-paused", "W-06-x.md", goodBrief("W-06", "paused", "", "blocked: W-01"))
	fails(t, Held, project, "W-06-x.md: a paused card is not held")
	put(t, project, "10-done", "W-07-x.md", goodBrief("W-07", "done", "", "waiting: Sam"))
	fails(t, Held, project, "W-07-x.md: a done card is not held")
}

func TestTheOldBlockedStageNamesTheHeldLine(t *testing.T) {
	project := soundProject(t)
	put(t, project, "1-open", "W-06-x.md", goodBrief("W-06", "blocked: W-01, from built", "", ""))
	fails(t, Stages, project, "a Held line")
}

func TestBrokenLink(t *testing.T) {
	project := soundProject(t)
	put(t, project, "1-open", "W-06-x.md", goodBrief("W-06", "designed", " And [gone](../nowhere.md).", ""))
	fails(t, Links, project, "broken link ../nowhere.md")
	for _, problem := range Links(project) {
		if strings.Contains(problem, "W-01") {
			t.Fatalf("a good link was reported: %q", problem)
		}
	}
}

func TestLastSectionIsNotComments(t *testing.T) {
	project := soundProject(t)
	put(t, project, "1-open", "W-06-x.md", goodBrief("W-06", "designed", "", "")+"\n## Notes\n")
	fails(t, Comments, project, "the last section must be '## Comments'")
}

func TestColumnFolderMissing(t *testing.T) {
	project := soundProject(t)
	if err := os.Remove(filepath.Join(project.Work, "x-paused", "W-04-work.md")); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(project.Work, "x-paused")); err != nil {
		t.Fatal(err)
	}
	fails(t, Folders, project, ".kanbanzai/work/x-paused/ is missing")
}

func TestAStrayFolderAndABriefOutsideTheBoard(t *testing.T) {
	project := soundProject(t)
	put(t, project, "2-dev", "W-06-x.md", goodBrief("W-06", "built", "", ""))
	fails(t, Folders, project, ".kanbanzai/work/2-dev/ is not a board folder")
	fails(t, Folders, project, "a brief must sit in")
}

func TestTheBatchesFolderIsAllowed(t *testing.T) {
	project := soundProject(t)
	if err := os.MkdirAll(project.Batches, 0o755); err != nil {
		t.Fatal(err)
	}
	if problems := Folders(project); len(problems) != 0 {
		t.Fatalf("problems = %v", problems)
	}
}

func TestTheTemplate(t *testing.T) {
	project := soundProject(t)
	template := filepath.Join(project.Root, "template.md")
	if err := os.WriteFile(template, []byte("# W-NN: x\n\n## Comments\n\n## Late\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	fails(t, Template, project, "the template has no '- **Stage:**' line")
	fails(t, Template, project, "the last section must be '## Comments'")
	if err := os.Remove(template); err != nil {
		t.Fatal(err)
	}
	fails(t, Template, project, "the card template is missing")
}

func TestAMissingShippedTemplateIsNotTheProjectsFault(t *testing.T) {
	t.Setenv("KANBANZAI_PROJECT", "")
	root := testutil.Repo(t, "[project]\nowner = \"Sam\"\n")
	testutil.Folders(t, root, "1-open", "x-paused", "10-done")
	project, err := config.Find(root)
	if err != nil {
		t.Fatal(err)
	}
	if !project.TemplateIsShipped {
		t.Fatal("the template should be shipped")
	}
	for _, problem := range Template(project) {
		if strings.Contains(problem, "missing") {
			t.Fatalf("the shipped template was called missing: %q", problem)
		}
	}
}

func TestACRLFTemplateAndBriefAreRead(t *testing.T) {
	project := soundProject(t)
	template := filepath.Join(project.Root, "template.md")
	if err := os.WriteFile(template, []byte(strings.ReplaceAll(goodTemplate, "\n", "\r\n")), 0o644); err != nil {
		t.Fatal(err)
	}
	if problems := Template(project); len(problems) != 0 {
		t.Fatalf("a CRLF template: %v", problems)
	}
	path := filepath.Join(project.Work, "1-open", "W-01-work.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(strings.ReplaceAll(string(data), "\n", "\r\n")), 0o644); err != nil {
		t.Fatal(err)
	}
	if problems := Comments(project); len(problems) != 0 {
		t.Fatalf("a CRLF brief: %v", problems)
	}
}

// withHead puts extra head lines (a Gate or Needs line) after Where in a good brief.
func withHead(card, stage, head string) string {
	return strings.Replace(goodBrief(card, stage, "", ""), "- **Where:** —\n", "- **Where:** —\n"+head, 1)
}

func TestAGateLineBelongsOnAGatedCardAndReadsPassedOrFailed(t *testing.T) {
	project := soundProject(t)
	put(t, project, "1-open", "W-02-work.md", withHead("W-02", "qa-approved", "- **Gate:** passed by claude:1, 2026-11-12 — yes\n"))
	put(t, project, "1-open", "W-06-x.md", withHead("W-06", "qa-approved", "- **Gate:** failed by Sam, 2026-11-12\n"))
	if problems := Gates(project); len(problems) != 0 {
		t.Fatalf("valid Gate lines were reported: %v", problems)
	}
	put(t, project, "1-open", "W-07-x.md", withHead("W-07", "built", "- **Gate:** passed by Sam, 2026-11-12\n"))
	fails(t, Gates, project, "W-07-x.md: a Gate line belongs on a card in a column with an owner, ci or external gate")
	for _, line := range []string{"passed", "approved by Sam, 2026-11-12", "passed by Sam", "passed by Sam, yesterday"} {
		put(t, project, "1-open", "W-09-x.md", withHead("W-09", "qa-approved", "- **Gate:** "+line+"\n"))
		fails(t, Gates, project, "W-09-x.md: a Gate line reads 'passed|failed by <who>, YYYY-MM-DD")
	}
}

func TestANeedsLineNamesCardsThatExistAndNeverCircles(t *testing.T) {
	project := soundProject(t)
	put(t, project, "1-open", "W-06-x.md", withHead("W-06", "built", "- **Needs:** W-01, W-05\n"))
	if problems := Gates(project); len(problems) != 0 {
		t.Fatalf("a valid Needs line was reported: %v", problems)
	}
	put(t, project, "1-open", "W-06-x.md", withHead("W-06", "built", "- **Needs:** W-06\n"))
	fails(t, Gates, project, "W-06-x.md: a card cannot need itself")
	put(t, project, "1-open", "W-06-x.md", withHead("W-06", "built", "- **Needs:** W-77\n"))
	fails(t, Gates, project, "W-06-x.md: Needs names W-77, which is not on the board")
	put(t, project, "1-open", "W-06-x.md", withHead("W-06", "built", "- **Needs:** W-07\n"))
	put(t, project, "1-open", "W-07-x.md", withHead("W-07", "built", "- **Needs:** W-06\n"))
	fails(t, Gates, project, "the cards wait on each other: W-06 -> W-07 -> W-06")
	put(t, project, "1-open", "W-07-x.md", withHead("W-07", "built", "- **Needs:** W-08\n"))
	put(t, project, "1-open", "W-08-work.md", withHead("W-08", "todo", "- **Needs:** W-06\n"))
	fails(t, Gates, project, "the cards wait on each other")
}
