package check

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cloudbusting/kanbanzai/internal/board"
	"github.com/cloudbusting/kanbanzai/internal/brief"
	"github.com/cloudbusting/kanbanzai/internal/config"
)

// BriefSchemas applies opt-in structural rules using the type derived from the filename.
func BriefSchemas(project *config.Project) []string {
	var problems []string
	for _, path := range board.Briefs(project) {
		match := project.CardFileRe.FindStringSubmatch(filepath.Base(path))
		if match == nil {
			continue
		}
		item, ok := project.TypeOf(match[1])
		if !ok || item.Schema == "" {
			continue
		}
		schema, ok := project.BriefSchemas[item.Schema]
		if !ok {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			problems = append(problems, fmt.Sprintf("%s: cannot read brief: %v", project.Rel(path), err))
			continue
		}
		problems = append(problems, checkBriefSchema(project.Rel(path), string(data), schema, false)...)
	}
	return problems
}

func checkBriefSchema(name, text string, schema config.BriefSchema, template bool) []string {
	var problems []string
	fields := map[string]int{}
	sections := map[string]int{}
	order := map[string]int{}
	for i, section := range schema.SectionOrder {
		order[section] = i
	}
	last, inHead := -1, true
	for _, raw := range strings.Split(text, "\n") {
		line := strings.TrimSuffix(raw, "\r")
		if strings.HasPrefix(line, "## ") {
			inHead = false
			section := strings.TrimPrefix(line, "## ")
			sections[section]++
			index, ok := order[section]
			if !ok {
				problems = append(problems, fmt.Sprintf("%s: undeclared section %q", name, section))
				continue
			}
			if sections[section] > 1 && !contains(schema.RepeatableSections, section) {
				problems = append(problems, fmt.Sprintf("%s: duplicate section %q", name, section))
			}
			if index < last {
				problems = append(problems, fmt.Sprintf("%s: section %q out of order", name, section))
			}
			last = index
		} else if inHead {
			if m := brief.FieldRE.FindStringSubmatch(line); m != nil {
				field := strings.TrimSpace(m[1])
				fields[field]++
				if !contains(schema.RequiredFields, field) && !contains(schema.OptionalFields, field) {
					problems = append(problems, fmt.Sprintf("%s: undeclared field %q", name, field))
				}
				if fields[field] > 1 {
					problems = append(problems, fmt.Sprintf("%s: duplicate field %q", name, field))
				}
			}
		}
	}
	_, values := brief.Header(text)
	for _, field := range schema.RequiredFields {
		if fields[field] == 0 || (!template && strings.TrimSpace(values[field]) == "") {
			problems = append(problems, fmt.Sprintf("%s: missing required field %q", name, field))
		}
	}
	for _, section := range schema.RequiredSections {
		if sections[section] == 0 {
			problems = append(problems, fmt.Sprintf("%s: missing required section %q", name, section))
		}
	}
	return problems
}
