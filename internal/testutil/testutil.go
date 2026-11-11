// Package testutil gives the Go tests the same throwaway git projects and environment the Python suite used.
//
// It also builds the command once into bin/kanbanzai-go (ignored by git), so a test can prove the real process's
// exit codes and output rather than only calling a package function.
package testutil

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

// Result is one command run.
type Result struct {
	Code   int
	Stdout string
	Stderr string
}

var (
	repoRootOnce sync.Once
	repoRootPath string
	repoRootErr  error

	buildOnce sync.Once
	buildPath string
	buildErr  error
)

// RepoRoot is the module's root: <root>/internal/testutil -> <root>.
func RepoRoot() (string, error) {
	repoRootOnce.Do(func() {
		_, file, _, ok := runtime.Caller(0)
		if !ok {
			repoRootErr = fmt.Errorf("cannot locate the test source")
			return
		}
		// <root>/internal/testutil/testutil.go
		repoRootPath = filepath.Dir(filepath.Dir(filepath.Dir(file)))
	})
	return repoRootPath, repoRootErr
}

// Binary builds cmd/kanbanzai once and returns the path to it.
func Binary(t *testing.T) string {
	t.Helper()
	buildOnce.Do(func() {
		root, err := RepoRoot()
		if err != nil {
			buildErr = err
			return
		}
		if err := os.MkdirAll(filepath.Join(root, "bin"), 0o755); err != nil {
			buildErr = err
			return
		}
		name := "kanbanzai-go"
		if runtime.GOOS == "windows" {
			name += ".exe"
		}
		out := filepath.Join(root, "bin", name)
		cmd := exec.Command("go", "build", "-trimpath", "-o", out, "./cmd/kanbanzai")
		cmd.Dir = root
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			buildErr = fmt.Errorf("go build ./cmd/kanbanzai: %w: %s", err, stderr.String())
			return
		}
		buildPath = out
	})
	if buildErr != nil {
		t.Fatal(buildErr)
	}
	return buildPath
}

// Run runs the built command in dir and returns its exit code, stdout and stderr.
//
// env overrides or adds variables; an empty value is left for the command to treat as unset. The process's own
// environment is the base, as a session would see it.
func Run(t *testing.T, dir string, env map[string]string, args ...string) Result {
	t.Helper()
	bin := Binary(t)
	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	cmd.Env = os.Environ()
	for key, value := range env {
		cmd.Env = setEnv(cmd.Env, key, value)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	code := 0
	if err != nil {
		if exit, ok := err.(*exec.ExitError); ok {
			code = exit.ExitCode()
		} else {
			t.Fatalf("running %s %v: %v", bin, args, err)
		}
	}
	return Result{Code: code, Stdout: stdout.String(), Stderr: stderr.String()}
}

// setEnv replaces a key in an environment slice, or appends it.
func setEnv(env []string, key, value string) []string {
	prefix := key + "="
	out := env[:0]
	for _, item := range env {
		if !strings.HasPrefix(item, prefix) {
			out = append(out, item)
		}
	}
	return append(out, prefix+value)
}

// Git runs git in dir and fails the test when it fails.
func Git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = gitEnv()
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, stderr.String())
	}
	return strings.TrimSpace(stdout.String())
}

func gitEnv() []string {
	env := os.Environ()
	for key, value := range map[string]string{
		"GIT_AUTHOR_NAME":     "Test",
		"GIT_AUTHOR_EMAIL":    "test@example.com",
		"GIT_COMMITTER_NAME":  "Test",
		"GIT_COMMITTER_EMAIL": "test@example.com",
		"GIT_CONFIG_GLOBAL":   os.DevNull,
		"GIT_CONFIG_NOSYSTEM": "1",
	} {
		env = setEnv(env, key, value)
	}
	return env
}

// Repo makes a git checkout on main with a first commit and the project's config file.
func Repo(t *testing.T, config string) string {
	t.Helper()
	dir := t.TempDir()
	cmd := exec.Command("git", "init", "-q", "-b", "main", dir)
	cmd.Env = gitEnv()
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("git init: %v: %s", err, stderr.String())
	}
	Write(t, filepath.Join(dir, "README.md"), "# A project\n")
	if config != "" {
		if !strings.Contains(config, "config_version") {
			config = "config_version = 1\n" + config
		}
		Write(t, filepath.Join(dir, ".kanbanzai", "config.toml"), config)
		Write(t, filepath.Join(dir, ".kanbanzai", ".gitignore"), "state/\n")
	}
	Git(t, dir, "add", "-A")
	Git(t, dir, "commit", "-q", "-m", "start")
	return dir
}

// Folders makes the default work folders.
func Folders(t *testing.T, root string, names ...string) {
	t.Helper()
	for _, name := range names {
		if err := os.MkdirAll(filepath.Join(root, ".kanbanzai", "work", name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
}

// Write writes a file, making its parents.
func Write(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

// Brief is a minimal valid card brief.
func Brief(card, title, stage string, extra ...string) string {
	lines := []string{
		fmt.Sprintf("# %s: %s", card, title), "",
		"- **Stage:** " + stage,
		"- **Updated:** 2026-11-09",
		"- **Next action:** Do it",
		"- **Where:** —",
	}
	lines = append(lines, extra...)
	lines = append(lines, "## Goal", "", "Body.", "", "## Comments", "")
	return strings.Join(lines, "\n")
}
