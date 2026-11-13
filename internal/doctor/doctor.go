// Package doctor diagnoses a local installation without repairing it, running adapters, or using the network.
package doctor

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/cloudbusting/kanbanzai/internal/config"
	"github.com/cloudbusting/kanbanzai/internal/fsutil"
	"github.com/cloudbusting/kanbanzai/internal/gitutil"
	"github.com/cloudbusting/kanbanzai/internal/version"
)

// Options controls the report format and destinations. Nil writers discard output.
type Options struct {
	JSON     bool
	Out, Err io.Writer
}

// Finding is a stable diagnostic check; Fix is empty when no action is needed.
type Finding struct {
	ID      string `json:"id"`
	Status  string `json:"status"`
	Message string `json:"message"`
	Fix     string `json:"fix"`
}

// Report is the versioned JSON contract, independent of the config schema version.
type Report struct {
	SchemaVersion int       `json:"schema_version"`
	OK            bool      `json:"ok"`
	Findings      []Finding `json:"findings"`
}

func (r *Report) add(id, status, message, fix string) {
	r.Findings = append(r.Findings, Finding{id, status, message, fix})
	if status == "error" {
		r.OK = false
	}
}

// Run diagnoses start, even when configuration loading fails. Exit 1 means at least one error.
// Only uniquely named temporary probe files are written, and each is removed before returning.
func Run(start string, options Options) int {
	out, stderr := options.Out, options.Err
	if out == nil {
		out = io.Discard
	}
	if stderr == nil {
		stderr = io.Discard
	}
	r := Report{SchemaVersion: 1, OK: true, Findings: []Finding{}}
	gitPath, gitErr := exec.LookPath("git")
	if gitErr != nil {
		r.add("git", "error", "git is not available on PATH: "+gitErr.Error(), "Install git and put it on PATH.")
	} else {
		r.add("git", "pass", "Local git executable: "+gitPath, "")
	}
	r.add("binary.version", versionStatus(), "Running binary: "+version.String()+"; a PATH candidate's version is not executed or certified.", versionFix())
	r.command(start)
	root, top, common, err := config.Locate(start)
	var p *config.Project
	if err != nil {
		r.add("project", "error", err.Error(), "Run doctor inside the project's git checkout, or correct KANBANZAI_PROJECT.")
		r.add("config", "skipped", "Project could not be located; config was not loaded.", "")
	} else {
		r.add("project", "pass", "Shared project: "+root+"; current checkout: "+top, "")
		p, err = config.New(root, top, common, nil)
		if err != nil {
			r.add("config", "error", err.Error(), "Correct the named config error; for legacy files run `kanbanzai config upgrade`, then `kanbanzai config upgrade --apply`.")
		} else {
			source := p.ConfigFile
			if source == "" {
				source = "embedded defaults (no project config; run `kanbanzai init` to initialize)"
			}
			r.add("config", "pass", fmt.Sprintf("Config version %d: %s", p.ConfigVersion, source), "")
		}
	}
	if p == nil {
		for _, id := range []string{"config.ignored", "git.main_branch", "folder.work", "folder.cards", "access.home", "access.state", "access.claims", "access.numbers", "template", "state.tracked", "state.ignore", "claims.tracked", "claims.ignore", "numbers.tracked", "numbers.ignore"} {
			r.add(id, "skipped", "Requires a valid project configuration.", "")
		}
	} else {
		if len(p.IgnoredUnknown) > 0 {
			r.add("config.ignored", "warning", "Explicitly ignored unknown config paths: "+strings.Join(p.IgnoredUnknown, ", "), "Remove obsolete keys or confirm that ignoring these settings is intentional.")
		} else {
			r.add("config.ignored", "pass", "No unknown config paths were ignored.", "")
		}
		if gitErr != nil {
			r.add("git.main_branch", "skipped", "Requires git.", "")
		} else if _, err := gitutil.Git(p.Root, "show-ref", "--verify", "--quiet", "refs/heads/"+p.MainBranch); err != nil {
			// A just-created repository has an unborn branch, but is otherwise a valid fresh init.
			branch, branchErr := gitutil.Git(p.Root, "symbolic-ref", "--quiet", "--short", "HEAD")
			_, headErr := gitutil.Git(p.Root, "rev-parse", "--verify", "HEAD")
			if branchErr == nil && headErr != nil && branch == p.MainBranch {
				r.add("git.main_branch", "warning", "Configured main branch is unborn: "+p.MainBranch, "Make the project's initial commit when ready.")
			} else {
				r.add("git.main_branch", "error", "Configured local main branch does not exist: "+p.MainBranch, "Create the local branch or correct project.main_branch; doctor does not fetch remote branches.")
			}
		} else {
			r.add("git.main_branch", "pass", "Configured local main branch exists: "+p.MainBranch, "")
		}
		r.folder("folder.work", p.Work)
		missing := []string{}
		for _, folder := range p.Folders {
			path := filepath.Join(p.Work, folder)
			if err := readableDirectory(path); err != nil {
				missing = append(missing, path+": "+err.Error())
			}
		}
		if len(missing) > 0 {
			r.add("folder.cards", "error", "Card folders unavailable: "+strings.Join(missing, "; "), "Run `kanbanzai init` to create missing folders; correct configured work/column paths or their permissions.")
		} else {
			r.add("folder.cards", "pass", "All configured card folders are readable.", "")
		}
		for _, item := range []struct{ name, path string }{{"home", p.Home}, {"state", p.State}, {"claims", p.Claims}, {"numbers", p.Numbers}} {
			message, err := probe(item.path)
			if err != nil {
				r.add("access."+item.name, "error", item.path+": "+err.Error(), "Correct this path or its write/lock permissions; doctor creates no project directories.")
			} else {
				r.add("access."+item.name, "pass", message, "")
			}
		}
		var templateErr error
		if !p.TemplateIsShipped {
			templateErr = readableFile(p.Template)
		}
		if templateErr == nil {
			_, templateErr = p.TemplateText()
		}
		if templateErr != nil {
			r.add("template", "error", "Card template is unreadable: "+templateErr.Error(), "Restore project.template or clear it to use the embedded template.")
		} else {
			r.add("template", "pass", "Card template is readable.", "")
		}
		for _, item := range []struct{ name, path string }{{"state", p.State}, {"claims", p.Claims}, {"numbers", p.Numbers}} {
			r.state(p.Root, p.GitCommon, item.name, item.path, gitErr == nil)
		}
	}
	if root == "" {
		for _, name := range []string{"claude", "opencode"} {
			r.add("adapter."+name, "skipped", "Project was not located; repository-local declarations were not inspected.", "")
		}
	} else {
		// Inspect the main checkout and the current linked checkout, without following ancestors or user config.
		bases := []string{root}
		if top != root {
			bases = append(bases, top)
		}
		r.claude(bases)
		r.opencode(bases)
	}
	if options.JSON {
		encoder := json.NewEncoder(out)
		encoder.SetIndent("", "  ")
		err = encoder.Encode(r)
	} else {
		for _, f := range r.Findings {
			if _, err = fmt.Fprintf(out, "%s %s: %s\n", f.Status, f.ID, f.Message); err != nil {
				break
			}
			if f.Fix != "" {
				if _, err = fmt.Fprintf(out, "  Fix: %s\n", f.Fix); err != nil {
					break
				}
			}
		}
	}
	if err != nil {
		fmt.Fprintln(stderr, "kanbanzai doctor: cannot write report:", err)
		return 1
	}
	if !r.OK {
		return 1
	}
	return 0
}

func versionStatus() string {
	if version.Effective() == "dev" {
		return "warning"
	}
	return "pass"
}
func versionFix() string {
	if version.Effective() == "dev" {
		return "Use a release binary when a versioned installation is required."
	}
	return ""
}

func (r *Report) command(start string) {
	if override := strings.TrimSpace(os.Getenv("KANBANZAI_BIN")); override != "" {
		candidate := override
		if !filepath.IsAbs(candidate) && start != "" {
			candidate = filepath.Join(start, candidate)
		}
		if err := executable(candidate); err != nil {
			r.add("binary.resolution", "error", "KANBANZAI_BIN does not name an executable file: "+override+": "+err.Error(), "Correct or unset KANBANZAI_BIN; an invalid override takes precedence over PATH.")
		} else {
			r.add("binary.resolution", "pass", "KANBANZAI_BIN resolves to "+candidate+" (not executed).", "")
		}
		return
	}
	path, err := exec.LookPath("kanbanzai")
	if err != nil {
		r.add("binary.resolution", "error", "kanbanzai was not resolved on PATH: "+err.Error(), "Install kanbanzai on PATH or set KANBANZAI_BIN to its executable file; adapter-local build fallbacks are not certified.")
	} else {
		r.add("binary.resolution", "pass", "PATH resolves kanbanzai to "+path+" (not executed).", "")
	}
}

func executable(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("not a regular file")
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o111 == 0 {
		return fmt.Errorf("no execute permission")
	}
	return nil
}

func readableDirectory(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", path)
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Readdirnames(1)
	if errors.Is(err, io.EOF) {
		return nil
	}
	return err
}

func (r *Report) folder(id, path string) {
	if err := readableDirectory(path); err != nil {
		r.add(id, "error", path+": "+err.Error(), "Run `kanbanzai init` to create missing folders; otherwise correct the path or read permissions.")
	} else {
		r.add(id, "pass", "Readable cards directory: "+path, "")
	}
}

// probe checks the directory, or the nearest existing parent if runtime directories are not made yet.
// It never opens a real ledger/claim/lock and never leaves a newly created directory behind.
func probe(path string) (message string, result error) {
	dir := path
	for {
		info, err := os.Stat(dir)
		if err == nil {
			if !info.IsDir() {
				return "", fmt.Errorf("%s is not a directory", dir)
			}
			break
		}
		if !os.IsNotExist(err) {
			return "", err
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", err
		}
		dir = parent
	}
	if err := readableDirectory(dir); err != nil {
		return "", err
	}
	f, err := os.CreateTemp(dir, ".kanbanzai-doctor-*")
	if err != nil {
		return "", err
	}
	name := f.Name()
	defer func() {
		if err := os.Remove(name); err != nil {
			result = errors.Join(result, fmt.Errorf("probe cleanup %s: %w", name, err))
		}
	}()
	_, writeErr := f.WriteString("kanbanzai doctor temporary write/lock probe\n")
	syncErr := f.Sync()
	closeErr := f.Close()
	if err := errors.Join(writeErr, syncErr, closeErr); err != nil {
		return "", err
	}
	lock, err := fsutil.TryLock(name)
	if err != nil {
		return "", err
	}
	if err := lock.Close(); err != nil {
		return "", err
	}
	if dir != path {
		return "Directory absent: " + path + "; nearest existing parent " + dir + " passed temporary write/lock probe (directory not created).", nil
	}
	return "Temporary write/lock probe passed: " + path, nil
}

func inside(root, path string) (string, bool) {
	rel, err := filepath.Rel(root, path)
	return filepath.ToSlash(rel), err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// physical resolves even paths whose final runtime directories do not exist yet.
func physical(path string) string {
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return resolved
	}
	parent := filepath.Dir(path)
	if parent == path {
		return path
	}
	return filepath.Join(physical(parent), filepath.Base(path))
}

func (r *Report) state(root, common, name, path string, gitAvailable bool) {
	if !gitAvailable {
		r.add(name+".tracked", "skipped", "Requires git.", "")
		r.add(name+".ignore", "skipped", "Requires git.", "")
		return
	}
	rel, local := inside(root, path)
	if !local {
		r.add(name+".tracked", "skipped", "External path is outside this repository: "+path+"; other repositories are not inspected.", "")
		r.add(name+".ignore", "skipped", "External path needs no ignore rule in this repository: "+path, "")
		return
	}
	tracked, err := gitutil.GitRaw(root, "ls-files", "-z", "--", ":(literal)"+rel)
	if err != nil {
		r.add(name+".tracked", "error", "Cannot inspect tracked state: "+err.Error(), "Ensure this is a readable git checkout.")
	} else if tracked != "" {
		r.add(name+".tracked", "error", "Runtime path has tracked entries: "+path, "Remove runtime state from the git index after preserving any needed data, and add an ignore rule; doctor does not change the index.")
	} else {
		r.add(name+".tracked", "pass", "No tracked entries under "+path, "")
	}
	if common != "" {
		if _, metadata := inside(physical(common), physical(path)); metadata {
			r.add(name+".ignore", "skipped", "Runtime path is inside git's metadata directory, not the working tree; no ignore rule is needed: "+path, "")
			return
		}
	}
	if _, local := inside(physical(root), physical(path)); !local {
		r.add(name+".ignore", "warning", "Runtime path resolves outside this repository through a symlink: "+path, "Check the external target's storage and ignore policy separately.")
		return
	}
	_, dirErr := gitutil.Git(root, "check-ignore", "--no-index", "-q", "--", rel+"/")
	_, childErr := gitutil.Git(root, "check-ignore", "--no-index", "-q", "--", rel+"/.kanbanzai-doctor-ignore-probe")
	if dirErr != nil || childErr != nil {
		r.add(name+".ignore", "error", "Runtime path is not fully gitignored: "+path, "Add an ignore rule covering this runtime directory and its contents (default: .kanbanzai/.gitignore contains state/).")
	} else {
		r.add(name+".ignore", "pass", "Runtime directory and probe child are gitignored: "+path, "")
	}
}
