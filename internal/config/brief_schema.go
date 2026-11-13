package config

import (
	"regexp"
	"strings"
)

// BriefSchema describes the opt-in structural rules for a type's brief.
type BriefSchema struct {
	RequiredFields, OptionalFields     []string
	RequiredSections, OptionalSections []string
	RepeatableSections, SectionOrder   []string
	FinalSection                       string
}

// BriefSectionOrder returns the selected schema's order, or nil for a legacy brief.
func (p *Project) BriefSectionOrder(id string) []string {
	item, ok := p.TypeOf(id)
	if !ok || item.Schema == "" {
		return nil
	}
	return p.BriefSchemas[item.Schema].SectionOrder
}

func (p *Project) loadBriefSchemas() error {
	raw, err := p.section("schemas")
	if err != nil {
		return err
	}
	p.BriefSchemas = make(map[string]BriefSchema, len(raw))
	for _, name := range sortedKeys(raw) {
		entry := raw[name].(map[string]any)
		s := BriefSchema{}
		lists := []struct {
			key  string
			dest *[]string
		}{
			{"required_fields", &s.RequiredFields}, {"optional_fields", &s.OptionalFields},
			{"required_sections", &s.RequiredSections}, {"optional_sections", &s.OptionalSections},
			{"repeatable_sections", &s.RepeatableSections}, {"section_order", &s.SectionOrder},
		}
		for _, list := range lists {
			value, ok := entry[list.key]
			if !ok {
				return errf("[schemas.%s]: missing %s", name, list.key)
			}
			*list.dest, err = stringList(value)
			if err != nil {
				return errf("[schemas.%s]: invalid %s", name, list.key)
			}
		}
		s.FinalSection = asString(entry["final_section"], "")
		if err := s.validate(name); err != nil {
			return err
		}
		p.BriefSchemas[name] = s
	}
	for _, item := range p.Types {
		if item.Schema != "" {
			if _, ok := p.BriefSchemas[item.Schema]; !ok {
				return errf("type %q names missing schema %q", item.Key, item.Schema)
			}
		}
	}
	return nil
}

func (s BriefSchema) validate(name string) error {
	fields := map[string]bool{}
	fieldRE := regexp.MustCompile(`^[A-Za-z]+(?: [A-Za-z]+)*$`)
	for _, field := range append(append([]string{}, s.RequiredFields...), s.OptionalFields...) {
		if !fieldRE.MatchString(field) || fields[field] {
			return errf("[schemas.%s]: invalid or duplicate field %q", name, field)
		}
		fields[field] = true
	}
	for _, field := range []string{"Stage", "Updated", "Next action", "Where"} {
		if !contains(s.RequiredFields, field) {
			return errf("[schemas.%s]: required_fields must include %s", name, field)
		}
	}
	for _, field := range []string{"Done", "Held", "Gate", "Needs"} {
		if !contains(s.OptionalFields, field) {
			return errf("[schemas.%s]: optional_fields must include %s for lifecycle operations", name, field)
		}
	}
	sections := map[string]bool{}
	for _, section := range append(append([]string{}, s.RequiredSections...), s.OptionalSections...) {
		if strings.TrimSpace(section) != section || section == "" || strings.ContainsAny(section, "\r\n") || sections[section] {
			return errf("[schemas.%s]: invalid or duplicate section %q", name, section)
		}
		sections[section] = true
	}
	for _, section := range []string{"Comments", "Progress", "Questions"} {
		if !sections[section] {
			return errf("[schemas.%s]: sections must permit %s", name, section)
		}
	}
	if !contains(s.RequiredSections, "Comments") || s.FinalSection != "Comments" || len(s.SectionOrder) == 0 || s.SectionOrder[len(s.SectionOrder)-1] != "Comments" {
		return errf("[schemas.%s]: final section must be required Comments and last", name)
	}
	seen := map[string]bool{}
	for _, section := range s.SectionOrder {
		if !sections[section] || seen[section] {
			return errf("[schemas.%s]: invalid or duplicate section_order entry %q", name, section)
		}
		seen[section] = true
	}
	for section := range sections {
		if !seen[section] {
			return errf("[schemas.%s]: section_order missing %s", name, section)
		}
	}
	repeats := map[string]bool{}
	for _, section := range s.RepeatableSections {
		if !sections[section] || repeats[section] || section == "Comments" || section == "Progress" || section == "Questions" {
			return errf("[schemas.%s]: invalid repeatable section %q", name, section)
		}
		repeats[section] = true
	}
	if !contains(s.SectionOrder, "Progress") || !contains(s.SectionOrder, "Questions") {
		return errf("[schemas.%s]: missing lifecycle sections", name)
	}
	return nil
}
