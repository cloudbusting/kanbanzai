package flow

import (
	"strings"
	"testing"

	"github.com/cloudbusting/kanbanzai/internal/config"
)

const presetRecord = "[preset]\nname = \"docs-only\"\ndelivery_style = \"docs\"\nversion = 1\n"

func TestSpliceWritesAPresetRecordOnlyWhenTheFlowFileCarriesOne(t *testing.T) {
	p := project(t, ownConfig+"\n[preset]\nname = \"old\"\nversion = 4\n")
	doc, err := Parse([]byte("[flow]\nwip_limit = 2\n"), false)
	if err != nil {
		t.Fatal(err)
	}
	plain, err := Splice(configText(t, p), doc)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(plain, `name = "old"`) {
		t.Fatalf("a plain flow file must leave the [preset] record alone:\n%s", plain)
	}
	doc.Record = presetRecord
	out, err := Splice(configText(t, p), doc)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, `name = "old"`) || !strings.Contains(out, `name = "docs-only"`) || strings.Count(out, "[preset]") != 1 {
		t.Fatalf("the record was not replaced exactly once:\n%s", out)
	}
	for _, kept := range []string{"# A project's own notes stay.", `owner = "Sam"            # the owner`} {
		if !strings.Contains(out, kept) {
			t.Errorf("lost %q:\n%s", kept, out)
		}
	}
	loaded, err := config.ValidateText(p.Root, p.Toplevel, p.GitCommon, out)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Preset.Name != "docs-only" || loaded.Preset.Version != 1 || loaded.WIPLimit != 2 {
		t.Fatalf("loaded %+v, wip %d", loaded.Preset, loaded.WIPLimit)
	}
}

func TestSpliceRefusesARecordThatIsNotJustThePresetTable(t *testing.T) {
	doc, _ := Parse([]byte("[flow]\nwip_limit = 2\n"), false)
	for name, record := range map[string]string{
		"another table": "[project]\nowner = \"x\"\n",
		"two tables":    presetRecord + "[claims]\nlease_minutes = 5\n",
	} {
		doc.Record = record
		if _, err := Splice("config_version = 1\n[project]\nowner = \"Sam\"\n", doc); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestAPresetRecordSurvivesAMultilineValueThatLooksLikeAHeader(t *testing.T) {
	old := "config_version = 1\n[project]\nowner = \"\"\"\n[preset]\nname = \"fake\"\n\"\"\"\n"
	doc, _ := Parse([]byte("[flow]\nwip_limit = 2\n"), false)
	doc.Record = presetRecord
	out, err := Splice(old, doc)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `name = "fake"`) {
		t.Fatalf("a header inside a string was treated as a table:\n%s", out)
	}
}
