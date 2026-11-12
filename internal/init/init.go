// Package initcmd sets a project up for Kanbanzai.
//
// Init is one pipeline: detect the checkout, build a plan, apply the plan (or only show it), then verify the board
// it left. The plan is a single JSON value that is the only source of writes, so a dry run cannot disagree with the
// real run, and `--plan FILE` applies exactly a plan that was shown. Init creates what is missing and never
// overwrites: an existing config is kept, and replaced only with --force, after a backup. It never commits.
package initcmd

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/cloudbusting/kanbanzai/internal/fsutil"
)

// Options is one init command.
type Options struct {
	DryRun       bool
	JSON         bool
	Force        bool
	WithTemplate bool
	PlanFile     string
	MainBranch   string
	Owner        string
	Prefix       string
	Context      []string
	Plugins      []string
	PluginSource string
	Out, Err     io.Writer

	// Test hooks.
	clock func() time.Time
	after func(index int) error
}

func (o Options) now() time.Time {
	if o.clock != nil {
		return o.clock()
	}
	return time.Now()
}

// Report is the JSON that `init --json` prints. A dry run has only Plan; a real run adds Result and Verify.
type Report struct {
	SchemaVersion int           `json:"schema_version"`
	Mode          string        `json:"mode"`
	Plan          *Plan         `json:"plan"`
	Result        *Result       `json:"result,omitempty"`
	Verify        *Verification `json:"verify,omitempty"`
}

// Run is the init command for the checkout that start belongs to. Exit 0: shown, nothing to do, or applied with
// `check` passing. 1: a write failed or `check` is red. 2: a usage error or a refusal (nothing was written).
func Run(start string, opts Options) int {
	out, errOut := opts.Out, opts.Err
	if out == nil {
		out = io.Discard
	}
	if errOut == nil {
		errOut = io.Discard
	}
	fail := func(code int, err error) int {
		fmt.Fprintf(errOut, "kanbanzai: init: %v\n", err)
		return code
	}
	detected, existing, err := Detect(start)
	if err != nil {
		return fail(2, err)
	}
	var plan *Plan
	if opts.PlanFile != "" {
		if plan, err = LoadPlan(opts.PlanFile); err == nil {
			err = ValidatePlan(plan, detected)
		}
	} else {
		plan, err = Build(detected, existing, opts)
		if err == nil {
			err = ValidatePlan(plan, detected)
		}
	}
	if err != nil {
		if IsRefusal(err) {
			return fail(2, err)
		}
		return fail(1, err)
	}
	report := &Report{SchemaVersion: 1, Mode: "apply", Plan: plan}
	if opts.DryRun {
		report.Mode = "dry-run"
		if err := Preflight(plan); err != nil {
			return fail(2, err)
		}
		return emit(out, errOut, report, opts)
	}
	result, err := Apply(plan, opts.after)
	report.Result = result
	if err != nil {
		if _, isStale := err.(*StaleError); isStale {
			return fail(2, err)
		}
		emit(out, errOut, report, opts)
		fmt.Fprintf(errOut, "kanbanzai: init stopped: %v\nwhat was made is listed above; run init again to finish\n", err)
		return 1
	}
	report.Verify = Verify(start, plan)
	code := emit(out, errOut, report, opts)
	if code == 0 && !report.Verify.Check.OK {
		return 1
	}
	return code
}

func emit(out, errOut io.Writer, report *Report, opts Options) int {
	if opts.JSON {
		data, err := fsutil.MarshalIndent(report)
		if err != nil {
			fmt.Fprintf(errOut, "kanbanzai: init: %v\n", err)
			return 1
		}
		_, _ = out.Write(append(data, '\n'))
		return 0
	}
	render(out, report)
	return 0
}

var dryVerb = map[string]string{OpMkdir: "would make", OpCreate: "would make", OpKeep: "kept",
	OpAppend: "would append", OpBackup: "would back up", OpReplace: "would replace", OpMerge: "would merge into"}

func render(out io.Writer, report *Report) {
	plan := report.Plan
	fmt.Fprintf(out, "kanbanzai: project %s\n", plan.Root)
	if plan.Detected.RanFromWorktree {
		fmt.Fprintf(out, "  run from the worktree %s; init works on the main checkout\n", plan.Detected.Toplevel)
	}
	if plan.Settings != nil {
		fmt.Fprintf(out, "  main branch: %s (%s)\n  owner:       %s (%s)\n", plan.Settings.MainBranch,
			plan.Settings.MainBranchSource, plan.Settings.Owner, plan.Settings.OwnerSource)
		if len(plan.Settings.Types) > 0 {
			fmt.Fprintf(out, "  issue types: %s (one global number sequence)\n",
				strings.Join(plan.Settings.Types, ", "))
		}
	}
	for _, warning := range plan.Warnings {
		fmt.Fprintf(out, "  note: %s\n", warning)
	}
	changed := 0
	if report.Result != nil {
		for _, applied := range report.Result.Applied {
			fmt.Fprintf(out, "  %-8s %s\n", applied.Status, applied.Path)
			if applied.Status != "kept" {
				changed++
			}
		}
	} else {
		for _, action := range plan.Actions {
			fmt.Fprintf(out, "  %-8s %s\n", dryVerb[action.Op], action.Path)
			if action.Op != OpKeep {
				changed++
			}
		}
	}
	switch {
	case changed == 0:
		fmt.Fprintln(out, "  nothing to do: already set up")
	case report.Result == nil:
		fmt.Fprintln(out, "  dry run: nothing was written")
	}
	if report.Verify != nil {
		renderVerify(out, report.Verify)
	}
}
