package flow

import (
	"reflect"
	"regexp"
	"strings"

	"github.com/cloudbusting/kanbanzai/internal/config"
)

var headerRE = regexp.MustCompile(`^\s*(\[\[?)\s*([^\]]+?)\s*\]\]?\s*(#.*)?$`)

// firstSegment is the table's first dotted name segment, unquoted.
func firstSegment(name string) string {
	name = strings.TrimSpace(name)
	if strings.HasPrefix(name, `"`) || strings.HasPrefix(name, "'") {
		quote := name[:1]
		if end := strings.Index(name[1:], quote); end >= 0 {
			return name[1 : 1+end]
		}
	}
	if dot := strings.Index(name, "."); dot >= 0 {
		name = name[:dot]
	}
	return strings.TrimSpace(name)
}

// owns says whether the flow file replaces the config table of that name: the three flow tables, and [preset] when
// the file carries a record for it.
func (d *Doc) owns(name string) bool {
	for _, t := range tables {
		if name == t {
			return true
		}
	}
	return name == "preset" && d.Record != ""
}

// Splice replaces the flow tables of a config text with the flow file's text. Everything else in the config —
// its other tables, keys and comments — stays as it was. It re-reads both results and refuses unless the other
// tables are unchanged and the flow tables are exactly the file's, so it can fail but never guess.
func Splice(configText string, doc *Doc) (string, error) {
	configText = strings.ReplaceAll(configText, "\r\n", "\n")
	lines := strings.Split(configText, "\n")
	keep := make([]bool, len(lines))
	inFlow := false
	inMulti := ""
	for i, line := range lines {
		keep[i] = true
		if inMulti == "" {
			if match := headerRE.FindStringSubmatch(line); match != nil && !strings.Contains(match[2], "=") {
				inFlow = doc.owns(firstSegment(match[2]))
				if inFlow {
					// a comment block directly above the header describes it
					for j := i - 1; j >= 0 && strings.HasPrefix(strings.TrimSpace(lines[j]), "#"); j-- {
						keep[j] = false
					}
				}
			}
		}
		if inFlow {
			keep[i] = false
		}
		inMulti = multiline(line, inMulti)
	}
	var kept []string
	for i, line := range lines {
		if keep[i] {
			kept = append(kept, line)
		}
	}
	rest := strings.TrimRight(strings.Join(kept, "\n"), "\n")
	out := doc.Text
	if doc.Record != "" {
		out = strings.TrimRight(out, "\n") + "\n\n" + strings.TrimRight(doc.Record, "\n") + "\n"
	}
	if rest != "" {
		out = rest + "\n\n" + out
	}
	if err := verifySplice(configText, out, doc); err != nil {
		return "", err
	}
	return out, nil
}

// multiline tracks whether a line leaves a multi-line string open, so a '[' inside one is not read as a header.
func multiline(line, open string) string {
	for _, quote := range []string{`"""`, `'''`} {
		if open != "" && open != quote {
			continue
		}
		count := strings.Count(line, quote)
		if open == quote {
			if count%2 == 1 {
				open = ""
			}
		} else if open == "" && count%2 == 1 {
			open = quote
		}
	}
	return open
}

func verifySplice(before, after string, doc *Doc) error {
	oldRaw, err := config.ParseTOML(before)
	if err != nil {
		return problem("the config cannot be read: %v", err)
	}
	newRaw, err := config.ParseTOML(after)
	if err != nil {
		return problem("the flow cannot be spliced into the config (%v): move any root-level flow.* keys under a "+
			"[flow] table and try again", err)
	}
	for key, value := range oldRaw {
		if doc.owns(key) {
			continue
		}
		if !reflect.DeepEqual(value, newRaw[key]) {
			return problem("splicing the flow would change '%s' in the config: edit the config by hand", key)
		}
	}
	for key := range newRaw {
		if _, had := oldRaw[key]; !had && !doc.owns(key) {
			return problem("splicing the flow would add '%s' to the config: edit the config by hand", key)
		}
	}
	for _, key := range tables {
		if !reflect.DeepEqual(newRaw[key], doc.Raw[key]) {
			return problem("the spliced config does not carry the flow file's [%s] exactly: edit the config by hand", key)
		}
	}
	if doc.Record != "" {
		record, err := config.ParseTOML(doc.Record)
		if err != nil || len(record) != 1 || record["preset"] == nil {
			return problem("a preset record is one [preset] table and nothing else")
		}
		if !reflect.DeepEqual(newRaw["preset"], record["preset"]) {
			return problem("the spliced config does not carry the preset record exactly: edit the config by hand")
		}
	}
	return nil
}

// Resolved is the project the config would be with a flow file applied.
type Resolved struct {
	Project    *config.Project
	OldText    string
	ConfigText string
}

// Resolve applies a flow file to the project's config text, in memory, and loads the result with the config's own
// loader. It writes nothing.
func Resolve(p *config.Project, doc *Doc, oldText string) (*Resolved, error) {
	if oldText == "" {
		oldText = "config_version = 1\n"
	}
	text, err := Splice(oldText, doc)
	if err != nil {
		return nil, err
	}
	project, err := config.ValidateText(p.Root, p.Toplevel, p.GitCommon, text)
	if err != nil {
		return nil, err
	}
	return &Resolved{Project: project, OldText: oldText, ConfigText: text}, nil
}
