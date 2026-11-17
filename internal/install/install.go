// Package install copies the running kanbanzai executable to the per-user folder, so a harness or an editor can
// find the command without a checkout or a PATH edit, and links it as ~/.local/bin/kanbanzai when nothing else is
// there. It never edits PATH or a shell profile (it says what to add instead) and never touches the network.
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
	NoLink     bool                     // copy only: no link in ~/.local/bin and no PATH notes
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
		current = version.Effective()
	}
	if versionOf == nil {
		versionOf = releaseOf
	}
	if goos == "windows" && !options.DryRun {
		os.Remove(target + ".old") // left by an earlier refresh, when the running command could not be replaced
	}
	// A conflicting plugin must be discovered before replacing the command.
	if !options.DryRun {
		err = installPlugins(filepath.Dir(folder), true, options.Refresh, io.Discard)
	}
	if err == nil {
		err = checkPrograms(goos, executable, folder, separator)
	}
	if err == nil {
		err = install(options.DryRun, options.Refresh, goos == "windows", executable, folder, target, out, current, versionOf)
	}
	if err == nil {
		err = installPlugins(filepath.Dir(folder), options.DryRun, options.Refresh, out)
	}
	if err == nil && !options.Refresh && !options.NoLink {
		err = onPath(options.DryRun, goos, getenv, folder, target, out)
	}
	if err == nil {
		err = installPrograms(options, goos, getenv, executable, folder, separator, out, current)
	}
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

// onPath makes the command reachable from a terminal, after the copy is in place. Unix-like systems get a symlink
// $HOME/.local/bin/kanbanzai to the copy; Windows gets no link, only where the file is and how to add its folder to
// PATH. Nothing here edits PATH or a profile. What is already at the link path is never replaced: it is left alone with
// a note, and that is not a failure. A failed write is.
func onPath(dryRun bool, goos string, getenv func(string) string, folder, target string, out io.Writer) error {
	return linkOnPath(dryRun, goos, getenv, folder, target, "kanbanzai", true, out)
}

// linkOnPath is onPath for one program: name is its name without .exe, and note says whether to say how to put
// ~/.local/bin on PATH (the command has said it already when a program follows it).
func linkOnPath(dryRun bool, goos string, getenv func(string) string, folder, target, name string, note bool, out io.Writer) error {
	if goos == "windows" {
		if note && !pathHas(goos, getenv("PATH"), folder) {
			fmt.Fprintf(out, "note: %s.exe is at %s\n", name, target)
			fmt.Fprintf(out, "note: to run it from a terminal add %s to your user PATH: Settings > System > About > "+
				"Advanced system settings > Environment Variables > Path > Edit > New; then open a new terminal\n", folder)
		}
		return nil
	}
	home := strings.TrimRight(getenv("HOME"), "/")
	if home == "" {
		fmt.Fprintf(out, "note: not linked: HOME is not set, so there is no ~/.local/bin; run the command at %s\n", target)
		return nil
	}
	dir := home + "/.local/bin"
	link := dir + "/" + name

	switch _, err := os.Lstat(dir); {
	case err == nil:
		if resolved, err := os.Stat(dir); err != nil || !resolved.IsDir() {
			return fmt.Errorf("cannot link the command: %s is not a folder", dir)
		}
	case errors.Is(err, os.ErrNotExist):
		if !dryRun {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return fmt.Errorf("cannot create %s: %w", dir, err)
			}
		}
	default:
		return fmt.Errorf("cannot inspect %s: %w", dir, err)
	}

	switch info, err := os.Lstat(link); {
	case err == nil:
		if info.Mode()&os.ModeSymlink != 0 {
			if to, err := os.Readlink(link); err == nil && to == target {
				fmt.Fprintf(out, "already linked: %s\n", link)
				if note {
					pathNote(goos, getenv, dir, out)
				}
				return nil
			}
		}
		fmt.Fprintf(out, "note: not linked: %s already exists and is not a link to %s; replace it yourself with "+
			"ln -sf %s %s\n", link, target, target, link)
		return nil
	case !errors.Is(err, os.ErrNotExist):
		return fmt.Errorf("cannot inspect %s: %w", link, err)
	}
	if dryRun {
		fmt.Fprintf(out, "would link %s -> %s (dry run, nothing written)\n", link, target)
		return nil
	}
	if err := os.Symlink(target, link); err != nil {
		return fmt.Errorf("cannot link %s: %w", link, err)
	}
	fmt.Fprintf(out, "linked %s -> %s\n", link, target)
	if note {
		pathNote(goos, getenv, dir, out)
	}
	return nil
}

// pathNote says what to add to the shell profile when dir is not on the running PATH. It edits nothing.
func pathNote(goos string, getenv func(string) string, dir string, out io.Writer) {
	if pathHas(goos, getenv("PATH"), dir) {
		return
	}
	fmt.Fprintf(out, "note: ~/.local/bin is not on your PATH; add  export PATH=\"$HOME/.local/bin:$PATH\"  to your shell "+
		"profile (~/.profile, ~/.bashrc or ~/.zshrc) and open a new terminal\n")
}

// pathHas reports whether dir is one of the entries of a PATH value; an empty entry is ignored.
func pathHas(goos, value, dir string) bool {
	separator, clean := ":", func(s string) string { return filepath.Clean(s) }
	if goos == "windows" {
		separator = ";"
		clean = func(s string) string { return strings.ToLower(strings.TrimRight(strings.ReplaceAll(s, "/", `\`), `\`)) }
	}
	want := clean(dir)
	for _, entry := range strings.Split(value, separator) {
		if entry != "" && clean(entry) == want {
			return true
		}
	}
	return false
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

// Programs are the store plugins a release ships beside the command. Install copies each that sits next to the running
// command (a source build has none), the way it copies the command, and links it onto PATH, where the command finds a
// plugin by its name.
var Programs = []string{"kanbanzai-store-github"}

// installPrograms installs every program that sits beside the running command. A refresh only replaces a copy that is
// already installed, and says nothing about one that is not.
func installPrograms(options Options, goos string, getenv func(string) string, executable func() (string, error),
	folder, separator string, out io.Writer, current string) error {
	path, err := executable()
	if err != nil {
		return nil // install already said why it cannot find the command
	}
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		path = resolved
	}
	for _, program := range Programs {
		file := program
		if goos == "windows" {
			file += ".exe"
		}
		source := filepath.Join(filepath.Dir(path), file)
		if info, err := os.Stat(source); err != nil || !info.Mode().IsRegular() {
			continue
		}
		target := folder + separator + file
		if options.Refresh {
			if info, err := os.Lstat(target); err != nil || !info.Mode().IsRegular() {
				continue
			}
		}
		beside := func() (string, error) { return source, nil }
		versionOf := func(path string) string {
			return strings.TrimSpace(strings.TrimPrefix(options.versionOf()(path), "-store-github"))
		}
		if err := install(options.DryRun, options.Refresh, goos == "windows", beside, folder, target, out, current, versionOf); err != nil {
			return err
		}
		if !options.Refresh && !options.NoLink {
			if err := linkOnPath(options.DryRun, goos, getenv, folder, target, program, false, out); err != nil {
				return err
			}
		}
	}
	return nil
}

// checkPrograms refuses, before anything is installed, a program's place that holds something install must not replace.
func checkPrograms(goos string, executable func() (string, error), folder, separator string) error {
	path, err := executable()
	if err != nil {
		return nil
	}
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		path = resolved
	}
	for _, program := range Programs {
		file := program
		if goos == "windows" {
			file += ".exe"
		}
		if info, err := os.Stat(filepath.Join(filepath.Dir(path), file)); err != nil || !info.Mode().IsRegular() {
			continue
		}
		target := folder + separator + file
		switch info, err := os.Lstat(target); {
		case err != nil:
		case info.Mode()&os.ModeSymlink != 0:
			return refusal{fmt.Sprintf("%s is a symlink; refusing to replace it", target)}
		case !info.Mode().IsRegular():
			return refusal{fmt.Sprintf("%s exists and is not a regular file", target)}
		}
	}
	return nil
}

func (o Options) versionOf() func(string) string {
	if o.VersionOf != nil {
		return o.VersionOf
	}
	return releaseOf
}
