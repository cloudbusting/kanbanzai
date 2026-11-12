package initcmd

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/cloudbusting/kanbanzai/internal/testutil"
)

func TestChooseMainBranch(t *testing.T) {
	git := func(branch string, branches ...string) *Detected {
		return &Detected{InGit: true, Branch: branch, Branches: branches}
	}
	cases := []struct {
		name string
		d    *Detected
		flag string
		want string
		fail string
	}{
		{"main only", git("main", "main"), "", "main", ""},
		{"master only", git("master", "master"), "", "master", ""},
		{"trunk beside a feature branch", git("feature", "feature", "trunk"), "", "trunk", ""},
		{"origin/HEAD wins", &Detected{InGit: true, OriginHead: "trunk", Branches: []string{"main", "trunk"}}, "", "trunk", ""},
		{"origin/HEAD that is not local is ignored", &Detected{InGit: true, OriginHead: "gone", Branch: "main",
			Branches: []string{"main"}}, "", "main", ""},
		{"unborn branch", git("primary"), "", "primary", ""},
		{"a lone odd branch", git("develop", "develop"), "", "develop", ""},
		{"main and master", git("main", "main", "master"), "", "", "more than one candidate"},
		{"odd branches, one checked out", git("feature", "develop", "feature"), "", "", "pass --main-branch"},
		{"detached", &Detected{InGit: true, Detached: true, Branches: []string{"develop"}}, "", "", "detached HEAD"},
		{"the flag wins", git("main", "main", "master"), "master", "master", ""},
		{"a bad flag", git("main", "main"), "bad name", "", "not a valid branch name"},
		{"not git", &Detected{}, "", "main", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if c.d.InGit {
				c.d.Root = repo(t, "main") // a real checkout for git check-ref-format
			}
			got, _, err := chooseMainBranch(c.d, c.flag)
			if c.fail != "" {
				if err == nil || !strings.Contains(err.Error(), c.fail) {
					t.Fatalf("err = %v, want %q", err, c.fail)
				}
				return
			}
			if err != nil || got != c.want {
				t.Fatalf("got %q, %v; want %q", got, err, c.want)
			}
		})
	}
}

func TestDetectReadsTheCheckout(t *testing.T) {
	root := repo(t, "trunk")
	testutil.Git(t, root, "remote", "add", "origin", "https://example.invalid/x.git")
	testutil.Git(t, root, "config", "user.name", "Grace")
	testutil.Write(t, filepath.Join(root, "AGENTS.md"), "x")
	testutil.Write(t, filepath.Join(root, ".gitignore"), "bin/\n")
	testutil.Write(t, filepath.Join(root, ".kanbanzai", "work", "1-open", "W-01-a.md"), testutil.Brief("W-01", "A", "todo"))
	testutil.Write(t, filepath.Join(root, ".kanbanzai", "config.toml"), "config_version = 1\n[project]\nmain_branch = \"trunk\"\n")

	d, existing, err := Detect(root)
	if err != nil {
		t.Fatal(err)
	}
	if !d.InGit || d.Branch != "trunk" || d.Detached || d.RanFromWorktree || d.GitUser != "Grace" ||
		len(d.Remotes) != 1 || d.Remotes[0] != "origin" || d.Cards != 1 || !d.RootGitignore ||
		len(d.SuggestedContext) != 2 || !d.Config.Exists || !d.Config.Valid || d.Config.MainBranch != "trunk" ||
		existing == nil {
		t.Fatalf("detected = %+v", d)
	}
}

func TestDetectOutsideAProjectFails(t *testing.T) {
	isolate(t)
	if _, _, err := Detect(t.TempDir()); err == nil {
		t.Fatal("expected a no-project error")
	}
}

func TestDetectReportsABrokenConfig(t *testing.T) {
	root := repo(t, "main")
	testutil.Write(t, filepath.Join(root, ".kanbanzai", "config.toml"), "[project\n")
	d, existing, err := Detect(root)
	if err != nil || existing != nil || !d.Config.Exists || d.Config.Valid || d.Config.Error == "" {
		t.Fatalf("detected = %+v, %v, %v", d, existing, err)
	}
}
