// Package install copies the running kanbanzai executable to the per-user folder, so a harness or an editor can
// find the command without a checkout or a PATH edit. It never edits PATH and never touches the network.
package install

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/cloudbusting/kanbanzai/internal/version"
)

// Options is one run of the command. Zero values use the real process: os.Executable, os.Getenv and runtime.GOOS.
type Options struct {
	DryRun     bool
	Where      bool
	Refresh    bool                     // only replace a copy that is already there; never create one
	Current    string                   // the running command's release, for the refresh message; "" reads the build's own
	VersionOf  func(path string) string // the release a command file says it is; "" reads it by running `path --version`
	Out, Err   io.Writer
	Executable func() (string, error)
	Getenv     func(string) string
	GOOS       string
}

// Folder is the per-user folder the command is copied to, for goos and an environment lookup. Unix-like systems
// use $XDG_DATA_HOME/kanbanzai/bin when XDG_DATA_HOME is set and absolute, else $HOME/.local/share/kanbanzai/bin;
// Windows uses %LOCALAPPDATA%\kanbanzai\bin, else %USERPROFILE%\AppData\Local\kanbanzai\bin.
func Folder(goos string, getenv func(string) string) (string, error) {
	if goos == "windows" {
		base := getenv("LOCALAPPDATA")
		if base == "" {
			profile := getenv("USERPROFILE")
			if profile == "" {
				return "", errors.New("cannot find the per-user folder: neither LOCALAPPDATA nor USERPROFILE is set")
			}
			base = strings.TrimRight(profile, `\/`) + `\AppData\Local`
		}
		return strings.TrimRight(base, `\/`) + `\kanbanzai\bin`, nil
	}
	if xdg := getenv("XDG_DATA_HOME"); strings.HasPrefix(xdg, "/") {
		return strings.TrimRight(xdg, "/") + "/kanbanzai/bin", nil
	}
	home := getenv("HOME")
	if home == "" {
		return "", errors.New("cannot find the per-user folder: HOME is not set and XDG_DATA_HOME is not an absolute path")
	}
	return strings.TrimRight(home, "/") + "/.local/share/kanbanzai/bin", nil
}

// exit codes: 0 done or nothing to do, 1 failed, 2 refused.
type refusal struct{ message string }

func (r refusal) Error() string { return r.message }

// Run is the command; it returns the exit code.
func Run(options Options) int {
	out, errOut := options.Out, options.Err
	if out == nil {
		out = io.Discard
	}
	if errOut == nil {
		errOut = io.Discard
	}
	goos, getenv, executable := options.GOOS, options.Getenv, options.Executable
	if goos == "" {
		goos = runtime.GOOS
	}
	if getenv == nil {
		getenv = os.Getenv
	}
	if executable == nil {
		executable = os.Executable
	}
	folder, err := Folder(goos, getenv)
	if err != nil {
		fmt.Fprintf(errOut, "kanbanzai: install: %v\n", err)
		return 1
	}
	if options.Where {
		fmt.Fprintln(out, folder)
		return 0
	}
	name := "kanbanzai"
	if goos == "windows" {
		name += ".exe"
	}
	separator := string(filepath.Separator)
	if goos == "windows" {
		separator = `\`
	}
	target := folder + separator + name

	current, versionOf := options.Current, options.VersionOf
	if current == "" {
		current = version.Version
	}
	if versionOf == nil {
		versionOf = releaseOf
	}
	if goos == "windows" && !options.DryRun {
		os.Remove(target + ".old") // left by an earlier refresh, when the running command could not be replaced
	}
	err = install(options.DryRun, options.Refresh, goos == "windows", executable, folder, target, out, current, versionOf)
	if err != nil {
		fmt.Fprintf(errOut, "kanbanzai: install: %v\n", err)
		var refused refusal
		if errors.As(err, &refused) {
			return 2
		}
		return 1
	}
	return 0
}

func install(dryRun, refresh, windows bool, executable func() (string, error), folder, target string, out io.Writer,
	current string, versionOf func(string) string) error {
	path, err := executable()
	if err != nil {
		return fmt.Errorf("cannot find the running command: %w", err)
	}
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		path = resolved
	}
	source, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("cannot read the running command %s: %w", path, err)
	}
	if !source.Mode().IsRegular() {
		return fmt.Errorf("the running command %s is not a regular file", path)
	}

	folderExists := false
	switch info, err := os.Lstat(folder); {
	case err == nil:
		if info.Mode()&os.ModeSymlink != 0 {
			return refusal{fmt.Sprintf("%s is a symlink; refusing to install through it", folder)}
		}
		if !info.IsDir() {
			return refusal{fmt.Sprintf("%s exists and is not a folder", folder)}
		}
		folderExists = true
	case !errors.Is(err, os.ErrNotExist):
		return fmt.Errorf("cannot inspect %s: %w", folder, err)
	}

	if folderExists {
		switch info, err := os.Lstat(target); {
		case err == nil:
			if info.Mode()&os.ModeSymlink != 0 {
				return refusal{fmt.Sprintf("%s is a symlink; refusing to replace it", target)}
			}
			if !info.Mode().IsRegular() {
				return refusal{fmt.Sprintf("%s exists and is not a regular file", target)}
			}
			same, err := identical(path, source, target, info)
			if err != nil {
				return fmt.Errorf("cannot compare %s with %s: %w", path, target, err)
			}
			if same {
				fmt.Fprintf(out, "already installed: %s\n", target)
				return nil
			}
		case !errors.Is(err, os.ErrNotExist):
			return fmt.Errorf("cannot inspect %s: %w", target, err)
		}
	}

	had := false
	if folderExists {
		if info, err := os.Lstat(target); err == nil && info.Mode().IsRegular() {
			had = true
		}
	}
	if refresh && !had {
		fmt.Fprintf(out, "nothing to refresh: no command is installed at %s\n", target)
		return nil
	}
	if dryRun {
		fmt.Fprintf(out, "would install %s to %s (dry run, nothing written)\n", path, target)
		return nil
	}
	previous := ""
	if had {
		previous = versionOf(target)
		if previous == "" {
			previous = "unknown"
		}
	}
	if err := os.MkdirAll(folder, 0o755); err != nil {
		return fmt.Errorf("cannot create %s: %w", folder, err)
	}
	if err := copyAtomic(path, folder, target, windows); err != nil {
		return err
	}
	if refresh {
		fmt.Fprintf(out, "refreshed %s: %s -> %s\n", target, previous, current)
		return nil
	}
	fmt.Fprintf(out, "installed %s to %s\n", path, target)
	return nil
}

// releaseOf is the release a command file says it is: its `--version` output without the name, "" when it cannot be
// run or answers nothing. A copy of a release this old or this broken is simply "unknown" in the refresh message.
func releaseOf(path string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, path, "--version").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(string(out)), "kanbanzai"))
}

// identical reports whether target already holds the source's bytes: the same file, or the same size and sha256.
func identical(sourcePath string, source os.FileInfo, targetPath string, target os.FileInfo) (bool, error) {
	if os.SameFile(source, target) {
		return true, nil
	}
	if source.Size() != target.Size() {
		return false, nil
	}
	a, err := digest(sourcePath)
	if err != nil {
		return false, err
	}
	b, err := digest(targetPath)
	if err != nil {
		return false, err
	}
	return a == b, nil
}

func digest(path string) ([sha256.Size]byte, error) {
	var sum [sha256.Size]byte
	file, err := os.Open(path)
	if err != nil {
		return sum, err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return sum, err
	}
	copy(sum[:], hash.Sum(nil))
	return sum, nil
}

// copyAtomic copies the file's bytes to a temporary file in folder, sets mode 0755 and renames it over target.
func copyAtomic(sourcePath, folder, target string, windows bool) error {
	input, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("cannot read %s: %w", sourcePath, err)
	}
	defer input.Close()
	temporary, err := os.CreateTemp(folder, ".kanbanzai.*")
	if err != nil {
		return fmt.Errorf("cannot write in %s: %w", folder, err)
	}
	name := temporary.Name()
	fail := func(format string, err error) error {
		temporary.Close()
		os.Remove(name)
		return fmt.Errorf(format, err)
	}
	if _, err := io.Copy(temporary, input); err != nil {
		return fail("cannot copy the command: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		return fail("cannot flush the copy: %w", err)
	}
	if err := temporary.Close(); err != nil {
		os.Remove(name)
		return fmt.Errorf("cannot close the copy: %w", err)
	}
	if err := os.Chmod(name, 0o755); err != nil {
		os.Remove(name)
		return fmt.Errorf("cannot make the copy executable: %w", err)
	}
	err = os.Rename(name, target)
	if err != nil && windows {
		// A running command cannot be renamed over on Windows, but it can be renamed away.
		aside := target + ".old"
		os.Remove(aside)
		if os.Rename(target, aside) == nil {
			if err = os.Rename(name, target); err != nil {
				os.Rename(aside, target)
			}
		}
	}
	if err != nil {
		os.Remove(name)
		return fmt.Errorf("cannot replace %s: %w (an installed command that is running cannot be replaced on "+
			"Windows; stop it and run install again)", target, err)
	}
	return nil
}
