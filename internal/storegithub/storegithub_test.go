package storegithub

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/cloudbusting/kanbanzai/internal/config"
	"github.com/cloudbusting/kanbanzai/internal/store"
	"github.com/cloudbusting/kanbanzai/internal/store/mirror"
	"github.com/cloudbusting/kanbanzai/internal/store/protocol"
	"github.com/cloudbusting/kanbanzai/internal/store/storetest"
	"github.com/cloudbusting/kanbanzai/internal/storegithub/fakegh"
	"github.com/cloudbusting/kanbanzai/internal/testutil"
)

// inProcess runs the fake gh in this process.
type inProcess struct{ state *fakegh.State }

func (r inProcess) Run(args []string, stdin []byte) ([]byte, error) {
	out, err := r.state.Run(args, stdin)
	var failure *fakegh.Error
	if errors.As(err, &failure) {
		return nil, &RunError{Stderr: failure.Message}
	}
	return out, err
}

const repo = "acme/widgets"

func open(t *testing.T, state *fakegh.State, extra map[string]string) *Backend {
	t.Helper()
	settings := map[string]string{"repo": repo, "prefix": "GH"}
	for key, value := range extra {
		settings[key] = value
	}
	backend, err := Open(settings, inProcess{state})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	return backend
}

func newState() *fakegh.State { return fakegh.New(repo) }

func wantCode(t *testing.T, err error, sentinel error) {
	t.Helper()
	if !errors.Is(err, sentinel) {
		t.Fatalf("error = %v, want %v", err, sentinel)
	}
}

func TestAnIssueIsATicketWithItsStageAndType(t *testing.T) {
	state := newState()
	state.Add("Plain", "A person's issue") // 1: open, no label
	state.Add("Started", storetest.Brief("GH-2", "Started", "building"), "stage:building", "type:bug", "agent")
	closed := state.Add("Finished", storetest.Brief("GH-3", "Finished", "done"), "type:change")
	closed.State = "closed"
	native := state.Add("Native", "x", "type:task")
	native.Type = "Bug"
	pr := state.Add("A pull request", "x")
	pr.PullRequest = true
	reopened := state.Add("Reopened", "x", "stage:done")
	_ = reopened

	tickets, err := open(t, state, nil).List("")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	got := map[string]string{}
	for _, ticket := range tickets {
		if ticket.Rev == "" {
			t.Errorf("%s has no revision", ticket.ID)
		}
		got[ticket.ID] = ticket.Title + "|" + ticket.Type + "|" + ticket.Stage
	}
	want := map[string]string{
		"GH-1": "Plain||todo",
		"GH-2": "Started|bug|building",
		"GH-3": "Finished|change|done",
		"GH-4": "Native|bug|todo", // the issue's own type wins over a label
		"GH-6": "Reopened||todo",  // an open issue is never Done, whatever its label says
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("List = %v\nwant %v", got, want)
	}
}

func TestTheQueryIsGitHubSearchOnTheRepositoryAndAnEmptyOneAddsNothing(t *testing.T) {
	state := newState()
	state.Add("A", "x", "agent")
	state.Add("B", "x")
	backend := open(t, state, nil)
	tickets, err := backend.List(`label:agent is:open`)
	if err != nil || len(tickets) != 1 || tickets[0].ID != "GH-1" {
		t.Fatalf("List(label:agent) = %v, %v", tickets, err)
	}
	if call := state.Calls[len(state.Calls)-1]; !strings.Contains(call, "q=repo:acme/widgets is:issue label:agent is:open") {
		t.Fatalf("the search was %q", call)
	}
	if all, err := backend.List(""); err != nil || len(all) != 2 {
		t.Fatalf("List(empty) = %v, %v; want every issue", all, err)
	}
	if call := state.Calls[len(state.Calls)-1]; !strings.Contains(call, "q=repo:acme/widgets is:issue -f per_page=100") {
		t.Fatalf("an empty query was %q", call)
	}
}

func TestAQueryThatNamesAnotherRepositoryIsRefused(t *testing.T) {
	backend := open(t, newState(), nil)
	for _, query := range []string{"repo:other/thing", "org:acme label:x", "label:x user:sam", "-repo:acme/old", "ORG:acme"} {
		if _, err := backend.List(query); !errors.Is(err, store.ErrRefused) || !strings.Contains(err.Error(), "one repository") {
			t.Errorf("List(%q) = %v, want a refusal that says the board keeps one repository", query, err)
		}
	}
}

func TestMoreThanAThousandMatchesIsRefusedNeverCut(t *testing.T) {
	state := newState()
	state.Add("A", "x")
	state.Total = 1500
	_, err := open(t, state, nil).List("")
	if !errors.Is(err, store.ErrRefused) || !strings.Contains(err.Error(), "narrow") {
		t.Fatalf("List with 1500 matches = %v, want a refusal telling the owner to narrow the query", err)
	}
}

func TestEveryPageOfAnAnswerIsRead(t *testing.T) {
	state := newState()
	for i := 0; i < 250; i++ {
		state.Add(fmt.Sprintf("Issue %d", i), "x")
	}
	tickets, err := open(t, state, nil).List("")
	if err != nil || len(tickets) != 250 {
		t.Fatalf("List = %d tickets, %v; want 250", len(tickets), err)
	}
}

func TestPutWithAStaleBaseIsAConflictAndWritesNothing(t *testing.T) {
	state := newState()
	state.Add("Mine", storetest.Brief("GH-1", "Mine", "todo"))
	backend := open(t, state, nil)
	got, _ := backend.Get([]string{"GH-1"})
	state.Issue(1).Body = "Changed on GitHub"
	_, err := backend.Put("GH-1", got[0].Rev, got[0].Text+"\nmore", protocol.Fields{Stage: "todo", Title: "Mine"})
	wantCode(t, err, store.ErrConflict)
	if state.Issue(1).Body != "Changed on GitHub" {
		t.Fatal("a refused write changed the issue")
	}
}

func TestPutKeepsOtherLabelsAndWritesOnlyStageAndType(t *testing.T) {
	state := newState()
	state.Add("Mine", storetest.Brief("GH-1", "Mine", "todo"), "agent", "stage:todo", "type:task", "good first issue")
	backend := open(t, state, nil)
	got, _ := backend.Get([]string{"GH-1"})
	text := strings.Replace(got[0].Text, "**Stage:** todo", "**Stage:** designing", 1)
	if _, err := backend.Put("GH-1", got[0].Rev, text, protocol.Fields{Stage: "designing", Title: "Mine", Type: "bug"}); err != nil {
		t.Fatalf("Put: %v", err)
	}
	issue := state.Issue(1)
	if fmt.Sprint(issue.Labels) != "[agent good first issue stage:designing type:bug]" {
		t.Fatalf("labels = %v", issue.Labels)
	}
	if issue.State != "open" || issue.Body != text {
		t.Fatalf("state %q, body %q", issue.State, issue.Body)
	}
}

func TestPutDoneClosesTheIssueAndALaterStageReopensIt(t *testing.T) {
	state := newState()
	state.Add("Mine", storetest.Brief("GH-1", "Mine", "building"), "stage:building")
	backend := open(t, state, nil)
	got, _ := backend.Get([]string{"GH-1"})
	done := strings.Replace(got[0].Text, "**Stage:** building", "**Stage:** done", 1)
	rev, err := backend.Put("GH-1", got[0].Rev, done, protocol.Fields{Stage: "done", Title: "Mine"})
	if err != nil {
		t.Fatalf("Put done: %v", err)
	}
	issue := state.Issue(1)
	if issue.State != "closed" || issue.StateReason != "completed" || fmt.Sprint(issue.Labels) != "[]" {
		t.Fatalf("after done: state %q (%s), labels %v", issue.State, issue.StateReason, issue.Labels)
	}
	back := strings.Replace(done, "**Stage:** done", "**Stage:** building", 1)
	if _, err := backend.Put("GH-1", rev, back, protocol.Fields{Stage: "building", Title: "Mine"}); err != nil {
		t.Fatalf("Put reopen: %v", err)
	}
	if issue.State != "open" || fmt.Sprint(issue.Labels) != "[stage:building]" {
		t.Fatalf("after reopening: state %q, labels %v", issue.State, issue.Labels)
	}
}

func TestPutKeepsTheIssueTitleEqualToTheBriefs(t *testing.T) {
	state := newState()
	state.Add("Old", storetest.Brief("GH-1", "Old", "todo"))
	backend := open(t, state, nil)
	got, _ := backend.Get([]string{"GH-1"})
	text := strings.Replace(got[0].Text, "# GH-1: Old", "# GH-1: A better title", 1)
	if _, err := backend.Put("GH-1", got[0].Rev, text, protocol.Fields{Stage: "todo", Title: "A better title"}); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if state.Issue(1).Title != "A better title" {
		t.Fatalf("title = %q", state.Issue(1).Title)
	}
}

func TestAChangeOnGitHubMovesTheCard(t *testing.T) {
	state := newState()
	state.Add("Mine", storetest.Brief("GH-1", "Mine", "todo"))
	backend := open(t, state, nil)
	before, _ := backend.Get([]string{"GH-1"})

	state.Issue(1).State = "closed"
	closed, _ := backend.Get([]string{"GH-1"})
	if !strings.Contains(closed[0].Text, "- **Stage:** done\n") || closed[0].Rev == before[0].Rev {
		t.Fatalf("a closed issue reads as %q (rev %s, was %s)", closed[0].Text, closed[0].Rev, before[0].Rev)
	}
	state.Issue(1).State = "open"
	state.Issue(1).Labels = []string{"stage:building"}
	reopened, _ := backend.Get([]string{"GH-1"})
	if !strings.Contains(reopened[0].Text, "- **Stage:** building\n") {
		t.Fatalf("a reopened issue with stage:building reads as %q", reopened[0].Text)
	}
	listed, _ := backend.List("")
	if listed[0].Stage != "building" {
		t.Fatalf("list stage = %q", listed[0].Stage)
	}
}

func TestActivityThatDoesNotChangeTheCardIsNotAConflict(t *testing.T) {
	state := newState()
	state.Add("Mine", storetest.Brief("GH-1", "Mine", "todo"))
	backend := open(t, state, nil)
	before, _ := backend.Get([]string{"GH-1"})
	issue := state.Issue(1)
	issue.Comments, issue.Assignee, issue.Labels = 3, "sam", []string{"help wanted"}
	after, _ := backend.Get([]string{"GH-1"})
	if after[0].Rev != before[0].Rev {
		t.Fatal("a comment, an assignee or another label changed the revision")
	}
}

func TestABodyThatIsNotABriefComesBackAsOne(t *testing.T) {
	state := newState()
	state.Add("Crash on start", "It crashes.\n\nSteps:\n1. start\n")
	got, err := open(t, state, nil).Get([]string{"GH-1"})
	if err != nil || len(got) != 1 {
		t.Fatalf("Get = %v, %v", got, err)
	}
	text := got[0].Text
	for _, want := range []string{"# GH-1: Crash on start\n", "- **Stage:** todo\n", "- **Updated:** ", "- **Next action:** ",
		"- **Where:** ", "## Goal\n\nIt crashes.\n\nSteps:\n1. start\n", "## Comments"} {
		if !strings.Contains(text, want) {
			t.Errorf("the built brief lacks %q:\n%s", want, text)
		}
	}
	if state.Issue(1).Body != "It crashes.\n\nSteps:\n1. start\n" {
		t.Fatal("reading changed the issue")
	}
}

func TestABriefOverGitHubsLimitIsRefusedNotCut(t *testing.T) {
	state := newState()
	state.Add("Mine", storetest.Brief("GH-1", "Mine", "todo"))
	backend := open(t, state, nil)
	got, _ := backend.Get([]string{"GH-1"})
	_, err := backend.Put("GH-1", got[0].Rev, got[0].Text+strings.Repeat("x", 70000), protocol.Fields{Stage: "todo", Title: "Mine"})
	if !errors.Is(err, store.ErrRefused) || !strings.Contains(err.Error(), "65,536") {
		t.Fatalf("Put of a huge brief = %v, want a refusal naming GitHub's limit", err)
	}
	if state.Issue(1).Body != got[0].Text {
		t.Fatal("a refused write changed the issue")
	}
}

func TestCreateOpensTheIssueAndPutsItsNumberInTheBrief(t *testing.T) {
	state := newState()
	state.Add("Existing", "x")
	backend := open(t, state, nil)
	// The core's new brief carries its Type line.
	text := strings.Replace(storetest.Brief(store.IDPlaceholder, "Made here", "todo"), "- **Where:** —\n", "- **Where:** —\n- **Type:** bug\n", 1)
	id, rev, err := backend.Create("bug", "Made here", text)
	if err != nil || id != "GH-2" || rev == "" {
		t.Fatalf("Create = %q, %q, %v", id, rev, err)
	}
	issue := state.Issue(2)
	if issue.Title != "Made here" || !strings.HasPrefix(issue.Body, "# GH-2: Made here\n") || issue.State != "open" ||
		fmt.Sprint(issue.Labels) != "[stage:todo type:bug]" {
		t.Fatalf("the new issue is %+v", issue)
	}
	got, _ := backend.Get([]string{"GH-2"})
	if got[0].Rev != rev || got[0].Text != issue.Body {
		t.Fatalf("Get after Create: rev %s (Create said %s)", got[0].Rev, rev)
	}
}

func TestAnIssueThatIsNotThereIsLeftOutAndAPullRequestToo(t *testing.T) {
	state := newState()
	state.Add("A", "x")
	state.Add("PR", "x").PullRequest = true
	got, err := open(t, state, nil).Get([]string{"GH-1", "GH-2", "GH-9", "NOPE-1", "GH-x"})
	if err != nil || len(got) != 1 || got[0].ID != "GH-1" {
		t.Fatalf("Get = %v, %v; want only GH-1", got, err)
	}
	_, err = open(t, state, nil).Put("GH-9", "r", "x", protocol.Fields{})
	wantCode(t, err, store.ErrNotFound)
}

func TestFailuresOfGhAreMappedToTheStoresCodes(t *testing.T) {
	cases := map[string]error{
		"gh: Not Found (HTTP 404)":                                   store.ErrNotFound,
		"gh: Resource not accessible (HTTP 403)":                     store.ErrRefused,
		"gh: API rate limit exceeded for user (HTTP 403)":            store.ErrUnavailable,
		"To get started with GitHub CLI, please run:  gh auth login": store.ErrUnavailable,
		"gh: Bad credentials (HTTP 401)":                             store.ErrUnavailable,
		"dial tcp: lookup api.github.com: no such host":              store.ErrUnavailable,
	}
	for message, want := range cases {
		state := newState()
		state.Down = message
		_, err := open(t, state, nil).List("")
		if !errors.Is(err, want) {
			t.Errorf("%q = %v, want %v", message, err, want)
		}
	}
}

func TestTheSettingsAreChecked(t *testing.T) {
	cases := map[string]map[string]string{
		"no repo":        {"prefix": "GH"},
		"a bad repo":     {"repo": "not-a-repo", "prefix": "GH"},
		"a repo with ..": {"repo": "../x", "prefix": "GH"},
		"no prefix":      {"repo": repo},
		"a lower prefix": {"repo": repo, "prefix": "gh"},
		"another host":   {"repo": repo, "prefix": "GH", "host": "ghe.example.com"},
	}
	for name, settings := range cases {
		if _, err := Open(settings, inProcess{newState()}); !errors.Is(err, store.ErrRefused) {
			t.Errorf("%s: %v, want refused", name, err)
		}
	}
	if _, err := Open(map[string]string{"repo": repo, "prefix": "GH", "host": "github.com"}, inProcess{newState()}); err != nil {
		t.Errorf("host github.com: %v", err)
	}
	_, err := Open(map[string]string{"repo": repo, "prefix": "GH", "host": "ghe.example.com"}, inProcess{newState()})
	if err == nil || !strings.Contains(err.Error(), "CHANGE-91") {
		t.Errorf("another host should name CHANGE-91: %v", err)
	}
}

func TestHello(t *testing.T) {
	hello, err := open(t, newState(), nil).Hello()
	if err != nil || hello.Protocol != 1 || hello.Name != "github" ||
		fmt.Sprint(hello.Capabilities) != "[list get put create]" {
		t.Fatalf("Hello = %+v, %v", hello, err)
	}
}

const remoteConfig = "[project]\nowner = \"Sam\"\n\n[store]\nplugin = \"github\"\nprefix = \"GH\"\n\n[store.settings]\nrepo = \"acme/widgets\"\n"

// The conformance suite every store passes, with the GitHub backend behind the mirror and a fake gh behind it.
func TestConformance(t *testing.T) {
	storetest.Run(t, func(t *testing.T) storetest.Harness {
		t.Setenv("KANBANZAI_PROJECT", "")
		project, err := config.Find(testutil.Repo(t, remoteConfig))
		if err != nil {
			t.Fatalf("config.Find: %v", err)
		}
		state := newState()
		s := mirror.New(project, open(t, state, nil), mirror.Options{Name: "github"})
		return storetest.Harness{
			Store: s, Folders: [2]string{"1-open", "10-done"}, Prefix: "GH",
			Seed: func(t *testing.T, id, title, stage, folder string) {
				var number int
				fmt.Sscanf(id, "GH-%d", &number)
				issue := state.Add(title, storetest.Brief(id, title, stage))
				issue.Number = number
				if stage == "done" {
					issue.State = "closed"
				} else if stage != "todo" {
					issue.Labels = []string{"stage:" + stage}
				}
				if err := s.Sync(); err != nil {
					t.Fatalf("Sync: %v", err)
				}
			},
			Tamper: func(t *testing.T, id string) {
				var number int
				fmt.Sscanf(id, "GH-%d", &number)
				issue := state.Issue(number)
				issue.Body = strings.Replace(issue.Body, "Body.", "Theirs.", 1)
			},
		}
	})
}

func TestTheTypeReachesTheBriefsTypeLineAndRoundTrips(t *testing.T) {
	state := newState()
	state.Add("Typed", storetest.Brief("GH-1", "Typed", "todo"), "type:bug")
	backend := open(t, state, nil)
	got, _ := backend.Get([]string{"GH-1"})
	if !strings.Contains(got[0].Text, "- **Where:** —\n- **Type:** bug\n\n## Goal") {
		t.Fatalf("the type is not in the brief's head:\n%s", got[0].Text)
	}
	rev, err := backend.Put("GH-1", got[0].Rev, got[0].Text, protocol.Fields{Stage: "todo", Title: "Typed", Type: "bug"})
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	again, _ := backend.Get([]string{"GH-1"})
	if again[0].Text != got[0].Text || again[0].Rev != rev {
		t.Fatalf("the round trip changed the brief or its revision:\n%s", again[0].Text)
	}
	// A type changed on GitHub changes the line and the revision; a brief with no type takes no label.
	state.Issue(1).Labels = []string{"type:task"}
	changed, _ := backend.Get([]string{"GH-1"})
	if !strings.Contains(changed[0].Text, "- **Type:** task\n") || changed[0].Rev == rev {
		t.Fatalf("a type changed on GitHub reads as:\n%s", changed[0].Text)
	}
	if _, err := backend.Put("GH-1", changed[0].Rev, strings.Replace(changed[0].Text, "- **Type:** task\n", "", 1),
		protocol.Fields{Stage: "todo", Title: "Typed"}); err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(state.Issue(1).Labels) != "[stage:todo]" {
		t.Fatalf("labels = %v, want the type label gone", state.Issue(1).Labels)
	}
}
