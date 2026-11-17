package kanbanzai

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"unicode"

	"github.com/cloudbusting/kanbanzai/internal/testutil"
)

// W-17: the extension's README states what it executes, reads and writes, and its network and telemetry position,
// and the setup guide leads with the editor install; a user decides to trust the extension from these words.
func TestTheExtensionReadmeStatesWhatItExecutesReadsAndWrites(t *testing.T) {
	root, err := testutil.RepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, "board-view", "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	flat := strings.Join(strings.Fields(string(data)), " ")
	for _, words := range []string{"What it executes, reads and writes", "**Executes:**", "**Reads:**", "**Writes:**",
		"until you click Apply, only the pending plan", "setup-plan.json", "**Network, telemetry:** none", "execFile", "trusted workspace", "install",
		"init --plan", "per-user folder", "not signed"} {
		if !strings.Contains(flat, words) {
			t.Errorf("board-view/README.md lacks %q", words)
		}
	}
	guide, err := os.ReadFile(filepath.Join(root, "docs", "setup.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(guide), "Kanbanzai: Apply") || !strings.Contains(string(guide), "Cancel Setup") ||
		!strings.Contains(string(guide), "Kanbanzai: Init") || !strings.Contains(string(guide), "claude plugin marketplace add") {
		t.Error("docs/setup.md must lead with the editor install and give the command-line route")
	}
}

// W-13: the delivery cookbook names every gate type and every delivery fixture, and each fixture it names exists.
func TestTheDeliveryCookbookNamesEveryGateAndFixture(t *testing.T) {
	root, err := testutil.RepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, "docs", "delivery.md"))
	if err != nil {
		t.Fatal(err)
	}
	flat := strings.Join(strings.Fields(string(data)), " ")
	for _, gate := range []string{"`owner`", "`ci`", "`external`", "`date`", "`card`"} {
		if !strings.Contains(flat, "| "+gate+" |") {
			t.Errorf("docs/delivery.md lacks a row for the %s gate", gate)
		}
	}
	for _, words := range []string{"skip_field", "skip_values", "kanbanzai gate pass", "kanbanzai gate status", "What the flow cannot express",
		"no parallel steps", "no branching", "no recurring schedule"} {
		if !strings.Contains(flat, words) {
			t.Errorf("docs/delivery.md lacks %q", words)
		}
	}
	fixtures, err := filepath.Glob(filepath.Join(root, "internal", "flow", "testdata", "delivery", "*.toml"))
	if err != nil || len(fixtures) < 6 {
		t.Fatalf("delivery fixtures: %v %v", fixtures, err)
	}
	for _, path := range fixtures {
		if name := filepath.Base(path); !strings.Contains(flat, "`"+name+"`") {
			t.Errorf("docs/delivery.md does not name the fixture %s", name)
		}
	}
}

// W-12: the isolation guide names every mode, says what each gives up, and states the rule for parallel work.
func TestTheIsolationGuideNamesEveryMode(t *testing.T) {
	root, err := testutil.RepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, "docs", "isolation.md"))
	if err != nil {
		t.Fatal(err)
	}
	flat := strings.Join(strings.Fields(string(data)), " ")
	for _, words := range []string{"### worktree", "### branch", "### shared", "### external", "kanbanzai workspace <ID>",
		"Parallel sessions collide", "use `worktree` mode for parallel work", "Changing mode", "any mode's word"} {
		if !strings.Contains(flat, words) {
			t.Errorf("docs/isolation.md lacks %q", words)
		}
	}
}

// W-34: the migration contract is described where an owner and a release author look for it, and the extension's
// trust statement names the one write path it gained.
func TestTheMigrationPathIsDocumented(t *testing.T) {
	root, err := testutil.RepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	read := func(parts ...string) string {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(append([]string{root}, parts...)...))
		if err != nil {
			t.Fatal(err)
		}
		return strings.Join(strings.Fields(string(data)), " ")
	}
	for file, words := range map[string][]string{
		"docs/upgrade.md": {"kanbanzai migrate", "migrate --apply --plan", "migrate --rollback", "migrate.md", "config upgrade"},
		"docs/migrate.md": {"plan_sha256", "recovery", "Registry", "Detect", "idempotent", "exit 2", "one approval", "an old board must still load",
			"Kanbanzai: Migrate", "question tool", "never commits"},
		"board-view/README.md":                     {"migrate --apply --plan", "only when you click Apply", "Kanbanzai: Migrate", "migrate --json"},
		"CHANGELOG.md":                             {"kanbanzai migrate", "W-34"},
		".claude/skills/kanbanzai-design/SKILL.md": {"internal/migrate", "docs/migrate.md"},
	} {
		flat := read(strings.Split(file, "/")...)
		for _, w := range words {
			if !strings.Contains(flat, w) {
				t.Errorf("%s lacks %q", file, w)
			}
		}
	}
}

// W-14: the presets guide's cookbook names every shipped delivery preset, and says how to write one.
func TestThePresetsCookbookNamesEveryShippedPreset(t *testing.T) {
	root, err := testutil.RepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(filepath.Join(root, "presets"))
	if err != nil || len(entries) < 7 {
		t.Fatalf("presets: %v %v", entries, err)
	}
	guide, err := os.ReadFile(filepath.Join(root, "docs", "presets.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(guide)
	at := strings.Index(text, "## The cookbook")
	if at < 0 {
		t.Fatal("docs/presets.md has no `## The cookbook` section")
	}
	table := text[at:]
	if end := strings.Index(table[4:], "\n## "); end > 0 {
		table = table[:end+4]
	}
	header := "| Preset | Delivery style | Best for | Flow | Gates | Isolation | Types, template and schema | What it does not do |"
	if !strings.Contains(table, header) {
		t.Errorf("the presets cookbook lacks the table header %q", header)
	}
	for _, entry := range entries {
		if !strings.Contains(table, "| `"+entry.Name()+"` |") {
			t.Errorf("the presets cookbook has no row for the %s preset", entry.Name())
		}
	}
	flat := strings.Join(strings.Fields(text), " ")
	for _, words := range []string{"flow.toml", "config.toml", "templates/", "--type", "--step-skill", "workflow plan --preset",
		"workflow presets", "-update", "What a preset does not do"} {
		if !strings.Contains(flat, words) {
			t.Errorf("docs/presets.md lacks %q", words)
		}
	}
}

// W-18: the documentation a marketplace visitor reads is a connected whole. Every relative link in the README and the
// guides resolves, including the heading it points at; every guide is reachable from the README; and the README opens
// with the install, leaving the checkout-oriented material to the contributor guide.
func docPages(t *testing.T) (string, []string) {
	t.Helper()
	root, err := testutil.RepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	guides, err := filepath.Glob(filepath.Join(root, "docs", "*.md"))
	if err != nil || len(guides) == 0 {
		t.Fatalf("docs/*.md: %v %v", guides, err)
	}
	pages := []string{"README.md", "SECURITY.md", "SUPPORT.md", "board-view/README.md"}
	for _, guide := range guides {
		pages = append(pages, "docs/"+filepath.Base(guide))
	}
	return root, pages
}

// anchorsOf returns the GitHub heading anchors of a Markdown file, ignoring fenced blocks.
func anchorsOf(text string) map[string]bool {
	anchors := map[string]bool{}
	inFence := false
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "```") {
			inFence = !inFence
			continue
		}
		if inFence || !strings.HasPrefix(line, "#") {
			continue
		}
		heading := strings.ToLower(strings.TrimSpace(strings.TrimLeft(line, "#")))
		var slug strings.Builder
		for _, r := range heading {
			switch {
			case r == ' ':
				slug.WriteRune('-')
			case r == '-' || r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r):
				slug.WriteRune(r)
			}
		}
		anchors[slug.String()] = true
	}
	return anchors
}

func TestEveryDocumentationLinkAndAnchorResolves(t *testing.T) {
	root, pages := docPages(t)
	link := regexp.MustCompile(`\[[^\]]+\]\(([^)\s]+)(?:\s+"[^"]*")?\)`)
	checked := 0
	for _, page := range pages {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(page)))
		if err != nil {
			t.Fatal(err)
		}
		text := regexp.MustCompile("(?s)```.*?```").ReplaceAllString(string(data), "")
		for _, match := range link.FindAllStringSubmatch(text, -1) {
			target := strings.Trim(match[1], "<>")
			if strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://") || strings.HasPrefix(target, "mailto:") {
				continue
			}
			file, anchor, _ := strings.Cut(target, "#")
			path := filepath.Join(root, filepath.FromSlash(page))
			if file != "" {
				path = filepath.Join(filepath.Dir(path), filepath.FromSlash(file))
			}
			info, err := os.Stat(path)
			if err != nil {
				t.Errorf("%s links to %q, which does not exist", page, match[1])
				continue
			}
			checked++
			if anchor == "" || info.IsDir() || !strings.HasSuffix(path, ".md") {
				continue
			}
			linked, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if !anchorsOf(string(linked))[anchor] {
				t.Errorf("%s links to %q, but that file has no such heading", page, match[1])
			}
		}
	}
	if checked < 80 {
		t.Fatalf("only %d links were checked: the docs moved or the check lost its way", checked)
	}
}

func TestTheReadmeLinksEveryGuideAndOpensWithTheInstall(t *testing.T) {
	root, _ := docPages(t)
	readme, err := os.ReadFile(filepath.Join(root, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(readme)
	guides, _ := filepath.Glob(filepath.Join(root, "docs", "*.md"))
	for _, guide := range guides {
		name := "docs/" + filepath.Base(guide)
		if !strings.Contains(text, "("+name+")") && !strings.Contains(text, "("+name+"#") {
			t.Errorf("README.md does not link %s: a guide nobody can find is not documentation", name)
		}
	}
	// The order a newcomer needs: what it is, who it is for, what is supported, the install, then the rest.
	last := -1
	for _, heading := range []string{"\n# Kanbanzai\n", "\n## Who it is for\n", "\n## What it is\n", "\n## Supported\n",
		"\n## Install in five minutes\n", "\n## How it works, briefly\n", "\n## The documentation\n", "\n## Compatibility\n",
		"\n## Contributing\n"} {
		at := strings.Index("\n"+text, heading)
		if at < 0 {
			t.Fatalf("README.md lacks the section %q", strings.TrimSpace(heading))
		}
		if at <= last {
			t.Errorf("README.md has %q out of order", strings.TrimSpace(heading))
		}
		last = at
	}
	for _, words := range []string{"Claude Code", "OpenCode", "VS Code", "Linux", "macOS", "Windows", "kanbanzai init", "kanbanzai doctor"} {
		if !strings.Contains(text, words) {
			t.Errorf("README.md lacks %q", words)
		}
	}
	// Checkout-oriented instructions belong to the contributor guide.
	for _, words := range []string{"go build -o bin/kanbanzai", "go test ./...", "go run ./tools/", "node --test"} {
		if strings.Contains(text, words) {
			t.Errorf("README.md carries the contributor command %q: it belongs in docs/contributing.md", words)
		}
	}
	contributing, err := os.ReadFile(filepath.Join(root, "docs", "contributing.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, words := range []string{"go build -o bin/kanbanzai", "go test ./...", "go run ./tools/vsix", "node --test"} {
		if !strings.Contains(string(contributing), words) {
			t.Errorf("docs/contributing.md lacks %q", words)
		}
	}
}

// The host guide says what a host changes and that its own rules and skills stay in the host; the other new guides
// carry the topics the marketplace visitor needs; none of them names a host project (neutral_test checks the words).
func TestTheMarketplaceGuidesSayWhatTheyMustSay(t *testing.T) {
	root, _ := docPages(t)
	for file, words := range map[string][]string{
		"docs/host-integration.md": {"The host's rules stay in the host", ".claude/settings.json", ".opencode/opencode.json", "enabledPlugins",
			"`[project] context`", "config.toml", "init --dry-run", "never edits", "One example, not a requirement", "[steps.build]"},
		"docs/multi-project.md":   {"All projects", "monorepo", "first** workspace folder", "board --tui", "no global registry"},
		"docs/upgrade.md":         {"SHA256SUMS", "kanbanzai install", "config upgrade", "migrate --apply --plan", "config_version"},
		"docs/troubleshooting.md": {"kanbanzai doctor", "Finding id", "KANBANZAI_BIN", "exits 3", "exits 4", "SUPPORT.md"},
		"docs/security.md":        {"no outbound network request", "127.0.0.1", "not signed", "SHA256SUMS", "committed or pushed for you"},
		"docs/contributing.md":    {"What the repository holds", "[types.md](types.md)"},
	} {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(file)))
		if err != nil {
			t.Fatal(err)
		}
		flat := strings.Join(strings.Fields(string(data)), " ")
		for _, w := range words {
			if !strings.Contains(flat, w) {
				t.Errorf("%s lacks %q", file, w)
			}
		}
	}
}

// TASK-75: the extension's README is its public listing on the Visual Studio Marketplace and Open VSX. It is written
// for someone deciding to install the extension: what it is, the install, setup, the board, the commands and settings
// it really has, what it runs and writes; the full behaviour lives in docs/editor.md and the internals in the
// contributor guide. The .vsix carries the file unchanged and the marketplaces resolve no relative link, so every
// link is absolute, and each one into this repository must name a file and heading that exist.
func TestTheExtensionListingIsWrittenForTheMarketplaceReader(t *testing.T) {
	root, err := testutil.RepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, "board-view", "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	flat := strings.Join(strings.Fields(text), " ")

	last := -1
	for _, heading := range []string{"\n# Kanbanzai Board\n", "\n## Install\n", "\n## Set up a project\n", "\n## Open the board\n",
		"\n## What the board shows\n", "\n## Commands and settings\n", "\n## What it executes, reads and writes\n",
		"\n## Requirements\n", "\n## More\n"} {
		at := strings.Index("\n"+text, heading)
		if at < 0 {
			t.Errorf("board-view/README.md lacks the section %q", strings.TrimSpace(heading))
			continue
		}
		if at <= last {
			t.Errorf("board-view/README.md has %q out of order", strings.TrimSpace(heading))
		}
		last = at
	}
	for _, words := range []string{"Claude Code", "OpenCode", "VSCodium", "VS Code", "read-only", "Extensions view",
		"Visual Studio Marketplace", "Open VSX", "nothing else"} {
		if !strings.Contains(flat, words) {
			t.Errorf("board-view/README.md lacks %q", words)
		}
	}

	// The commands and settings it names are the ones package.json contributes, all of them and no others.
	var manifest struct {
		Contributes struct {
			Commands []struct {
				Title string `json:"title"`
			} `json:"commands"`
			Configuration struct {
				Properties map[string]json.RawMessage `json:"properties"`
			} `json:"configuration"`
		} `json:"contributes"`
	}
	pkg, err := os.ReadFile(filepath.Join(root, "board-view", "package.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(pkg, &manifest); err != nil {
		t.Fatal(err)
	}
	titles := map[string]bool{}
	for _, command := range manifest.Contributes.Commands {
		titles[command.Title] = true
		if !strings.Contains(flat, "**"+command.Title+"**") {
			t.Errorf("board-view/README.md does not name the command **%s**", command.Title)
		}
	}
	for key := range manifest.Contributes.Configuration.Properties {
		if !strings.Contains(flat, "`"+key+"`") {
			t.Errorf("board-view/README.md does not name the setting `%s`", key)
		}
	}
	if len(titles) == 0 || len(manifest.Contributes.Configuration.Properties) == 0 {
		t.Fatal("board-view/package.json contributes no commands or settings: the check lost its way")
	}
	for _, m := range regexp.MustCompile(`\*\*(Kanbanzai: [^*]+)\*\*`).FindAllStringSubmatch(flat, -1) {
		if !titles[m[1]] {
			t.Errorf("board-view/README.md names **%s**, which board-view/package.json does not contribute", m[1])
		}
	}
	for _, m := range regexp.MustCompile("`(kanbanzai\\.[A-Za-z]+)`").FindAllStringSubmatch(flat, -1) {
		if _, ok := manifest.Contributes.Configuration.Properties[m[1]]; !ok {
			t.Errorf("board-view/README.md names the setting `%s`, which board-view/package.json does not contribute", m[1])
		}
	}

	// Links: absolute, and the repository's own resolve in this checkout.
	const repo = "https://github.com/cloudbusting/kanbanzai/"
	unfenced := regexp.MustCompile("(?s)```.*?```").ReplaceAllString(text, "")
	links := regexp.MustCompile(`\[[^\]]+\]\(([^)\s]+)(?:\s+"[^"]*")?\)`).FindAllStringSubmatch(unfenced, -1)
	own := 0
	for _, m := range links {
		target := strings.Trim(m[1], "<>")
		if strings.HasPrefix(target, "#") {
			if !anchorsOf(text)[target[1:]] {
				t.Errorf("board-view/README.md links to %q, but it has no such heading", target)
			}
			continue
		}
		if !strings.HasPrefix(target, "https://") && !strings.HasPrefix(target, "mailto:") {
			t.Errorf("board-view/README.md has the relative link %q: the marketplaces do not resolve it", target)
			continue
		}
		if !strings.HasPrefix(target, repo) {
			continue
		}
		rest := strings.TrimPrefix(target, repo)
		var path string
		switch {
		case strings.HasPrefix(rest, "blob/main/"):
			path = strings.TrimPrefix(rest, "blob/main/")
		case strings.HasPrefix(rest, "tree/main/"):
			path = strings.TrimPrefix(rest, "tree/main/")
		case rest == "" || rest == "issues" || rest == "releases" || strings.HasPrefix(rest, "#"):
			continue
		default:
			t.Errorf("board-view/README.md links to %q: link a file as %sblob/main/<path>", target, repo)
			continue
		}
		own++
		file, anchor, _ := strings.Cut(path, "#")
		info, err := os.Stat(filepath.Join(root, filepath.FromSlash(file)))
		if err != nil {
			t.Errorf("board-view/README.md links to %q, but %s does not exist", target, file)
			continue
		}
		if anchor == "" || info.IsDir() {
			continue
		}
		linked, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(file)))
		if err != nil {
			t.Fatal(err)
		}
		if !anchorsOf(string(linked))[anchor] {
			t.Errorf("board-view/README.md links to %q, but %s has no such heading", target, file)
		}
	}
	if own < 5 {
		t.Errorf("board-view/README.md links only %d files of this repository: the full guide, setup, security, troubleshooting and support belong there", own)
	}
	if !strings.Contains(flat, repo+"blob/main/docs/editor.md") {
		t.Error("board-view/README.md does not link the editor guide, docs/editor.md")
	}

	// The internals belong to the editor guide and the contributor guide.
	for _, words := range []string{"node --test", "go run ./tools/", "render.js", "toasts.js", "output.js", "PROGRESS_MS",
		"shared renderer", "## Tests", "## Without an editor"} {
		if strings.Contains(text, words) {
			t.Errorf("board-view/README.md carries %q: it belongs in docs/editor.md or docs/contributing.md", words)
		}
	}
	if poll := regexp.MustCompile(`every \d+ ?(s|seconds)\b`).FindString(flat); poll != "" {
		t.Errorf("board-view/README.md gives the poll interval %q: it belongs in docs/editor.md", poll)
	}
	if n := strings.Count(flat, "board --web"); n > 1 {
		t.Errorf("board-view/README.md mentions board --web %d times: one line pointing to the boards guide is enough", n)
	}
}

// TASK-84: the extension's listing shows a short GIF of the board beside a real Claude Code session. Neither
// marketplace resolves a relative image path against the package, so the image is an https link into the public
// repository; the file it points at is checked in here, is a GIF, and stays small. The extension itself still
// makes no request: the marketplace page loads the image.
func TestTheExtensionListingShowsTheDemoGif(t *testing.T) {
	root, err := testutil.RepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, "board-view", "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	flat := strings.Join(strings.Fields(text), " ")

	const url = "https://raw.githubusercontent.com/cloudbusting/kanbanzai/main/docs/media/board-demo.gif"
	image := regexp.MustCompile(`!\[([^\]]+)\]\(` + regexp.QuoteMeta(url) + `\)`).FindStringSubmatchIndex(text)
	if image == nil {
		t.Fatalf("board-view/README.md does not show the demo GIF as an image linked to %s", url)
	}
	if alt := text[image[2]:image[3]]; len(strings.Fields(alt)) < 12 {
		t.Errorf("the demo GIF's alt text %q is too short to tell the story (triage, breakdown, work moving across the board)", alt)
	}
	if at, install := image[0], strings.Index(text, "\n## Install\n"); install < 0 || at > install {
		t.Error("the demo GIF belongs near the top of the listing, before ## Install")
	}
	for _, words := range []string{"real Claude Code session", "cut and sped up", "open-source web build of VS Code",
		"The extension makes no request; the marketplace page loads the image"} {
		if !strings.Contains(flat, words) {
			t.Errorf("board-view/README.md lacks %q beside the demo GIF", words)
		}
	}

	gif, err := os.ReadFile(filepath.Join(root, "docs", "media", "board-demo.gif"))
	if err != nil {
		t.Fatalf("the demo GIF is not checked in: %v", err)
	}
	if len(gif) < 6 || string(gif[:6]) != "GIF89a" {
		t.Error("docs/media/board-demo.gif is not a GIF89a file")
	}
	if len(gif) > 4<<20 {
		t.Errorf("docs/media/board-demo.gif is %d bytes; the listing's budget is 4 MiB", len(gif))
	}
	if !strings.Contains(flat, "**Network, telemetry:** none") {
		t.Error("board-view/README.md must keep saying **Network, telemetry:** none")
	}
}
