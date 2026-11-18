// tools/migrate-board moves this project's cards from a files board (the kanbanzai-board branch's work folder) into the
// store its config now names (GitHub issues, over the kanbanzai-store-github plugin), keeping each card's type prefix
// and rewriting every reference between cards to the new ids in one pass. It is a one-shot tool for this repository,
// not part of the shipped command: docs/contributing.md has the steps it belongs to.
//
//	go run ./tools/migrate-board --from .kanbanzai/work --dry-run   what it would do; writes nothing
//	go run ./tools/migrate-board --from .kanbanzai/work             prints that, and stops
//	go run ./tools/migrate-board --from .kanbanzai/work --yes       does it
//
// It only reads --from. The store's issues are public when the repository is: it makes them only with --yes.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/cloudbusting/kanbanzai/internal/config"
	"github.com/cloudbusting/kanbanzai/internal/store/plugin"
	"github.com/cloudbusting/kanbanzai/internal/store/stores"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, out, errOut io.Writer) int {
	flags := flag.NewFlagSet("migrate-board", flag.ContinueOnError)
	flags.SetOutput(errOut)
	from := flags.String("from", "", "the old board's work folder (holds 1-open, 10-done, x-paused); required")
	dry := flags.Bool("dry-run", false, "print what would happen and write nothing")
	yes := flags.Bool("yes", false, "make the issues (public when the repository is); without it the run stops after the report")
	legacy := flags.String("legacy-type", "", "the type key a legacy card (W-NN) takes; default: the project's default type")
	pace := flags.Duration("pace", time.Second, "the wait after every write to the store, to stay under GitHub's limits")
	settleFor := flags.Duration("settle-for", 3*time.Minute, "how long to wait for the store's search to show every new card")
	journalPath := flags.String("journal", "", "the journal that makes a stopped run resumable (default: under the project's state folder)")
	mapPath := flags.String("map", "", "where to write the old-to-new id table (default: beside the journal)")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *from == "" || flags.NArg() != 0 {
		fmt.Fprintln(errOut, "usage: migrate-board --from <old work folder> [--dry-run | --yes] [--legacy-type KEY] [--pace 1s] [--journal FILE] [--map FILE]")
		return 2
	}
	project, err := config.Find(".")
	if err != nil {
		fmt.Fprintf(errOut, "migrate-board: %v\n", err)
		return 1
	}
	source, err := filepath.Abs(*from)
	if err != nil {
		fmt.Fprintf(errOut, "migrate-board: %v\n", err)
		return 1
	}
	if *legacy == "" {
		*legacy = project.DefaultType
	}
	p, err := readPlan(project, source, *legacy)
	if err != nil {
		fmt.Fprintf(errOut, "migrate-board: %v\n", err)
		return 1
	}
	say := func(format string, args ...any) { fmt.Fprintf(out, format+"\n", args...) }
	problems := p.report(say, liveClaims(project, source))
	if *dry || !*yes {
		if !*dry {
			say("\nnothing was written: run again with --yes to make the issues")
		}
		if problems > 0 {
			return 1
		}
		return 0
	}
	if problems > 0 {
		fmt.Fprintln(errOut, "migrate-board: fix the problems above first")
		return 1
	}
	if !project.StoreTypedIDs() {
		fmt.Fprintln(errOut, "migrate-board: this project's [store] has no plugin, or names a prefix: the new ids are the types' "+
			"prefixes, so the config must have [store] plugin = \"github\" and no prefix (docs/contributing.md)")
		return 1
	}
	backend, err := plugin.Open(project, errOut)
	if err != nil {
		fmt.Fprintf(errOut, "migrate-board: %v\n", err)
		return 1
	}
	if *journalPath == "" {
		*journalPath = filepath.Join(project.State, "migrate-board.json")
	}
	if *mapPath == "" {
		*mapPath = filepath.Join(filepath.Dir(*journalPath), "migrate-board-id-map.md")
	}
	jr, err := loadJournal(*journalPath)
	if err != nil {
		fmt.Fprintf(errOut, "migrate-board: %v\n", err)
		return 1
	}
	opts := options{Query: project.StoreQuery, Pace: *pace, SettleEvery: 5 * time.Second, SettleFor: *settleFor, Out: out,
		JournalPath: *journalPath, MapPath: *mapPath}
	if err := migrate(p, stores.For(project, nil), backend, jr, opts); err != nil {
		fmt.Fprintf(errOut, "migrate-board: %v\n", err)
		return 1
	}
	return 0
}
