package preset

import (
	"flag"
	"os"
	"path/filepath"
	"testing"
)

var update = flag.Bool("update", false, "rewrite the golden configs of the shipped presets")

// fixtureSettings are the settings the golden configs and the CLI matrix use.
var fixtureSettings = Settings{Owner: "Sam", MainBranch: "main"}

func TestEveryShippedPresetHasAFixtureRepositoryAndAGoldenConfig(t *testing.T) {
	names, err := List()
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 7 {
		t.Fatalf("the first set of presets is seven, found %v", names)
	}
	for _, name := range names {
		dir := filepath.Join("testdata", name)
		p, err := Load(name)
		if err != nil {
			t.Fatal(err)
		}
		golden := filepath.Join(dir, "config.golden.toml")
		want := p.ConfigText(fixtureSettings)
		if *update {
			if err := os.WriteFile(golden, []byte(want), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		got, err := os.ReadFile(golden)
		if err != nil {
			t.Errorf("%s has no golden config: %v (go test ./internal/preset -update writes it)", name, err)
			continue
		}
		if string(got) != want {
			t.Errorf("%s: the golden config differs from what the preset writes; review the change, then go test ./internal/preset -update", name)
		}
		for _, folder := range []string{"1-open", "10-done"} {
			cards, _ := filepath.Glob(filepath.Join(dir, "board", folder, "*.md"))
			if len(cards) == 0 {
				t.Errorf("%s: the fixture board has no card in %s", name, folder)
			}
		}
	}
}
