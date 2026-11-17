package cli

import (
	"io"

	kanbanzai "github.com/cloudbusting/kanbanzai"
	"github.com/cloudbusting/kanbanzai/internal/guide"
)

const guideUsage = "usage: kanbanzai guide [--json]\nprint the cheat sheet of Kanbanzai's commands and common workflows (the help skill's sheet); --json prints it parsed for the editor's Help popup. Works outside a project.\n"

// runGuide prints the help skill's cheat sheet from the embedded copy. It reads no project: the sheet is the same
// text in every checkout of a release.
func runGuide(args []string, out, errOut io.Writer) int {
	positionals, _, set, err := parse(args, map[string]bool{"json": true}, nil)
	if err != nil || len(positionals) != 0 {
		return usageError(errOut, "guide", err)
	}
	return guide.Run(kanbanzai.Files, set["json"], out, errOut)
}
