package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cloudbusting/kanbanzai/internal/testutil"
)

// W-01: a fresh project installs the plugin from a packaged artifact, with no path back to the checkout. The test
// needs the claude CLI, so it runs only when KANBANZAI_PLUGIN_SMOKE=1 (CI's plugin job) and otherwise skips.
func TestThePackagedPluginInstallsInAFreshProject(t *testing.T) {
	if os.Getenv("KANBANZAI_PLUGIN_SMOKE") == "" {
		t.Skip("set KANBANZAI_PLUGIN_SMOKE=1 to install the packaged plugin with the claude CLI")
	}
	claude, err := exec.LookPath("claude")
	if err != nil {
		t.Skip("the claude CLI is not installed")
	}
	root, err := testutil.RepoRoot()
	if err != nil {
		t.Fatal(err)
	}
	base := t.TempDir()
	archive := filepath.Join(base, "kanbanzai-plugin.tar.gz")
	artifact := filepath.Join(base, "artifact")
	if err := packagePlugin(root, archive); err != nil {
		t.Fatalf("packaging the plugin: %v", err)
	}
	if err := extractPlugin(archive, artifact); err != nil {
		t.Fatalf("extracting the plugin: %v", err)
	}
	assertNoCheckoutPath(t, root, artifact)

	validated := runCommand(t, base, nil, claude, "plugin", "validate", artifact, "--strict")
	if validated.code != 0 || !strings.Contains(validated.output(), "Validation passed") {
		t.Fatalf("the packaged plugin does not validate (exit %d):\n%s", validated.code, validated.output())
	}

	home := filepath.Join(base, "home")
	project := filepath.Join(base, "project")
	for _, dir := range []string{home, project} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	env := replaceEnv(os.Environ(), "HOME", home)
	added := runCommand(t, project, env, claude, "plugin", "marketplace", "add", artifact, "--scope", "project", "--json")
	if added.code != 0 {
		t.Fatalf("adding the marketplace failed (exit %d):\n%s", added.code, added.output())
	}
	installed := runCommand(t, project, env, claude, "plugin", "install", "kanbanzai@kanbanzai", "--scope", "project", "-y", "--json")
	if installed.code != 0 {
		t.Fatalf("installing the plugin failed (exit %d):\n%s", installed.code, installed.output())
	}
	listed := runCommand(t, project, env, claude, "plugin", "list", "--json")
	if listed.code != 0 {
		t.Fatalf("listing the plugins failed (exit %d):\n%s", listed.code, listed.output())
	}
	var plugins []struct {
		ID          string `json:"id"`
		Version     string `json:"version"`
		InstallPath string `json:"installPath"`
		Enabled     bool   `json:"enabled"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(listed.stdout)), &plugins); err != nil {
		t.Fatalf("plugin list is not JSON: %v:\n%s", err, listed.stdout)
	}
	var plugin *struct {
		ID          string `json:"id"`
		Version     string `json:"version"`
		InstallPath string `json:"installPath"`
		Enabled     bool   `json:"enabled"`
	}
	for index := range plugins {
		if plugins[index].ID == "kanbanzai@kanbanzai" {
			plugin = &plugins[index]
		}
	}
	if plugin == nil {
		t.Fatalf("plugin list has no kanbanzai@kanbanzai:\n%s", listed.stdout)
	}
	if !plugin.Enabled {
		t.Fatalf("kanbanzai@kanbanzai is installed but not enabled:\n%s", listed.stdout)
	}
	want := artifactVersion(artifact)
	if plugin.Version != want {
		t.Fatalf("installed version = %q, want the artifact's %q", plugin.Version, want)
	}
	for _, relative := range []string{".claude-plugin/plugin.json", "skills", "hooks/hooks.json"} {
		if _, err := os.Stat(filepath.Join(plugin.InstallPath, filepath.FromSlash(relative))); err != nil {
			t.Fatalf("the installed copy has no %s: %v", relative, err)
		}
	}
}

func artifactVersion(root string) string {
	data, err := os.ReadFile(filepath.Join(root, ".claude-plugin", "plugin.json"))
	if err != nil {
		return ""
	}
	var manifest struct {
		Version string `json:"version"`
	}
	if json.Unmarshal(data, &manifest) != nil {
		return ""
	}
	return manifest.Version
}

type commandResult struct {
	code   int
	stdout string
	stderr string
}

func (result commandResult) output() string {
	return result.stdout + result.stderr
}

func runCommand(t *testing.T, dir string, env []string, name string, args ...string) commandResult {
	t.Helper()
	command := exec.Command(name, args...)
	command.Dir = dir
	if env == nil {
		env = os.Environ()
	}
	command.Env = env
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	err := command.Run()
	code := 0
	if err != nil {
		var exit *exec.ExitError
		if !errors.As(err, &exit) {
			t.Fatalf("running %s %v: %v", name, args, err)
		}
		code = exit.ExitCode()
	}
	return commandResult{code: code, stdout: stdout.String(), stderr: stderr.String()}
}

func replaceEnv(env []string, key, value string) []string {
	prefix := key + "="
	out := make([]string, 0, len(env)+1)
	for _, item := range env {
		if !strings.HasPrefix(item, prefix) {
			out = append(out, item)
		}
	}
	return append(out, prefix+value)
}

// packagePlugin writes the tracked working tree into a gzip tar without the VCS, the build outputs or local state,
// which is what a marketplace visitor downloads.
func packagePlugin(root, destination string) error {
	file, err := os.Create(destination)
	if err != nil {
		return err
	}
	gzipWriter := gzip.NewWriter(file)
	writer := tar.NewWriter(gzipWriter)
	walkErr := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if relative == "." {
			return nil
		}
		name := filepath.ToSlash(relative)
		if skipInArtifact(name, entry) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		header := &tar.Header{Name: name, Mode: int64(info.Mode().Perm()), Size: int64(len(data)), ModTime: info.ModTime()}
		if err := writer.WriteHeader(header); err != nil {
			return err
		}
		_, err = writer.Write(data)
		return err
	})
	if walkErr != nil {
		writer.Close()
		gzipWriter.Close()
		file.Close()
		return walkErr
	}
	if err := writer.Close(); err != nil {
		gzipWriter.Close()
		file.Close()
		return err
	}
	if err := gzipWriter.Close(); err != nil {
		file.Close()
		return err
	}
	return file.Close()
}

func skipInArtifact(name string, entry fs.DirEntry) bool {
	if entry.IsDir() {
		switch entry.Name() {
		case ".git", "bin", "dist", "node_modules":
			return true
		}
	}
	if name == ".kanbanzai/state" || strings.HasPrefix(name, ".kanbanzai/state/") {
		return true
	}
	return name == ".claude/settings.local.json"
}

func extractPlugin(archive, destination string) error {
	file, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer file.Close()
	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	reader := tar.NewReader(gzipReader)
	base := filepath.Clean(destination) + string(os.PathSeparator)
	for {
		header, err := reader.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		target := filepath.Join(destination, filepath.FromSlash(header.Name))
		if !strings.HasPrefix(target, base) {
			return fmt.Errorf("artifact entry escapes the directory: %s", header.Name)
		}
		if header.Typeflag == tar.TypeDir {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if header.Typeflag != tar.TypeReg {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		output, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode))
		if err != nil {
			return err
		}
		if _, err := io.Copy(output, reader); err != nil {
			output.Close()
			return err
		}
		if err := output.Close(); err != nil {
			return err
		}
	}
}

func assertNoCheckoutPath(t *testing.T, root, artifact string) {
	t.Helper()
	err := filepath.WalkDir(artifact, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		if bytes.Contains(data, []byte(root)) {
			relative, _ := filepath.Rel(artifact, path)
			t.Errorf("%s carries the checkout path %s", filepath.ToSlash(relative), root)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
