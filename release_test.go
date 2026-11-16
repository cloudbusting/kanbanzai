package kanbanzai

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/cloudbusting/kanbanzai/internal/testutil"
)

// W-01: the licence, the marketplace metadata, the changelog, the support docs and the release pipeline agree, so the
// repository can be published as a Claude Code plugin marketplace and released from a tag.

var releaseSemver = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`)

type pluginManifest struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Author  struct {
		Name string `json:"name"`
	} `json:"author"`
	Homepage   string   `json:"homepage"`
	Repository string   `json:"repository"`
	License    string   `json:"license"`
	Keywords   []string `json:"keywords"`
}

type marketplaceEntry struct {
	Name    string `json:"name"`
	Source  string `json:"source"`
	Version string `json:"version"`
	Author  struct {
		Name string `json:"name"`
	} `json:"author"`
	Homepage   string   `json:"homepage"`
	Repository string   `json:"repository"`
	License    string   `json:"license"`
	Keywords   []string `json:"keywords"`
	Category   string   `json:"category"`
}

type marketplaceManifest struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Owner   struct {
		Name string `json:"name"`
	} `json:"owner"`
	Plugins []marketplaceEntry `json:"plugins"`
}

type extensionManifest struct {
	Version   string            `json:"version"`
	License   string            `json:"license"`
	Publisher string            `json:"publisher"`
	Engines   map[string]string `json:"engines"`
}

func TestTheLicenceAgreesEverywhere(t *testing.T) {
	root := releaseRoot(t)
	licence, err := os.ReadFile(filepath.Join(root, "LICENSE"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(licence), "MIT License") {
		t.Fatalf("LICENSE does not start with the MIT licence: %q", firstLine(string(licence)))
	}
	plugin := readManifest[pluginManifest](t, filepath.Join(root, ".claude-plugin", "plugin.json"))
	extension := readManifest[extensionManifest](t, filepath.Join(root, "board-view", "package.json"))
	marketplace := readManifest[marketplaceManifest](t, filepath.Join(root, ".claude-plugin", "marketplace.json"))
	if plugin.License != "MIT" {
		t.Fatalf("plugin.json license = %q, want MIT", plugin.License)
	}
	if extension.License != "MIT" {
		t.Fatalf("board-view/package.json license = %q, want MIT", extension.License)
	}
	if len(marketplace.Plugins) != 1 {
		t.Fatalf("marketplace.json lists %d plugins, want 1", len(marketplace.Plugins))
	}
	if marketplace.Plugins[0].License != "MIT" {
		t.Fatalf("the marketplace entry's license = %q, want MIT", marketplace.Plugins[0].License)
	}
}

func TestThePluginAndMarketplaceCarryTheReleaseMetadata(t *testing.T) {
	root := releaseRoot(t)
	plugin := readManifest[pluginManifest](t, filepath.Join(root, ".claude-plugin", "plugin.json"))
	marketplace := readManifest[marketplaceManifest](t, filepath.Join(root, ".claude-plugin", "marketplace.json"))
	if plugin.Name != "kanbanzai" {
		t.Fatalf("plugin.json name = %q, want kanbanzai", plugin.Name)
	}
	for field, value := range map[string]string{
		"author":     plugin.Author.Name,
		"homepage":   plugin.Homepage,
		"repository": plugin.Repository,
		"license":    plugin.License,
	} {
		if strings.TrimSpace(value) == "" {
			t.Fatalf("plugin.json has no %s", field)
		}
	}
	if len(plugin.Keywords) == 0 {
		t.Fatal("plugin.json has no keywords")
	}
	if !releaseSemver.MatchString(plugin.Version) {
		t.Fatalf("plugin.json version = %q, want a semver like 0.6.0", plugin.Version)
	}
	if marketplace.Name != "kanbanzai" {
		t.Fatalf("marketplace.json name = %q, want kanbanzai", marketplace.Name)
	}
	if marketplace.Owner.Name == "" {
		t.Fatal("marketplace.json has no owner")
	}
	if !releaseSemver.MatchString(marketplace.Version) {
		t.Fatalf("marketplace.json version = %q, want a semver like 0.6.0", marketplace.Version)
	}
	if len(marketplace.Plugins) != 1 {
		t.Fatalf("marketplace.json lists %d plugins, want 1", len(marketplace.Plugins))
	}
	entry := marketplace.Plugins[0]
	if entry.Name != plugin.Name {
		t.Fatalf("the marketplace entry is %q but plugin.json is %q", entry.Name, plugin.Name)
	}
	if entry.Source != "./" {
		t.Fatalf("the marketplace entry's source = %q, want the repository root", entry.Source)
	}
	for field, value := range map[string]string{
		"author":     entry.Author.Name,
		"homepage":   entry.Homepage,
		"repository": entry.Repository,
		"license":    entry.License,
		"category":   entry.Category,
	} {
		if strings.TrimSpace(value) == "" {
			t.Fatalf("the marketplace entry has no %s", field)
		}
	}
	if len(entry.Keywords) == 0 {
		t.Fatal("the marketplace entry has no keywords")
	}
	if entry.Author.Name != plugin.Author.Name {
		t.Fatalf("the entry's author %q and plugin.json's author %q differ", entry.Author.Name, plugin.Author.Name)
	}
	if entry.Homepage != plugin.Homepage || entry.Repository != plugin.Repository {
		t.Fatal("the entry and plugin.json disagree on homepage or repository")
	}
	if strings.Join(entry.Keywords, " ") != strings.Join(plugin.Keywords, " ") {
		t.Fatal("the entry and plugin.json disagree on keywords")
	}
}

func TestTheVersionsAgreeAndTheChangelogRecordsThem(t *testing.T) {
	root := releaseRoot(t)
	plugin := readManifest[pluginManifest](t, filepath.Join(root, ".claude-plugin", "plugin.json"))
	marketplace := readManifest[marketplaceManifest](t, filepath.Join(root, ".claude-plugin", "marketplace.json"))
	extension := readManifest[extensionManifest](t, filepath.Join(root, "board-view", "package.json"))
	entry := marketplace.Plugins[0]
	for name, version := range map[string]string{
		"plugin.json":             plugin.Version,
		"marketplace.json":        marketplace.Version,
		"the marketplace entry":   entry.Version,
		"board-view/package.json": extension.Version,
	} {
		if version != plugin.Version {
			t.Fatalf("%s version = %q, but plugin.json says %q", name, version, plugin.Version)
		}
	}
	changelog, err := os.ReadFile(filepath.Join(root, "CHANGELOG.md"))
	if err != nil {
		t.Fatal(err)
	}
	heading := regexp.MustCompile(`(?m)^## \[` + regexp.QuoteMeta(plugin.Version) + `\]`)
	if !heading.Match(changelog) {
		t.Fatalf("CHANGELOG.md has no `## [%s]` entry for the version the manifests carry", plugin.Version)
	}
}

func TestTheSupportAndSecurityDocsArePresent(t *testing.T) {
	root := releaseRoot(t)
	for _, name := range []string{"SECURITY.md", "SUPPORT.md"} {
		data, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatalf("%s is missing: %v", name, err)
		}
		if strings.TrimSpace(string(data)) == "" {
			t.Fatalf("%s is empty", name)
		}
	}
	readme, err := os.ReadFile(filepath.Join(root, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, link := range []string{"(SECURITY.md)", "(SUPPORT.md)"} {
		if !strings.Contains(string(readme), link) {
			t.Fatalf("README.md does not link to %s", strings.Trim(link, "()"))
		}
	}
}

// TestEveryRelativeDocumentationLinkResolves proves the docs a marketplace visitor follows do not point at a file
// that is not there. External URLs are somebody else's to keep alive.
func TestEveryRelativeDocumentationLinkResolves(t *testing.T) {
	root := releaseRoot(t)
	link := regexp.MustCompile(`\[[^\]]+\]\(([^)\s]+)(?:\s+"[^"]*")?\)`)
	for _, name := range []string{"README.md", "SECURITY.md", "SUPPORT.md", "CHANGELOG.md"} {
		path := filepath.Join(root, name)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("%s is missing: %v", name, err)
		}
		for _, match := range link.FindAllStringSubmatch(string(data), -1) {
			target := strings.Trim(match[1], "<>")
			if target == "" || strings.HasPrefix(target, "#") ||
				strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://") ||
				strings.HasPrefix(target, "mailto:") {
				continue
			}
			if index := strings.Index(target, "#"); index >= 0 {
				target = target[:index]
			}
			if target == "" {
				continue
			}
			if _, err := os.Stat(filepath.Join(filepath.Dir(path), target)); err != nil {
				t.Fatalf("%s links to %q, which does not exist", name, match[1])
			}
		}
	}
}

func TestTheReadmeStatesTheTestedCompatibility(t *testing.T) {
	root := releaseRoot(t)
	readme, err := os.ReadFile(filepath.Join(root, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(readme)
	if !strings.Contains(text, "## Compatibility") {
		t.Fatal("README.md has no `## Compatibility` section")
	}
	for _, part := range []string{"Claude Code", "OpenCode", "Go 1.24", "VS Code"} {
		if !strings.Contains(text, part) {
			t.Fatalf("the compatibility table does not name %q", part)
		}
	}
	// The Claude Code version the table names is the version CI pins, so the table cannot promise a version the
	// pipeline does not test.
	pin := regexp.MustCompile(`CLAUDE_CODE_VERSION:\s*"([0-9]+\.[0-9]+\.[0-9]+)"`)
	for _, workflow := range []string{".github/workflows/ci.yml", ".github/workflows/release.yml"} {
		data, err := os.ReadFile(filepath.Join(root, workflow))
		if err != nil {
			t.Fatalf("%s is missing: %v", workflow, err)
		}
		match := pin.FindStringSubmatch(string(data))
		if len(match) != 2 {
			t.Fatalf("%s does not pin CLAUDE_CODE_VERSION", workflow)
		}
		if !strings.Contains(text, match[1]) {
			t.Fatalf("README.md's compatibility table does not name Claude Code %s, which %s tests", match[1], workflow)
		}
	}
}

func TestCIRunsTheReleaseChecks(t *testing.T) {
	root := releaseRoot(t)
	ci, err := os.ReadFile(filepath.Join(root, ".github", "workflows", "ci.yml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, command := range []string{
		"go test ./...",
		"node --test board-view/*.test.mjs opencode/*.test.mjs",
		"bash -n hooks/*.sh",
		"claude plugin validate",
		"go run ./tools/release -check",
		"KANBANZAI_PLUGIN_SMOKE",
	} {
		if !strings.Contains(string(ci), command) {
			t.Fatalf("ci.yml does not run %q", command)
		}
	}
	release, err := os.ReadFile(filepath.Join(root, ".github", "workflows", "release.yml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, command := range []string{
		`tags: ["v*"]`,
		`go run ./tools/release -check -version "$GITHUB_REF_NAME"`,
		`go run ./tools/release -version "$GITHUB_REF_NAME"`,
	} {
		if !strings.Contains(string(release), command) {
			t.Fatalf("release.yml does not run %q", command)
		}
	}
}

func releaseRoot(t *testing.T) string {
	t.Helper()
	root, err := testutil.RepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func readManifest[T any](t *testing.T, path string) T {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var value T
	if err := json.Unmarshal(data, &value); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	return value
}

func firstLine(text string) string {
	if index := strings.IndexByte(text, '\n'); index >= 0 {
		return text[:index]
	}
	return text
}

// W-14: the source bundle the package and release skills archive must carry every folder the command embeds
// (assets.go), or a build from the bundle fails. Adding an embedded folder without adding it to both lists is how
// presets/ was nearly left out.
func TestTheSourceBundleListsEveryEmbeddedFolder(t *testing.T) {
	root, err := testutil.RepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	assets, err := os.ReadFile(filepath.Join(root, "assets.go"))
	if err != nil {
		t.Fatal(err)
	}
	var folders []string
	for _, line := range strings.Split(string(assets), "\n") {
		rest, ok := strings.CutPrefix(line, "//go:embed ")
		if !ok {
			continue
		}
		for _, path := range strings.Fields(rest) {
			if top, _, nested := strings.Cut(path, "/"); nested || !strings.Contains(top, ".") {
				folders = append(folders, top)
			}
		}
	}
	if len(folders) == 0 {
		t.Fatal("assets.go embeds nothing")
	}
	for _, skill := range []string{"kanbanzai-package", "kanbanzai-release"} {
		data, err := os.ReadFile(filepath.Join(root, ".claude", "skills", skill, "SKILL.md"))
		if err != nil {
			t.Fatal(err)
		}
		flat := strings.Join(strings.Fields(string(data)), " ")
		at := strings.Index(flat, "git archive --format=tar.gz")
		if at < 0 {
			t.Fatalf("%s has no source bundle command", skill)
		}
		command := flat[at:]
		if end := strings.Index(command, "CHANGELOG.md"); end > 0 {
			command = command[:end]
		}
		for _, folder := range folders {
			if !strings.Contains(" "+command+" ", " "+folder+" ") {
				t.Errorf("%s: the source bundle command lacks %s, which assets.go embeds", skill, folder)
			}
		}
	}
}

// TASK-72: the tag that releases the editor extension also publishes the .vsix the workflow built, to the Visual
// Studio Marketplace and to Open VSX, without a token in the repository and without building it again.

// workflowJob returns the text of one top-level job of a workflow file: from its `  name:` line to the next job.
func workflowJob(t *testing.T, workflow, name string) string {
	t.Helper()
	jobs := workflow[strings.Index(workflow, "\njobs:\n")+1:]
	start := regexp.MustCompile(`(?m)^  ` + regexp.QuoteMeta(name) + `:\s*$`).FindStringIndex(jobs)
	if start == nil {
		t.Fatalf("release.yml has no %q job", name)
	}
	rest := jobs[start[1]:]
	if next := regexp.MustCompile(`(?m)^  [A-Za-z0-9_-]+:\s*$`).FindStringIndex(rest); next != nil {
		rest = rest[:next[0]]
	}
	return rest
}

func TestReleaseWorkflowPublishesTheVsixToBothMarketplaces(t *testing.T) {
	root := releaseRoot(t)
	data, err := os.ReadFile(filepath.Join(root, ".github", "workflows", "release.yml"))
	if err != nil {
		t.Fatal(err)
	}
	workflow := string(data)

	// The .vsix is unpacked and scanned in the build job, before the artifact that the publish jobs use is uploaded.
	build := workflowJob(t, workflow, "build")
	scan := strings.Index(build, `go run ./tools/release -scan "$RUNNER_TEMP/vsix`)
	upload := strings.Index(build, "actions/upload-artifact")
	if scan < 0 || !strings.Contains(build, "unzip") {
		t.Fatal("the build job does not unpack the .vsix and run `go run ./tools/release -scan` on it")
	}
	if upload < 0 || scan > upload {
		t.Fatal("the .vsix is scanned after the artifact is uploaded; scan it first")
	}

	jobs := map[string]string{
		"open-vsx":       workflowJob(t, workflow, "open-vsx"),
		"vs-marketplace": workflowJob(t, workflow, "vs-marketplace"),
	}
	for name, job := range jobs {
		// They run after the build and after the GitHub release, so a marketplace failure never hides the release.
		if !regexp.MustCompile(`needs:\s*\[[^\]]*\bbuild\b[^\]]*\bpublish\b[^\]]*\]`).MatchString(job) {
			t.Fatalf("%s does not need the build and publish jobs", name)
		}
		// They upload the .vsix the build made; they never build one.
		for _, forbidden := range []string{"vsce package", "tools/vsix", "go run", "npm run", "npm ci", "npm install"} {
			if strings.Contains(job, forbidden) {
				t.Fatalf("%s runs %q: it must publish the built .vsix, not build one", name, forbidden)
			}
		}
		if !strings.Contains(job, "name: binaries") || !strings.Contains(job, "dist/*.vsix") {
			t.Fatalf("%s does not take the .vsix from the build's artifact", name)
		}
	}

	// The tools are pinned to an exact version, never latest.
	if !regexp.MustCompile(`@vscode/vsce@[0-9]+\.[0-9]+\.[0-9]+\b`).MatchString(jobs["vs-marketplace"]) {
		t.Fatal("vs-marketplace does not pin @vscode/vsce to an exact version")
	}
	if !regexp.MustCompile(`\bovsx@[0-9]+\.[0-9]+\.[0-9]+\b`).MatchString(jobs["open-vsx"]) {
		t.Fatal("open-vsx does not pin ovsx to an exact version")
	}
	if strings.Contains(workflow, "@latest") {
		t.Fatal("release.yml uses @latest; pin the version")
	}

	// Visual Studio Marketplace: trusted publishing (an OIDC token, no stored secret), and nothing broader.
	vs := jobs["vs-marketplace"]
	if !strings.Contains(vs, "publish --oidc") || !strings.Contains(vs, "--packagePath") {
		t.Fatal("vs-marketplace does not run `vsce publish --oidc --packagePath`")
	}
	if !strings.Contains(vs, "id-token: write") || !strings.Contains(vs, "contents: read") {
		t.Fatal("vs-marketplace does not ask for exactly `id-token: write` and `contents: read`")
	}
	if strings.Contains(vs, "contents: write") || strings.Contains(vs, "secrets.") || strings.Contains(vs, "VSCE_PAT") {
		t.Fatal("vs-marketplace has a write permission or a stored token; trusted publishing needs neither")
	}
	if !strings.Contains(vs, "github.repository == 'cloudbusting/kanbanzai'") {
		t.Fatal("vs-marketplace is not limited to the cloudbusting/kanbanzai repository, so a fork's tag would try to publish")
	}

	// Open VSX: the token is a repository secret, and a missing one skips with a notice instead of failing.
	ovsx := jobs["open-vsx"]
	for _, part := range []string{
		"OVSX_PAT: ${{ secrets.OVSX_PAT }}",
		"if: env.OVSX_PAT == ''",
		"::notice",
		"if: env.OVSX_PAT != ''",
		`publish "$VSIX" -p "$OVSX_PAT"`,
		"--skip-duplicate",
		"contents: read",
	} {
		if !strings.Contains(ovsx, part) {
			t.Fatalf("open-vsx does not contain %q", part)
		}
	}

	// The workflow names secrets but holds none.
	for _, line := range strings.Split(workflow, "\n") {
		if regexp.MustCompile(`(?i)(pat|token|password|secret)\s*[:=]\s*["']?[A-Za-z0-9+/_-]{20,}`).MatchString(line) {
			t.Fatalf("release.yml looks as if it holds a credential: %q", line)
		}
	}
}

func TestReleaseSkillDescribesTheMarketplaceRoute(t *testing.T) {
	root := releaseRoot(t)
	data, err := os.ReadFile(filepath.Join(root, ".claude", "skills", "kanbanzai-release", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	flat := strings.Join(strings.Fields(string(data)), " ")
	for _, part := range []string{
		"The tag push publishes it",
		"`OVSX_PAT`",
		"`vsce publish --oidc`",
		"2026-12-01",
		"never read a secret's value",
		"re-run only the failed one",
		"never move the tag",
		"run `vsce` or `ovsx` by hand over a version the workflow publishes",
	} {
		if !strings.Contains(flat, part) {
			t.Errorf("the release skill does not say %q", part)
		}
	}
	for _, stale := range []string{`npx @vscode/vsce publish --packagePath "$OUT`, "and the owner's publishing token is in the environment"} {
		if strings.Contains(flat, stale) {
			t.Errorf("the release skill still has the by-hand route: %q", stale)
		}
	}
}
