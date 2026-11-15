package cli

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/cloudbusting/kanbanzai/internal/testutil"
)

// W-18: every command the documentation tells a reader to run is one the command has, and every finding doctor can
// report has its fix in the troubleshooting guide.

func docText(t *testing.T, names ...string) map[string]string {
	t.Helper()
	root, err := testutil.RepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	if len(names) == 0 {
		guides, _ := filepath.Glob(filepath.Join(root, "docs", "*.md"))
		names = []string{"README.md"}
		for _, guide := range guides {
			names = append(names, "docs/"+filepath.Base(guide))
		}
	}
	files := map[string]string{}
	for _, name := range names {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
		if err != nil {
			t.Fatal(err)
		}
		files[name] = string(data)
	}
	return files
}

func TestEveryCommandTheDocsRunExists(t *testing.T) {
	// A command is known if the help lists it as a line of its own.
	known := map[string]bool{}
	for _, line := range strings.Split(usage, "\n") {
		if fields := strings.Fields(line); len(fields) > 0 && !strings.HasPrefix(fields[0], "-") {
			known[fields[0]] = true
		}
	}
	for _, name := range []string{"board", "wip", "claim", "move", "init", "doctor", "migrate", "workflow", "config", "install"} {
		if !known[name] {
			t.Fatalf("the help does not list %q: the check cannot trust its own list (%v)", name, known)
		}
	}
	// A verb is known if the command accepts it after its parent.
	verbs := map[string][]string{
		"workflow": {"show", "validate", "plan", "apply", "export", "presets"},
		"config":   {"upgrade"},
		"claim":    {"owner", "status", "claim", "renew", "release", "takeover", "heartbeat"},
		"gate":     {"pass", "fail", "status"},
	}
	command := regexp.MustCompile("`?kanbanzai ([a-z][a-z-]*)(?: ([a-z][a-z-]*))?")
	checked := 0
	for name, text := range docText(t) {
		// Only what a reader runs: a fenced block or an inline code span, never prose such as "kanbanzai is local".
		for _, span := range codeSpans(text) {
			for _, match := range command.FindAllStringSubmatch(span, -1) {
				word := match[1]
				if word == "is" || word == "board-view" || word == "will" {
					continue
				}
				checked++
				if !known[word] {
					t.Errorf("%s: `kanbanzai %s` is not a command (kanbanzai --help lists %d)", name, word, len(known))
					continue
				}
				if allowed, ok := verbs[word]; ok && match[2] != "" && !strings.HasPrefix(match[2], "-") {
					found := false
					for _, verb := range allowed {
						found = found || verb == match[2]
					}
					if !found {
						t.Errorf("%s: `kanbanzai %s %s` is not a verb of %s (%s)", name, word, match[2], word, strings.Join(allowed, ", "))
					}
				}
			}
		}
	}
	if checked < 100 {
		t.Fatalf("only %d command examples were checked: the docs moved or the check lost its way", checked)
	}
}

// codeSpans returns every fenced block and every inline code span in a Markdown text.
func codeSpans(text string) []string {
	var spans []string
	fenced := regexp.MustCompile("(?s)```[a-z]*\n(.*?)```")
	for _, match := range fenced.FindAllStringSubmatch(text, -1) {
		spans = append(spans, match[1])
	}
	text = fenced.ReplaceAllString(text, "")
	for _, match := range regexp.MustCompile("`([^`\n]+)`").FindAllStringSubmatch(text, -1) {
		spans = append(spans, match[1])
	}
	return spans
}

func TestEveryDoctorFindingHasItsFixInTheTroubleshootingGuide(t *testing.T) {
	newProject(t, "")
	_, out, _ := runCLI(t, "doctor", "--json")
	guide := docText(t, "docs/troubleshooting.md")["docs/troubleshooting.md"]
	id := regexp.MustCompile(`"id":\s*"([^"]+)"`)
	var names []string
	for _, match := range id.FindAllStringSubmatch(out, -1) {
		names = append(names, match[1])
	}
	if len(names) < 15 {
		t.Fatalf("doctor reported %d findings: %s", len(names), out)
	}
	sort.Strings(names)
	// The first cell of a table row names the findings it covers (`access.home`, `access.state`; `template.<type>`).
	cells := map[string]bool{}
	for _, line := range strings.Split(guide, "\n") {
		if cell, _, ok := strings.Cut(strings.TrimPrefix(line, "| "), " | "); ok && strings.HasPrefix(line, "| `") {
			for _, name := range regexp.MustCompile("`([^`]+)`").FindAllStringSubmatch(cell, -1) {
				cells[name[1]] = true
			}
		}
	}
	for _, name := range names {
		documented := cells[name]
		if dot := strings.Index(name, "."); !documented && strings.HasPrefix(name, "template.") && dot > 0 {
			documented = cells["template.<type>"]
		}
		if !documented {
			t.Errorf("docs/troubleshooting.md has no table row for the doctor finding %q", name)
		}
	}
}
