package cli

import (
	"fmt"
	"io"

	"github.com/cloudbusting/kanbanzai/internal/config"
	"github.com/cloudbusting/kanbanzai/internal/tree"
)

// runTree is `kanbanzai tree [--merge ID] [--json]`: read-only, exit 0 when a session may go ahead, 1 when it may
// not, 2 for a usage error.
func runTree(project *config.Project, args []string, out, errOut io.Writer) int {
	positionals, flags, set, err := parse(args, map[string]bool{"json": true}, map[string]bool{"merge": true})
	if err != nil || len(positionals) != 0 {
		return usageError(errOut, "tree", err)
	}
	report := tree.Classify(project)
	if _, merging := flags["merge"]; merging {
		if report, err = tree.Preflight(project, flags["merge"]); err != nil {
			return usageError(errOut, "tree", err)
		}
	}
	if set["json"] {
		writeCompact(out, report)
	} else {
		fmt.Fprint(out, tree.Text(report))
	}
	if !report.OK() {
		return 1
	}
	return 0
}
