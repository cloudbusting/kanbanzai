package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cloudbusting/kanbanzai/internal/config"
	"github.com/cloudbusting/kanbanzai/internal/testutil"
)

// The delivery fixtures (internal/flow/testdata/delivery) are flows for different kinds of work. Each test drives a
// card through one of them with the real commands, from its first column to Done, and ends with check and a wip
// audit. Together they prove the gates, the skip rules and the close are data, not Kanbanzai's own shipped flow.

func deliveryProject(t *testing.T, fixture string, replace ...string) *config.Project {
	t.Helper()
	path := filepath.Join("..", "flow", "testdata", "delivery", fixture+".toml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if len(replace) == 2 {
		text = strings.ReplaceAll(text, replace[0], replace[1])
	}
	// A fixture is a flow file; the project around it is the same for every one.
	project := newProject(t, "config_version = 1\n[project]\nowner = \"Sam\"\n\n"+text)
	// The flow the config resolves to exports, and what the fixture says is a flow file the command validates.
	exported := filepath.Join(t.TempDir(), "flow.toml")
	testutil.Write(t, exported, ok(t, "workflow", "export"))
	if code, out, errOut := runCLI(t, "workflow", "validate", "--file", exported); code != 0 {
		t.Fatalf("%s: the exported flow does not validate: %d %s%s", fixture, code, out, errOut)
	}
	direct := filepath.Join(t.TempDir(), "fixture.toml")
	testutil.Write(t, direct, text)
	if code, out, errOut := runCLI(t, "workflow", "validate", "--file", direct); code != 0 {
		t.Fatalf("%s: the fixture is not a valid flow file: %d %s%s", fixture, code, out, errOut)
	}
	return project
}

func putDeliveryCard(t *testing.T, project *config.Project, id, extraHead string) {
	t.Helper()
	text := "# " + id + ": the work\n\n- **Stage:** " + project.Columns[0].Key + "\n- **Updated:** 2026-11-09\n" +
		"- **Next action:** Do it\n- **Where:** —\n" + extraHead + "\n## Goal\n\nBody.\n\n## Comments\n"
	testutil.Write(t, filepath.Join(project.Work, "1-open", id+"-x.md"), text)
}

// ok runs a command that must succeed and returns what it printed.
func ok(t *testing.T, args ...string) string {
	t.Helper()
	code, out, errOut := runCLI(t, args...)
	if code != 0 {
		t.Fatalf("kanbanzai %s: exit %d\n%s%s", strings.Join(args, " "), code, out, errOut)
	}
	return out
}

// refused runs a command that must be refused with exit 2 and a message containing want.
func refused(t *testing.T, want string, args ...string) {
	t.Helper()
	code, out, errOut := runCLI(t, args...)
	if code != 2 || !strings.Contains(errOut, want) {
		t.Fatalf("kanbanzai %s: exit %d, want 2 with %q\n%s%s", strings.Join(args, " "), code, want, out, errOut)
	}
}

func stageOf(t *testing.T, id string) string {
	t.Helper()
	var parsed map[string]any
	if err := json.Unmarshal([]byte(ok(t, "board", "--json")), &parsed); err != nil {
		t.Fatal(err)
	}
	return cardStage(parsed, id)
}

func TestEarlyCloseThroughCLI(t *testing.T) {
	project := newProject(t, "config_version = 1\n[project]\nowner = \"Sam\"\n")
	putDeliveryCard(t, project, "W-01", "")
	refused(t, "needs --reason", "move", "W-01", "close")
	ok(t, "move", "W-01", "close", "--reason", "false positive: reload needed")
	atStage(t, "W-01", "done")
	data, err := os.ReadFile(filepath.Join(project.Work, "10-done", "W-01-x.md"))
	if err != nil || !strings.Contains(string(data), "closed, no change: false positive: reload needed") {
		t.Fatalf("brief: %s %v", data, err)
	}
	log, err := os.ReadFile(filepath.Join(project.State, "moves.jsonl"))
	if err != nil || !strings.Contains(string(log), `"forced":false`) || !strings.Contains(string(log), `"reason":"false positive: reload needed"`) {
		t.Fatalf("move log: %s %v", log, err)
	}
}

func atStage(t *testing.T, id, want string) {
	t.Helper()
	if got := stageOf(t, id); got != want {
		t.Fatalf("%s is in %q, want %q", id, got, want)
	}
}

// settle ends a scenario: the board checks, the card is Done and a wip audit passes once the work is committed.
func settle(t *testing.T, project *config.Project, ids ...string) {
	t.Helper()
	if out := ok(t, "check"); !strings.Contains(out, "board ok") {
		t.Fatalf("check = %q", out)
	}
	for _, id := range ids {
		atStage(t, id, "done")
		ok(t, "claim", "release", id)
	}
	testutil.Git(t, project.Root, "add", "-A")
	testutil.Git(t, project.Root, "commit", "-q", "-m", "board")
	if out := ok(t, "wip", "--audit"); !strings.Contains(out, "Audit: pass") {
		t.Fatalf("audit = %q", out)
	}
}

func TestDocsOnlyFlowClosesFromItsLastCheckWithNoDeployColumn(t *testing.T) {
	project := deliveryProject(t, "docs-only")
	putDeliveryCard(t, project, "W-01", "")
	if out := ok(t, "claim", "claim", "W-01"); !strings.Contains(out, "moved W-01: Todo -> Writing") {
		t.Fatalf("claim = %q", out)
	}
	ok(t, "move", "W-01", "finish", "--next", "check it")
	atStage(t, "W-01", "written")
	if out := ok(t, "step", "--for", "W-01"); !strings.Contains(out, "the next step is `check`") ||
		strings.Contains(out, "gate:") {
		t.Fatalf("step = %q", out)
	}
	ok(t, "move", "W-01", "start")
	refused(t, "needs --done", "move", "W-01", "finish")
	ok(t, "move", "W-01", "finish", "--done", "published to the site")
	settle(t, project, "W-01")
	if columns := len(project.Columns); columns != 6 {
		t.Fatalf("the docs flow has %d columns", columns)
	}
}

func TestCIGatedFlowWaitsForARecordedResultAndNeedsNoOwner(t *testing.T) {
	project := deliveryProject(t, "ci-gated")
	putDeliveryCard(t, project, "W-01", "")
	ok(t, "claim", "claim", "W-01")
	ok(t, "move", "W-01", "finish")
	atStage(t, "W-01", "built")

	refused(t, "W-01 is held by the Built gate (ci: GitHub Actions)", "move", "W-01", "start")
	if out := ok(t, "step", "--for", "W-01"); !strings.Contains(out, "gate: W-01 is held by the Built gate") {
		t.Fatalf("step = %q", out)
	}
	if out := ok(t, "wip"); !strings.Contains(out, "W-01 🔒") {
		t.Fatalf("wip does not mark the waiting card:\n%s", out)
	}
	ok(t, "gate", "fail", "W-01", "--reason", "the lint job is red")
	refused(t, "the lint job is red", "move", "W-01", "start")
	// Nobody had to approve: the recorded result is all the gate asks for.
	out := ok(t, "gate", "pass", "W-01")
	if !strings.Contains(out, "gate passed on Built (ci)") {
		t.Fatalf("pass = %q", out)
	}
	ok(t, "move", "W-01", "start")
	if got := ok(t, "step", "--for", "W-01", "--json"); strings.Contains(got, `"state":"closed"`) {
		t.Fatalf("a card past its gate still has one: %s", got)
	}
	ok(t, "move", "W-01", "finish", "--done", "released v1.0")
	settle(t, project, "W-01")
}

func TestReleaseTrainHoldsApprovedCardsUntilTheTrainOpens(t *testing.T) {
	project := deliveryProject(t, "release-train")
	for _, id := range []string{"W-01", "W-02"} {
		putDeliveryCard(t, project, id, "")
		ok(t, "claim", "claim", id)
		ok(t, "move", id, "finish")
		atStage(t, id, "approved")
		refused(t, "(external: train)", "move", id, "start")
	}
	// A card that is still being built is not waiting at the train.
	putDeliveryCard(t, project, "W-03", "")
	if out := ok(t, "gate", "status"); !strings.Contains(out, "W-01 is in Approved behind a gate (external: train): closed") ||
		!strings.Contains(out, "W-02 is in Approved") || strings.Contains(out, "W-03") {
		t.Fatalf("status = %q", out)
	}
	// The train opens once, for every card waiting for it.
	if out := ok(t, "gate", "pass", "W-01", "W-02", "--note", "train 2026.11 leaves today"); strings.Count(out, "gate passed") != 2 {
		t.Fatalf("pass = %q", out)
	}
	for _, id := range []string{"W-01", "W-02"} {
		ok(t, "move", id, "start")
		ok(t, "move", id, "finish", "--done", "shipped on the train")
	}
	settle(t, project, "W-01", "W-02")
}

func TestStoreReviewFlowWaitsForTheStoreThenPublishes(t *testing.T) {
	project := deliveryProject(t, "store-review")
	putDeliveryCard(t, project, "W-01", "")
	ok(t, "claim", "claim", "W-01")
	ok(t, "move", "W-01", "finish")
	ok(t, "move", "W-01", "start")
	ok(t, "move", "W-01", "finish", "--next", "wait for the store")
	atStage(t, "W-01", "in-review")
	refused(t, "(external: store review)", "move", "W-01", "start")
	ok(t, "gate", "pass", "W-01", "--note", "approved in App Store Connect")
	ok(t, "move", "W-01", "start")
	ok(t, "move", "W-01", "finish", "--done", "live in the store")
	settle(t, project, "W-01")
}

func TestOwnerApprovalFlowIsTheShippedFlowWithTheApprovalRecorded(t *testing.T) {
	project := deliveryProject(t, "owner-approval")
	putDeliveryCard(t, project, "W-01", "")
	ok(t, "claim", "claim", "W-01")
	if err := os.MkdirAll(project.WorktreePath("W-01"), 0o755); err != nil {
		t.Fatal(err)
	}
	where := []string{"--where", "worktree W-01"}
	ok(t, append([]string{"move", "W-01", "finish"}, where...)...)
	for _, want := range []string{"built", "checked"} {
		ok(t, append([]string{"move", "W-01", "start"}, where...)...)
		ok(t, "move", "W-01", "finish")
		atStage(t, "W-01", want)
	}
	ok(t, "move", "W-01", "start") // In QA
	ok(t, "move", "W-01", "finish")
	atStage(t, "W-01", "qa-approved")

	refused(t, "W-01 is held by the QA-approved gate (owner)", "move", "W-01", "start")
	refused(t, "needs --note", "gate", "pass", "W-01")
	if out := ok(t, "gate", "status", "W-01"); !strings.Contains(out, "behind a gate (owner): closed") ||
		!strings.Contains(out, "opens when Sam's approval is recorded") {
		t.Fatalf("status = %q", out)
	}
	ok(t, "gate", "pass", "W-01", "--note", "Sam: yes, ship it")
	ok(t, "move", "W-01", "start") // Shipping
	if strings.Contains(readBrief(t, project, "W-01"), "Gate:") {
		t.Fatal("the Gate line survived the move out of QA-approved")
	}
	ok(t, "move", "W-01", "finish", "--done", "v1 live")
	settle(t, project, "W-01")
}

func TestOwnerApprovalFlowLetsADeployNoneCardSkipTheGate(t *testing.T) {
	project := deliveryProject(t, "owner-approval")
	putDeliveryCard(t, project, "W-01", "- **Deploy:** None (tooling)\n")
	ok(t, "claim", "claim", "W-01")
	if err := os.MkdirAll(project.WorktreePath("W-01"), 0o755); err != nil {
		t.Fatal(err)
	}
	where := []string{"--where", "worktree W-01"}
	ok(t, append([]string{"move", "W-01", "finish"}, where...)...)
	for range []string{"build", "check"} {
		ok(t, append([]string{"move", "W-01", "start"}, where...)...)
		ok(t, "move", "W-01", "finish")
	}
	atStage(t, "W-01", "checked")
	if out := ok(t, "step", "--for", "W-01"); !strings.Contains(out, "skip: this card skips `deploy-qa`, `ship` (Deploy: None (tooling)) and closes") {
		t.Fatalf("step = %q", out)
	}
	ok(t, "move", "W-01", "done", "--done", "no deploy")
	settle(t, project, "W-01")
}

func TestScheduledFlowOpensOnItsDate(t *testing.T) {
	for date, opens := range map[string]bool{"2999-01-01": false, "2000-01-01": true} {
		project := deliveryProject(t, "scheduled", "@DATE@", date)
		putDeliveryCard(t, project, "W-01", "")
		ok(t, "claim", "claim", "W-01")
		ok(t, "move", "W-01", "finish")
		atStage(t, "W-01", "built")
		if opens {
			ok(t, "move", "W-01", "start")
			ok(t, "move", "W-01", "finish", "--done", "shipped on the day")
			settle(t, project, "W-01")
			continue
		}
		refused(t, "the date is "+date+" or later", "move", "W-01", "start")
		refused(t, "is computed", "gate", "pass", "W-01")
	}
}

func TestSiblingCardGateHoldsACardUntilTheCardItNeedsIsDone(t *testing.T) {
	project := deliveryProject(t, "sibling-card")
	putDeliveryCard(t, project, "W-01", "")
	putDeliveryCard(t, project, "W-02", "- **Needs:** W-01\n")

	// W-02 is claimed first, but its gate keeps it in Todo: the claim says so rather than moving it.
	if out := ok(t, "claim", "claim", "W-02"); !strings.Contains(out, "not moved: W-02 is held by the Todo gate (card)") ||
		!strings.Contains(out, "waiting for W-01 (in Todo) to reach Done") {
		t.Fatalf("claim = %q", out)
	}
	atStage(t, "W-02", "todo")
	if out := ok(t, "gate", "status", "W-02"); !strings.Contains(out, "closed") {
		t.Fatalf("status = %q", out)
	}
	ok(t, "claim", "claim", "W-01")
	ok(t, "move", "W-01", "finish", "--done", "first")
	atStage(t, "W-01", "done")
	ok(t, "claim", "release", "W-01")

	ok(t, "move", "W-02", "start")
	atStage(t, "W-02", "building")
	ok(t, "move", "W-02", "finish", "--done", "second")
	settle(t, project, "W-01", "W-02")
}

func readBrief(t *testing.T, project *config.Project, id string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(project.Work, "1-open", id+"-x.md"))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
