// Package guide serves the cheat sheet in skills/help/SKILL.md: the marked section as Markdown, or parsed into the
// small JSON shape the editor's Help popup renders. The skill is the one source; this package only reads it, so the
// skill, `kanbanzai guide` and the popup cannot disagree.
package guide

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"regexp"
	"strings"
)

const (
	// File is the skill the sheet lives in, inside the embedded files.
	File  = "skills/help/SKILL.md"
	start = "<!-- cheat sheet: start -->"
	end   = "<!-- cheat sheet: end -->"
	// Title heads the JSON; the sheet's own text starts at its first section.
	Title = "Kanbanzai cheat sheet"
)

// Item is one command (or flow) and when to use it.
type Item struct {
	Command string `json:"command"`
	Text    string `json:"text"`
}

// Section is a heading with either one paragraph or a list of items, never both. Items is never null.
type Section struct {
	Title string `json:"title"`
	Text  string `json:"text"`
	Items []Item `json:"items"`
}

// Sheet is the parsed cheat sheet.
type Sheet struct {
	Title    string    `json:"title"`
	Sections []Section `json:"sections"`
}

// Markdown returns the sheet exactly as written between its markers, trimmed of blank lines at both ends and ending
// in one newline.
func Markdown(files fs.FS) (string, error) {
	data, err := fs.ReadFile(files, File)
	if err != nil {
		return "", fmt.Errorf("the help skill is missing: %w", err)
	}
	text := string(data)
	if strings.Count(text, start) != 1 || strings.Count(text, end) != 1 {
		return "", fmt.Errorf("%s must carry one %q and one %q", File, start, end)
	}
	from := strings.Index(text, start) + len(start)
	to := strings.Index(text, end)
	if to < from {
		return "", fmt.Errorf("%s: the sheet's end comes before its start", File)
	}
	return strings.TrimSpace(text[from:to]) + "\n", nil
}

var itemLine = regexp.MustCompile("^- `([^`]+)` — (\\S.*)$")

// Parse reads the fixed format: `## ` section headings, and under each either one paragraph line or `- ` items of the
// form "- `command` — text". Anything else is an error, so the sheet cannot grow Markdown the popup would show wrongly.
func Parse(markdown string) (Sheet, error) {
	sheet := Sheet{Title: Title, Sections: []Section{}}
	var current *Section
	seen := map[string]bool{}
	closeSection := func() error {
		if current == nil {
			return nil
		}
		if current.Text == "" && len(current.Items) == 0 {
			return fmt.Errorf("the section %q is empty", current.Title)
		}
		sheet.Sections = append(sheet.Sections, *current)
		current = nil
		return nil
	}
	for number, raw := range strings.Split(markdown, "\n") {
		line := strings.TrimRight(raw, " \t\r")
		where := fmt.Sprintf("sheet line %d", number+1)
		switch {
		case line == "":
			continue
		case strings.HasPrefix(line, "## "):
			if err := closeSection(); err != nil {
				return Sheet{}, err
			}
			title := strings.TrimSpace(strings.TrimPrefix(line, "## "))
			if title == "" || strings.HasPrefix(title, "#") || seen[title] {
				return Sheet{}, fmt.Errorf("%s: a section heading must be new and plain: %q", where, line)
			}
			seen[title] = true
			current = &Section{Title: title, Items: []Item{}}
		case current == nil:
			return Sheet{}, fmt.Errorf("%s: text before the first `## ` heading: %q", where, line)
		case strings.HasPrefix(line, "- "):
			m := itemLine.FindStringSubmatch(line)
			if m == nil {
				return Sheet{}, fmt.Errorf("%s: an item is \"- `command` — text\": %q", where, line)
			}
			if current.Text != "" {
				return Sheet{}, fmt.Errorf("%s: a section holds a paragraph or items, not both: %q", where, current.Title)
			}
			current.Items = append(current.Items, Item{Command: m[1], Text: m[2]})
		case strings.ContainsAny(line[:1], "#|<>*"):
			return Sheet{}, fmt.Errorf("%s: only `## ` headings, one paragraph or `- ` items are allowed: %q", where, line)
		default:
			if current.Text != "" || len(current.Items) > 0 {
				return Sheet{}, fmt.Errorf("%s: a section holds one paragraph line, or items, not both: %q", where, current.Title)
			}
			current.Text = line
		}
	}
	if err := closeSection(); err != nil {
		return Sheet{}, err
	}
	if len(sheet.Sections) == 0 {
		return Sheet{}, fmt.Errorf("the sheet has no sections")
	}
	return sheet, nil
}

// Run prints the sheet: its Markdown, or with jsonOut the parsed JSON (indented, one trailing newline). It needs no
// project. A sheet outside the format is exit 1 with the reason on errOut and nothing on out.
func Run(files fs.FS, jsonOut bool, out, errOut io.Writer) int {
	markdown, err := Markdown(files)
	if err != nil {
		fmt.Fprintf(errOut, "kanbanzai: guide: %v\n", err)
		return 1
	}
	sheet, err := Parse(markdown)
	if err != nil {
		fmt.Fprintf(errOut, "kanbanzai: guide: %v\n", err)
		return 1
	}
	if !jsonOut {
		_, _ = io.WriteString(out, markdown)
		return 0
	}
	data, err := json.MarshalIndent(sheet, "", "  ")
	if err != nil {
		fmt.Fprintf(errOut, "kanbanzai: guide: %v\n", err)
		return 1
	}
	_, _ = out.Write(append(data, '\n'))
	return 0
}
