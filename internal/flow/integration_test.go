package flow

import (
	"reflect"
	"strings"
	"testing"
)

const remoteConfig = "config_version = 1\n[flow]\nintegration = \"remote\"\n"

func TestShowReportsTheIntegrationMode(t *testing.T) {
	if got := Build(project(t, "")).Isolation.Integration; got != "local" {
		t.Fatalf("default integration = %q, want local", got)
	}
	if got := Build(project(t, remoteConfig)).Isolation.Integration; got != "remote" {
		t.Fatalf("integration = %q, want remote", got)
	}
}

// The export must keep a remote project remote, or applying it would put the project back to local merges (the bug
// isolation once had), and must not change a single byte for a project that never set the key.
func TestExportCarriesIntegrationOnlyWhenRemote(t *testing.T) {
	remote := FromProject(project(t, remoteConfig))
	if !strings.Contains(remote.Text, `integration = "remote"`) {
		t.Fatalf("the export dropped integration:\n%s", remote.Text)
	}
	if local := FromProject(project(t, "")); strings.Contains(local.Text, "integration") {
		t.Fatalf("a local project's export mentions integration, which changes every shipped preset's flow file:\n%s", local.Text)
	}
	again, err := Parse([]byte(remote.Text), false)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(again.Raw, remote.Raw) {
		t.Fatalf("the remote export is not stable:\n%s", remote.Text)
	}
}

func TestValidateRefusesABadIntegrationAndWarnsWhenItCannotApply(t *testing.T) {
	base := project(t, "")
	bad, err := Parse([]byte(strings.Replace(FromProject(base).Text, `isolation = "worktree"`,
		"isolation = \"worktree\"\nintegration = \"sideways\"", 1)), false)
	if err != nil {
		t.Fatal(err)
	}
	report := Validate(base, bad, "")
	if report.Errors() == 0 {
		t.Fatalf("integration = sideways passed validation: %+v", report)
	}
	for _, shared := range []string{"shared", "external"} {
		text := "config_version = 1\n[flow]\nisolation = \"" + shared + "\"\nintegration = \"remote\"\n"
		doc := FromProject(project(t, text))
		report := Validate(project(t, text), doc, "")
		found := false
		for _, f := range report.Findings {
			if f.Severity == Warning && strings.Contains(f.Message, "integration") && strings.Contains(f.Message, shared) {
				found = true
			}
		}
		if !found {
			t.Fatalf("isolation %s with integration remote gave no warning: %+v", shared, report.Findings)
		}
	}
}
