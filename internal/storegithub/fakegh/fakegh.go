// Package fakegh is a stand-in for the `gh` program, holding a few GitHub issues in memory (or in a JSON file, so
// separate processes share them). The GitHub store plugin's tests run it in process, and the plugin program's tests
// run it as a `gh` on PATH. It answers only the `gh api` calls the plugin makes, and no test ever reaches github.com.
package fakegh

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Issue is one issue.
type Issue struct {
	Number      int      `json:"number"`
	Title       string   `json:"title"`
	Body        string   `json:"body"`
	State       string   `json:"state"`
	StateReason string   `json:"state_reason,omitempty"`
	Labels      []string `json:"labels"`
	Type        string   `json:"type,omitempty"`
	PullRequest bool     `json:"pull_request,omitempty"`
	Assignee    string   `json:"assignee,omitempty"`
	Comments    int      `json:"comments"`
}

// State is the whole fake repository.
type State struct {
	Repo   string   `json:"repo"`
	Issues []*Issue `json:"issues"`
	// Calls is every `gh` call made, one line each: its arguments joined by a space.
	Calls []string `json:"calls"`
	// Total, when above zero, is the total_count every search claims, as a repository with that many matches would.
	Total int `json:"total"`
	// Down, when set, makes every call fail with this message (a network failure or a sign-in problem).
	Down string `json:"down,omitempty"`
}

// New is an empty repository.
func New(repo string) *State { return &State{Repo: repo} }

// Add puts an issue in the repository and returns it, numbered after the highest.
func (s *State) Add(title, body string, labels ...string) *Issue {
	number := 1
	for _, existing := range s.Issues {
		if existing.Number >= number {
			number = existing.Number + 1
		}
	}
	issue := &Issue{Number: number, Title: title, Body: body, State: "open", Labels: labels}
	s.Issues = append(s.Issues, issue)
	return issue
}

// Issue is the issue with this number, or nil.
func (s *State) Issue(number int) *Issue {
	for _, issue := range s.Issues {
		if issue.Number == number {
			return issue
		}
	}
	return nil
}

// Load reads a state file; a missing file is an empty repository named repo.
func Load(path, repo string) (*State, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return New(repo), nil
	}
	if err != nil {
		return nil, err
	}
	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}
	return &state, nil
}

// Save writes the state file.
func (s *State) Save(path string) error {
	data, err := json.MarshalIndent(s, "", " ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// Error is a failed `gh` call: what the real program writes to standard error, and its exit code.
type Error struct {
	Message string
	Code    int
}

func (e *Error) Error() string { return e.Message }

// Run answers one `gh` call with the arguments after `gh`, and the request body from standard input.
func (s *State) Run(args []string, stdin []byte) ([]byte, error) {
	s.Calls = append(s.Calls, strings.Join(args, " "))
	if s.Down != "" {
		return nil, &Error{Message: s.Down, Code: 1}
	}
	if len(args) == 0 || args[0] != "api" {
		return nil, &Error{Message: "fakegh: only `gh api` is faked", Code: 1}
	}
	method, endpoint, fields := "GET", "", map[string]string{}
	input := false
	for i := 1; i < len(args); i++ {
		switch arg := args[i]; arg {
		case "--paginate", "--slurp":
		case "-X":
			i++
			method = args[i]
		case "-f":
			i++
			key, value, _ := strings.Cut(args[i], "=")
			fields[key] = value
		case "--input":
			i++
			input = args[i] == "-"
		default:
			endpoint = arg
		}
	}
	parts := strings.Split(endpoint, "/")
	switch {
	case method == "GET" && endpoint == "search/issues":
		return s.search(fields["q"])
	case len(parts) >= 4 && parts[0] == "repos":
		if parts[1]+"/"+parts[2] != s.Repo {
			return nil, &Error{Message: "gh: Not Found (HTTP 404)", Code: 1}
		}
		return s.issues(method, parts[3:], input, stdin)
	}
	return nil, &Error{Message: "gh: Not Found (HTTP 404)", Code: 1}
}

func (s *State) issues(method string, rest []string, input bool, stdin []byte) ([]byte, error) {
	if len(rest) == 0 || rest[0] != "issues" {
		return nil, &Error{Message: "gh: Not Found (HTTP 404)", Code: 1}
	}
	if method == "POST" && len(rest) == 1 {
		var in change
		if err := json.Unmarshal(stdin, &in); err != nil || !input {
			return nil, &Error{Message: "gh: Invalid request (HTTP 422)", Code: 1}
		}
		issue := s.Add("", "")
		in.apply(issue)
		return s.one(issue)
	}
	if len(rest) != 2 {
		return nil, &Error{Message: "gh: Not Found (HTTP 404)", Code: 1}
	}
	number, err := strconv.Atoi(rest[1])
	issue := s.Issue(number)
	if err != nil || issue == nil || issue.PullRequest && method != "GET" {
		return nil, &Error{Message: "gh: Not Found (HTTP 404)", Code: 1}
	}
	switch method {
	case "GET":
		return s.one(issue)
	case "PATCH":
		var in change
		if err := json.Unmarshal(stdin, &in); err != nil || !input {
			return nil, &Error{Message: "gh: Invalid request (HTTP 422)", Code: 1}
		}
		if in.Body != nil && len(*in.Body) > 65536 {
			return nil, &Error{Message: "gh: Validation Failed (HTTP 422)", Code: 1}
		}
		in.apply(issue)
		return s.one(issue)
	}
	return nil, &Error{Message: "gh: Not Found (HTTP 404)", Code: 1}
}

// change is the body of a create or an update; a field that is absent is left as it is.
type change struct {
	Title       *string   `json:"title"`
	Body        *string   `json:"body"`
	State       *string   `json:"state"`
	StateReason *string   `json:"state_reason"`
	Labels      *[]string `json:"labels"`
}

func (c change) apply(issue *Issue) {
	if c.Title != nil {
		issue.Title = *c.Title
	}
	if c.Body != nil {
		issue.Body = *c.Body
	}
	if c.State != nil {
		issue.State = *c.State
		issue.StateReason = ""
		if c.StateReason != nil {
			issue.StateReason = *c.StateReason
		}
	}
	if c.Labels != nil {
		issue.Labels = *c.Labels
	}
}

type wire struct {
	Number      int            `json:"number"`
	Title       string         `json:"title"`
	Body        *string        `json:"body"`
	State       string         `json:"state"`
	StateReason string         `json:"state_reason,omitempty"`
	Labels      []wireLabel    `json:"labels"`
	Type        *wireLabel     `json:"type"`
	PullRequest map[string]any `json:"pull_request,omitempty"`
	Comments    int            `json:"comments"`
	UpdatedAt   string         `json:"updated_at"`
}

type wireLabel struct {
	Name string `json:"name"`
}

func (s *State) wire(issue *Issue) wire {
	out := wire{Number: issue.Number, Title: issue.Title, State: issue.State, StateReason: issue.StateReason,
		Labels: []wireLabel{}, Comments: issue.Comments, UpdatedAt: "2026-11-18T00:00:00Z"}
	if issue.Body != "" {
		body := issue.Body
		out.Body = &body
	}
	for _, name := range issue.Labels {
		out.Labels = append(out.Labels, wireLabel{Name: name})
	}
	if issue.Type != "" {
		out.Type = &wireLabel{Name: issue.Type}
	}
	if issue.PullRequest {
		out.PullRequest = map[string]any{"url": "https://example.invalid/pull"}
	}
	return out
}

func (s *State) one(issue *Issue) ([]byte, error) {
	return json.Marshal(s.wire(issue))
}

// search answers an issue search, as `gh api --paginate --slurp` does: an array with one object per page of 100. It
// understands repo:, is:issue, is:pr, is:open, is:closed, label:, assignee: and quoted values; free text is ignored.
func (s *State) search(query string) ([]byte, error) {
	var repo, state, label, assignee string
	kind := ""
	for _, token := range tokens(query) {
		name, value, found := strings.Cut(token, ":")
		if !found {
			continue
		}
		value = strings.Trim(value, `"`)
		switch name {
		case "repo":
			repo = value
		case "is":
			switch value {
			case "open", "closed":
				state = value
			case "issue", "pr":
				kind = value
			}
		case "label":
			label = value
		case "assignee":
			assignee = value
		}
	}
	if repo != s.Repo {
		return nil, &Error{Message: "gh: Validation Failed (HTTP 422)", Code: 1}
	}
	items := []wire{}
	for _, issue := range s.Issues {
		if kind == "issue" && issue.PullRequest || kind == "pr" && !issue.PullRequest {
			continue
		}
		if state != "" && issue.State != state || assignee != "" && issue.Assignee != assignee {
			continue
		}
		if label != "" && !has(issue.Labels, label) {
			continue
		}
		items = append(items, s.wire(issue))
	}
	total := len(items)
	if s.Total > 0 {
		total = s.Total
	}
	if total > 1000 {
		return nil, &Error{Message: "gh: Validation Failed: Only the first 1000 search results are available (HTTP 422)", Code: 1}
	}
	type page struct {
		TotalCount int    `json:"total_count"`
		Incomplete bool   `json:"incomplete_results"`
		Items      []wire `json:"items"`
	}
	pages := []page{}
	for len(items) > 100 {
		pages = append(pages, page{TotalCount: total, Items: items[:100]})
		items = items[100:]
	}
	pages = append(pages, page{TotalCount: total, Items: items})
	return json.Marshal(pages)
}

func has(labels []string, name string) bool {
	for _, label := range labels {
		if strings.EqualFold(label, name) {
			return true
		}
	}
	return false
}

// tokens splits a query on spaces, keeping a quoted value whole.
func tokens(query string) []string {
	var out []string
	var current bytes.Buffer
	quoted := false
	for _, r := range query {
		switch {
		case r == '"':
			quoted = !quoted
			current.WriteRune(r)
		case r == ' ' && !quoted:
			if current.Len() > 0 {
				out = append(out, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(r)
		}
	}
	if current.Len() > 0 {
		out = append(out, current.String())
	}
	return out
}

// Main is the whole of a fake `gh` program: it keeps its state in the file named by KANBANZAI_FAKE_GH_STATE, answers
// the call in args, writes the answer to out and the failure to errOut, and returns the exit code.
func Main(args []string, stdin []byte, out, errOut *bytes.Buffer) int {
	path := os.Getenv("KANBANZAI_FAKE_GH_STATE")
	if path == "" {
		fmt.Fprintln(errOut, "fakegh: KANBANZAI_FAKE_GH_STATE is not set")
		return 2
	}
	state, err := Load(path, os.Getenv("KANBANZAI_FAKE_GH_REPO"))
	if err != nil {
		fmt.Fprintln(errOut, err)
		return 2
	}
	answer, runErr := state.Run(args, stdin)
	if err := state.Save(path); err != nil {
		fmt.Fprintln(errOut, err)
		return 2
	}
	if runErr != nil {
		fmt.Fprintln(errOut, runErr.Error())
		if failure, ok := runErr.(*Error); ok {
			return failure.Code
		}
		return 1
	}
	out.Write(answer)
	return 0
}
