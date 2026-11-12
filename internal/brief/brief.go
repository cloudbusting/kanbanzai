// Package brief reads and edits a card brief's head: the title line, the `- **Field:** value` lines before the
// first `## `, the Stage word, a Held line and its mark.
package brief

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/cloudbusting/kanbanzai/internal/config"
)

// FieldRE matches one head field. A value may wrap onto lines indented by two spaces.
var FieldRE = regexp.MustCompile(`^- \*\*([A-Za-z ]+):\*\*\s*(.*)$`)

// Order is where a missing head field is inserted: after the nearest field before it.
var Order = []string{"Stage", "Updated", "Done", "ADR", "Next action", "Where", "Held", "Gate", "Needs"}

var dateRE = regexp.MustCompile(`\d{4}-\d{2}-\d{2}`)

// Header is a brief's title line and its head fields, before the first `## `.
func Header(text string) (string, map[string]string) {
	lines := strings.Split(text, "\n")
	title := ""
	if len(lines) > 0 {
		title = lines[0]
	}
	fields := map[string]string{}
	last := ""
	for _, line := range lines[1:] {
		if strings.HasPrefix(line, "## ") {
			break
		}
		if found := FieldRE.FindStringSubmatch(line); found != nil {
			name := strings.TrimSpace(found[1])
			if _, exists := fields[name]; exists {
				last = ""
				continue
			}
			last = name
			fields[name] = strings.TrimSpace(found[2])
			continue
		}
		if last != "" && strings.HasPrefix(line, "  ") && strings.TrimSpace(line) != "" {
			fields[last] = strings.TrimSpace(fields[last] + " " + strings.TrimSpace(line))
			continue
		}
		last = ""
	}
	return title, fields
}

// StageWord is the first word of a Stage line, lower case: "waiting: Sam" gives "waiting".
func StageWord(stage string) string {
	trimmed := strings.TrimSpace(stage)
	if index := strings.IndexAny(trimmed, " \t\n\r:—,("); index >= 0 {
		return strings.ToLower(strings.TrimSpace(trimmed[:index]))
	}
	return strings.ToLower(trimmed)
}

// HoldWords are the words a Held line may start with.
var HoldWords = []string{"blocked", "waiting"}

// HeldOf is a brief's Held text, or nil when it is not held (no line, an em dash or a hyphen).
func HeldOf(value string) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || trimmed == "—" || trimmed == "-" || strings.HasPrefix(trimmed, "— ") {
		return nil
	}
	return &trimmed
}

// MarkOf is the card's mark: blocked, waiting-owner, waiting, paused, or nil.
func MarkOf(project *config.Project, stage string, held *string) *string {
	column, ok := project.ByKey[project.StageColumn[StageWord(stage)]]
	if ok && column.Kind == "parked" {
		word := StageWord(stage)
		return &word
	}
	if !ok || column.Kind != "open" || held == nil {
		return nil
	}
	word := StageWord(*held)
	if word == "waiting" {
		rest := strings.ToLower(strings.TrimLeft((*held)[len("waiting"):], " :"))
		if strings.HasPrefix(rest, strings.ToLower(project.Owner)) {
			value := "waiting-owner"
			return &value
		}
		value := "waiting"
		return &value
	}
	if word == "blocked" {
		value := "blocked"
		return &value
	}
	return nil
}

// FirstDate is the first YYYY-MM-DD in a value, or nil.
func FirstDate(value *string) *string {
	if value == nil {
		return nil
	}
	found := dateRE.FindString(*value)
	if found == "" {
		return nil
	}
	return &found
}

// SetFields is the brief with head fields set to a value or removed (nil). Only the head is read: a body line that
// looks like a field is never touched. A field's wrapped continuation lines go with it; a missing field is inserted
// after the nearest field that precedes it in Order.
func SetFields(text string, changes map[string]*string) string {
	lines := strings.Split(text, "\n")
	end := splitHead(lines)
	head := append([]string{}, lines[:end]...)
	for _, name := range orderedChanges(changes) {
		value := changes[name]
		found := -1
		for index, line := range head {
			if match := FieldRE.FindStringSubmatch(line); match != nil && strings.TrimSpace(match[1]) == name {
				found = index
				break
			}
		}
		if found >= 0 {
			stop := found + 1
			for stop < len(head) && strings.HasPrefix(head[stop], "  ") && strings.TrimSpace(head[stop]) != "" {
				stop++
			}
			var replacement []string
			if value != nil {
				replacement = []string{fmt.Sprintf("- **%s:** %s", name, *value)}
			}
			head = append(head[:found], append(replacement, head[stop:]...)...)
			continue
		}
		if value == nil {
			continue
		}
		after := 0
		for _, prior := range Order {
			if prior == name {
				break
			}
			for index, line := range head {
				if match := FieldRE.FindStringSubmatch(line); match != nil && strings.TrimSpace(match[1]) == prior {
					stop := index + 1
					for stop < len(head) && strings.HasPrefix(head[stop], "  ") && strings.TrimSpace(head[stop]) != "" {
						stop++
					}
					if stop > after {
						after = stop
					}
					break
				}
			}
		}
		position := len(head)
		if after > 0 {
			position = after
		}
		head = append(head[:position], append([]string{fmt.Sprintf("- **%s:** %s", name, *value)}, head[position:]...)...)
	}
	return strings.Join(append(head, lines[end:]...), "\n")
}

// AddProgress puts a dated line at the end of the Progress section, making the section before Comments when there
// is none.
func AddProgress(text, line string) string {
	lines := strings.Split(text, "\n")
	for index, current := range lines {
		if strings.TrimSpace(current) != "## Progress" {
			continue
		}
		stop := index + 1
		for stop < len(lines) && !strings.HasPrefix(lines[stop], "## ") {
			stop++
		}
		for stop > index+1 && strings.TrimSpace(lines[stop-1]) == "" {
			stop--
		}
		lines = append(lines[:stop], append([]string{line}, lines[stop:]...)...)
		return strings.Join(lines, "\n")
	}
	for index, current := range lines {
		if strings.TrimSpace(current) == "## Comments" {
			insert := []string{"## Progress", "", line, ""}
			lines = append(lines[:index], append(insert, lines[index:]...)...)
			return strings.Join(lines, "\n")
		}
	}
	return strings.TrimRight(text, "\n") + "\n\n## Progress\n\n" + line + "\n"
}

func splitHead(lines []string) int {
	for index := 1; index < len(lines); index++ {
		if strings.HasPrefix(lines[index], "## ") {
			return index
		}
	}
	return len(lines)
}

func orderedChanges(changes map[string]*string) []string {
	names := make([]string, 0, len(changes))
	seen := map[string]bool{}
	for _, name := range Order {
		if _, ok := changes[name]; ok {
			names = append(names, name)
			seen[name] = true
		}
	}
	for name := range changes {
		if !seen[name] {
			names = append(names, name)
		}
	}
	return names
}

// FieldValue is what a brief says under a name: the head line `- **Name:** value`, else the first non-blank line
// under a `## Name` section. The second result is false when the brief says nothing under that name. Names compare
// case-insensitively.
func FieldValue(text, name string) (string, bool) {
	_, fields := Header(text)
	for key, value := range fields {
		if strings.EqualFold(key, name) {
			return value, true
		}
	}
	inSection := false
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "## ") {
			if inSection {
				return "", false
			}
			inSection = strings.EqualFold(strings.TrimSpace(line[3:]), name)
			continue
		}
		if inSection && strings.TrimSpace(line) != "" {
			return strings.TrimSpace(line), true
		}
	}
	return "", false
}

// StartsWithWord is true when value begins with one of the wanted words, whole-word and case-insensitively: "None
// (tooling)" and "none: the board" start with "None", "Nonexistent" does not.
func StartsWithWord(value string, wanted []string) bool {
	lower := strings.ToLower(strings.TrimSpace(value))
	for _, word := range wanted {
		word = strings.ToLower(strings.TrimSpace(word))
		if word == "" || !strings.HasPrefix(lower, word) {
			continue
		}
		rest := lower[len(word):]
		if rest == "" {
			return true
		}
		next := rest[0]
		if !(next >= 'a' && next <= 'z' || next >= '0' && next <= '9' || next == '_' || next >= 0x80) {
			return true
		}
	}
	return false
}
