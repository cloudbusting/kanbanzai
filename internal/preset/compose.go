package preset

import (
	"regexp"
	"strings"

	"github.com/cloudbusting/kanbanzai/internal/flow"
)

// Compose is what a project adds to a preset without editing the generated file. Each entry is one setting:
//
//	Types        KEY=PREFIX[:Label]       add a type, or replace the preset's type of that key
//	StepSkills   STEP=SKILL               the skill a step runs
//	StepExtends  STEP=SKILL[+SKILL...]    the skills a step also follows
type Compose struct {
	Types       []string
	StepSkills  []string
	StepExtends []string
}

var (
	typeKeyRE = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
	prefixRE  = regexp.MustCompile(`^[A-Z][A-Z0-9]*$`)
)

func pair(entry, what string) (key, rest string, err error) {
	key, rest, ok := strings.Cut(entry, "=")
	if !ok || strings.TrimSpace(key) == "" || strings.TrimSpace(rest) == "" {
		return "", "", problem("%s %q: use NAME=VALUE", what, entry)
	}
	return strings.TrimSpace(key), strings.TrimSpace(rest), nil
}

// Compose applies the additions to the preset in memory and validates it again as a whole.
func (p *Preset) Compose(c Compose) error {
	seen := map[string]bool{}
	for _, entry := range c.Types {
		key, rest, err := pair(entry, "--type")
		if err != nil {
			return err
		}
		prefix, label, _ := strings.Cut(rest, ":")
		label = strings.TrimSpace(label)
		if !typeKeyRE.MatchString(key) {
			return problem("--type %q: the key must be lower-case letters, digits and underscores, starting with a letter", entry)
		}
		if !prefixRE.MatchString(prefix) {
			return problem("--type %q: the prefix must be upper-case letters and digits, starting with a letter", entry)
		}
		if seen[key] {
			return problem("--type names %s twice", key)
		}
		seen[key] = true
		if label == "" {
			label = strings.ToUpper(key[:1]) + key[1:]
		}
		item := map[string]any{"key": key, "label": label, "prefix": prefix}
		replaced := false
		for i, old := range p.Types {
			if old["key"] == key {
				p.Types[i], replaced = item, true
			}
		}
		if !replaced {
			p.Types = append(p.Types, item)
		}
	}
	steps, _ := p.Flow.Raw["steps"].(map[string]any)
	edit := func(entries []string, what, field string, list bool) error {
		for _, entry := range entries {
			name, rest, err := pair(entry, what)
			if err != nil {
				return err
			}
			step, ok := steps[name].(map[string]any)
			if !ok {
				return problem("%s %q: the preset has no step %q", what, entry, name)
			}
			if list {
				var skills []any
				for _, skill := range strings.Split(rest, "+") {
					skills = append(skills, strings.TrimSpace(skill))
				}
				step[field] = skills
			} else {
				step[field] = rest
			}
		}
		return nil
	}
	if err := edit(c.StepSkills, "--step-skill", "skill", false); err != nil {
		return err
	}
	if err := edit(c.StepExtends, "--step-extend", "extend", true); err != nil {
		return err
	}
	if len(c.StepSkills)+len(c.StepExtends) > 0 {
		p.Flow = &flow.Doc{Raw: p.Flow.Raw, Text: flow.ToTOML(p.Flow.Raw), Format: "toml"}
	}
	return p.validate()
}
