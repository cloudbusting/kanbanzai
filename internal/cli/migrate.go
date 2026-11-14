package cli

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/cloudbusting/kanbanzai/internal/context"
	"github.com/cloudbusting/kanbanzai/internal/migrate"
)

// runMigrate is `kanbanzai migrate`. It runs before the project's config is loaded, because the config it
// migrates may be too old to load. Exit 0: done or nothing to do; 1: an error or a failed apply; 2: a usage error
// or a refusal, with nothing written.
func runMigrate(args []string, out, errOut io.Writer) int {
	positionals, flags, set, err := parse(args, map[string]bool{"json": true, "apply": true, "rollback": true},
		map[string]bool{"plan": true})
	if err != nil || len(positionals) != 0 {
		return usageError(errOut, "migrate", err)
	}
	_, planGiven := flags["plan"]
	switch {
	case set["apply"] && set["rollback"]:
		return usageError(errOut, "migrate", errors.New("--apply and --rollback cannot be used together"))
	case set["rollback"] && (set["json"] || planGiven):
		return usageError(errOut, "migrate", errors.New("--rollback takes no other option"))
	case planGiven && !set["apply"]:
		return usageError(errOut, "migrate", errors.New("--plan is for --apply"))
	case set["json"] && set["apply"]:
		return usageError(errOut, "migrate", errors.New("--json is for the preview"))
	}
	fail := func(err error) int {
		fmt.Fprintf(errOut, "kanbanzai: migrate: %v\n", err)
		var refusal *migrate.Refusal
		if errors.As(err, &refusal) {
			return 2
		}
		return 1
	}
	switch {
	case set["rollback"]:
		result, err := migrate.Rollback("")
		if err != nil {
			return fail(err)
		}
		if result.Message != "" {
			fmt.Fprintln(out, result.Message)
			return 0
		}
		fmt.Fprintf(out, "Rolled back: restored %s.\n", strings.Join(result.Changed, ", "))
		return 0
	case set["apply"]:
		result, err := migrate.Apply("", flags["plan"], migrate.Options{})
		if err != nil {
			return fail(err)
		}
		fmt.Fprintf(out, "Migrated: wrote %s.\nThe old files are backed up under the project's state folder (migrate/backup). Nothing is committed: review the changes and commit them when you are ready.\n",
			strings.Join(result.Changed, ", "))
		return 0
	}
	plan, err := migrate.Preview("", nil)
	if err != nil {
		return fail(err)
	}
	if set["json"] {
		data, err := plan.JSON()
		if err != nil {
			return fail(err)
		}
		out.Write(data)
		return 0
	}
	fmt.Fprint(out, plan.Text())
	return 0
}

// migrationNotice is the one line `context` starts with when a migration is waiting or unfinished; "" when not.
func migrationNotice() string {
	plan, err := migrate.Preview("", nil)
	if err != nil {
		return ""
	}
	ids := make([]string, 0, len(plan.Migrations))
	for _, m := range plan.Migrations {
		ids = append(ids, m.ID)
	}
	return context.MigrationNotice(ids, plan.Recovery == "rollback-needed")
}
