// The kanbanzai-store-github program: the GitHub issues store plugin. `kanbanzai` runs it (after `kanbanzai store
// trust`) with one JSON request on standard input, and it answers with one JSON reply on standard output. It reaches
// GitHub only by running the user's own `gh`, so it holds no token and has no HTTP client.
package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"

	"github.com/cloudbusting/kanbanzai/internal/store/protocol"
	"github.com/cloudbusting/kanbanzai/internal/storegithub"
	"github.com/cloudbusting/kanbanzai/internal/version"
)

// timeout is how long one `gh` call may take: inside the 60 seconds kanbanzai gives a whole request.
const timeout = 50 * time.Second

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, in io.Reader, out, errOut io.Writer) int {
	if len(args) > 0 {
		switch args[0] {
		case "--version", "version":
			fmt.Fprintln(out, "kanbanzai-store-github", version.Effective())
			return 0
		default:
			fmt.Fprintln(errOut, "kanbanzai-store-github is run by `kanbanzai` (see `kanbanzai store trust`); it takes no arguments")
			return 2
		}
	}
	open := func(settings map[string]string) (protocol.Backend, error) {
		return storegithub.Open(settings, ghRunner{})
	}
	if err := protocol.Serve(in, out, open); err != nil {
		fmt.Fprintln(errOut, err)
		return 1
	}
	return 0
}

// ghRunner runs `gh` with an argument array, never a shell.
type ghRunner struct{}

func (ghRunner) Run(args []string, stdin []byte) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	command := exec.CommandContext(ctx, "gh", args...)
	// gh must never wait for a person: no prompts, and no update notice in what it writes.
	command.Env = append(os.Environ(), "GH_PROMPT_DISABLED=1", "GH_NO_UPDATE_NOTIFIER=1", "NO_COLOR=1")
	if stdin != nil {
		command.Stdin = bytes.NewReader(stdin)
	}
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	if err := command.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, &storegithub.RunError{Stderr: "gh did not answer within " + timeout.String(), Err: err}
		}
		return nil, &storegithub.RunError{Stderr: stderr.String(), Err: err}
	}
	return stdout.Bytes(), nil
}
