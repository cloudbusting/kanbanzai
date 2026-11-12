package claim

import (
	"bytes"
	"testing"
)

const typedClaimConfig = `
[project]
owner = "Sam"
default_type = "task"

[[types]]
key = "task"
label = "Task"
prefix = "TASK"

[[types]]
key = "bug"
label = "Bug"
prefix = "BUG"
`

func TestTypedClaimIsCanonical(t *testing.T) {
	clearIdentity(t)
	project := testProject(t, typedClaimConfig)
	var out, errOut bytes.Buffer
	code := Claim(project, ClaimOptions{Card: "bug-7", Owner: "tester:me", Minutes: floatPtr(60), NoMove: true,
		Out: &out, Err: &errOut})
	if code != OK {
		t.Fatalf("claim: %d %q", code, errOut.String())
	}
	store, err := NewStore(project)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := store.Load("BUG-07"); !ok {
		t.Fatalf("claim file is not canonical: %q", out.String())
	}
	if record, ok := store.Records()["BUG-07"]; !ok || record["owner"] != "tester:me" {
		t.Fatalf("records = %#v", store.Records())
	}
	if card, err := CheckCard(project, "task-1"); err != nil || card != "TASK-01" {
		t.Fatalf("CheckCard = %q, %v", card, err)
	}
	for _, bad := range []string{"W-01", "DEF-01", "BUG-7\n"} {
		if _, err := CheckCard(project, bad); err == nil {
			t.Fatalf("CheckCard accepted %q", bad)
		}
	}
}
