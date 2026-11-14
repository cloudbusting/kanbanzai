package migrate

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cloudbusting/kanbanzai/internal/board"
	"github.com/cloudbusting/kanbanzai/internal/check"
	"github.com/cloudbusting/kanbanzai/internal/config"
	"github.com/cloudbusting/kanbanzai/internal/doctor"
	"github.com/cloudbusting/kanbanzai/internal/fsutil"
)

// ErrKilled, returned by Options.After, ends the apply on the spot with no rollback, as a killed process would.
// Tests use it to leave a half-applied project for Rollback.
var ErrKilled = errors.New("killed")

// JournalVersion is the version of the journal file.
const JournalVersion = 1

// Journal statuses.
const (
	StatusApplying   = "applying"
	StatusApplied    = "applied"
	StatusRolledBack = "rolled-back"
)

// Journal step: one file, recorded before it is written so a rollback knows what may have happened.
type Step struct {
	Kind      string `json:"kind"`
	Path      string `json:"path"`
	Backup    string `json:"backup,omitempty"`
	BeforeSHA string `json:"before_sha256,omitempty"`
	AfterSHA  string `json:"after_sha256"`
	Done      bool   `json:"done"`
}

// Journal is the record of an apply in progress, in <state>/migrate/journal.json beside the backups.
type Journal struct {
	SchemaVersion int    `json:"schema_version"`
	Plan          string `json:"plan_sha256"`
	Status        string `json:"status"`
	Steps         []Step `json:"steps"`
}

// Options are the parts of an apply a caller or a test can see into.
type Options struct {
	// Registry is the migrations to look for; nil is this release's Registry.
	Registry []Migration
	// After runs after each step, named by its kind and path, and after "verify". An error rolls the apply back;
	// ErrKilled stops it with no rollback.
	After func(step string) error
	// Doctor runs the installation check on the migrated project; nil runs the real one.
	Doctor func(root string) []doctor.Finding
}

// Result is what an apply or a rollback did.
type Result struct {
	Changed []string `json:"changed"`
	// Message is set when there was nothing to do (a rollback with nothing to roll back).
	Message string `json:"message,omitempty"`
}

// Refusal is an apply or rollback that was stopped before it wrote anything.
type Refusal struct{ Message string }

func (r *Refusal) Error() string { return r.Message }

func refuse(format string, args ...any) error { return &Refusal{Message: fmt.Sprintf(format, args...)} }

// Failed is an apply that went wrong after it began writing.
type Failed struct {
	Cause      error
	RolledBack bool
	Detail     string
}

func (f *Failed) Error() string {
	if f.RolledBack {
		return fmt.Sprintf("migration failed: %v; the project was put back to what it was", f.Cause)
	}
	return fmt.Sprintf("migration failed: %v; the project could NOT be fully put back: %s; fix it, then run `kanbanzai migrate --rollback`", f.Cause, f.Detail)
}

func (f *Failed) Unwrap() error { return f.Cause }

func stateDir(p *config.Project) string    { return filepath.Join(p.State, "migrate") }
func journalPath(p *config.Project) string { return filepath.Join(stateDir(p), "journal.json") }
func lockPath(p *config.Project) string    { return filepath.Join(p.State, "migrate.lock") }

func absolute(root, rel string) string { return filepath.Join(root, filepath.FromSlash(rel)) }

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

// refusals are the conditions that stop an apply before it writes.
func (p *Plan) refusals(sha string) error {
	if !p.Pending {
		return refuse("there is nothing to migrate")
	}
	if p.Recovery == "rollback-needed" {
		return refuse("an earlier migration did not finish: run `kanbanzai migrate --rollback` first")
	}
	if sha != p.SHA {
		return refuse("the plan is now %s, not %s: preview again with `kanbanzai migrate` and approve that plan", p.SHA, sha)
	}
	claims := board.LiveClaims(p.project)
	for _, s := range p.steps {
		if owner, claimed := claims[s.Card]; s.Card != "" && claimed {
			return refuse("%s is claimed by %s: wait for the claim to end, then preview again", s.Card, owner)
		}
	}
	return nil
}

// Apply carries out the plan the owner approved, named by its hash: backup, journal, each file, then check and
// doctor on the result. A failure after the first write puts every file back to the bytes it started with.
func Apply(start, sha string, options Options) (*Result, error) {
	if sha == "" {
		return nil, refuse("an apply needs the plan to approve: `kanbanzai migrate --apply --plan <hash>`, with the hash `kanbanzai migrate` printed")
	}
	plan, err := Preview(start, options.Registry)
	if err != nil {
		return nil, err
	}
	if err := plan.refusals(sha); err != nil {
		return nil, err
	}
	p := plan.project
	lock, err := fsutil.TryLock(lockPath(p))
	if err != nil {
		if errors.Is(err, fsutil.ErrLocked) {
			return nil, refuse("another `kanbanzai migrate` is running")
		}
		return nil, err
	}
	defer lock.Close()
	// again under the lock: nothing may have moved while waiting for it
	if plan, err = Preview(start, options.Registry); err != nil {
		return nil, err
	}
	if err := plan.refusals(sha); err != nil {
		return nil, err
	}

	backups := filepath.Join(stateDir(p), "backup", plan.SHA[:12])
	if err := os.RemoveAll(backups); err != nil {
		return nil, err
	}
	journal := &Journal{SchemaVersion: JournalVersion, Plan: plan.SHA, Status: StatusApplying}
	for index, s := range plan.steps {
		step := Step{Kind: s.Kind, Path: s.Path, BeforeSHA: s.beforeSHA(), AfterSHA: digest(s.After)}
		if s.Before != nil {
			step.Backup = fmt.Sprintf("%03d", index+1)
			if err := fsutil.WriteFileAtomic(filepath.Join(backups, step.Backup), s.Before, 0o600); err != nil {
				return nil, err
			}
		}
		journal.Steps = append(journal.Steps, step)
	}
	if err := writeJournal(journalPath(p), journal); err != nil {
		return nil, err
	}

	fail := func(cause error) (*Result, error) {
		if errors.Is(cause, ErrKilled) {
			return nil, cause
		}
		rolled, detail := rollBack(p.Root, journal, backups)
		if rolled {
			journal.Status = StatusRolledBack
			_ = writeJournal(journalPath(p), journal)
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
		step, s := &journal.Steps[index], plan.steps[index]
		current, readErr := os.ReadFile(absolute(p.Root, s.Path))
		if (s.Before == nil && readErr == nil) || (s.Before != nil && (readErr != nil || !bytes.Equal(current, s.Before))) {
			return fail(fmt.Errorf("%s changed during the apply", s.Path))
		}
		mode := s.Mode
		if mode == 0 {
			mode = 0o644
		}
		if err := fsutil.WriteFileAtomic(absolute(p.Root, s.Path), s.After, mode); err != nil {
			return fail(fmt.Errorf("%s: %v", s.Path, err))
		}
		step.Done = true
		if err := writeJournal(journalPath(p), journal); err != nil {
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
	for _, finding := range runDoctor(options, p.Root) {
		// Only what a migration changes can fail it: an environment error (an unwritable folder, no git) is the same
		// before and after, and is the owner's to fix.
		if finding.Status == "error" && (finding.ID == "config" || strings.HasPrefix(finding.ID, "folder.")) {
			return fail(fmt.Errorf("`kanbanzai doctor` reports an error after the migration: %s: %s", finding.ID, finding.Message))
		}
	}
	if err := after("verify"); err != nil {
		return fail(err)
	}
	journal.Status = StatusApplied
	if err := writeJournal(journalPath(p), journal); err != nil {
		return nil, err
	}
	result := &Result{Changed: []string{}}
	for _, step := range journal.Steps {
		result.Changed = append(result.Changed, step.Path)
	}
	return result, nil
}

func runDoctor(options Options, root string) []doctor.Finding {
	if options.Doctor != nil {
		return options.Doctor(root)
	}
	var out bytes.Buffer
	doctor.Run(root, doctor.Options{JSON: true, Out: &out})
	var report doctor.Report
	if json.Unmarshal(out.Bytes(), &report) != nil {
		return nil
	}
	return report.Findings
}

// rollBack puts every step's file back, last step first. It is safe to run twice. It refuses to overwrite a file
// that is neither what the apply wrote nor what was there before.
func rollBack(root string, journal *Journal, backups string) (bool, string) {
	var problems []string
	for index := len(journal.Steps) - 1; index >= 0; index-- {
		step := journal.Steps[index]
		path := absolute(root, step.Path)
		data, readErr := os.ReadFile(path)
		exists := readErr == nil
		if step.Backup == "" {
			// the file did not exist before: remove what the apply wrote
			if exists {
				if digest(data) != step.AfterSHA {
					problems = append(problems, step.Path+" holds something other than what the apply wrote")
				} else if err := os.Remove(path); err != nil {
					problems = append(problems, err.Error())
				}
			}
			continue
		}
		if exists && digest(data) == step.BeforeSHA {
			continue
		}
		if exists && digest(data) != step.AfterSHA {
			problems = append(problems, step.Path+" holds something other than what the apply wrote or what was there before")
			continue
		}
		old, err := os.ReadFile(filepath.Join(backups, step.Backup))
		if err != nil || digest(old) != step.BeforeSHA {
			problems = append(problems, "the backup of "+step.Path+" is missing or damaged")
			continue
		}
		if err := fsutil.WriteFileAtomic(path, old, 0o644); err != nil {
			problems = append(problems, err.Error())
		}
	}
	return len(problems) == 0, strings.Join(problems, "; ")
}

// Rollback undoes an apply that did not finish, from its journal.
func Rollback(start string) (*Result, error) {
	upgrade, err := config.PlanUpgrade(start)
	if err != nil {
		return nil, err
	}
	p := upgrade.Project
	if p == nil {
		return &Result{Changed: []string{}, Message: "There is nothing to roll back."}, nil
	}
	lock, err := fsutil.TryLock(lockPath(p))
	if err != nil {
		if errors.Is(err, fsutil.ErrLocked) {
			return nil, refuse("another `kanbanzai migrate` is running")
		}
		return nil, err
	}
	defer lock.Close()
	journal, err := readJournal(journalPath(p))
	if err != nil {
		if os.IsNotExist(err) {
			return &Result{Changed: []string{}, Message: "There is nothing to roll back."}, nil
		}
		return nil, refuse("%v", err)
	}
	if journal.Status != StatusApplying {
		return &Result{Changed: []string{}, Message: fmt.Sprintf("The last migration is %s: there is nothing to roll back (git can undo a finished one).", journal.Status)}, nil
	}
	if len(journal.Plan) < 12 {
		return nil, refuse("the journal is damaged: restore the files from git")
	}
	rolled, detail := rollBack(p.Root, journal, filepath.Join(stateDir(p), "backup", journal.Plan[:12]))
	if !rolled {
		return nil, &Failed{Cause: errors.New("rollback"), RolledBack: false, Detail: detail}
	}
	journal.Status = StatusRolledBack
	if err := writeJournal(journalPath(p), journal); err != nil {
		return nil, err
	}
	result := &Result{Changed: []string{}}
	for _, step := range journal.Steps {
		result.Changed = append(result.Changed, step.Path)
	}
	return result, nil
}
