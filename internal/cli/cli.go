// Package cli is the kanbanzai command surface: one parser, the project lookup, and the dispatch to the core.
package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/cloudbusting/kanbanzai/internal/board"
	"github.com/cloudbusting/kanbanzai/internal/check"
	"github.com/cloudbusting/kanbanzai/internal/claim"
	"github.com/cloudbusting/kanbanzai/internal/config"
	"github.com/cloudbusting/kanbanzai/internal/context"
	"github.com/cloudbusting/kanbanzai/internal/doctor"
	"github.com/cloudbusting/kanbanzai/internal/fsutil"
	initcmd "github.com/cloudbusting/kanbanzai/internal/init"
	"github.com/cloudbusting/kanbanzai/internal/move"
	"github.com/cloudbusting/kanbanzai/internal/number"
	sessionstate "github.com/cloudbusting/kanbanzai/internal/sessions"
	"github.com/cloudbusting/kanbanzai/internal/step"
	"github.com/cloudbusting/kanbanzai/internal/version"
	"github.com/cloudbusting/kanbanzai/internal/wip"
)

const usage = `usage: kanbanzai <command> [options]

commands:
  board [--json]                 the board
  wip [--next|--title ID|--audit [--card ID]|--cleanup]
  claim <action> ...             owner, status, claim, renew, release, takeover, heartbeat
  move ID TARGET [options]       move a card in one step
  number [PREFIX] [options]      allocate the next card number
  check                          validate every brief
  step NAME | --for ID           what runs a step
  sessions [--json]              the claimed cards with a live session
  config [--json]                the versioned, resolved settings
  config upgrade [--apply]       preview/apply a safe config schema upgrade
  doctor [--json]                diagnose this installation without repairs
  context                        the standing context
  init [--dry-run] [--json] [--plan FILE] [--force] [--main-branch NAME] [--owner NAME] [--with-template]
                                 set a project up (detect, plan, apply, verify; never commits)

  --version                      print the version
  --help                         this help
`

// Run is the command. It parses first, as the Python command did, so an unknown command or flag is exit 2 even
// outside a project; then it finds the project and dispatches.
func Run(args []string, in io.Reader, out, errOut io.Writer) int {
	if out == nil {
		out = io.Discard
	}
	if errOut == nil {
		errOut = io.Discard
	}
	if len(args) == 0 {
		fmt.Fprintln(errOut, "kanbanzai: a command is needed (--help)")
		return 2
	}
	switch args[0] {
	case "--version", "-version":
		fmt.Fprintln(out, version.String())
		return 0
	case "--help", "-h", "help":
		fmt.Fprint(out, usage)
		return 0
	}
	command := args[0]
	rest := args[1:]
	if hasHelp(rest) {
		fmt.Fprintf(out, "usage: kanbanzai %s [options]\n", command)
		return 0
	}
	if command == "config" && len(rest) > 0 && rest[0] == "upgrade" {
		positionals, _, set, err := parse(rest[1:], map[string]bool{"apply": true}, nil)
		if err != nil || len(positionals) != 0 {
			return usageError(errOut, "config upgrade", err)
		}
		text, err := config.Upgrade("", set["apply"])
		if err != nil {
			fmt.Fprintf(errOut, "kanbanzai: config upgrade: %v\n", err)
			return 1
		}
		fmt.Fprintln(out, text)
		return 0
	}
	if command == "doctor" {
		positionals, _, set, err := parse(rest, map[string]bool{"json": true}, nil)
		if err != nil || len(positionals) != 0 {
			return usageError(errOut, command, err)
		}
		return doctor.Run("", doctor.Options{JSON: set["json"], Out: out, Err: errOut})
	}
	if command == "init" {
		positionals, flags, set, err := parse(rest, map[string]bool{"dry-run": true, "json": true, "force": true,
			"with-template": true}, map[string]bool{"plan": true, "main-branch": true, "owner": true})
		if err != nil || len(positionals) != 0 {
			return usageError(errOut, command, err)
		}
		if _, planned := flags["plan"]; planned {
			for _, name := range []string{"main-branch", "owner"} {
				if _, given := flags[name]; given {
					return usageError(errOut, command, fmt.Errorf("--plan applies a plan as it is: --%s cannot be "+
						"combined with it", name))
				}
			}
			if set["force"] || set["with-template"] {
				return usageError(errOut, command, fmt.Errorf("--plan applies a plan as it is: --force and "+
					"--with-template cannot be combined with it"))
			}
		}
		return initcmd.Run("", initcmd.Options{DryRun: set["dry-run"], JSON: set["json"], Force: set["force"],
			WithTemplate: set["with-template"], PlanFile: flags["plan"], MainBranch: flags["main-branch"],
			Owner: flags["owner"], Out: out, Err: errOut})
	}
	quiet := command == "context" || command == "sessions" || (command == "claim" && len(rest) > 0 &&
		rest[0] == "heartbeat")
	project, err := config.Find("")
	if err != nil {
		if config.IsNoProject(err) {
			if quiet {
				return 0
			}
			fmt.Fprintf(errOut, "kanbanzai: %v\n", err)
			return 1
		}
		var configErr *config.ConfigError
		if errors.As(err, &configErr) {
			if command == "context" {
				fmt.Fprintf(out, "Kanbanzai: this project's config cannot be used: %v\n", err)
				return 0
			}
			if quiet {
				return 0
			}
			fmt.Fprintf(errOut, "kanbanzai: config: %v\n", err)
			return 1
		}
		fmt.Fprintf(errOut, "kanbanzai: %v\n", err)
		return 1
	}
	return dispatch(project, command, rest, in, out, errOut)
}

func dispatch(project *config.Project, command string, args []string, in io.Reader, out, errOut io.Writer) int {
	switch command {
	case "board":
		positionals, flags, set, err := parse(args, map[string]bool{"json": true}, nil)
		if err != nil || len(positionals) != 0 {
			return usageError(errOut, command, err)
		}
		loaded := board.Load(project, nil, nil)
		if set["json"] {
			writeCompact(out, loaded)
			return 0
		}
		fmt.Fprintln(out, board.AsText(loaded, project.Owner))
		_ = flags
		return 0
	case "wip":
		positionals, flags, set, err := parse(args, map[string]bool{"audit": true, "cleanup": true, "next": true},
			map[string]bool{"title": true, "card": true})
		if err != nil || len(positionals) != 0 {
			return usageError(errOut, command, err)
		}
		return wip.Run(project, wip.Options{Audit: set["audit"], Cleanup: set["cleanup"], Next: set["next"],
			Title: flags["title"], Card: flags["card"], Out: out, Err: errOut})
	case "claim":
		return runClaim(project, args, in, out, errOut)
	case "move":
		positionals, flags, set, err := parse(args, map[string]bool{"clear-held": true, "force": true},
			map[string]bool{"owner": true, "next": true, "where": true, "held": true, "progress": true,
				"done": true, "reason": true})
		if err != nil || len(positionals) != 2 {
			return usageError(errOut, command, err)
		}
		return move.Run(project, move.Options{
			Card: positionals[0], Target: positionals[1], Owner: flags["owner"],
			Next: ptr(flags, "next"), Where: ptr(flags, "where"), Held: ptr(flags, "held"),
			Progress: ptr(flags, "progress"), Done: ptr(flags, "done"), Reason: ptr(flags, "reason"),
			ClearHeld: set["clear-held"], Force: set["force"], Out: out, Err: errOut,
		})
	case "number":
		positionals, flags, set, err := parse(args, map[string]bool{"peek": true, "status": true},
			map[string]bool{"release": true, "mark": true, "note": true})
		if err != nil || len(positionals) > 1 {
			return usageError(errOut, command, err)
		}
		prefix := ""
		if len(positionals) == 1 {
			prefix = positionals[0]
		}
		return number.Run(project, number.Options{Prefix: prefix, Peek: set["peek"], Status: set["status"],
			Release: flags["release"], Mark: flags["mark"], Note: flags["note"], Out: out, Err: errOut})
	case "check":
		positionals, _, _, err := parse(args, nil, nil)
		if err != nil || len(positionals) != 0 {
			return usageError(errOut, command, err)
		}
		return check.Run(project, out)
	case "step":
		positionals, flags, set, err := parse(args, map[string]bool{"json": true}, map[string]bool{"for": true})
		if err != nil || len(positionals) > 1 {
			return usageError(errOut, command, err)
		}
		name := ""
		if len(positionals) == 1 {
			name = positionals[0]
		}
		return step.Run(project, step.Options{Name: name, For: flags["for"], JSON: set["json"], Out: out,
			Err: errOut})
	case "sessions":
		positionals, _, set, err := parse(args, map[string]bool{"json": true}, nil)
		if err != nil || len(positionals) != 0 {
			return usageError(errOut, command, err)
		}
		return runSessions(project, set["json"], out)
	case "config":
		positionals, _, set, err := parse(args, map[string]bool{"json": true}, nil)
		if err != nil || len(positionals) != 0 {
			return usageError(errOut, command, err)
		}
		if set["json"] {
			data, err := fsutil.MarshalIndent(project.AsDict())
			if err != nil {
				fmt.Fprintf(errOut, "kanbanzai: config: %v\n", err)
				return 1
			}
			_, _ = out.Write(append(data, '\n'))
			return 0
		}
		showConfig(project, out)
		return 0
	case "context":
		positionals, _, _, err := parse(args, nil, nil)
		if err != nil || len(positionals) != 0 {
			return usageError(errOut, command, err)
		}
		return context.Run(project, out)
	}
	fmt.Fprintf(errOut, "kanbanzai: no such command '%s' (--help)\n", command)
	return 2
}

func runClaim(project *config.Project, args []string, in io.Reader, out, errOut io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(errOut, "kanbanzai: claim needs an action: owner, status, claim, renew, release, takeover, "+
			"heartbeat")
		return 2
	}
	action := args[0]
	rest := args[1:]
	bools := map[string]bool{}
	values := map[string]bool{"owner": true}
	switch action {
	case "owner":
	case "status":
	case "claim":
		values["minutes"], values["note"], values["branch"], values["takeover"] = true, true, true, true
		bools["no-move"] = true
	case "renew":
		values["minutes"] = true
	case "release":
	case "takeover":
		values["minutes"], values["note"], values["branch"], values["reason"] = true, true, true, true
		bools["no-move"] = true
	case "heartbeat":
		values["minutes"] = true
		bools["hook"] = true
	default:
		fmt.Fprintf(errOut, "kanbanzai: claim has no action '%s' (owner, status, claim, renew, release, takeover, "+
			"heartbeat)\n", action)
		return 2
	}
	positionals, flags, set, err := parse(rest, bools, values)
	if err != nil {
		return usageError(errOut, "claim "+action, err)
	}
	card := ""
	needCard := action == "claim" || action == "renew" || action == "release" || action == "takeover"
	if needCard {
		if len(positionals) != 1 {
			return usageError(errOut, "claim "+action, errors.New("a card id is needed"))
		}
		card = positionals[0]
	} else if action == "status" {
		if len(positionals) > 1 {
			return usageError(errOut, "claim status", errors.New("too many arguments"))
		}
		if len(positionals) == 1 {
			card = positionals[0]
		}
	} else if len(positionals) != 0 {
		return usageError(errOut, "claim "+action, errors.New("no positional argument is expected"))
	}
	if action == "takeover" && flags["reason"] == "" {
		return usageError(errOut, "claim takeover", errors.New("--reason is needed"))
	}
	minutes, err := floatPtr(flags, "minutes")
	if err != nil {
		return usageError(errOut, "claim "+action, err)
	}
	options := claim.ClaimOptions{Card: card, Owner: flags["owner"], Minutes: minutes, Note: flags["note"],
		NoMove: set["no-move"], Takeover: flags["takeover"], Out: out, Err: errOut,
		Start: func(card, owner string) (string, error) { return move.StartStep(project, card, owner) }}
	if branch, ok := flags["branch"]; ok {
		options.Branch = &branch
	}
	switch action {
	case "owner":
		return claim.Owner(project, flags["owner"], out)
	case "status":
		return claim.Status(project, flags["owner"], card, out, errOut)
	case "claim":
		return claim.Claim(project, options)
	case "renew":
		return claim.Renew(project, flags["owner"], card, minutes, out, errOut)
	case "release":
		return claim.Release(project, flags["owner"], card, out, errOut)
	case "takeover":
		return claim.Takeover(project, options, flags["reason"])
	case "heartbeat":
		return claim.Heartbeat(project, claim.HeartbeatOptions{Owner: flags["owner"], Minutes: minutes,
			Hook: set["hook"], In: in})
	}
	return 2
}

func runSessions(project *config.Project, asJSON bool, out io.Writer) int {
	claims := board.LiveClaims(project)
	cards := sessionstate.ForClaims(claims, project)
	waiting := 0
	for _, record := range cards {
		if record.State == "waiting" {
			waiting++
		}
	}
	if asJSON {
		writeCompact(out, map[string]any{"waiting": waiting, "claims": len(claims), "cards": cards})
		return 0
	}
	keys := make([]string, 0, len(cards))
	for card := range cards {
		keys = append(keys, card)
	}
	sortStrings(keys)
	for _, card := range keys {
		record := cards[card]
		name := record.Name
		if name == "" {
			name = "unnamed"
		}
		fmt.Fprintf(out, "%s: %s (%s)\n", card, record.State, name)
	}
	if len(cards) == 0 {
		fmt.Fprintln(out, "no claimed card has a live session")
	}
	return 0
}

func showConfig(project *config.Project, out io.Writer) {
	configFile := project.ConfigFile
	if configFile == "" {
		configFile = "none (the shipped defaults)"
	}
	fmt.Fprintf(out, "project root:   %s\n", project.Root)
	fmt.Fprintf(out, "this worktree:  %s\n", project.Toplevel)
	fmt.Fprintf(out, "config file:    %s\n", configFile)
	fmt.Fprintf(out, "config_version: %d\n", project.ConfigVersion)
	values := []struct {
		name  string
		value string
	}{
		{"prefix", project.Prefix},
		{"digits", strconv.Itoa(project.Digits)},
		{"owner", project.Owner},
		{"main_branch", project.MainBranch},
		{"home", project.Home},
		{"work", project.Work},
		{"state", project.State},
		{"claims", project.Claims},
		{"numbers", project.Numbers},
		{"batches", project.Batches},
		{"template", project.Template},
		{"worktree", project.WorktreePattern},
		{"branch", project.BranchPattern},
		{"batch_branch", project.BatchBranchPattern},
		{"context", strings.Join(project.ContextFiles(), ", ")},
	}
	for _, item := range values {
		fmt.Fprintf(out, "  %-14s %s\n", item.name+":", item.value)
	}
	fmt.Fprintf(out, "  claims.lease_minutes: %g\n", project.LeaseMinutes)
	fmt.Fprintf(out, "  claims.owner_env: %s\n", project.OwnerEnv)
	fmt.Fprintf(out, "  flow.wip_limit: %d\n", project.WIPLimit)
	fmt.Fprintf(out, "  flow.stale_days: %d\n", project.StaleDays)
	fmt.Fprintf(out, "  flow.move_grace_minutes: %g\n", project.MoveGraceMinutes)
	fmt.Fprintf(out, "  flow.pull_order: %s\n", strings.Join(project.PullOrder, ", "))
	fmt.Fprintln(out, "columns:")
	for _, column := range project.Columns {
		step := "next: " + column.Next
		if column.Doing != "" {
			step = "doing: " + column.Doing
		} else if column.Next == "" {
			step = "next: —"
		}
		lane := ""
		if column.Lane != "" {
			lane = "  lane: " + column.Lane
		}
		fmt.Fprintf(out, "  %-12s %-20s %-7s %-10s %s%s\n", column.Key, column.Label, column.Kind,
			column.Folder, step, lane)
	}
	fmt.Fprintln(out, "steps:")
	for _, name := range project.StepOrder {
		item := project.Steps[name]
		note := ""
		if !item.Active {
			note = "  (inactive: its column is not on the board)"
		}
		skill := item.Skill
		if skill == "" {
			skill = "—"
		}
		fmt.Fprintf(out, "  %-10s into %-12s skill: %s%s\n", name, item.Into, skill, note)
	}
}

func parse(args []string, bools, values map[string]bool) (positionals []string, flags map[string]string,
	set map[string]bool, err error) {
	flags = map[string]string{}
	set = map[string]bool{}
	for index := 0; index < len(args); {
		arg := args[index]
		if arg == "--" {
			positionals = append(positionals, args[index+1:]...)
			break
		}
		if strings.HasPrefix(arg, "--") {
			name := strings.TrimPrefix(arg, "--")
			if equals := strings.IndexByte(name, '='); equals >= 0 {
				value := name[equals+1:]
				name = name[:equals]
				if !values[name] {
					if bools[name] {
						return nil, nil, nil, fmt.Errorf("--%s takes no value", name)
					}
					return nil, nil, nil, fmt.Errorf("unknown flag --%s", name)
				}
				flags[name] = value
				index++
				continue
			}
			if bools[name] {
				set[name] = true
				index++
				continue
			}
			if values[name] {
				if index+1 >= len(args) {
					return nil, nil, nil, fmt.Errorf("--%s needs a value", name)
				}
				flags[name] = args[index+1]
				index += 2
				continue
			}
			return nil, nil, nil, fmt.Errorf("unknown flag --%s", name)
		}
		if strings.HasPrefix(arg, "-") && arg != "-" {
			return nil, nil, nil, fmt.Errorf("unknown flag %s", arg)
		}
		positionals = append(positionals, arg)
		index++
	}
	return positionals, flags, set, nil
}

func hasHelp(args []string) bool {
	for _, arg := range args {
		if arg == "--help" || arg == "-h" {
			return true
		}
	}
	return false
}

func usageError(errOut io.Writer, command string, err error) int {
	if err != nil {
		fmt.Fprintf(errOut, "kanbanzai: %s: %v\n", command, err)
	} else {
		fmt.Fprintf(errOut, "kanbanzai: %s: bad arguments (--help)\n", command)
	}
	return 2
}

func ptr(flags map[string]string, name string) *string {
	value, ok := flags[name]
	if !ok {
		return nil
	}
	return &value
}

func floatPtr(flags map[string]string, name string) (*float64, error) {
	value, ok := flags[name]
	if !ok {
		return nil, nil
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return nil, fmt.Errorf("--%s must be a number", name)
	}
	return &parsed, nil
}

func writeCompact(out io.Writer, value any) {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return
	}
	_, _ = out.Write(buffer.Bytes())
}

func sortStrings(values []string) {
	for index := 1; index < len(values); index++ {
		for current := index; current > 0 && values[current] < values[current-1]; current-- {
			values[current], values[current-1] = values[current-1], values[current]
		}
	}
}
