package move

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestEarlyCloseRecordsNoChangeReason(t *testing.T) {
	project := testProject(t, defaultConfig)
	putCard(t, project, "W-01", "false positive", "todo", cardOptions{})
	code, _, errOut := runMove(t, project, "W-01", "close", func(o *Options) {
		o.Reason = str("false positive: reload needed")
	})
	if code != 0 {
		t.Fatalf("early close: %d %s", code, errOut)
	}
	text := readText(t, filepath.Join(project.Work, "10-done", "W-01-x.md"))
	if !strings.Contains(text, "closed, no change: false positive: reload needed") {
		t.Fatalf("missing outcome: %s", text)
	}
	moves := movesOf(t, project)
	if len(moves) != 1 || moves[0]["forced"] != false || moves[0]["reason"] != "false positive: reload needed" {
		t.Fatalf("moves = %#v", moves)
	}
}

func TestEarlyCloseRefusesUnsafeCases(t *testing.T) {
	cases := []struct {
		name, stage, folder, held, owner string
		reason                           *string
		expired                          bool
	}{
		{name: "missing reason", stage: "todo"},
		{name: "blank reason", stage: "todo", reason: str(" ")},
		{name: "held", stage: "todo", held: "blocked: dependency", reason: str("duplicate")},
		{name: "parked", stage: "paused", folder: "x-paused", reason: str("duplicate")},
		{name: "done", stage: "done", folder: "10-done", reason: str("duplicate")},
		{name: "other claim", stage: "todo", owner: "tester:other", reason: str("duplicate")},
		{name: "expired claim", stage: "todo", owner: "tester:other", expired: true, reason: str("duplicate")},
		{name: "unclaimed doing", stage: "building", reason: str("duplicate")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			project := testProject(t, defaultConfig)
			path := putCard(t, project, "W-01", "work", tc.stage, cardOptions{Folder: tc.folder, Held: tc.held})
			if tc.owner != "" {
				claimCard(t, project, "W-01", tc.owner)
				if tc.expired {
					expireClaim(t, project, "W-01")
				}
			}
			before := readText(t, path)
			code, _, _ := runMove(t, project, "W-01", "close", func(o *Options) { o.Reason = tc.reason })
			if code == 0 || readText(t, path) != before || len(movesOf(t, project)) != 0 {
				t.Fatalf("unsafe close: code=%d", code)
			}
		})
	}
}

func TestEarlyClosePreservesWorkspaceRecord(t *testing.T) {
	project := testProject(t, defaultConfig)
	putCard(t, project, "W-01", "work", "built", cardOptions{Where: "worktree W-01"})
	code, _, errOut := runMove(t, project, "W-01", "close", func(o *Options) { o.Reason = str("not needed") })
	if code != 0 {
		t.Fatalf("close: %d %s", code, errOut)
	}
	text := readText(t, filepath.Join(project.Work, "10-done", "W-01-x.md"))
	if !strings.Contains(text, "- **Where:** worktree W-01") {
		t.Fatalf("workspace lost: %s", text)
	}
}

func TestEarlyCloseApplyNormalizesReason(t *testing.T) {
	project := testProject(t, defaultConfig)
	putCard(t, project, "W-01", "work", "todo", cardOptions{})
	reason := "false positive:\n  reload needed"
	if _, err := Apply(project, "W-01", "close", "tester:me", Changes{Reason: &reason}, nil); err != nil {
		t.Fatal(err)
	}
	text := readText(t, filepath.Join(project.Work, "10-done", "W-01-x.md"))
	if !strings.Contains(text, "closed, no change: false positive: reload needed") {
		t.Fatalf("reason was not normalized in brief: %s", text)
	}
	if got := movesOf(t, project)[0]["reason"]; got != "false positive: reload needed" {
		t.Fatalf("reason in log = %#v", got)
	}
}
