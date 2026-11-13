package main

import (
	"strings"
	"testing"
)

func ready(id string) Candidate {
	return Candidate{
		ID:     id,
		Stage:  "checked",
		Column: "checked",
		Route:  Route{Action: "step", Step: "deploy-qa"},
		Merge:  MergeYes,
	}
}

func TestClassify(t *testing.T) {
	cases := []struct {
		name   string
		change func(*Candidate)
		want   bool
		reason string
	}{
		{"eligible", func(c *Candidate) {}, true, ""},
		{"another session's claim", func(c *Candidate) { c.Owner = "claude:other" }, false, "claimed by claude:other"},
		{"expired claim still names an owner", func(c *Candidate) { c.Owner = "claude:gone" }, false, "claimed"},
		{"held", func(c *Candidate) { c.Held = "waiting: the owner" }, false, "held: waiting: the owner"},
		{"blocked route", func(c *Candidate) { c.Route.Blocked = true; c.Route.Reason = "gate closed" }, false, "blocked: gate closed"},
		{"deploy none", func(c *Candidate) { c.SkippedSteps = []string{"deploy-qa", "ship"} }, false, "Deploy is None"},
		{"not in checked on the board", func(c *Candidate) { c.Stage = "built" }, false, "not in Checked"},
		{"not in checked by the route", func(c *Candidate) { c.Column = "in-qa" }, false, "not in Checked"},
		{"route is another step", func(c *Candidate) { c.Route.Step = "check" }, false, "route is not deploy-qa"},
		{"unrecognised route action", func(c *Candidate) { c.Route.Action = "close" }, false, "route action"},
		{"branch not merged", func(c *Candidate) { c.Merge = MergeNo }, false, "not merged"},
		{"merge unproven", func(c *Candidate) { c.Merge = MergeUnknown }, false, "could not prove"},
		{"read error", func(c *Candidate) { c.Err = "step --for: exit 2" }, false, "could not read"},
		{"nothing filled in", func(c *Candidate) { *c = Candidate{ID: c.ID} }, false, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := ready("CHANGE-1")
			tc.change(&c)
			got := Classify(c)
			if got.ID != "CHANGE-1" {
				t.Fatalf("decision names %q", got.ID)
			}
			if got.Include != tc.want {
				t.Fatalf("include = %v (%s), want %v", got.Include, got.Reason, tc.want)
			}
			if !tc.want && got.Reason == "" {
				t.Fatal("a card left out must say why")
			}
			if !strings.Contains(got.Reason, tc.reason) {
				t.Fatalf("reason %q lacks %q", got.Reason, tc.reason)
			}
			if tc.want && got.Reason != "" {
				t.Fatalf("an included card carries reason %q", got.Reason)
			}
		})
	}
}

// Three eligible cards are all included, in the order given, whatever else is on the board.
func TestClassifyManyEligible(t *testing.T) {
	cards := []Candidate{ready("BUG-1"), ready("CHANGE-2"), ready("TASK-3")}
	for _, c := range cards {
		if d := Classify(c); !d.Include {
			t.Fatalf("%s left out: %s", c.ID, d.Reason)
		}
	}
}
