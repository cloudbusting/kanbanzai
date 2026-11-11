// Package initcmd sets a project up for Kanbanzai.
//
// It makes, and never overwrites, a file that already exists: .kanbanzai/config.toml with only the owner set, the
// work folders with a .gitkeep each, and .kanbanzai/.gitignore holding state/. Running it again changes nothing.
package initcmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/cloudbusting/kanbanzai/internal/config"
)

// ConfigText is the short config a new project starts from; the shipped defaults fill the rest.
const ConfigText = `# Kanbanzai config for this project. Only what differs from Kanbanzai's shipped defaults belongs here:
# the columns, the cap of five started cards, the two-hour lease and the pull order are the defaults.
# ` + "`kanbanzai config`" + ` prints the settings in force. Tables merge key by key; a list (such as [[columns]])
# replaces the default one whole.

[project]
owner = "the user"      # the person the skills address and whose decisions they ask for

# More you can set (see Kanbanzai's defaults/config.toml for every key):
#   [project]  prefix, work, claims, numbers, worktree, context = ["rules file", ...]
#   [flow]     wip_limit, pull_order
#   [steps.check]  skill = "your-skill", extend = ["your-extension"], checks = ["your-test-skill"]
`

// GitignoreText keeps the leases and numbers out of git.
const GitignoreText = "state/\n"

// Run makes the project's home, config and folders.
func Run(project *config.Project, out io.Writer) int {
	if out == nil {
		out = io.Discard
	}
	made := []string{}
	kept := []string{}
	writeNew(filepath.Join(project.Home, "config.toml"), ConfigText, &made, &kept, project)
	writeNew(filepath.Join(project.Home, ".gitignore"), GitignoreText, &made, &kept, project)
	for _, folder := range project.Folders {
		writeNew(filepath.Join(project.Work, folder, ".gitkeep"), "", &made, &kept, project)
	}
	fmt.Fprintf(out, "kanbanzai: project %s\n", project.Root)
	for _, name := range made {
		fmt.Fprintf(out, "  made     %s\n", name)
	}
	for _, name := range kept {
		fmt.Fprintf(out, "  kept     %s\n", name)
	}
	if len(made) == 0 {
		fmt.Fprintln(out, "  nothing to do: already set up")
	}
	return 0
}

func writeNew(path, text string, made, kept *[]string, project *config.Project) {
	if _, err := os.Stat(path); err == nil {
		*kept = append(*kept, project.Rel(path))
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		return
	}
	*made = append(*made, project.Rel(path))
}
