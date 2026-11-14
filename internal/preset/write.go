package preset

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/cloudbusting/kanbanzai/internal/flow"
)

func quote(text string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range text {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\t':
			b.WriteString(`\t`)
		case '\r':
			b.WriteString(`\r`)
		default:
			if r < 0x20 || r == 0x7f {
				fmt.Fprintf(&b, `\u%04x`, r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}

func value(v any) string {
	switch typed := v.(type) {
	case string:
		return quote(typed)
	case int64:
		return strconv.FormatInt(typed, 10)
	case bool:
		return strconv.FormatBool(typed)
	case []any:
		items := make([]string, len(typed))
		for i, item := range typed {
			items[i] = value(item)
		}
		return "[" + strings.Join(items, ", ") + "]"
	case []string:
		items := make([]string, len(typed))
		for i, item := range typed {
			items[i] = quote(item)
		}
		return "[" + strings.Join(items, ", ") + "]"
	}
	return quote(fmt.Sprint(v))
}

// keys writes the keys of table that are set, in the given order, then any others in name order (so a key a later
// version adds is written, not lost).
func keys(b *strings.Builder, table map[string]any, order []string) {
	seen := map[string]bool{}
	for _, key := range order {
		if v, ok := table[key]; ok {
			fmt.Fprintf(b, "%s = %s\n", key, value(v))
			seen[key] = true
		}
	}
	var rest []string
	for key := range table {
		if !seen[key] {
			rest = append(rest, key)
		}
	}
	sort.Strings(rest)
	for _, key := range rest {
		fmt.Fprintf(b, "%s = %s\n", key, value(table[key]))
	}
}

// ConfigText is the config a project starts from when it takes this preset: the short settings init always writes,
// then everything the preset decides written out in full (the [preset] record, the [project] keys, the types, the
// brief schemas and the flow tables as `workflow export` writes them), so no resolved value is left to a default.
func (p *Preset) ConfigText(s Settings) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Kanbanzai config for this project, written by `kanbanzai init --preset %s` (%s).\n", p.Name, p.Summary)
	b.WriteString("# Every column, step, gate and path the preset chose is written out below: edit what you need, and\n")
	b.WriteString("# `kanbanzai workflow show` says whether the flow still matches the shipped preset.\n\n")
	b.WriteString("config_version = 1\n\n[project]\n")
	fmt.Fprintf(&b, "owner = %s      # the person the skills address and whose decisions they ask for\n", quote(s.Owner))
	if s.MainBranch != "" && s.MainBranch != "main" {
		fmt.Fprintf(&b, "main_branch = %s      # the branch the board and finished work live on\n", quote(s.MainBranch))
	}
	b.WriteString("prefix = \"\"      # no legacy card id prefix; typed ids are the default\n")
	if len(s.Context) > 0 {
		quoted := make([]string, len(s.Context))
		for i, item := range s.Context {
			quoted[i] = quote(item)
		}
		fmt.Fprintf(&b, "context = [%s]      # files every skill reads first\n", strings.Join(quoted, ", "))
	}
	keys(&b, p.Project, projectKeys)
	b.WriteString("\n" + p.Record())
	for _, item := range p.Types {
		b.WriteString("\n[[types]]\n")
		keys(&b, item, typeKeys)
	}
	names := make([]string, 0, len(p.Schemas))
	for name := range p.Schemas {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		fmt.Fprintf(&b, "\n[schemas.%s]\n", name)
		keys(&b, p.Schemas[name], schemaKeys)
	}
	fmt.Fprintf(&b, "\n# The flow of the %s preset.\n", p.Name)
	b.WriteString(leadingComments(p.Flow.Text))
	return b.String()
}

// Record is the [preset] table a project's config carries for this preset.
func (p *Preset) Record() string {
	return fmt.Sprintf("[preset]\nname = %s\ndelivery_style = %s\nversion = %d\n", quote(p.Name), quote(p.DeliveryStyle), p.Version)
}

// FlowDoc is the preset's flow file with its [preset] record, ready for `workflow plan`: splicing it into a config
// replaces the flow tables and the record, and nothing else.
func (p *Preset) FlowDoc() *flow.Doc {
	return &flow.Doc{Raw: p.Flow.Raw, Text: p.Flow.Text, Format: "toml", Record: p.Record()}
}
