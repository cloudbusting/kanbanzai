package claim

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cloudbusting/kanbanzai/internal/config"
	"github.com/cloudbusting/kanbanzai/internal/testutil"
	"github.com/cloudbusting/kanbanzai/internal/timeutil"
)

const defaultConfig = "[project]\nowner = \"Sam\"\n"

func testProject(t *testing.T, text string) *config.Project {
	t.Helper()
	t.Setenv("KANBANZAI_PROJECT", "")
	root := testutil.Repo(t, text)
	testutil.Folders(t, root, "1-open", "x-paused", "10-done")
	project, err := config.Find(root)
	if err != nil {
		t.Fatalf("config.Find: %v", err)
	}
	return project
}

func clearIdentity(t *testing.T) {
	t.Helper()
	for _, key := range []string{"KANBANZAI_PROJECT", "KANBANZAI_OWNER", "MY_SESSION", "OPENCODE_SESSION_ID",
		"CLAUDE_CODE_SESSION_ID", "CLAUDE_SESSION_ID", "LOGNAME", "USER", "LNAME", "USERNAME"} {
		t.Setenv(key, "")
	}
	old := parentProcess
	parentProcess = func() string { return "" }
	t.Cleanup(func() { parentProcess = old })
}

func floatPtr(value float64) *float64 { return &value }

func stringPtr(value string) *string { return &value }

func TestStoreAndThePythonClaim(t *testing.T) {
	project := testProject(t, defaultConfig)
	store, err := NewStore(project)
	if err != nil {
		t.Fatal(err)
	}
	fixture, err := os.ReadFile(filepath.Join("testdata", "python-claim.json"))
	if err != nil {
		t.Fatal(err)
	}
	testutil.Write(t, store.Path("W-01"), string(fixture))
	record, ok := store.Load("W-01")
	if !ok {
		t.Fatal("the Python claim did not load")
	}
	if record["owner"] != "tester:python" || record["note"] != "written by the Python core" {
		t.Fatalf("record = %#v", record)
	}
	if !Live(record, timeutil.Now()) {
		t.Fatal("the fixture should be live")
	}
	if !strings.Contains(Describe(record, "tester:python"), "held by you") {
		t.Fatalf("describe = %q", Describe(record, "tester:python"))
	}

	var out, errOut bytes.Buffer
	if code := Renew(project, "tester:python", "W-01", floatPtr(60), &out, &errOut); code != OK {
		t.Fatalf("renew: code %d, %q", code, errOut.String())
	}
	record, _ = store.Load("W-01")
	if record["claimed_at"] != "2026-11-11T05:00:00+00:00" {
		t.Fatalf("claimed_at changed: %#v", record["claimed_at"])
	}
	if record["minutes"] != float64(60) || record["note"] != "written by the Python core" {
		t.Fatalf("renewed record = %#v", record)
	}
	if record["branch"] != "W-01" || record["where"] != "/tmp/kanbanzai-W-01" {
		t.Fatalf("branch/where changed: %#v", record)
	}
	if history := readHistory(t, project); len(history) != 1 || history[0]["event"] != "renew" {
		t.Fatalf("history = %#v", history)
	}
	if _, ok := store.Records()["W-01"]; !ok {
		t.Fatal("the record is missing from the store")
	}
	if err := store.Remove("W-01"); err != nil {
		t.Fatal(err)
	}
	if _, ok := store.Load("W-01"); ok {
		t.Fatal("the record was not removed")
	}
}

func TestClaimStatusRenewRelease(t *testing.T) {
	project := testProject(t, defaultConfig)
	store, _ := NewStore(project)
	var out, errOut bytes.Buffer

	code := Claim(project, ClaimOptions{Card: "W-01", Owner: "tester:me", Note: "building", Out: &out,
		Err: &errOut})
	if code != OK {
		t.Fatalf("claim: %d %q", code, errOut.String())
	}
	if !strings.Contains(out.String(), "claimed W-01 by tester:me") ||
		!strings.Contains(out.String(), "owner: tester:me") {
		t.Fatalf("output = %q", out.String())
	}
	record, ok := store.Load("W-01")
	if !ok {
		t.Fatal("no claim file")
	}
	if record["owner"] != "tester:me" || record["note"] != "building" || record["branch"] != "main" {
		t.Fatalf("record = %#v", record)
	}
	if record["where"] != project.Toplevel {
		t.Fatalf("where = %#v, want %q", record["where"], project.Toplevel)
	}
	if record["minutes"] != float64(120) {
		t.Fatalf("minutes = %#v", record["minutes"])
	}
	if !strings.HasSuffix(record["expires_at"].(string), "+00:00") {
		t.Fatalf("expires_at = %#v", record["expires_at"])
	}

	out.Reset()
	if code := Status(project, "tester:me", "W-01", &out, &errOut); code != OK {
		t.Fatalf("status: %d %q", code, errOut.String())
	}
	if !strings.Contains(out.String(), "held by you") {
		t.Fatalf("status = %q", out.String())
	}

	out.Reset()
	if code := Claim(project, ClaimOptions{Card: "W-01", Owner: "tester:me", Out: &out, Err: &errOut}); code != OK {
		t.Fatalf("renew by claim: %d %q", code, errOut.String())
	}
	if !strings.HasPrefix(strings.TrimSpace(out.String()), "renewed") {
		t.Fatalf("output = %q", out.String())
	}

	out.Reset()
	if code := Renew(project, "tester:me", "W-01", floatPtr(30), &out, &errOut); code != OK {
		t.Fatalf("renew: %d %q", code, errOut.String())
	}
	record, _ = store.Load("W-01")
	if record["minutes"] != float64(30) {
		t.Fatalf("renewed minutes = %#v", record["minutes"])
	}

	out.Reset()
	if code := Release(project, "tester:me", "W-01", &out, &errOut); code != OK {
		t.Fatalf("release: %d %q", code, errOut.String())
	}
	if strings.TrimSpace(out.String()) != "released W-01" {
		t.Fatalf("release = %q", out.String())
	}
	if _, ok := store.Load("W-01"); ok {
		t.Fatal("the claim survived release")
	}
	out.Reset()
	if code := Status(project, "tester:me", "W-01", &out, &errOut); code != OK ||
		strings.TrimSpace(out.String()) != "W-01: free" {
		t.Fatalf("status after release: %d %q", code, out.String())
	}
	out.Reset()
	if code := Status(project, "tester:me", "", &out, &errOut); code != OK ||
		strings.TrimSpace(out.String()) != "no claims" {
		t.Fatalf("empty status: %d %q", code, out.String())
	}
	history := readHistory(t, project)
	events := map[string]int{}
	for _, entry := range history {
		events[entry["event"].(string)]++
	}
	if events["claim"] != 1 || events["renew"] != 2 || events["release"] != 1 {
		t.Fatalf("history = %#v", events)
	}
}

func TestAnotherLiveClaimIsHeld(t *testing.T) {
	project := testProject(t, defaultConfig)
	var out, errOut bytes.Buffer
	if code := Claim(project, ClaimOptions{Card: "W-02", Owner: "tester:other", Out: &out, Err: &errOut}); code != OK {
		t.Fatalf("other claim: %d %q", code, errOut.String())
	}
	errOut.Reset()
	if code := Claim(project, ClaimOptions{Card: "W-02", Owner: "tester:me", Out: &out, Err: &errOut}); code != Held {
		t.Fatalf("claim = %d, want %d", code, Held)
	}
	if !strings.Contains(errOut.String(), "held by tester:other") {
		t.Fatalf("claim error = %q", errOut.String())
	}
	if code := Status(project, "tester:me", "W-02", &out, &errOut); code != Held {
		t.Fatalf("status = %d", code)
	}
	if code := Renew(project, "tester:me", "W-02", nil, &out, &errOut); code != Held {
		t.Fatalf("renew = %d", code)
	}
	if code := Release(project, "tester:me", "W-02", &out, &errOut); code != Held {
		t.Fatalf("release = %d", code)
	}
	record := loadClaim(t, project, "W-02")
	if record["owner"] != "tester:other" {
		t.Fatalf("owner changed: %#v", record["owner"])
	}
}

func TestExpiredClaimNeedsATakeover(t *testing.T) {
	project := testProject(t, defaultConfig)
	store, _ := NewStore(project)
	var out, errOut bytes.Buffer
	if code := Claim(project, ClaimOptions{Card: "W-03", Owner: "tester:other", Out: &out, Err: &errOut}); code != OK {
		t.Fatalf("other claim: %d", code)
	}
	record, _ := store.Load("W-03")
	record["expires_at"] = "2026-01-01T00:00:00+00:00"
	if err := store.Save(record); err != nil {
		t.Fatal(err)
	}
	errOut.Reset()
	if code := Claim(project, ClaimOptions{Card: "W-03", Owner: "tester:me", Out: &out, Err: &errOut}); code != Expired {
		t.Fatalf("claim = %d, want %d", code, Expired)
	}
	if !strings.Contains(errOut.String(), "EXPIRED") {
		t.Fatalf("claim error = %q", errOut.String())
	}
	if code := Status(project, "tester:me", "W-03", &out, &errOut); code != Expired {
		t.Fatalf("status = %d", code)
	}
	out.Reset()
	if code := Takeover(project, ClaimOptions{Card: "W-03", Owner: "tester:me", Out: &out, Err: &errOut},
		"the owner's session died"); code != OK {
		t.Fatalf("takeover = %d %q", code, errOut.String())
	}
	if !strings.Contains(out.String(), "claimed W-03 by tester:me") {
		t.Fatalf("takeover output = %q", out.String())
	}
	record, _ = store.Load("W-03")
	if record["owner"] != "tester:me" {
		t.Fatalf("owner = %#v", record["owner"])
	}
	found := false
	for _, entry := range readHistory(t, project) {
		if entry["event"] == "takeover" && entry["reason"] == "the owner's session died" {
			found = true
		}
	}
	if !found {
		t.Fatal("the takeover was not logged with its reason")
	}
}

func TestAnyoneMayReleaseAnExpiredClaim(t *testing.T) {
	project := testProject(t, defaultConfig)
	store, _ := NewStore(project)
	var out, errOut bytes.Buffer
	if code := Claim(project, ClaimOptions{Card: "W-04", Owner: "tester:other", Out: &out, Err: &errOut}); code != OK {
		t.Fatalf("other claim: %d", code)
	}
	record, _ := store.Load("W-04")
	record["expires_at"] = "2026-01-01T00:00:00+00:00"
	_ = store.Save(record)
	out.Reset()
	if code := Release(project, "tester:me", "W-04", &out, &errOut); code != OK {
		t.Fatalf("release = %d %q", code, errOut.String())
	}
	if !strings.Contains(out.String(), "expired claim by tester:other") {
		t.Fatalf("release = %q", out.String())
	}
	if _, ok := store.Load("W-04"); ok {
		t.Fatal("the expired claim survived")
	}
}

func TestBadInputs(t *testing.T) {
	project := testProject(t, defaultConfig)
	var out, errOut bytes.Buffer
	if code := Claim(project, ClaimOptions{Card: "banana", Owner: "tester:me", Out: &out, Err: &errOut}); code != Error {
		t.Fatalf("bad card = %d", code)
	}
	if !strings.Contains(errOut.String(), "card must look like W-NN") {
		t.Fatalf("error = %q", errOut.String())
	}
	errOut.Reset()
	if code := Renew(project, "tester:me", "W-09", nil, &out, &errOut); code != Error {
		t.Fatalf("missing renew = %d", code)
	}
	if !strings.Contains(errOut.String(), "has no claim to renew") {
		t.Fatalf("error = %q", errOut.String())
	}
	errOut.Reset()
	if code := Claim(project, ClaimOptions{Card: "W-05", Owner: "tester:me", Minutes: floatPtr(0), Out: &out,
		Err: &errOut}); code != Error {
		t.Fatalf("zero minutes = %d", code)
	}
	if !strings.Contains(errOut.String(), "greater than zero") {
		t.Fatalf("error = %q", errOut.String())
	}
	if _, err := CheckOwner("two words"); err == nil {
		t.Fatal("a two-word owner was accepted")
	}
	if _, err := CheckOwner("tester:a\n"); err == nil {
		t.Fatal("a trailing newline owner was accepted")
	}
	if _, err := CheckCard(project, "W-01\n"); err == nil {
		t.Fatal("a trailing newline card was accepted")
	}
	if _, err := CheckCard(project, "W-01"); err != nil {
		t.Fatalf("W-01: %v", err)
	}
}

func TestOwnerCommand(t *testing.T) {
	project := testProject(t, defaultConfig)
	var out bytes.Buffer
	if code := Owner(project, "tester:me", &out); code != OK {
		t.Fatalf("owner = %d", code)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 2 || lines[0] != "tester:me" || lines[1] != "state: "+project.Claims {
		t.Fatalf("owner = %q", out.String())
	}
}

func TestClaimStartCallbackAndNoMove(t *testing.T) {
	project := testProject(t, defaultConfig)
	var out, errOut bytes.Buffer
	called := false
	code := Claim(project, ClaimOptions{
		Card: "W-06", Owner: "tester:me", Out: &out, Err: &errOut,
		Start: func(card, owner string) (string, error) {
			called = true
			if card != "W-06" || owner != "tester:me" {
				t.Fatalf("start(%q, %q)", card, owner)
			}
			return "moved W-06: Todo -> Designing", nil
		},
	})
	if code != OK || !called || !strings.Contains(out.String(), "moved W-06: Todo -> Designing") {
		t.Fatalf("claim = %d, called %v, out %q", code, called, out.String())
	}

	out.Reset()
	code = Claim(project, ClaimOptions{Card: "W-07", Owner: "tester:me", NoMove: true, Out: &out, Err: &errOut,
		Start: func(card, owner string) (string, error) {
			t.Fatal("start was called for --no-move")
			return "", nil
		}})
	if code != OK {
		t.Fatalf("no-move claim = %d", code)
	}
	record := loadClaim(t, project, "W-07")
	if record["step"] != false {
		t.Fatalf("step = %#v", record["step"])
	}

	out.Reset()
	code = Claim(project, ClaimOptions{Card: "W-08", Owner: "tester:me", Out: &out, Err: &errOut,
		Start: func(card, owner string) (string, error) {
			return "", errors.New("no worktree")
		}})
	if code != OK || !strings.Contains(out.String(), "not moved: no worktree") {
		t.Fatalf("failed start: %d %q", code, out.String())
	}
}

func TestOwnerLabel(t *testing.T) {
	project := testProject(t, "[project]\nowner = \"Sam\"\n[claims]\nowner_env = \"MY_SESSION\"\n")
	clearIdentity(t)

	if got := OwnerLabel(project); !strings.Contains(got, "@") {
		t.Fatalf("last resort = %q", got)
	}
	t.Setenv("MY_SESSION", "mine")
	t.Setenv("KANBANZAI_OWNER", "shipped")
	if got := OwnerLabel(project); got != "mine" {
		t.Fatalf("configured label = %q", got)
	}
	t.Setenv("MY_SESSION", "")
	if got := OwnerLabel(project); got != "shipped" {
		t.Fatalf("shipped label = %q", got)
	}
	t.Setenv("KANBANZAI_OWNER", "")
	t.Setenv("OPENCODE_SESSION_ID", "ses_1")
	t.Setenv("CLAUDE_SESSION_ID", "c")
	if got := OwnerLabel(project); got != "opencode:ses_1" {
		t.Fatalf("opencode label = %q", got)
	}
	t.Setenv("OPENCODE_SESSION_ID", "")
	t.Setenv("CLAUDE_SESSION_ID", "abc")
	if got := OwnerLabel(project); got != "claude:abc" {
		t.Fatalf("claude label = %q", got)
	}
	t.Setenv("CLAUDE_SESSION_ID", "")
	t.Setenv("CLAUDE_CODE_SESSION_ID", "193f0c70-5c58-4f5f-affa-4b4c839e72b5")
	if got := OwnerLabel(project); got != "claude:193f0c70-5c58-4f5f-affa-4b4c839e72b5" {
		t.Fatalf("claude code label = %q", got)
	}
	if _, err := CheckOwner(OwnerLabel(project)); err != nil {
		t.Fatalf("the label does not fit the owner pattern: %v", err)
	}
	t.Setenv("CLAUDE_CODE_SESSION_ID", "")
	parentProcess = func() string { return "claude:77" }
	if got := OwnerLabel(project); got != "claude:77" {
		t.Fatalf("parent label = %q", got)
	}
	parentProcess = func() string { return "" }
	t.Setenv("MY_SESSION", "  ")
	t.Setenv("KANBANZAI_OWNER", "shipped")
	if got := OwnerLabel(project); got != "shipped" {
		t.Fatalf("empty variable label = %q", got)
	}
}

func TestHeartbeat(t *testing.T) {
	project := testProject(t, defaultConfig)
	store, _ := NewStore(project)
	put := func(card, owner string, leftMinutes float64) {
		now := timeutil.Now()
		err := store.Save(map[string]any{
			"card": card, "owner": owner, "claimed_at": timeutil.ToISO(now),
			"renewed_at": timeutil.ToISO(now),
			"expires_at": timeutil.ToISO(now.Add(time.Duration(leftMinutes * float64(time.Minute)))),
			"minutes":    120.0,
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	left := func(card string) float64 {
		record, _ := store.Load(card)
		return LeaseLeft(record, timeutil.Now()) / 60
	}

	put("W-01", "tester:me", 20)
	put("W-02", "tester:me", 100)
	put("W-03", "tester:other", 5)
	put("W-04", "tester:me", -5)
	if code := Heartbeat(project, HeartbeatOptions{Owner: "tester:me"}); code != OK {
		t.Fatalf("heartbeat = %d", code)
	}
	if left("W-01") < 119 || left("W-04") < 119 {
		t.Fatalf("my used leases were not renewed: %v %v", left("W-01"), left("W-04"))
	}
	if left("W-02") < 99 || left("W-02") > 101 {
		t.Fatalf("a lease with time in hand was touched: %v", left("W-02"))
	}
	if left("W-03") > 6 {
		t.Fatalf("another session's lease was touched: %v", left("W-03"))
	}
	put("W-05", "tester:me", 10)
	put("W-06", "tester:me", 10)
	if code := Heartbeat(project, HeartbeatOptions{Owner: "tester:me"}); code != OK {
		t.Fatalf("heartbeat = %d", code)
	}
	if left("W-05") < 119 || left("W-06") < 119 {
		t.Fatalf("not every claim was renewed: %v %v", left("W-05"), left("W-06"))
	}

	// A broken record is ignored and a broken store never fails the hook.
	testutil.Write(t, store.Path("W-07"), "{not json")
	if code := Heartbeat(project, HeartbeatOptions{Owner: "tester:me"}); code != OK {
		t.Fatalf("broken record = %d", code)
	}
	broken := testProject(t, defaultConfig)
	testutil.Write(t, broken.Claims, "not a directory")
	if code := Heartbeat(broken, HeartbeatOptions{Owner: "tester:me"}); code != OK {
		t.Fatalf("broken store = %d", code)
	}
}

func TestHeartbeatHookSession(t *testing.T) {
	project := testProject(t, defaultConfig)
	clearIdentity(t)
	store, _ := NewStore(project)
	put := func(card, owner string, leftMinutes float64) {
		now := timeutil.Now()
		_ = store.Save(map[string]any{
			"card": card, "owner": owner, "claimed_at": timeutil.ToISO(now),
			"renewed_at": timeutil.ToISO(now),
			"expires_at": timeutil.ToISO(now.Add(time.Duration(leftMinutes * float64(time.Minute)))),
			"minutes":    120.0,
		})
	}
	left := func(card string) float64 {
		record, _ := store.Load(card)
		return LeaseLeft(record, timeutil.Now()) / 60
	}
	put("W-05", "claude:sid-1", 10)
	put("W-06", "claude:sid-2", 10)
	if code := Heartbeat(project, HeartbeatOptions{Hook: true, In: strings.NewReader(
		`{"session_id": "sid-1", "tool_name": "Bash"}`)}); code != OK {
		t.Fatalf("heartbeat = %d", code)
	}
	if left("W-05") < 119 {
		t.Fatalf("the hook did not renew its session: %v", left("W-05"))
	}
	if left("W-06") > 11 {
		t.Fatalf("the hook touched another session: %v", left("W-06"))
	}
	if record, _ := store.Load("W-05"); record["owner"] != "claude:sid-1" {
		t.Fatalf("owner = %#v", record["owner"])
	}

	t.Setenv("CLAUDE_CODE_SESSION_ID", "sid-1")
	put("W-07", "claude:sid-1", 10)
	put("W-08", "claude:sid-2", 10)
	if code := Heartbeat(project, HeartbeatOptions{Hook: true, In: strings.NewReader(
		`{"session_id": "other"}`)}); code != OK {
		t.Fatalf("heartbeat with env = %d", code)
	}
	if left("W-07") < 119 || left("W-08") > 11 {
		t.Fatalf("the environment did not win: %v %v", left("W-07"), left("W-08"))
	}

	t.Setenv("CLAUDE_CODE_SESSION_ID", "")
	put("W-09", "claude:sid-9", 10)
	for _, stdin := range []string{"", "{not json", "[]", `{"session_id": 5}`, `{"session_id": "two words"}`} {
		if code := Heartbeat(project, HeartbeatOptions{Hook: true, In: strings.NewReader(stdin)}); code != OK {
			t.Fatalf("bad stdin %q = %d", stdin, code)
		}
	}
	if left("W-09") > 11 {
		t.Fatalf("a bad stdin renewed a claim: %v", left("W-09"))
	}
}

func TestLeaseAndTimeText(t *testing.T) {
	now := time.Date(2026, 11, 11, 6, 0, 0, 0, time.UTC)
	lease := Lease(now, 120)
	if lease["renewed_at"] != "2026-11-11T06:00:00+00:00" ||
		lease["expires_at"] != "2026-11-11T08:00:00+00:00" || lease["minutes"] != float64(120) {
		t.Fatalf("lease = %#v", lease)
	}
	live := map[string]any{"card": "W-01", "owner": "tester:a", "claimed_at": "2026-11-11T05:00:00+00:00",
		"expires_at": "2026-11-11T08:00:00+00:00"}
	if !Live(live, now) {
		t.Fatal("a live claim read as expired")
	}
	if got := Remaining(live, now); got != "2h 0m left" {
		t.Fatalf("remaining = %q", got)
	}
	if got := Remaining(map[string]any{"expires_at": "2026-11-11T06:05:00+00:00"}, now); got != "0h 5m left" {
		t.Fatalf("remaining = %q", got)
	}
	if got := Remaining(map[string]any{"expires_at": "2026-11-11T05:00:00+00:00"}, now); got != "expired 1h" {
		t.Fatalf("expired = %q", got)
	}
	if got := Remaining(map[string]any{"expires_at": "2026-11-11T05:55:00+00:00"}, now); got != "expired 5m" {
		t.Fatalf("expired = %q", got)
	}
	if got := Remaining(map[string]any{}, now); got != "unknown" {
		t.Fatalf("unknown = %q", got)
	}
	if got := When("2026-11-11T06:00:00+00:00"); got != "2026-11-11 06:00 UTC" {
		t.Fatalf("when = %q", got)
	}
	if got := When("nope"); got != "unknown" {
		t.Fatalf("when unknown = %q", got)
	}
	// Describe reads the real clock, so its records are built around now rather than the fixed now above.
	realNow := timeutil.Now()
	if text := Describe(map[string]any{"card": "W-01", "owner": "tester:a",
		"claimed_at": timeutil.ToISO(realNow.Add(-time.Hour)),
		"expires_at": timeutil.ToISO(realNow.Add(time.Hour))}, "tester:a"); !strings.Contains(text, "held by you") {
		t.Fatalf("describe = %q", text)
	}
	if text := Describe(map[string]any{"card": "W-01", "owner": "tester:a",
		"claimed_at": timeutil.ToISO(realNow.Add(-2 * time.Hour)),
		"expires_at": timeutil.ToISO(realNow.Add(-time.Hour))}, "tester:a"); !strings.Contains(text, "EXPIRED claim by you") {
		t.Fatalf("expired describe = %q", text)
	}
}

func TestTakeoverOfALiveClaimNeedsAReason(t *testing.T) {
	project := testProject(t, defaultConfig)
	var out, errOut bytes.Buffer
	if code := Claim(project, ClaimOptions{Card: "W-10", Owner: "tester:a", Out: &out, Err: &errOut}); code != OK {
		t.Fatalf("other claim: %d", code)
	}
	if code := Claim(project, ClaimOptions{Card: "W-10", Owner: "tester:b", Out: &out, Err: &errOut}); code != Held {
		t.Fatalf("claim without reason = %d", code)
	}
	out.Reset()
	if code := Takeover(project, ClaimOptions{Card: "W-10", Owner: "tester:b", Out: &out, Err: &errOut},
		"deliberate handover"); code != OK {
		t.Fatalf("takeover = %d %q", code, errOut.String())
	}
	record := loadClaim(t, project, "W-10")
	if record["owner"] != "tester:b" {
		t.Fatalf("owner = %#v", record["owner"])
	}
}

func loadClaim(t *testing.T, project *config.Project, card string) map[string]any {
	t.Helper()
	store, err := NewStore(project)
	if err != nil {
		t.Fatal(err)
	}
	record, ok := store.Load(card)
	if !ok {
		t.Fatalf("no claim for %s", card)
	}
	return record
}

func readHistory(t *testing.T, project *config.Project) []map[string]any {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(project.Claims, "history.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	var entries []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line == "" {
			continue
		}
		var entry map[string]any
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatal(err)
		}
		entries = append(entries, entry)
	}
	return entries
}
