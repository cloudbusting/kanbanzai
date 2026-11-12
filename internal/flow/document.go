package flow

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/cloudbusting/kanbanzai/internal/config"
)

// maxFlowBytes bounds a flow file; a real one is a few kilobytes.
const maxFlowBytes = 1 << 20

// DocVersion is the schema_version of a JSON flow file and of `workflow export --format json`.
const DocVersion = 1

// Tables are the only top-level tables of a flow file.
var tables = []string{"flow", "columns", "steps"}

// Doc is a flow file: the three flow tables, as the config parser reads them, and the TOML text that carries them.
// A TOML file keeps its own text (comments and all); a JSON file is written out as TOML.
type Doc struct {
	Raw    map[string]any
	Text   string
	Format string // "toml" or "json"
}

// Problem is a refusal to read a file as a flow file.
type Problem struct{ Message string }

func (p *Problem) Error() string { return p.Message }

func problem(format string, args ...any) error {
	return &Problem{Message: fmt.Sprintf(format, args...)}
}

// ReadFile reads a flow file: TOML, or JSON when the name ends in .json or the text starts with '{'.
func ReadFile(path string) (*Doc, error) {
	handle, err := os.Open(path)
	if err != nil {
		return nil, problem("--file: %v", err)
	}
	defer handle.Close()
	data, err := io.ReadAll(io.LimitReader(handle, maxFlowBytes+1))
	if err != nil {
		return nil, problem("--file: %v", err)
	}
	if len(data) > maxFlowBytes {
		return nil, problem("--file: %s is larger than %d bytes", path, maxFlowBytes)
	}
	return Parse(data, strings.HasSuffix(strings.ToLower(path), ".json"))
}

// Parse reads a flow file's bytes.
func Parse(data []byte, asJSON bool) (*Doc, error) {
	if !utf8Valid(data) {
		return nil, problem("flow file: not valid UTF-8")
	}
	trimmed := bytes.TrimSpace(data)
	if asJSON || bytes.HasPrefix(trimmed, []byte("{")) {
		return parseJSON(data)
	}
	raw, err := config.ParseTOML(string(data))
	if err != nil {
		return nil, problem("flow file: %v", err)
	}
	if err := checkShape(raw); err != nil {
		return nil, err
	}
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	if !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	return &Doc{Raw: raw, Text: text, Format: "toml"}, nil
}

func utf8Valid(data []byte) bool { return strings.ToValidUTF8(string(data), "\x00") == string(data) }

func parseJSON(data []byte) (*Doc, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var top map[string]any
	if err := decoder.Decode(&top); err != nil {
		return nil, problem("flow file: not a JSON object: %v", err)
	}
	if decoder.More() {
		return nil, problem("flow file: trailing data after the JSON object")
	}
	if v, ok := top["schema_version"]; ok {
		number, isNumber := v.(json.Number)
		if !isNumber || number.String() != strconv.Itoa(DocVersion) {
			return nil, problem("flow file: schema_version must be %d", DocVersion)
		}
		delete(top, "schema_version")
	}
	converted, err := convert(top)
	if err != nil {
		return nil, err
	}
	raw := converted.(map[string]any)
	if err := checkShape(raw); err != nil {
		return nil, err
	}
	if flow, ok := raw["flow"].(map[string]any); ok {
		if n, isInt := flow["move_grace_minutes"].(int64); isInt {
			flow["move_grace_minutes"] = float64(n)
		}
	}
	// The TOML text is the document; read it back so the raw tables are exactly what the config parser makes of it.
	text := ToTOML(raw)
	canonical, err := config.ParseTOML(text)
	if err != nil {
		return nil, problem("flow file: %v", err)
	}
	return &Doc{Raw: canonical, Text: text, Format: "json"}, nil
}

// convert turns decoded JSON into what the TOML parser produces: int64, float64, string, bool, []any, map.
func convert(value any) (any, error) {
	switch typed := value.(type) {
	case json.Number:
		if n, err := typed.Int64(); err == nil {
			return n, nil
		}
		f, err := typed.Float64()
		if err != nil {
			return nil, problem("flow file: bad number %s", typed)
		}
		return f, nil
	case map[string]any:
		out := make(map[string]any, len(typed))
		for k, v := range typed {
			c, err := convert(v)
			if err != nil {
				return nil, err
			}
			out[k] = c
		}
		return out, nil
	case []any:
		var out []any // nil when empty, as the TOML parser reads an empty list
		for _, v := range typed {
			c, err := convert(v)
			if err != nil {
				return nil, err
			}
			out = append(out, c)
		}
		return out, nil
	case string, bool:
		return typed, nil
	case nil:
		return nil, problem("flow file: null is not a value (omit the key)")
	}
	return nil, problem("flow file: unsupported value %v", value)
}

// checkShape allows only the three flow tables at the top, with the right kind of value in each.
func checkShape(raw map[string]any) error {
	for _, key := range sortedKeys(raw) {
		switch key {
		case "flow":
			if _, ok := raw[key].(map[string]any); !ok {
				return problem("flow file: [flow] must be a table")
			}
		case "columns":
			items, ok := raw[key].([]any)
			if !ok {
				return problem("flow file: columns must be a list of [[columns]] tables")
			}
			for _, item := range items {
				if _, ok := item.(map[string]any); !ok {
					return problem("flow file: every [[columns]] entry must be a table")
				}
			}
		case "steps":
			steps, ok := raw[key].(map[string]any)
			if !ok {
				return problem("flow file: steps must be [steps.<name>] tables")
			}
			for _, name := range sortedKeys(steps) {
				if _, ok := steps[name].(map[string]any); !ok {
					return problem("flow file: [steps.%s] must be a table", name)
				}
			}
		default:
			return problem("flow file: '%s' is not part of the flow: a flow file holds only [flow], [[columns]] and "+
				"[steps.<name>] (the rest of the config is not this command's to change)", key)
		}
	}
	return nil
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// FromProject is the flow file that says exactly what the project's flow is now: every column and step key is
// written, so applying it to the project it came from changes nothing.
func FromProject(p *config.Project) *Doc {
	flow := map[string]any{"wip_limit": int64(p.WIPLimit), "stale_days": int64(p.StaleDays),
		"move_grace_minutes": p.MoveGraceMinutes, "pull_order": strs(p.PullOrder)}
	columns := []any{}
	for _, c := range p.Columns {
		entry := map[string]any{"key": c.Key, "label": c.Label, "words": strs(c.Words), "folder": c.Folder,
			"kind": c.Kind}
		if c.CountsWithClaim() {
			entry["started"] = "claimed"
		} else {
			entry["started"] = c.StartedBool()
		}
		for key, value := range map[string]string{"next": c.Next, "settled_next": c.SettledNext, "title": c.Title,
			"settled_title": c.SettledTitle, "lane": c.Lane, "doing": c.Doing} {
			if value != "" {
				entry[key] = value
			}
		}
		if c.Worktree {
			entry["worktree"] = true
		}
		if c.WaitsForOwner {
			entry["waits_for_owner"] = true
		}
		columns = append(columns, entry)
	}
	steps := map[string]any{}
	for _, name := range p.StepOrder {
		s := p.Steps[name]
		steps[name] = map[string]any{"into": s.Into, "during": s.During, "skill": s.Skill, "extend": strs(s.Extend),
			"context": strs(s.Context), "checks": strs(s.Checks)}
	}
	raw := map[string]any{"flow": flow, "columns": columns, "steps": steps}
	return &Doc{Raw: raw, Text: ToTOML(raw), Format: "toml"}
}

func strs(values []string) []any {
	var out []any // nil when empty, as the TOML parser reads an empty list
	for _, v := range values {
		out = append(out, v)
	}
	return out
}

// JSON is the document as `workflow export --format json` prints it.
func (d *Doc) JSON() ([]byte, error) {
	top := map[string]any{"schema_version": DocVersion}
	for k, v := range d.Raw {
		top[k] = jsonValue(v)
	}
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(top); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// jsonValue makes an empty list [] rather than null.
func jsonValue(v any) any {
	switch typed := v.(type) {
	case []any:
		out := make([]any, 0, len(typed))
		for _, item := range typed {
			out = append(out, jsonValue(item))
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(typed))
		for k, item := range typed {
			out[k] = jsonValue(item)
		}
		return out
	}
	return v
}

var (
	flowKeys   = []string{"wip_limit", "stale_days", "move_grace_minutes", "pull_order"}
	columnKeys = []string{"key", "label", "words", "folder", "kind", "started", "next", "settled_next", "title",
		"settled_title", "worktree", "waits_for_owner", "lane", "doing"}
	stepKeys = []string{"into", "during", "skill", "extend", "context", "checks"}
)

// ToTOML writes the three tables in a fixed order, with their keys in a fixed order, so equal flows give equal text.
func ToTOML(raw map[string]any) string {
	var b strings.Builder
	if flow, ok := raw["flow"].(map[string]any); ok {
		b.WriteString("[flow]\n")
		writeKeys(&b, flow, flowKeys)
		b.WriteString("\n")
	}
	if columns, ok := raw["columns"].([]any); ok {
		for _, item := range columns {
			b.WriteString("[[columns]]\n")
			writeKeys(&b, item.(map[string]any), columnKeys)
			b.WriteString("\n")
		}
	}
	if steps, ok := raw["steps"].(map[string]any); ok {
		for _, name := range sortedKeys(steps) {
			b.WriteString("[steps." + tomlKey(name) + "]\n")
			writeKeys(&b, steps[name].(map[string]any), stepKeys)
			b.WriteString("\n")
		}
	}
	return strings.TrimRight(b.String(), "\n") + "\n"
}

// writeKeys writes the known keys in order, then any others sorted (a flow file the schema will reject still
// round-trips as text).
func writeKeys(b *strings.Builder, table map[string]any, order []string) {
	done := map[string]bool{}
	write := func(key string) {
		if value, ok := table[key]; ok && !done[key] {
			done[key] = true
			b.WriteString(tomlKey(key) + " = " + tomlValue(value) + "\n")
		}
	}
	for _, key := range order {
		write(key)
	}
	for _, key := range sortedKeys(table) {
		write(key)
	}
}

func tomlKey(key string) string {
	if key != "" && strings.Trim(key, "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789_-") == "" {
		return key
	}
	return tomlString(key)
}

func tomlValue(value any) string {
	switch typed := value.(type) {
	case string:
		return tomlString(typed)
	case bool:
		return strconv.FormatBool(typed)
	case int64:
		return strconv.FormatInt(typed, 10)
	case float64:
		text := strconv.FormatFloat(typed, 'g', -1, 64)
		if !strings.ContainsAny(text, ".eEnN") {
			text += ".0"
		}
		return text
	case []any:
		parts := make([]string, 0, len(typed))
		for _, item := range typed {
			parts = append(parts, tomlValue(item))
		}
		return "[" + strings.Join(parts, ", ") + "]"
	}
	return tomlString(fmt.Sprint(value))
}

func tomlString(text string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range text {
		switch {
		case r == '"':
			b.WriteString(`\"`)
		case r == '\\':
			b.WriteString(`\\`)
		case r == '\n':
			b.WriteString(`\n`)
		case r == '\t':
			b.WriteString(`\t`)
		case r == '\r':
			b.WriteString(`\r`)
		case r < 0x20 || r == 0x7f:
			fmt.Fprintf(&b, `\u%04X`, r)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}
