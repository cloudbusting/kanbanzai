// The kanbanzai command.
//
// The command surface is wired in internal/cli; this entry point only keeps the process boundary small.
package main

import (
	"fmt"
	"os"

	"github.com/cloudbusting/kanbanzai/internal/version"
)

func main() {
	args := os.Args[1:]
	if len(args) == 1 && (args[0] == "--version" || args[0] == "-version") {
		fmt.Println(version.String())
		return
	}
	// The full dispatch lands with the CLI task; until then, fail loudly rather than pretend.
	fmt.Fprintln(os.Stderr, "kanbanzai: this build has no command dispatch yet")
	os.Exit(1)
}
