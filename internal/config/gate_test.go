package config

import (
	"strings"
	"testing"

	"github.com/cloudbusting/kanbanzai/internal/testutil"
)

// gateConfig puts one extra gate or step table over the shipped defaults, so the column list is the default.
func columnsWith(extra string) string { return flowWith(extra, "") }

// flowWith is that flow with extra [[columns]] and extra keys on the finish step.
func flowWith(extra, finish string) string {
	return "config_version=1\n[flow]\npull_order=['ready','todo']\n[[columns]]\nkey='todo'\nfolder='1-open'\nstarted='claimed'\nnext='work'\n" +
		"[[columns]]\nkey='working'\nfolder='1-open'\nstarted=true\ndoing='work'\n" +
		"[[columns]]\nkey='ready'\nfolder='1-open'\nstarted=true\nnext='finish'\n" + extra +
		"[[columns]]\nkey='finishing'\nfolder='1-open'\ndoing='finish'\n" +
		"[[columns]]\nkey='done'\nfolder='10-done'\nkind='done'\n" +
		"[steps.work]\ninto='ready'\nduring='working'\nskill='x'\n[steps.finish]\ninto='done'\nduring='finishing'\nskill='y'\n" + finish
}

func TestTheShippedOwnerGateIsTheOldWaitsForOwner(t *testing.T) {
	project := loadProject(t, testutil.Repo(t, ""))
	column := project.ByKey["qa-approved"]
	if column.Gate.Type != "owner" || !column.WaitsForOwner {
		t.Fatalf("qa-approved: gate %+v waits %v", column.Gate, column.WaitsForOwner)
	}
	for _, other := range project.Columns {
		if other.Key != "qa-approved" && (other.Gate.Set() || other.WaitsForOwner) {
			t.Fatalf("%s has a gate: %+v", other.Key, other.Gate)
		}
	}
}

func TestWaitsForOwnerStillMeansTheOwnerGate(t *testing.T) {
	root := testutil.Repo(t, columnsWith("[[columns]]\nkey='held'\nfolder='1-open'\nwaits_for_owner=true\nnext='finish'\n"))
	column := loadProject(t, root).ByKey["held"]
	if column.Gate.Type != "owner" || !column.WaitsForOwner {
		t.Fatalf("held: %+v", column)
	}
}

func TestEachGateTypeLoadsWithItsOwnKeys(t *testing.T) {
	cases := map[string]struct {
		extra string
		want  Gate
	}{
		"owner":    {"gate='owner'\n", Gate{Type: "owner"}},
		"ci":       {"gate='ci'\n", Gate{Type: "ci"}},
		"ci named": {"gate='ci'\ngate_name='GitHub Actions'\n", Gate{Type: "ci", Name: "GitHub Actions"}},
		"external": {"gate='external'\ngate_name='train'\n", Gate{Type: "external", Name: "train"}},
		"date":     {"gate='date'\ngate_after='2030-01-31'\n", Gate{Type: "date", After: "2030-01-31"}},
		"card":     {"gate='card'\n", Gate{Type: "card"}},
		"card to":  {"gate='card'\ngate_until='ready'\n", Gate{Type: "card", Until: "ready"}},
	}
	for name, c := range cases {
		root := testutil.Repo(t, columnsWith("[[columns]]\nkey='held'\nfolder='1-open'\nnext='finish'\n"+c.extra))
		got := loadProject(t, root).ByKey["held"].Gate
		if got != c.want {
			t.Errorf("%s: got %+v want %+v", name, got, c.want)
		}
	}
}

func TestEachInvalidGateFailsWithOneMessage(t *testing.T) {
	cases := []struct{ name, extra, want string }{
		{"unknown type", "gate='bank'\n", "gate must be one of owner, ci, external, date, card, not 'bank'"},
		{"external without a name", "gate='external'\n", "an external gate needs gate_name"},
		{"date without a date", "gate='date'\n", "a date gate needs gate_after"},
		{"date that is not a date", "gate='date'\ngate_after='soon'\n", "gate_after must be a date as YYYY-MM-DD"},
		{"card to a missing column", "gate='card'\ngate_until='nowhere'\n", "gate_until names 'nowhere', which is not a column"},
		{"name on an owner gate", "gate='owner'\ngate_name='x'\n", "gate_name goes with a ci or external gate"},
		{"after on a ci gate", "gate='ci'\ngate_after='2030-01-01'\n", "gate_after goes with a date gate"},
		{"until on a ci gate", "gate='ci'\ngate_until='ready'\n", "gate_until goes with a card gate"},
		{"a parameter with no gate", "gate_name='x'\n", "gate_name needs a gate"},
		{"disagreeing alias", "gate='ci'\nwaits_for_owner=true\n", "waits_for_owner = true means gate = \"owner\""},
	}
	for _, c := range cases {
		root := testutil.Repo(t, columnsWith("[[columns]]\nkey='held'\nfolder='1-open'\nnext='finish'\n"+c.extra))
		err := loadError(t, root)
		if err == nil || !strings.Contains(err.Error(), c.want) || !strings.Contains(err.Error(), "column 'held'") {
			t.Errorf("%s: %v (want %q)", c.name, err, c.want)
		}
	}
}

func TestAGateNeedsAnOpenColumnThatIsNotDoing(t *testing.T) {
	for name, text := range map[string]string{
		"parked": "[[columns]]\nkey='held'\nfolder='x'\nkind='parked'\ngate='owner'\n",
		"doing":  "[[columns]]\nkey='held'\nfolder='1-open'\ndoing='finish'\ngate='owner'\n",
	} {
		err := loadError(t, testutil.Repo(t, columnsWith(text)))
		if err == nil || !strings.Contains(err.Error(), "column 'held'") || !strings.Contains(err.Error(), "gate") {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestSkipRulesComeAsAPairOnAStep(t *testing.T) {
	root := testutil.Repo(t, flowWith("", "skip_field='Type'\nskip_values=['docs','note']\n"))
	step := loadProject(t, root).Steps["finish"]
	if step.SkipField != "Type" || len(step.SkipValues) != 2 || step.SkipValues[1] != "note" {
		t.Fatalf("finish: %+v", step)
	}
	for name, extra := range map[string]string{
		"field only":  "skip_field='Type'\n",
		"values only": "skip_values=['docs']\n",
		"empty value": "skip_field='Type'\nskip_values=['']\n",
	} {
		err := loadError(t, testutil.Repo(t, flowWith("", extra)))
		if err == nil || !strings.Contains(err.Error(), "step 'finish'") || !strings.Contains(err.Error(), "skip_") {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestTheShippedDeployStepsSkipForDeployNone(t *testing.T) {
	steps := loadProject(t, testutil.Repo(t, "")).Steps
	for _, name := range []string{"deploy-qa", "ship"} {
		if steps[name].SkipField != "Deploy" || len(steps[name].SkipValues) != 1 || steps[name].SkipValues[0] != "None" {
			t.Errorf("%s: %+v", name, steps[name])
		}
	}
	for _, name := range []string{"design", "build", "check"} {
		if steps[name].SkipField != "" {
			t.Errorf("%s skips by default: %+v", name, steps[name])
		}
	}
}

func TestAProjectThatOverridesOnlyTheDeploySkillKeepsTheDeploySkip(t *testing.T) {
	root := testutil.Repo(t, "config_version=1\n[steps.deploy-qa]\nskill='my-deploy'\n")
	step := loadProject(t, root).Steps["deploy-qa"]
	if step.Skill != "my-deploy" || step.SkipField != "Deploy" {
		t.Fatalf("deploy-qa: %+v", step)
	}
}
