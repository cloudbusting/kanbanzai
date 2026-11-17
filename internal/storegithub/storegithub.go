// Package storegithub is the GitHub issues store plugin: it keeps a project's cards as the issues of one repository,
// reaching GitHub only through the user's own `gh` program (`gh api`), so it has no HTTP client and holds no token.
// docs/store-plugins.md is its public description.
//
//   - A card is the issue numbered n, with the id <prefix>-n. The issue body is the whole brief.
//   - The Stage is shown by a `stage:<column>` label, and a closed issue is Done. A change made on GitHub moves the card.
//   - The type is the issue's own type, or a `type:<key>` label.
//   - The query is GitHub's issue search, always narrowed to this repository and to issues.
package storegithub

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/cloudbusting/kanbanzai/internal/brief"
	"github.com/cloudbusting/kanbanzai/internal/store"
	"github.com/cloudbusting/kanbanzai/internal/store/protocol"
)

// Version is the plugin's own version, answered by hello.
const Version = "1"

const (
	stagePrefix = "stage:"
	typePrefix  = "type:"
	// BodyLimit is GitHub's longest issue body, in characters.
	bodyLimit = 65536
	// searchLimit is how many results GitHub's search will ever return.
	searchLimit = 1000
	// defaultInitial is the stage of an open issue with no stage label: the shipped first column's key.
	defaultInitial = "todo"
)

// Runner runs `gh` with these arguments and this standard input and returns its standard output. A failure is a
// *RunError carrying what `gh` wrote to standard error.
type Runner interface {
	Run(args []string, stdin []byte) ([]byte, error)
}

// RunError is a failed `gh` call.
type RunError struct {
	Stderr string
	Err    error
}

func (e *RunError) Error() string {
	if e.Stderr != "" {
		return strings.TrimSpace(e.Stderr)
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return "gh failed"
}

func (e *RunError) Unwrap() error { return e.Err }

// Backend is one repository's issues.
type Backend struct {
	run    Runner
	repo   string
	prefix string
	done   string
	first  string // the stage of an open issue with no stage label
	id     *regexp.Regexp
}

var (
	repoRE   = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`)
	prefixRE = regexp.MustCompile(`^[A-Z][A-Z0-9]*$`)
	// scopeRE finds a qualifier that widens the search beyond the repository: the board keeps one repository.
	scopeRE = regexp.MustCompile(`(?i)(^|\s)-?(repo|org|user|owner):`)
	// groupRE finds what could let the query's own precedence outgrow `repo:<repo> is:issue`: OR, and parentheses.
	groupRE = regexp.MustCompile(`(^|\s)(OR|or)(\s|$)|[()]`)
	quoted  = regexp.MustCompile(`"[^"]*"`)
)

// Open is the backend its settings name: repo ("owner/name", required), prefix (the ids' upper-case prefix,
// required: the core sends the project's [store] prefix), host (github.com only), done (the column key a closed
// issue is in, default "done") and initial (the first column's key, the stage of an open issue with no stage label,
// default "todo").
func Open(settings map[string]string, run Runner) (*Backend, error) {
	repo := strings.TrimSpace(settings["repo"])
	if repo == "" {
		return nil, store.Refuse(store.CodeRefused, "the github plugin needs [store.settings] repo, as owner/name")
	}
	if !repoRE.MatchString(repo) || strings.Contains(repo, "..") {
		return nil, store.Refuse(store.CodeRefused, fmt.Sprintf("repo %q must be owner/name", repo))
	}
	prefix := strings.TrimSpace(settings["prefix"])
	if !prefixRE.MatchString(prefix) {
		return nil, store.Refuse(store.CodeRefused, fmt.Sprintf("prefix %q must be upper-case letters and digits; set "+
			"[store] prefix", prefix))
	}
	if host := strings.TrimSpace(settings["host"]); host != "" && !strings.EqualFold(host, "github.com") {
		return nil, store.Refuse(store.CodeRefused, fmt.Sprintf("host %q: this plugin keeps issues on github.com; GitHub "+
			"Enterprise Server is CHANGE-91", host))
	}
	done := strings.ToLower(strings.TrimSpace(settings["done"]))
	if done == "" {
		done = "done"
	}
	first := strings.ToLower(strings.TrimSpace(settings["initial"]))
	if first == "" {
		first = defaultInitial
	}
	return &Backend{run: run, repo: repo, prefix: prefix, done: done, first: first,
		id: regexp.MustCompile(`^` + regexp.QuoteMeta(prefix) + `-([1-9]\d*)$`)}, nil
}

var _ protocol.Backend = (*Backend)(nil)

// Hello describes the plugin.
func (b *Backend) Hello() (protocol.Hello, error) {
	return protocol.Hello{Protocol: protocol.Version, Name: "github", Version: Version,
		Capabilities: []string{protocol.MethodList, protocol.MethodGet, protocol.MethodPut, protocol.MethodCreate}}, nil
}

// issue is what the plugin reads of an issue.
type issue struct {
	Number      int     `json:"number"`
	Title       string  `json:"title"`
	Body        *string `json:"body"`
	State       string  `json:"state"`
	StateReason string  `json:"state_reason"`
	Labels      []struct {
		Name string `json:"name"`
	} `json:"labels"`
	Type *struct {
		Name string `json:"name"`
	} `json:"type"`
	PullRequest *json.RawMessage `json:"pull_request"`
	UpdatedAt   string           `json:"updated_at"`
}

func (i issue) body() string {
	if i.Body == nil {
		return ""
	}
	return *i.Body
}

func (i issue) labelValue(prefix string) string {
	for _, label := range i.Labels {
		if strings.HasPrefix(strings.ToLower(label.Name), prefix) {
			return strings.ToLower(strings.TrimSpace(label.Name[len(prefix):]))
		}
	}
	return ""
}

// stage is the card's column key: closed is done, else the stage label, else the initial column. An open issue
// whose label says done is not done: it was reopened.
func (b *Backend) stage(i issue) string {
	if i.State == "closed" {
		return b.done
	}
	if label := i.labelValue(stagePrefix); label != "" && label != b.done {
		return label
	}
	return b.first
}

// kind is the card's type key: the issue's own type, else the type label, else none.
func kind(i issue) string {
	if i.Type != nil && strings.TrimSpace(i.Type.Name) != "" {
		return strings.ToLower(strings.TrimSpace(i.Type.Name))
	}
	return i.labelValue(typePrefix)
}

// rev changes exactly when the card does: its title, body, state, stage label and type. A comment, an assignee,
// another label or the issue's updated time is not the card.
func rev(i issue) string {
	sum := sha256.New()
	for _, part := range []string{i.Title, i.body(), i.State, i.labelValue(stagePrefix), kind(i)} {
		fmt.Fprintf(sum, "%d:%s\x00", len(part), part)
	}
	return hex.EncodeToString(sum.Sum(nil))[:16]
}

func (b *Backend) ticketID(number int) string { return b.prefix + "-" + strconv.Itoa(number) }

// number is the issue number an id names, or 0 when it is not this plugin's.
func (b *Backend) number(id string) int {
	match := b.id.FindStringSubmatch(id)
	if match == nil {
		return 0
	}
	n, err := strconv.Atoi(match[1])
	if err != nil {
		return 0
	}
	return n
}

// List is every issue the query matches in this repository, without text.
func (b *Backend) List(query string) ([]protocol.Ticket, error) {
	bare := quoted.ReplaceAllString(query, `""`) // a quoted value is text, whatever it says
	if scopeRE.MatchString(bare) {
		return nil, store.Refuse(store.CodeRefused, "the query names another repository, organisation or user, but the "+
			"board keeps one repository (settings repo); take repo:, org: and user: out of it")
	}
	if groupRE.MatchString(bare) {
		return nil, store.Refuse(store.CodeRefused, "the query uses OR or parentheses, which could reach past the "+
			"repository; write it as plain qualifiers (label:x is:open -label:y)")
	}
	q := "repo:" + b.repo + " is:issue"
	if strings.TrimSpace(query) != "" {
		q += " " + strings.TrimSpace(query)
	}
	out, err := b.run.Run([]string{"api", "--paginate", "--slurp", "-X", "GET", "-f", "q=" + q, "-f", "per_page=100",
		"search/issues"}, nil)
	if err != nil {
		return nil, b.refusal(err)
	}
	var pages []struct {
		TotalCount int     `json:"total_count"`
		Incomplete bool    `json:"incomplete_results"`
		Items      []issue `json:"items"`
	}
	if err := json.Unmarshal(out, &pages); err != nil {
		return nil, store.Refuse(store.CodeUnavailable, "gh answered the search with something that is not JSON")
	}
	tickets := []protocol.Ticket{}
	for _, page := range pages {
		if page.Incomplete {
			return nil, store.Refuse(store.CodeUnavailable, "GitHub's search timed out and returned part of the answer; try again")
		}
		if page.TotalCount > searchLimit {
			return nil, tooMany()
		}
		for _, i := range page.Items {
			if i.PullRequest != nil {
				continue
			}
			tickets = append(tickets, protocol.Ticket{ID: b.ticketID(i.Number), Rev: rev(i), Title: i.Title,
				Type: kind(i), Stage: b.stage(i)})
		}
	}
	sort.Slice(tickets, func(a, c int) bool { return b.number(tickets[a].ID) < b.number(tickets[c].ID) })
	return tickets, nil
}

func tooMany() error {
	return store.Refuse(store.CodeRefused, "the query matches more than 1000 issues, which is all GitHub's search will "+
		"return; narrow the query (a label, a milestone, is:open) so the board is not left partial")
}

// getWorkers is how many issues Get reads at once: a first sync of a big board must finish inside the 60 seconds the
// command gives a request.
const getWorkers = 8

// Get is the tickets with these ids and their text, in the order asked; an id that is not an issue of this repository,
// or that is gone, is left out.
func (b *Backend) Get(ids []string) ([]protocol.Ticket, error) {
	type result struct {
		ticket protocol.Ticket
		found  bool
		err    error
	}
	results := make([]result, len(ids))
	work := make(chan int)
	var wg sync.WaitGroup
	for w := 0; w < getWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for n := range work {
				id := ids[n]
				number := b.number(id)
				if number == 0 {
					continue
				}
				i, err := b.fetch(number)
				if errors.Is(err, store.ErrNotFound) {
					continue
				}
				if err != nil {
					results[n].err = err
					continue
				}
				results[n] = result{found: true, ticket: protocol.Ticket{ID: id, Rev: rev(i), Title: i.Title, Type: kind(i),
					Stage: b.stage(i), Text: b.text(id, i)}}
			}
		}()
	}
	for n := range ids {
		work <- n
	}
	close(work)
	wg.Wait()
	tickets := []protocol.Ticket{}
	for _, r := range results {
		if r.err != nil {
			return nil, r.err
		}
		if r.found {
			tickets = append(tickets, r.ticket)
		}
	}
	return tickets, nil
}

// fetch reads one issue; a pull request is not a card.
func (b *Backend) fetch(number int) (issue, error) {
	out, err := b.run.Run([]string{"api", "repos/" + b.repo + "/issues/" + strconv.Itoa(number)}, nil)
	if err != nil {
		return issue{}, b.refusal(err)
	}
	var i issue
	if err := json.Unmarshal(out, &i); err != nil {
		return issue{}, store.Refuse(store.CodeUnavailable, "gh answered an issue with something that is not JSON")
	}
	if i.PullRequest != nil {
		return issue{}, store.Refuse(store.CodeNotFound, fmt.Sprintf("#%d is a pull request, not an issue", number))
	}
	return i, nil
}

// text is the card's brief: the issue body when it is one, else a brief built around the body. Either way its Stage
// line follows the labels and the state, and its Type line the issue's type, so a change made on GitHub reaches the
// working copy.
func (b *Backend) text(id string, i issue) string {
	body := i.body()
	stage := b.stage(i)
	if !strings.HasPrefix(body, "# "+id+":") {
		date := i.UpdatedAt
		if len(date) >= 10 {
			date = date[:10]
		}
		return setType(fmt.Sprintf("# %s: %s\n\n- **Stage:** %s\n- **Updated:** %s\n- **Next action:** —\n- **Where:** —\n\n"+
			"## Goal\n\n%s\n\n## Comments\n", id, i.Title, stage, date, strings.TrimRight(body, "\n")), kind(i))
	}
	return setType(setStage(setTitle(body, id, i.Title), stage), kind(i))
}

// setTitle is text whose heading carries title, as the issue's title says; a title changed on GitHub reaches the brief.
func setTitle(text, id, title string) string {
	first, rest, found := strings.Cut(text, "\n")
	heading := "# " + id + ": " + title
	if strings.TrimSpace(first) == heading || strings.TrimSpace(title) == "" {
		return text
	}
	if !found {
		return heading
	}
	return heading + "\n" + rest
}

var typeLine = regexp.MustCompile(`^- \*\*Type:\*\*(.*)$`)

// setType is text with its Type line saying cardType, added after the last head field when there is none. No type
// leaves text as it is: the brief is the truth there.
func setType(text, cardType string) string {
	if cardType == "" {
		return text
	}
	lines := strings.Split(text, "\n")
	last := 0
	for n := 1; n < len(lines); n++ {
		if strings.HasPrefix(lines[n], "## ") {
			break
		}
		if match := typeLine.FindStringSubmatch(lines[n]); match != nil {
			if strings.ToLower(strings.TrimSpace(match[1])) == cardType {
				return text
			}
			lines[n] = "- **Type:** " + cardType
			return strings.Join(lines, "\n")
		}
		if brief.FieldRE.MatchString(lines[n]) || last > 0 && strings.HasPrefix(lines[n], "  ") && strings.TrimSpace(lines[n]) != "" {
			last = n
		}
	}
	if last == 0 {
		return text
	}
	return strings.Join(append(lines[:last+1], append([]string{"- **Type:** " + cardType}, lines[last+1:]...)...), "\n")
}

var stageLine = regexp.MustCompile(`^- \*\*Stage:\*\*(.*)$`)

// setStage is text with its Stage line saying stage; it is text unchanged when the line already does.
func setStage(text, stage string) string {
	lines := strings.Split(text, "\n")
	for n := 1; n < len(lines); n++ {
		if strings.HasPrefix(lines[n], "## ") {
			break
		}
		match := stageLine.FindStringSubmatch(lines[n])
		if match == nil {
			continue
		}
		if brief.StageWord(match[1]) == stage {
			return text
		}
		end := n + 1
		for end < len(lines) && strings.HasPrefix(lines[end], "  ") && strings.TrimSpace(lines[end]) != "" {
			end++
		}
		lines = append(lines[:n], append([]string{"- **Stage:** " + stage}, lines[end:]...)...)
		return strings.Join(lines, "\n")
	}
	return text
}

// patch is the body of an issue update.
type patch struct {
	Title       string   `json:"title"`
	Body        string   `json:"body"`
	State       string   `json:"state"`
	StateReason string   `json:"state_reason,omitempty"`
	Labels      []string `json:"labels"`
}

// Put replaces an issue with a brief, refusing a stale base revision. It never touches labels other than stage: and
// type:, and never the assignees, milestone or comments.
func (b *Backend) Put(id, baseRev, text string, fields protocol.Fields) (string, error) {
	number := b.number(id)
	if number == 0 {
		return "", store.Refuse(store.CodeNotFound, id+": not an issue of "+b.repo)
	}
	if len([]rune(text)) > bodyLimit {
		return "", store.Refuse(store.CodeRefused, fmt.Sprintf("%s is %d characters; GitHub keeps at most 65,536 in an "+
			"issue body", id, len([]rune(text))))
	}
	current, err := b.fetch(number)
	if err != nil {
		return "", err
	}
	if rev(current) != baseRev {
		return "", store.Refuse(store.CodeConflict, id+" changed on GitHub since it was read")
	}
	title := fields.Title
	if title == "" {
		title = current.Title
	}
	update := patch{Title: title, Body: text, State: "open", Labels: b.labels(current, fields)}
	switch stage := strings.ToLower(strings.TrimSpace(fields.Stage)); {
	case stage == "":
		// A Stage word the board does not know says nothing: the issue's state and stage label stay as they are.
		update.State, update.StateReason = current.State, current.StateReason
	case stage == b.done:
		update.State, update.StateReason = "closed", "completed"
		if current.State == "closed" {
			update.StateReason = current.StateReason // already closed: keep how it was closed
		}
	case current.State == "closed":
		update.StateReason = "reopened"
	}
	updated, err := b.send("PATCH", "repos/"+b.repo+"/issues/"+strconv.Itoa(number), update)
	if err != nil {
		return "", err
	}
	if !sameText(updated.body(), text) || updated.State != update.State {
		return "", store.Refuse(store.CodeUnavailable, id+": GitHub answered without applying the write (a redirect, "+
			"perhaps, if the repository was renamed or moved); check repo and try again")
	}
	return rev(updated), nil
}

// sameText compares two bodies as GitHub may have stored them: line endings and trailing space do not count.
func sameText(a, b string) bool {
	clean := func(s string) string { return strings.TrimRight(strings.ReplaceAll(s, "\r\n", "\n"), " \n\t") }
	return clean(a) == clean(b)
}

// labels is the issue's labels with its stage: and type: ones replaced by the card's: a done card carries no stage
// label, and a card with no type carries no type label.
func (b *Backend) labels(current issue, fields protocol.Fields) []string {
	labels := []string{}
	typeTaken := false
	stage := strings.ToLower(strings.TrimSpace(fields.Stage))
	for _, label := range current.Labels {
		lower := strings.ToLower(label.Name)
		switch {
		case strings.HasPrefix(lower, stagePrefix):
			if stage == "" { // no stage to write: the label stays
				labels = append(labels, label.Name)
			}
		case strings.HasPrefix(lower, typePrefix) && !typeTaken:
			typeTaken = true // the one type: label this plugin reads is the one it replaces; a person's others stay
		default:
			labels = append(labels, label.Name)
		}
	}
	if stage != "" && stage != b.done {
		labels = append(labels, stagePrefix+stage)
	}
	if cardType := strings.ToLower(strings.TrimSpace(fields.Type)); cardType != "" {
		labels = append(labels, typePrefix+cardType)
	}
	return labels
}

// Create opens a new issue, then writes the brief with the issue's number as its id.
func (b *Backend) Create(cardType, title, text string) (string, string, error) {
	_, fields := brief.Header(text)
	stage := brief.StageWord(fields["Stage"])
	if stage == "" {
		stage = b.first
	}
	if len([]rune(text)) > bodyLimit {
		return "", "", store.Refuse(store.CodeRefused, fmt.Sprintf("the new brief is %d characters; GitHub keeps at most "+
			"65,536 in an issue body", len([]rune(text))))
	}
	made := patch{Title: title, Body: store.IDPlaceholder, Labels: b.labels(issue{}, protocol.Fields{Stage: stage, Type: cardType})}
	created, err := b.send("POST", "repos/"+b.repo+"/issues", made)
	if err != nil {
		return "", "", err
	}
	id := b.ticketID(created.Number)
	body := strings.Replace(text, store.IDPlaceholder, id, 1)
	if len([]rune(body)) > bodyLimit {
		return "", "", store.Refuse(store.CodeRefused, fmt.Sprintf("the new brief is %d characters; GitHub keeps at most "+
			"65,536 in an issue body (the empty issue %s was created)", len([]rune(body)), id))
	}
	updated, err := b.send("PATCH", "repos/"+b.repo+"/issues/"+strconv.Itoa(created.Number),
		patch{Title: title, Body: body, State: "open", Labels: b.labelNames(created)})
	if err != nil {
		return "", "", err
	}
	return id, rev(updated), nil
}

func (b *Backend) labelNames(i issue) []string {
	names := []string{}
	for _, label := range i.Labels {
		names = append(names, label.Name)
	}
	return names
}

// send makes a write and returns the issue as GitHub now has it.
func (b *Backend) send(method, endpoint string, change patch) (issue, error) {
	body, err := json.Marshal(change)
	if err != nil {
		return issue{}, err
	}
	out, err := b.run.Run([]string{"api", "-X", method, "--input", "-", endpoint}, body)
	if err != nil {
		return issue{}, b.refusal(err)
	}
	var i issue
	if err := json.Unmarshal(out, &i); err != nil {
		return issue{}, store.Refuse(store.CodeUnavailable, "gh answered a write with something that is not JSON")
	}
	return i, nil
}

// refusal maps a failed `gh` call to the store's codes: a 404 is not found; a 401, a sign-in prompt, a rate limit or
// no network is unavailable (nothing is wrong with the request); another refusal by GitHub is refused.
func (b *Backend) refusal(err error) error {
	var failure *RunError
	message := err.Error()
	if errors.As(err, &failure) && errors.Is(failure.Err, exec.ErrNotFound) {
		return store.Refuse(store.CodeUnavailable, "gh is not installed or not on PATH; install it from https://cli.github.com "+
			"and run `gh auth login`")
	}
	lower := strings.ToLower(message)
	switch {
	case strings.Contains(lower, "first 1000 search results"):
		return tooMany()
	case strings.Contains(lower, "http 404"), strings.Contains(lower, "http 410"):
		return store.Refuse(store.CodeNotFound, message)
	case strings.Contains(lower, "http 401") || strings.Contains(lower, "gh auth login") || strings.Contains(lower, "bad credentials"):
		return store.Refuse(store.CodeUnavailable, "not signed in to GitHub: run `gh auth login` ("+message+")")
	case strings.Contains(lower, "rate limit"):
		return store.Refuse(store.CodeUnavailable, "GitHub's rate limit: try again later ("+message+")")
	case strings.Contains(lower, "http 403") || strings.Contains(lower, "http 422"):
		return store.Refuse(store.CodeRefused, message)
	}
	return store.Refuse(store.CodeUnavailable, message)
}
