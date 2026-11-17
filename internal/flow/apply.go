package flow

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cloudbusting/kanbanzai/internal/board"
	"github.com/cloudbusting/kanbanzai/internal/boardbranch"
	"github.com/cloudbusting/kanbanzai/internal/check"
	"github.com/cloudbusting/kanbanzai/internal/config"
	"github.com/cloudbusting/kanbanzai/internal/doctor"
	"github.com/cloudbusting/kanbanzai/internal/fsutil"
	"github.com/cloudbusting/kanbanzai/internal/gitutil"
)

// ErrKilled, returned by ApplyOptions.After, ends the apply on the spot with no rollback, as a killed process
// would. Tests use it to leave a half-applied board for `--rollback`.
var ErrKilled = errors.New("killed")

// JournalVersion is the version of the journal file.
const JournalVersion = 1

// Journal statuses.
const (
	StatusApplying   = "applying"
	StatusApplied    = "applied"
	StatusRolledBack = "rolled-back"
)

// Step kinds.
const (
	StepMkdir    = "mkdir"
	StepBrief    = "brief"
	StepTemplate = "template"
	StepConfig   = "config"
)

// Step is one action of the apply, recorded before it runs so a rollback knows what may have happened.
type Step struct {
	Kind      string `json:"kind"`
	Path      string `json:"path"`
	NewPath   string `json:"new_path,omitempty"`
	Backup    string `json:"backup,omitempty"`
	BeforeSHA string `json:"before_sha256,omitempty"`
	AfterSHA  string `json:"after_sha256,omitempty"`
	Created   bool   `json:"created,omitempty"`
	Done      bool   `json:"done"`
}

// Journal is the record of an apply in progress. It lives in <state>/workflow/journal.json, beside the backups.
type Journal struct {
	SchemaVersion int    `json:"schema_version"`
	Plan          string `json:"plan_sha256"`
	Status        string `json:"status"`
	Steps         []Step `json:"steps"`
}

// ApplyOptions are the parts of an apply a caller or a test can see into.
type ApplyOptions struct {
	// After runs after each step, named by its kind and path. An error rolls the apply back; ErrKilled stops it.
	After func(step string) error
	// Doctor runs the installation check; nil uses the real one.
	Doctor func(root string) int
}

// Result is what an apply or a rollback did.
type Result struct {
	Changed []string `json:"changed"`
	Unused  []string `json:"unused_folders"`
	// Notes say what to do next when the change is not on the main branch: merge it there. Never a reason to refuse.
	Notes []string `json:"notes"`
}

// branchNotes is the instruction that goes with a board change made off the main branch.
func branchNotes(p *config.Project) []string {
	branch, err := gitutil.Git(p.Root, "symbolic-ref", "--quiet", "--short", "HEAD")
	if err != nil || branch == "" || p.MainBranch == "" || branch == p.MainBranch {
		return []string{}
	}
	return []string{fmt.Sprintf("the board change is on branch %q, not on %q: push %q and merge it into %s through your review "+
		"before other clones see it", branch, p.MainBranch, branch, p.MainBranch)}
}

// Failed is an apply that went wrong after it began writing.
type Failed struct {
	Cause      error
	RolledBack bool
	Detail     string
}

func (f *Failed) Error() string {
	state := "the board was rolled back to what it was"
	if !f.RolledBack {
		state = "the board could NOT be fully rolled back: " + f.Detail + "; fix it, then run `kanbanzai workflow apply --rollback`"
	}
	return fmt.Sprintf("apply failed: %v; %s", f.Cause, state)
}

func (f *Failed) Unwrap() error { return f.Cause }

func dirs(p *config.Project) (lockFile, journalFile, backupDir string) {
	base := filepath.Join(p.State, "workflow")
	return filepath.Join(p.State, "workflow.lock"), filepath.Join(base, "journal.json"), filepath.Join(base, "backup")
}

func abs(p *config.Project, rel string) string { return p.Abs(rel) }

func readText(path string) (string, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	return string(data), true
}

func readJournal(path string) (*Journal, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	journal := &Journal{}
	if err := json.Unmarshal(data, journal); err != nil {
		return nil, fmt.Errorf("%s: %v", path, err)
	}
	return journal, nil
}

func writeJournal(path string, journal *Journal) error {
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(journal); err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(path, buf.Bytes(), 0o600)
}

// Preflight proves the board is exactly as the plan saw it. It reads only.
func Preflight(p *config.Project, plan *Plan) error {
	if err := ValidatePlan(plan, p); err != nil {
		return err
	}
	_, journalFile, _ := dirs(p)
	if journal, err := readJournal(journalFile); err == nil && journal.Status == StatusApplying {
		return refuse("an earlier apply did not finish: run `kanbanzai workflow apply --rollback` first")
	}
	if p.BoardOnBranch() {
		if board, err := boardbranch.New(p); err != nil || !board.Attached() {
			return refuse("the board is kept on a branch that is not checked out at %s: run `kanbanzai sync --attach` first", p.Rel(p.Work))
		}
	}
	configPath := abs(p, plan.Config.Path)
	if text, ok := readText(configPath); ok != plan.Config.Existed || (ok && Digest(text) != plan.Config.ExpectSHA256) {
		return refuse("%s changed since the plan was made: plan again", plan.Config.Path)
	}
	touched := []string{plan.Config.Path}
	for _, template := range plan.AllTemplates() {
		touched = append(touched, template.Path)
		if text, ok := readText(abs(p, template.Path)); !ok || Digest(text) != template.ExpectSHA256 {
			return refuse("%s changed since the plan was made: plan again", template.Path)
		}
	}
	claims := board.LiveClaims(p)
	for _, c := range plan.Cards {
		if owner, claimed := claims[c.ID]; claimed {
			return refuse("%s is claimed by %s: wait for the claim to end, then plan again", c.ID, owner)
		}
		text, ok := readText(abs(p, c.Path))
		if !ok || Digest(text) != c.ExpectSHA256 {
			return refuse("%s changed since the plan was made: plan again", c.Path)
		}
		if Digest(Migrate(text, c, p.BriefSectionOrder(c.ID))) != c.AfterSHA256 {
			return refuse("%s: the migration does not give the text the plan promised", c.Path)
		}
		if c.NewPath != c.Path {
			if _, err := os.Lstat(abs(p, c.NewPath)); err == nil {
				return refuse("%s already exists", c.NewPath)
			}
		}
		touched = append(touched, c.Path)
	}
	if err := cleanTree(p, touched); err != nil {
		return err
	}
	if problems := check.ProblemsOf(p); len(problems) > 0 {
		return refuse("`kanbanzai check` is red before the migration; fix it first:\n  %s", strings.Join(problems, "\n  "))
	}
	return nil
}

// cleanTree refuses when anything the apply would write, or that a rollback would have to restore, is uncommitted.
// Where the board is on the main branch that is one git status over the briefs, config and templates. Where it is on a
// branch of its own the work folder is a separate worktree that the main checkout's git cannot see into, so the
// briefs, claims and ledger are checked there, over the whole folder, and only the config and templates in the main
// checkout. touched lists the config, the templates and the briefs the plan names.
func cleanTree(p *config.Project, touched []string) error {
	if !p.BoardOnBranch() {
		status, err := gitutil.Git(p.Root, append([]string{"status", "--porcelain", "--"}, append(touched, slash(p.Root, p.Work))...)...)
		if err != nil {
			return refuse("git status: %v", err)
		}
		if strings.TrimSpace(status) != "" {
			return refuse("the board has uncommitted changes; commit or discard them first so a rollback is exact:\n%s", status)
		}
		return nil
	}
	var outside []string
	for _, path := range touched {
		if !strings.HasPrefix(path, slash(p.Root, p.Work)+"/") {
			outside = append(outside, path)
		}
	}
	status, err := gitutil.Git(p.Root, append([]string{"status", "--porcelain", "--"}, outside...)...)
	if err != nil {
		return refuse("git status: %v", err)
	}
	if strings.TrimSpace(status) != "" {
		return refuse("the config or a template has uncommitted changes; commit or discard them first so a rollback is exact:\n%s", status)
	}
	status, err = gitutil.Git(p.Work, "status", "--porcelain")
	if err != nil {
		return refuse("git status in the board worktree %s: %v", p.Rel(p.Work), err)
	}
	if strings.TrimSpace(status) != "" {
		return refuse("the board worktree %s has uncommitted changes; commit them (`kanbanzai sync`) or discard them first so a rollback is exact:\n%s", p.Rel(p.Work), status)
	}
	return nil
}

func runDoctor(options ApplyOptions, root string) int {
	if options.Doctor != nil {
		return options.Doctor(root)
	}
	return doctor.Run(root, doctor.Options{JSON: true})
}

// Apply carries out a plan: backup, journal, briefs, template, config last, then check and doctor on the result.
// A failure after the first write rolls everything back to the bytes it started with.
func Apply(p *config.Project, plan *Plan, options ApplyOptions) (*Result, error) {
	if err := Preflight(p, plan); err != nil {
		return nil, err
	}
	lockFile, journalFile, backupDir := dirs(p)
	lock, err := fsutil.TryLock(lockFile)
	if err != nil {
		if errors.Is(err, fsutil.ErrLocked) {
			return nil, refuse("another `kanbanzai workflow` is running")
		}
		return nil, err
	}
	defer lock.Close()
	// preflight again under the lock: nothing may have moved while waiting for it
	if err := Preflight(p, plan); err != nil {
		return nil, err
	}
	doctorBefore := runDoctor(options, p.Root)

	planJSON, err := plan.JSON()
	if err != nil {
		return nil, err
	}
	journal := &Journal{SchemaVersion: JournalVersion, Plan: Digest(string(planJSON)), Status: StatusApplying}
	backups := filepath.Join(backupDir, journal.Plan[:12])
	if err := os.RemoveAll(backups); err != nil {
		return nil, err
	}
	n := 0
	backup := func(rel string) (string, string, error) {
		text, ok := readText(abs(p, rel))
		if !ok {
			return "", "", nil
		}
		n++
		name := fmt.Sprintf("%03d", n)
		if err := fsutil.WriteFileAtomic(filepath.Join(backups, name), []byte(text), 0o600); err != nil {
			return "", "", err
		}
		return name, Digest(text), nil
	}
	for _, folder := range plan.Folders {
		_, statErr := os.Stat(abs(p, folder))
		journal.Steps = append(journal.Steps, Step{Kind: StepMkdir, Path: folder, Created: statErr != nil})
	}
	for _, c := range plan.Cards {
		name, sum, err := backup(c.Path)
		if err != nil {
			return nil, err
		}
		journal.Steps = append(journal.Steps, Step{Kind: StepBrief, Path: c.Path, NewPath: c.NewPath, Backup: name, BeforeSHA: sum, AfterSHA: c.AfterSHA256})
	}
	for _, template := range plan.AllTemplates() {
		name, sum, err := backup(template.Path)
		if err != nil {
			return nil, err
		}
		journal.Steps = append(journal.Steps, Step{Kind: StepTemplate, Path: template.Path, Backup: name, BeforeSHA: sum, AfterSHA: template.AfterSHA256})
	}
	name, sum, err := backup(plan.Config.Path)
	if err != nil {
		return nil, err
	}
	journal.Steps = append(journal.Steps, Step{Kind: StepConfig, Path: plan.Config.Path, Backup: name, BeforeSHA: sum, AfterSHA: plan.Config.AfterSHA256})
	if err := writeJournal(journalFile, journal); err != nil {
		return nil, err
	}

	fail := func(cause error) (*Result, error) {
		if errors.Is(cause, ErrKilled) {
			return nil, cause
		}
		rolled, detail := rollBack(p, journal, backups)
		if rolled {
			journal.Status = StatusRolledBack
			_ = writeJournal(journalFile, journal)
		}
		return nil, &Failed{Cause: cause, RolledBack: rolled, Detail: detail}
	}
	after := func(label string) error {
		if options.After != nil {
			return options.After(label)
		}
		return nil
	}

	for index := range journal.Steps {
		step := &journal.Steps[index]
		if err := execute(p, plan, step); err != nil {
			return fail(fmt.Errorf("%s %s: %v", step.Kind, step.Path, err))
		}
		step.Done = true
		if err := writeJournal(journalFile, journal); err != nil {
			return fail(err)
		}
		if err := after(step.Kind + " " + step.Path); err != nil {
			return fail(err)
		}
	}

	next, err := config.New(p.Root, p.Toplevel, p.GitCommon, nil)
	if err != nil {
		return fail(fmt.Errorf("the new config cannot be loaded: %v", err))
	}
	if problems := check.ProblemsOf(next); len(problems) > 0 {
		return fail(fmt.Errorf("`kanbanzai check` is red after the migration:\n  %s", strings.Join(problems, "\n  ")))
	}
	if doctorBefore == 0 {
		if code := runDoctor(options, p.Root); code != 0 {
			return fail(fmt.Errorf("`kanbanzai doctor` reports an error after the migration"))
		}
	}
	if err := after("verify"); err != nil {
		return fail(err)
	}
	journal.Status = StatusApplied
	if err := writeJournal(journalFile, journal); err != nil {
		return nil, err
	}
	result := &Result{Changed: []string{}, Unused: plan.Unused, Notes: branchNotes(p)}
	if p.BoardOnBranch() {
		saveBoardBranch(p, plan, journal)
		result.Notes = append(result.Notes, "the moved briefs are committed on the board branch "+fmt.Sprintf("%q", p.BoardBranch)+
			", but the config and the templates are only uncommitted files in the main checkout: commit them and merge them into "+
			p.MainBranch+" before teammates sync the board, or their `kanbanzai check` is red")
	}
	for _, step := range journal.Steps {
		if step.Kind == StepMkdir && !step.Created {
			continue
		}
		result.Changed = append(result.Changed, step.Kind+" "+step.Path)
		if step.NewPath != "" && step.NewPath != step.Path {
			result.Changed[len(result.Changed)-1] += " -> " + step.NewPath
		}
	}
	return result, nil
}

// saveBoardBranch commits every brief the apply rewrote or moved, old path and new, in one commit on the board branch,
// as every other board command does through the after-write hook (which also publishes under `publish = "auto"`). It
// runs last, once check and doctor have passed and the journal says applied, so a failed or killed apply has nothing
// committed to undo and a rollback never rewrites the branch's history. A commit the hook cannot make is its warning:
// the briefs stay uncommitted for `kanbanzai sync`.
func saveBoardBranch(p *config.Project, plan *Plan, journal *Journal) {
	var paths []string
	seen := map[string]bool{}
	for _, c := range plan.Cards {
		for _, rel := range []string{c.Path, c.NewPath} {
			if rel != "" && !seen[rel] {
				seen[rel] = true
				paths = append(paths, abs(p, rel))
			}
		}
	}
	boardbranch.Saved(p, "workflow: apply "+journal.Plan[:12], paths...)
}

func execute(p *config.Project, plan *Plan, step *Step) error {
	switch step.Kind {
	case StepMkdir:
		return os.MkdirAll(abs(p, step.Path), 0o755)
	case StepBrief:
		var change CardChange
		for _, c := range plan.Cards {
			if c.Path == step.Path {
				change = c
			}
		}
		text, ok := readText(abs(p, step.Path))
		if !ok || Digest(text) != change.ExpectSHA256 {
			return fmt.Errorf("it changed during the apply")
		}
		migrated := Migrate(text, change, p.BriefSectionOrder(change.ID))
		if Digest(migrated) != change.AfterSHA256 {
			return fmt.Errorf("the migration does not give the text the plan promised")
		}
		if err := fsutil.WriteFileAtomic(abs(p, step.NewPath), []byte(migrated), 0o644); err != nil {
			return err
		}
		if step.NewPath != step.Path {
			return os.Remove(abs(p, step.Path))
		}
		return nil
	case StepTemplate:
		for _, template := range plan.AllTemplates() {
			if template.Path == step.Path {
				return fsutil.WriteFileAtomic(abs(p, step.Path), []byte(template.Content), 0o644)
			}
		}
		return fmt.Errorf("the plan has no template %s", step.Path)
	case StepConfig:
		return fsutil.WriteFileAtomic(abs(p, step.Path), []byte(plan.Config.Content), 0o644)
	}
	return fmt.Errorf("unknown step kind %q", step.Kind)
}

// rollBack puts every step's files back, last step first. It is safe to run twice. It refuses to remove or
// overwrite a file that is neither what the apply wrote nor what was there before.
func rollBack(p *config.Project, journal *Journal, backups string) (bool, string) {
	var problems []string
	for index := len(journal.Steps) - 1; index >= 0; index-- {
		step := journal.Steps[index]
		switch step.Kind {
		case StepMkdir:
			if step.Created {
				_ = os.Remove(abs(p, step.Path)) // only if empty
			}
		case StepBrief, StepTemplate, StepConfig:
			if step.NewPath != "" && step.NewPath != step.Path {
				if text, ok := readText(abs(p, step.NewPath)); ok {
					if Digest(text) != step.AfterSHA {
						problems = append(problems, step.NewPath+" holds something other than what the apply wrote")
						continue
					}
					if err := os.Remove(abs(p, step.NewPath)); err != nil {
						problems = append(problems, err.Error())
						continue
					}
				}
			}
			if step.Backup == "" {
				// the file did not exist before (a new config): remove what the apply wrote
				if text, ok := readText(abs(p, step.Path)); ok && Digest(text) == step.AfterSHA {
					_ = os.Remove(abs(p, step.Path))
				}
				continue
			}
			text, ok := readText(abs(p, step.Path))
			if ok && Digest(text) == step.BeforeSHA {
				continue
			}
			if ok && Digest(text) != step.AfterSHA {
				problems = append(problems, step.Path+" holds something other than what the apply wrote or what was there before")
				continue
			}
			old, readable := readText(filepath.Join(backups, step.Backup))
			if !readable || Digest(old) != step.BeforeSHA {
				problems = append(problems, "the backup of "+step.Path+" is missing or damaged")
				continue
			}
			if err := fsutil.WriteFileAtomic(abs(p, step.Path), []byte(old), 0o644); err != nil {
				problems = append(problems, err.Error())
			}
		}
	}
	return len(problems) == 0, strings.Join(problems, "; ")
}

// Rollback undoes an apply that did not finish, from its journal.
func Rollback(p *config.Project) (*Result, error) {
	lockFile, journalFile, backupDir := dirs(p)
	lock, err := fsutil.TryLock(lockFile)
	if err != nil {
		if errors.Is(err, fsutil.ErrLocked) {
			return nil, refuse("another `kanbanzai workflow` is running")
		}
		return nil, err
	}
	defer lock.Close()
	journal, err := readJournal(journalFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, refuse("there is no apply to roll back")
		}
		return nil, refuse("%v", err)
	}
	if journal.Status != StatusApplying {
		return nil, refuse("the last apply is %s: there is nothing to roll back (git can undo a finished one)", journal.Status)
	}
	if len(journal.Plan) < 12 {
		return nil, refuse("the journal is damaged: restore the files from git")
	}
	rolled, detail := rollBack(p, journal, filepath.Join(backupDir, journal.Plan[:12]))
	if !rolled {
		return nil, &Failed{Cause: errors.New("rollback"), RolledBack: false, Detail: detail}
	}
	journal.Status = StatusRolledBack
	if err := writeJournal(journalFile, journal); err != nil {
		return nil, err
	}
	result := &Result{Changed: []string{}, Unused: []string{}, Notes: branchNotes(p)}
	for _, step := range journal.Steps {
		if step.Kind != StepMkdir || step.Created {
			result.Changed = append(result.Changed, "restored "+step.Path)
		}
	}
	return result, nil
}
