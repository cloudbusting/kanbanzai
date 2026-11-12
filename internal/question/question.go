// Package question reads and edits the Questions section of a card brief.
package question

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/cloudbusting/kanbanzai/internal/brief"
)

// Entry is one question and its recorded answer.
type Entry struct {
	Heading, AskedBy, Text, Recommendation, Then, Answer, Answered string
	Options                                                        []string
}

var headingRE = regexp.MustCompile(`^### Q([0-9]+) — (\d{4}-\d{2}-\d{2})$`)

// Parse returns all entries in order, rejecting incomplete or ambiguous records.
func Parse(text string) ([]Entry, error) {
	lines := strings.Split(text, "\n")
	start, end, err := section(lines)
	if err != nil || start < 0 {
		return nil, err
	}
	var result []Entry
	for i := start + 1; i < end; {
		if strings.TrimSpace(lines[i]) == "" {
			i++
			continue
		}
		if !headingRE.MatchString(lines[i]) {
			return nil, fmt.Errorf("invalid question heading: %s", lines[i])
		}
		entry := Entry{Heading: strings.TrimPrefix(lines[i], "### ")}
		i++
		fields := map[string]string{}
		last := ""
		for i < end && !strings.HasPrefix(lines[i], "### ") {
			line := lines[i]
			if m := brief.FieldRE.FindStringSubmatch(line); m != nil {
				if _, exists := fields[m[1]]; exists {
					return nil, fmt.Errorf("duplicate question field %s", m[1])
				}
				last = m[1]
				fields[last] = strings.TrimSpace(m[2])
			} else if last != "" && strings.HasPrefix(line, "  ") && strings.TrimSpace(line) != "" {
				fields[last] += " " + strings.TrimSpace(line)
			} else if strings.TrimSpace(line) != "" {
				return nil, fmt.Errorf("invalid question line: %s", line)
			} else {
				last = ""
			}
			i++
		}
		entry.AskedBy, entry.Text, entry.Then = fields["Asked by"], fields["Question"], fields["Then"]
		entry.Answer, entry.Answered, entry.Recommendation = fields["Answer"], fields["Answered"], fields["Recommendation"]
		if value := fields["Options"]; value != "" {
			for _, option := range strings.Split(value, ";") {
				entry.Options = append(entry.Options, strings.TrimSpace(option))
			}
		}
		_, answerExists := fields["Answer"]
		_, answeredExists := fields["Answered"]
		if entry.Text == "" || entry.AskedBy == "" || entry.Then == "" || !answerExists || !answeredExists {
			return nil, fmt.Errorf("incomplete question %s", entry.Heading)
		}
		if isEmpty(entry.Answer) != isEmpty(entry.Answered) {
			return nil, fmt.Errorf("inconsistent answer for %s", entry.Heading)
		}
		result = append(result, entry)
	}
	return result, nil
}

func isEmpty(s string) bool { return strings.TrimSpace(s) == "" || strings.TrimSpace(s) == "—" }

func section(lines []string) (int, int, error) {
	start, end, comments := -1, len(lines), -1
	for i, line := range lines {
		if line == "## Comments" {
			comments = i
		}
		if line == "## Questions" {
			if start >= 0 {
				return -1, 0, fmt.Errorf("duplicate Questions section")
			}
			start = i
			continue
		}
		if start >= 0 && end == len(lines) && strings.HasPrefix(line, "## ") {
			end = i
		}
	}
	if start >= 0 && comments >= 0 && start > comments {
		return -1, 0, fmt.Errorf("Questions must precede Comments")
	}
	return start, end, nil
}

// Append adds a new entry immediately before the next section.
func Append(text string, entry Entry, date string) (string, error) {
	entries, err := Parse(text)
	if err != nil {
		return text, err
	}
	for _, e := range entries {
		if isEmpty(e.Answer) {
			return text, fmt.Errorf("a question is already open")
		}
	}
	if strings.TrimSpace(entry.Text) == "" || strings.TrimSpace(entry.AskedBy) == "" || strings.TrimSpace(entry.Then) == "" {
		return text, fmt.Errorf("question, asker and next action are required")
	}
	values := append([]string{entry.Text, entry.AskedBy, entry.Then, entry.Recommendation, date}, entry.Options...)
	for _, s := range values {
		if strings.ContainsAny(s, "\r\n") {
			return text, fmt.Errorf("question fields must be single-line")
		}
	}
	for _, option := range entry.Options {
		if strings.Contains(option, ";") || strings.TrimSpace(option) == "" {
			return text, fmt.Errorf("invalid option")
		}
	}
	lines := strings.Split(text, "\n")
	start, end, err := section(lines)
	if err != nil {
		return text, err
	}
	if start < 0 {
		end = len(lines)
		for i, line := range lines {
			if line == "## Comments" {
				end = i
				break
			}
		}
		lines = append(lines[:end], append([]string{"## Questions", ""}, lines[end:]...)...)
		end += 2
	}
	for end > 0 && strings.TrimSpace(lines[end-1]) == "" {
		end--
	}
	next := 1
	for _, existing := range entries {
		match := headingRE.FindStringSubmatch("### " + existing.Heading)
		n, err := strconv.Atoi(match[1])
		if err != nil {
			return text, fmt.Errorf("invalid question number")
		}
		if n >= next {
			next = n + 1
		}
	}
	if !headingRE.MatchString(fmt.Sprintf("### Q%d — %s", next, date)) {
		return text, fmt.Errorf("invalid question date")
	}
	insert := []string{"", fmt.Sprintf("### Q%d — %s", next, date), "", "- **Asked by:** " + entry.AskedBy, "- **Question:** " + entry.Text}
	if len(entry.Options) > 0 {
		insert = append(insert, "- **Options:** "+strings.Join(entry.Options, "; "))
	}
	if entry.Recommendation != "" {
		insert = append(insert, "- **Recommendation:** "+entry.Recommendation)
	}
	insert = append(insert, "- **Then:** "+entry.Then, "- **Answer:** —", "- **Answered:** —", "")
	lines = append(lines[:end], append(insert, lines[end:]...)...)
	return strings.Join(lines, "\n"), nil
}

// Close fills the last open entry without changing other sections.
func Close(text, answer, answered string) (string, error) {
	entries, err := Parse(text)
	if err != nil {
		return text, err
	}
	if isEmpty(answer) || isEmpty(answered) || strings.ContainsAny(answer+answered, "\r\n") {
		return text, fmt.Errorf("answer and answered are required on one line")
	}
	index := -1
	for i := range entries {
		if isEmpty(entries[i].Answer) {
			index = i
		}
	}
	if index < 0 {
		return text, fmt.Errorf("no open question")
	}
	lines := strings.Split(text, "\n")
	start, end, _ := section(lines)
	active := false
	for i := start + 1; i < end; i++ {
		if strings.HasPrefix(lines[i], "### ") {
			active = strings.TrimPrefix(lines[i], "### ") == entries[index].Heading
			continue
		}
		if active && strings.HasPrefix(lines[i], "- **Answer:**") {
			lines[i] = "- **Answer:** " + answer
		}
		if active && strings.HasPrefix(lines[i], "- **Answered:**") {
			lines[i] = "- **Answered:** " + answered
		}
	}
	return strings.Join(lines, "\n"), nil
}
