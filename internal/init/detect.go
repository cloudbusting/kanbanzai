package initcmd

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/cloudbusting/kanbanzai/internal/config"
	"github.com/cloudbusting/kanbanzai/internal/gitutil"
)

// ConfigState is what init found at .kanbanzai/config.toml.
type ConfigState struct {
	Exists     bool   `json:"exists"`
	Regular    bool   `json:"regular"`
	Valid      bool   `json:"valid"`
	Error      string `json:"error,omitempty"`
	MainBranch string `json:"main_branch,omitempty"`
}

// Detected is everything init learns about the checkout before it plans anything. Detecting writes nothing.
type Detected struct {
	Root             string      `json:"root"`
	Toplevel         string      `json:"toplevel"`
	GitCommon        string      `json:"git_common,omitempty"`
	InGit            bool        `json:"in_git"`
	RanFromWorktree  bool        `json:"ran_from_worktree"`
	Branch           string      `json:"branch,omitempty"`
	Detached         bool        `json:"detached"`
	Branches         []string    `json:"branches"`
	OriginHead       string      `json:"origin_head,omitempty"`
	Remotes          []string    `json:"remotes"`
	GitUser          string      `json:"git_user,omitempty"`
	Config           ConfigState `json:"config"`
	Cards            int         `json:"cards"`
	RootGitignore    bool        `json:"root_gitignore"`
	SuggestedContext []string    `json:"suggested_context"`
}

// hostRuleFiles are files a host project may already keep its rules in. Init only lists them.
var hostRuleFiles = []string{"AGENTS.md", "CLAUDE.md", "CONTRIBUTING.md", "README.md"}

var cardFileName = regexp.MustCompile(`^[A-Za-z]+-\d+-.+\.md$`)

// Detect inspects the checkout that start belongs to. The project it returns is the existing configuration when
// that loads, and nil otherwise.
func Detect(start string) (*Detected, *config.Project, error) {
	root, top, common, err := config.Locate(start)
	if err != nil {
		return nil, nil, err
	}
	d := &Detected{Root: root, Toplevel: top, GitCommon: common, Branches: []string{}, Remotes: []string{},
		SuggestedContext: []string{}}
	if _, err := gitutil.Git(root, "rev-parse", "--git-dir"); err == nil {
		d.InGit = true
	}
	d.RanFromWorktree = d.InGit && !samePath(top, root)
	if d.InGit {
		if branch, err := gitutil.Git(root, "symbolic-ref", "-q", "--short", "HEAD"); err == nil && branch != "" {
			d.Branch = branch
		} else {
			d.Detached = true
		}
		if out, err := gitutil.Git(root, "for-each-ref", "--format=%(refname:short)", "refs/heads"); err == nil {
			d.Branches = lines(out)
		}
		if out, err := gitutil.Git(root, "symbolic-ref", "-q", "--short", "refs/remotes/origin/HEAD"); err == nil {
			d.OriginHead = strings.TrimPrefix(out, "origin/")
		}
		if out, err := gitutil.Git(root, "remote"); err == nil {
			d.Remotes = lines(out)
		}
		if out, err := gitutil.Git(root, "config", "user.name"); err == nil {
			d.GitUser = out
		}
	}
	sort.Strings(d.Branches)
	sort.Strings(d.Remotes)

	var existing *config.Project
	path := config.BootstrapPath(root)
	if info, err := os.Lstat(path); err == nil {
		d.Config.Exists = true
		d.Config.Regular = info.Mode().IsRegular()
		if !d.Config.Regular {
			d.Config.Error = path + ": config must be a regular file"
		} else if project, err := config.New(root, top, common, nil); err != nil {
			d.Config.Error = err.Error()
		} else {
			d.Config.Valid = true
			d.Config.MainBranch = project.MainBranch
			existing = project
		}
	}

	work := filepath.Join(root, ".kanbanzai", "work")
	if existing != nil {
		work = existing.Work
	}
	d.Cards = countCards(work)
	if _, err := os.Stat(filepath.Join(root, ".gitignore")); err == nil {
		d.RootGitignore = true
	}
	for _, name := range hostRuleFiles {
		if info, err := os.Stat(filepath.Join(root, name)); err == nil && info.Mode().IsRegular() {
			d.SuggestedContext = append(d.SuggestedContext, name)
		}
	}
	return d, existing, nil
}

// chooseMainBranch decides the main branch for a config init is about to write, and says how it knew. It never
// guesses: an answer it cannot support is an error that asks for --main-branch.
func chooseMainBranch(d *Detected, flag string) (branch, source string, err error) {
	if flag != "" {
		if strings.TrimSpace(flag) != flag || strings.ContainsAny(flag, " \t\n\\\"") || !validBranchName(d, flag) {
			return "", "", fmt.Errorf("--main-branch '%s' is not a valid branch name", flag)
		}
		return flag, "--main-branch", nil
	}
	if !d.InGit {
		return "main", "default (not a git checkout)", nil
	}
	if d.OriginHead != "" && contains(d.Branches, d.OriginHead) {
		return d.OriginHead, "origin/HEAD", nil
	}
	standard := []string{}
	for _, name := range []string{"main", "master", "trunk"} {
		if contains(d.Branches, name) {
			standard = append(standard, name)
		}
	}
	if len(standard) == 1 {
		return standard[0], "the only one of main, master and trunk here", nil
	}
	if d.Branch != "" && (len(d.Branches) == 0 || (len(d.Branches) == 1 && d.Branches[0] == d.Branch)) {
		return d.Branch, "the checked-out branch (the only one)", nil
	}
	switch {
	case d.Detached:
		return "", "", fmt.Errorf("the main checkout has a detached HEAD, so the main branch cannot be told "+
			"(branches: %s): pass --main-branch NAME", listOrNone(d.Branches))
	case len(standard) > 1:
		return "", "", fmt.Errorf("more than one candidate main branch (%s): pass --main-branch NAME",
			strings.Join(standard, ", "))
	}
	return "", "", fmt.Errorf("cannot tell which branch is the main branch (checked out: %s; branches: %s): "+
		"pass --main-branch NAME", orNone(d.Branch), listOrNone(d.Branches))
}

func validBranchName(d *Detected, name string) bool {
	if !d.InGit {
		return name != "" && !strings.HasPrefix(name, "-")
	}
	_, err := gitutil.Git(d.Root, "check-ref-format", "--branch", name)
	return err == nil && !strings.HasPrefix(name, "-")
}

func countCards(work string) int {
	count := 0
	_ = filepath.WalkDir(work, func(path string, entry os.DirEntry, err error) error {
		if err == nil && !entry.IsDir() && cardFileName.MatchString(entry.Name()) {
			count++
		}
		return nil
	})
	return count
}

func lines(text string) []string {
	out := []string{}
	for _, line := range strings.Split(text, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			out = append(out, line)
		}
	}
	return out
}

func contains(list []string, item string) bool {
	for _, entry := range list {
		if entry == item {
			return true
		}
	}
	return false
}

func orNone(text string) string {
	if text == "" {
		return "none"
	}
	return text
}

func listOrNone(list []string) string {
	if len(list) == 0 {
		return "none"
	}
	return strings.Join(list, ", ")
}

func samePath(a, b string) bool {
	if a == b {
		return true
	}
	ra, errA := filepath.EvalSymlinks(a)
	rb, errB := filepath.EvalSymlinks(b)
	return errA == nil && errB == nil && ra == rb
}
