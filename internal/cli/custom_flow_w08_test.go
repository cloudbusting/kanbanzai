package cli

import (
	"strings"
	"testing"

	"github.com/cloudbusting/kanbanzai/internal/testutil"
)

const w08Flow = `config_version = 1
[project]
owner = "Sam"
main_branch = "trunk"
[flow]
isolation = "shared"
pull_order = ["review", "queue"]
[[columns]]
key = "queue"
label = "Queue"
folder = "1-open"
kind = "open"
started = "claimed"
next = "write"
[[columns]]
key = "writing"
label = "Writing"
folder = "1-open"
kind = "open"
started = true
doing = "write"
[[columns]]
key = "review"
label = "Review"
lane = "Review"
folder = "1-open"
kind = "open"
started = true
next = "review"
gate = "owner"
[[columns]]
key = "reviewing"
label = "Reviewing"
lane = "Review"
folder = "1-open"
kind = "open"
started = true
doing = "review"
[[columns]]
key = "parked"
label = "Parked"
folder = "x-paused"
kind = "parked"
[[columns]]
key = "done"
label = "Done"
folder = "10-done"
kind = "done"
[steps.write]
into = "review"
during = "writing"
skill = "kanbanzai:build"
[steps.review]
into = "done"
during = "reviewing"
skill = "kanbanzai:check"
`

func TestW08CustomFlowFromClaimThroughClose(t *testing.T) {
	project := newProject(t, w08Flow)
	testutil.Git(t, project.Root, "branch", "-m", "trunk")
	putDeliveryCard(t, project, "W-01", "")
	show := ok(t, "workflow", "show")
	if !strings.Contains(show, "reviewing      Reviewing") || strings.Contains(show, "-> in-qa") {
		t.Fatalf("custom workflow: %s", show)
	}
	workspace := ok(t, "workspace", "W-01")
	if !strings.Contains(workspace, "isolation: shared") || !strings.Contains(workspace, "needs a workspace: no") {
		t.Fatalf("workspace: %s", workspace)
	}
	ok(t, "claim", "claim", "W-01")
	atStage(t, "W-01", "writing")
	if route := ok(t, "step", "--for", "W-01"); !strings.Contains(route, "write") {
		t.Fatalf("resumed route: %s", route)
	}
	ok(t, "move", "W-01", "finish")
	atStage(t, "W-01", "review")
	refused(t, "gate (owner)", "move", "W-01", "start")
	if status := ok(t, "gate", "status", "W-01"); !strings.Contains(status, "closed") {
		t.Fatalf("gate status: %s", status)
	}
	refused(t, "needs --note", "gate", "pass", "W-01")
	ok(t, "gate", "pass", "W-01", "--note", "Sam: approved")
	ok(t, "move", "W-01", "start")
	atStage(t, "W-01", "reviewing")
	ok(t, "move", "W-01", "finish", "--done", "review complete")
	atStage(t, "W-01", "done")
	ok(t, "claim", "release", "W-01")
	ok(t, "check")
}

func TestW08WorkspaceUsesConfiguredTrunk(t *testing.T) {
	project := newProject(t, strings.Replace(w08Flow, `isolation = "shared"`, `isolation = "branch"`, 1))
	testutil.Git(t, project.Root, "branch", "-m", "trunk")
	if out := ok(t, "workspace", "W-01"); !strings.Contains(out, "git switch -c W-01 trunk") {
		t.Fatalf("workspace does not use configured trunk: %s", out)
	}
}

func TestW08NoDoingColumnsFinishThroughTheClaim(t *testing.T) {
	flow := strings.Replace(w08Flow, `doing = "write"`, ``, 1)
	flow = strings.Replace(flow, `doing = "review"`, ``, 1)
	flow = strings.Replace(flow, `during = "writing"`, ``, 1)
	flow = strings.Replace(flow, `during = "reviewing"`, ``, 1)
	flow = strings.Replace(flow, "gate = \"owner\"\n", "", 1)
	project := newProject(t, flow)
	putDeliveryCard(t, project, "W-01", "")
	refused(t, "not claimed by you", "move", "W-01", "finish")
	ok(t, "claim", "claim", "W-01")
	atStage(t, "W-01", "queue")
	if route := ok(t, "step", "--for", "W-01"); !strings.Contains(route, "route: step; step: write; finish: review") {
		t.Fatalf("no-Doing route: %s", route)
	}
	ok(t, "move", "W-01", "finish")
	atStage(t, "W-01", "review")
	refused(t, "--done", "move", "W-01", "finish")
	ok(t, "move", "W-01", "finish", "--done", "review complete")
	atStage(t, "W-01", "done")
	ok(t, "claim", "release", "W-01")
	ok(t, "check")
}

func TestW08SkippedReviewClosesWithoutRunningIt(t *testing.T) {
	flow := strings.Replace(w08Flow, "gate = \"owner\"\n", "", 1)
	flow += "\nskip_field = \"Review\"\nskip_values = [\"None\"]\n"
	project := newProject(t, flow)
	putDeliveryCard(t, project, "W-01", "- **Review:** None\n")
	ok(t, "claim", "claim", "W-01")
	ok(t, "move", "W-01", "finish")
	if out := ok(t, "step", "--for", "W-01"); !strings.Contains(out, "skips `review`") || !strings.Contains(out, "closes") {
		t.Fatalf("skipped route: %s", out)
	}
	refused(t, "skip", "move", "W-01", "start")
	ok(t, "move", "W-01", "done", "--done", "review not needed")
	atStage(t, "W-01", "done")
}

func TestW08NoSkillClose(t *testing.T) {
	flow := strings.Replace(w08Flow, "gate = \"owner\"\n", "", 1)
	flow = strings.Replace(flow, `skill = "kanbanzai:check"`, `skill = ""`, 1)
	project := newProject(t, flow)
	putDeliveryCard(t, project, "W-01", "")
	ok(t, "claim", "claim", "W-01")
	ok(t, "move", "W-01", "finish")
	if out := ok(t, "step", "--for", "W-01"); !strings.Contains(out, "review") {
		t.Fatalf("no-skill route: %s", out)
	}
	if out := ok(t, "step", "review"); !strings.Contains(out, "skill is empty") || !strings.Contains(out, "closes") {
		t.Fatalf("no-skill step: %s", out)
	}
	ok(t, "move", "W-01", "done", "--done", "no review skill")
	atStage(t, "W-01", "done")
}
