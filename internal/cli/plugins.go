package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/cloudbusting/kanbanzai/internal/config"
	"github.com/cloudbusting/kanbanzai/internal/pluginlink"
)

// runPlugins is `kanbanzai plugins link [DIR]`: give a card's worktree the bundled OpenCode plugin's pointer file
// (BUG-79). DIR is any folder in the worktree; it defaults to the current one. Exit 0 when it linked, kept or had
// nothing to do, 1 when it refused, 2 for bad arguments.
func runPlugins(project *config.Project, args []string, out, errOut io.Writer) int {
	positionals, _, _, err := parse(args, nil, nil)
	if err != nil || len(positionals) == 0 || positionals[0] != "link" || len(positionals) > 2 {
		return usageError(errOut, "plugins", fmt.Errorf("usage: plugins link [DIR]"))
	}
	dir := ""
	if len(positionals) == 2 {
		dir = positionals[1]
	} else if dir, err = os.Getwd(); err != nil {
		fmt.Fprintf(errOut, "kanbanzai: plugins link: %v\n", err)
		return 1
	}
	if dir, err = filepath.Abs(dir); err != nil {
		fmt.Fprintf(errOut, "kanbanzai: plugins link: %v\n", err)
		return 1
	}
	result, err := pluginlink.Link(project.Root, dir)
	if err != nil {
		fmt.Fprintf(errOut, "kanbanzai: plugins link: %v\n", err)
		return 1
	}
	switch result.Op {
	case pluginlink.OpCreated:
		fmt.Fprintf(out, "created %s\n", result.Path)
	case pluginlink.OpKept:
		fmt.Fprintf(out, "kept %s (already linked)\n", result.Path)
	default:
		fmt.Fprintf(out, "nothing to link: %s\n", result.Reason)
	}
	return 0
}
