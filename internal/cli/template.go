package cli

import (
	"fmt"
	"io"
	"strings"

	kanbanzai "github.com/cloudbusting/kanbanzai"
	"github.com/cloudbusting/kanbanzai/internal/config"
	"github.com/cloudbusting/kanbanzai/internal/typeid"
)

// examples are the embedded, opt-in per-type templates and the schema example. None is applied to a project.
var examples = map[string]string{
	"task": "templates/task.md", "bug": "templates/bug.md", "change": "templates/change.md",
	"schemas": "templates/schemas.example.toml",
}

// runTemplate prints the brief template for a type, a card id or the project, or one embedded example. The ID
// prefix is the only source of type: there is no Type line.
func runTemplate(project *config.Project, args []string, out, errOut io.Writer) int {
	positionals, flags, set, err := parse(args, map[string]bool{"where": true}, map[string]bool{"example": true})
	if err != nil || len(positionals) > 1 {
		return usageError(errOut, "template", err)
	}
	if name, ok := flags["example"]; ok {
		file, known := examples[name]
		if !known || len(positionals) != 0 || set["where"] {
			fmt.Fprintf(errOut, "kanbanzai: template: --example takes one of task, bug, change, schemas, and nothing else\n")
			return 2
		}
		data, err := kanbanzai.Files.ReadFile(file)
		if err != nil {
			fmt.Fprintf(errOut, "kanbanzai: template: %v\n", err)
			return 1
		}
		_, _ = out.Write(data)
		return 0
	}
	var item typeid.Type
	if len(positionals) == 1 {
		found, ok := templateType(project, positionals[0])
		if !ok {
			fmt.Fprintf(errOut, "kanbanzai: template: %q is not a type key, a type prefix or a card id of this project (types: %s)\n",
				positionals[0], strings.Join(typeKeys(project), ", "))
			return 2
		}
		item = found
	}
	text, source, err := project.TemplateTextFor(item)
	if err != nil {
		fmt.Fprintf(errOut, "kanbanzai: template: %v\n", err)
		return 1
	}
	if set["where"] {
		fmt.Fprintf(out, "template: %s\n", source)
		if item.Schema != "" {
			fmt.Fprintf(out, "schema: %s\n", item.Schema)
		} else {
			fmt.Fprintln(out, "schema: none (the project's current brief rules)")
		}
		return 0
	}
	_, _ = io.WriteString(out, text)
	return 0
}

// templateType finds a type by key, by prefix, or from a card id. A legacy id has no type, so it gets the
// project's own template (the zero type).
func templateType(project *config.Project, value string) (typeid.Type, bool) {
	for _, item := range project.Types {
		if strings.EqualFold(item.Key, value) || strings.EqualFold(item.Prefix, value) {
			return item, true
		}
	}
	if item, ok := project.TypeOf(value); ok {
		return item, true
	}
	// the template's own placeholder, BUG-NN or KANZ-BUG-NN, names the type by its prefix
	if upper := strings.ToUpper(value); strings.HasSuffix(upper, "-NN") {
		stem := strings.TrimSuffix(upper, "-NN")
		stem = stem[strings.LastIndex(stem, "-")+1:]
		for _, item := range project.Types {
			if strings.EqualFold(item.Prefix, stem) {
				return item, true
			}
		}
		if project.Prefix != "" && strings.EqualFold(project.Prefix, stem) {
			return typeid.Type{}, true
		}
	}
	if project.CardRe.MatchString(value) {
		return typeid.Type{}, true
	}
	return typeid.Type{}, false
}

func typeKeys(project *config.Project) []string {
	keys := make([]string, 0, len(project.Types))
	for _, item := range project.Types {
		keys = append(keys, item.Key)
	}
	return keys
}
