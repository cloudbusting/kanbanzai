package config

import (
	"reflect"
	"testing"

	"github.com/cloudbusting/kanbanzai/internal/testutil"
)

const routesConfig = `
[project]
owner = "Sam"

[routes.do]
extend = ["our-bridge"]
context = ["docs/delivery.md"]

[routes.cleanup]
extend = ["our-cleanup"]
`

func TestRoutesLoadAndAreInTheConfigJSON(t *testing.T) {
	project := loadProject(t, testutil.Repo(t, routesConfig))
	do := project.Routes["do"]
	if !reflect.DeepEqual(do.Extend, []string{"our-bridge"}) || !reflect.DeepEqual(do.Context, []string{"docs/delivery.md"}) {
		t.Fatalf("do = %+v", do)
	}
	if got := project.Routes["cleanup"]; !reflect.DeepEqual(got.Extend, []string{"our-cleanup"}) || len(got.Context) != 0 {
		t.Fatalf("cleanup = %+v", got)
	}
	if _, has := project.Routes["triage"]; has {
		t.Fatal("a route nobody configured exists")
	}
	routes, ok := project.AsDict()["routes"].(map[string]any)
	if !ok || routes["do"] == nil || routes["cleanup"] == nil || routes["triage"] != nil {
		t.Fatalf("config JSON routes = %#v", project.AsDict()["routes"])
	}
	if bare := loadProject(t, testutil.Repo(t, defaultConfig)); len(bare.Routes) != 0 {
		t.Fatalf("a project with no [routes] has %v", bare.Routes)
	}
}

func TestRoutesAreChecked(t *testing.T) {
	cases := []struct{ name, text, want string }{
		{"a step is not a route", "[routes.build]\nextend = [\"x\"]\n", "do, triage, cleanup"},
		{"an unknown route", "[routes.foo]\nextend = [\"x\"]\n", "do, triage, cleanup"},
		{"extend is a list", "[routes.do]\nextend = \"x\"\n", "routes.do.extend"},
		{"context is a list", "[routes.do]\ncontext = 3\n", "routes.do.context"},
		{"a route has no skill of its own", "[routes.do]\nskill = \"x\"\n", "skill"},
		{"a route is a table", "routes = 3\n", "routes"},
		{"a step and a route share a name", "[routes.do]\nextend = [\"x\"]\n[steps.do]\ninto = \"done\"\n", "step"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := loadError(t, testutil.Repo(t, "[project]\nowner = \"Sam\"\n"+tc.text))
			if err == nil {
				t.Fatal("the config loaded without an error")
			}
			requireContains(t, err.Error(), tc.want)
		})
	}
}
