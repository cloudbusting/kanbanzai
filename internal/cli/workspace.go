package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/cloudbusting/kanbanzai/internal/config"
	"github.com/cloudbusting/kanbanzai/internal/workspace"
)

// runWorkspace is `kanbanzai workspace ID [--json]`: what the project's isolation mode asks of the card, and the
// commands for it. The skills quote it instead of carrying git commands of their own.
func runWorkspace(project *config.Project, args []string, out, errOut io.Writer) int {
	positionals, _, set, err := parse(args, map[string]bool{"json": true}, nil)
	if err != nil || len(positionals) != 1 {
		return usageError(errOut, "workspace", err)
	}
	card, ok := project.CanonicalCardID(positionals[0])
	if !ok {
		fmt.Fprintf(errOut, "kanbanzai: workspace: card must look like %s: '%s'\n", project.IDsExample(),
			positionals[0])
		return 2
	}
	info := workspace.Describe(project, card)
	if set["json"] {
		writeCompact(out, info)
		return 0
	}
	fmt.Fprint(out, workspaceText(project, info))
	return 0
}

func workspaceText(project *config.Project, info workspace.Info) string {
	var b strings.Builder
	yes := map[bool]string{true: "yes", false: "no"}
	fmt.Fprintf(&b, "%s: isolation: %s (%s)\n", info.Card, info.Setting, info.Mode)
	fmt.Fprintf(&b, "needs a workspace: %s\n", yes[info.NeedsWorkspace])
	fmt.Fprintf(&b, "record: Where: %s   (an open card may carry: %s)\n", info.Record, strings.Join(info.WhereVocabulary, ", "))
	if len(info.Make) == 0 {
		b.WriteString("make: nothing")
		if info.Setting == workspace.External {
			b.WriteString(" (the host makes the workspace; record its reference)")
		}
		b.WriteString("\n")
	} else {
		b.WriteString("make (a new card):\n  " + info.Make[0] + "\nmake (its branch exists):\n  " + info.Make[1] + "\n")
	}
	fmt.Fprintf(&b, "then: %s\n", info.Then)
	fmt.Fprintf(&b, "work in: %s\n", info.WorkIn)
	if len(info.Remove) == 0 {
		b.WriteString("remove: nothing\n")
	} else {
		b.WriteString("remove (once merged and the card is closed):\n")
		for _, step := range info.Remove {
			b.WriteString("  " + step + "\n")
		}
	}
	if info.Parallel {
		b.WriteString("parallel: yes\n")
	} else {
		b.WriteString("parallel: no — one checkout; parallel work needs worktree mode\n")
	}
	return b.String()
}
