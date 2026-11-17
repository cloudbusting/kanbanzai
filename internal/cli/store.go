package cli

import (
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/cloudbusting/kanbanzai/internal/brief"
	"github.com/cloudbusting/kanbanzai/internal/config"
	cardstore "github.com/cloudbusting/kanbanzai/internal/store"
	"github.com/cloudbusting/kanbanzai/internal/store/mirror"
	"github.com/cloudbusting/kanbanzai/internal/store/plugin"
	"github.com/cloudbusting/kanbanzai/internal/store/protocol"
	"github.com/cloudbusting/kanbanzai/internal/store/reference"
	"github.com/cloudbusting/kanbanzai/internal/store/stores"
	"github.com/cloudbusting/kanbanzai/internal/timeutil"
)

const storeUsage = `usage: kanbanzai store serve | trust | status
  serve    the built-in reference plugin: answer one protocol request from standard input
  trust    trust the store plugin this project's [store] names, as it is now, in this clone
  status   which store keeps the cards, whether its plugin is trusted, and when it last synced
`

// runStoreServe is the reference plugin. It needs no project: the request carries its settings.
func runStoreServe(in io.Reader, out, errOut io.Writer) int {
	if err := protocol.Serve(in, out, reference.Open); err != nil {
		fmt.Fprintf(errOut, "kanbanzai: store serve: %v\n", err)
		return 1
	}
	return 0
}

func runStore(project *config.Project, args []string, out, errOut io.Writer) int {
	positionals, _, _, err := parse(args, nil, nil)
	if err != nil || len(positionals) != 1 {
		fmt.Fprint(errOut, storeUsage)
		return 2
	}
	switch positionals[0] {
	case "trust":
		if !project.RemoteStore() {
			fmt.Fprintln(errOut, "kanbanzai: store trust: this project keeps its cards in files; there is no plugin to trust")
			return 2
		}
		program, err := plugin.Find(project)
		if err != nil {
			fmt.Fprintf(errOut, "kanbanzai: store trust: %v\n", err)
			return 1
		}
		if program.BuiltIn {
			fmt.Fprintf(out, "%s is built in (%s store serve): it needs no trust\n", program.Name, program.Path)
			return 0
		}
		record, err := plugin.Trust(project, program, timeutil.ToISO(timeutil.Now()))
		if err != nil {
			fmt.Fprintf(errOut, "kanbanzai: store trust: %v\n", err)
			return 1
		}
		fmt.Fprintf(out, "trusted %s: %s\nsha256 %s\nrecorded in %s (this clone only); a changed program needs trust again\n",
			program.Name, record.Path, record.SHA256, project.Rel(plugin.TrustFile(project)))
		return 0
	case "status":
		if !project.RemoteStore() {
			fmt.Fprintln(out, "store: files (the project's own briefs)")
			return 0
		}
		fmt.Fprintf(out, "store: %s (prefix %s)\n", project.StorePlugin, project.StorePrefix)
		trusted := "yes"
		if program, err := plugin.Find(project); err != nil {
			trusted = "no — " + err.Error()
		} else if err := plugin.Trusted(project, program); err != nil {
			trusted = "no — " + err.Error()
		}
		fmt.Fprintf(out, "trusted: %s\n", trusted)
		query := project.StoreQuery
		if query == "" {
			query = "(none: every ticket)"
		}
		fmt.Fprintf(out, "query: %s\n", query)
		synced := mirror.SyncedAt(project)
		if synced == "" {
			synced = "never: run `kanbanzai sync`"
		}
		fmt.Fprintf(out, "synced: %s\nworking copy: %s\n", synced, project.Rel(project.Work))
		for _, line := range storeFlags(project) {
			fmt.Fprintln(out, line)
		}
		return 0
	}
	fmt.Fprint(errOut, storeUsage)
	return 2
}

// storeFlags is one line per card the last sync or write flagged.
func storeFlags(project *config.Project) []string {
	var lines []string
	for id, state := range mirror.States(project) {
		if state.Conflict {
			lines = append(lines, fmt.Sprintf("conflict: %s changed both here and in %s; keep your edit somewhere, delete its "+
				"working copy and run `kanbanzai sync` to take the store's", id, project.StorePlugin))
		}
		if state.LeftQuery {
			lines = append(lines, fmt.Sprintf("left the query: %s no longer matches the query and stays while it is in flight", id))
		}
	}
	sort.Strings(lines)
	return lines
}

// runStoreSync is `kanbanzai sync` for a store plugin: fetch, publish hand edits, and report what needs a person.
func runStoreSync(project *config.Project, out, errOut io.Writer) int {
	cards := stores.For(project, nil)
	if err := cards.Ready(); err != nil {
		fmt.Fprintf(errOut, "kanbanzai: sync: %v\n", err)
		return 1
	}
	if err := cards.Sync(); err != nil {
		fmt.Fprintf(errOut, "kanbanzai: sync: %v\n", err)
		return 1
	}
	entries, _ := cards.List()
	fmt.Fprintf(out, "synced %d card(s) with %s\n", len(entries), project.StorePlugin)
	for _, line := range storeFlags(project) {
		fmt.Fprintln(out, line)
	}
	return 0
}

// storeReady stops a command that needs the plugin before it does anything, saying why.
func storeReady(project *config.Project, command string, errOut io.Writer) bool {
	if !project.RemoteStore() {
		return true
	}
	if err := stores.For(project, nil).Ready(); err != nil {
		fmt.Fprintf(errOut, "kanbanzai: %s: %v\n", command, err)
		return false
	}
	return true
}

// newRemoteCard asks the store plugin for a new card of a type, from the type's template, and prints its id.
func newRemoteCard(project *config.Project, typeKey, title string, out, errOut io.Writer) int {
	if strings.TrimSpace(title) == "" || strings.ContainsAny(title, "\r\n") {
		fmt.Fprintln(errOut, "kanbanzai: number: a store plugin issues the id when it creates the ticket: give --title \"<one line>\"")
		return 2
	}
	if !storeReady(project, "number", errOut) {
		return 1
	}
	item, ok := templateType(project, typeKey)
	if typeKey == "" {
		item, ok = templateType(project, project.DefaultType)
	}
	if !ok {
		fmt.Fprintf(errOut, "kanbanzai: number: %q is not a type of this project\n", typeKey)
		return 2
	}
	text, _, err := project.TemplateTextFor(item)
	if err != nil {
		fmt.Fprintf(errOut, "kanbanzai: number: %v\n", err)
		return 1
	}
	_, rest, _ := strings.Cut(text, "\n")
	text = "# " + cardstore.IDPlaceholder + ": " + strings.TrimSpace(title) + "\n" + rest
	first, today, typeName := project.Columns[0].Key, time.Now().UTC().Format("2006-01-02"), item.Key
	text = brief.SetFields(text, map[string]*string{"Stage": &first, "Updated": &today, "Type": &typeName})
	entry, err := stores.For(project, nil).Create(item.Key, strings.TrimSpace(title), []byte(text), project.Columns[0].Folder)
	if err != nil {
		var refusal *cardstore.Error
		if errors.As(err, &refusal) {
			fmt.Fprintf(errOut, "kanbanzai: number: %v\n", err)
			return 1
		}
		fmt.Fprintf(errOut, "kanbanzai: number: %v\n", err)
		return 1
	}
	fmt.Fprintln(out, entry.ID)
	fmt.Fprintf(errOut, "brief: %s\n", project.Rel(entry.Path))
	return 0
}
