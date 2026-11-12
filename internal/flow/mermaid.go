package flow

import (
	"fmt"
	"strings"
)

// Mermaid is the flow as a stateDiagram-v2: the columns as states, the transitions as arrows labelled with why.
// It is output only; `validate` does not read it.
func Mermaid(m *Model) string {
	var b strings.Builder
	b.WriteString("stateDiagram-v2\n")
	id := func(key string) string {
		var out strings.Builder
		out.WriteString("c_")
		for _, r := range key {
			if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' {
				out.WriteRune(r)
			} else {
				out.WriteByte('_')
			}
		}
		return out.String()
	}
	for _, c := range m.Columns {
		label := strings.NewReplacer(`"`, "'", "\n", " ").Replace(c.Label)
		fmt.Fprintf(&b, "    state \"%s\" as %s\n", label, id(c.Key))
	}
	if len(m.Columns) > 0 {
		fmt.Fprintf(&b, "    [*] --> %s\n", id(m.Columns[0].Key))
	}
	for _, t := range m.Transitions {
		fmt.Fprintf(&b, "    %s --> %s: %s\n", id(t.From), id(t.To), t.Kind)
	}
	for _, c := range m.Columns {
		if c.Kind == "done" {
			fmt.Fprintf(&b, "    %s --> [*]\n", id(c.Key))
		}
	}
	return b.String()
}
