// Package epic validates membership and computes read-only delivery progress. Membership is separate from Needs;
// no read changes a stage, hold or gate.
package epic

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cloudbusting/kanbanzai/internal/brief"
	"github.com/cloudbusting/kanbanzai/internal/config"
)

// Progress is the additive board JSON contract on epic cards.
type Progress struct {
	Total     int  `json:"total"`
	Delivered int  `json:"delivered"`
	Cancelled int  `json:"cancelled"`
	Remaining int  `json:"remaining"`
	Complete  bool `json:"complete"`
}

type member struct {
	id, path, items string
	epic, hasItems  bool
	done, cancelled bool
}

// Load reads the same board paths as the caller, validating all members before counting delivery. Missing,
// duplicate or malformed references cannot make an epic complete, even on a board that has not passed check.
func Load(project *config.Project, paths []string) (map[string]*Progress, []string) {
	cards := []member{}
	known := map[string]member{}
	ambiguous := map[string]bool{}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue // the board reader reports unreadable briefs; references to them remain missing
		}
		title, fields := brief.Header(string(data))
		found := project.TitleRe.FindStringSubmatch(title)
		if found == nil {
			continue
		}
		id := found[1]
		item, typed := project.TypeOf(id)
		items, present := fields["Work items"]
		col, validStage := project.ByKey[project.StageColumn[brief.StageWord(fields["Stage"])]]
		validPath := strings.HasPrefix(filepath.Base(path), id+"-") && filepath.Base(filepath.Dir(path)) == col.Folder
		c := member{id: id, path: project.Rel(path), items: items, hasItems: present, epic: typed && item.Key == "epic",
			done:      validStage && validPath && col.Kind == "done",
			cancelled: strings.Contains(fields["Done"], " — closed, no change: ")}
		if _, exists := known[id]; exists {
			ambiguous[id] = true
		}
		known[id] = c
		cards = append(cards, c)
	}
	progress := map[string]*Progress{}
	problems := []string{}
	for _, c := range cards {
		if !c.epic {
			if c.hasItems {
				problems = append(problems, fmt.Sprintf("%s: Work items belongs only on an epic", c.path))
			}
			continue
		}
		p := &Progress{}
		progress[c.id] = p
		valid := !ambiguous[c.id]
		report := func(message string) {
			problems = append(problems, fmt.Sprintf("%s: Work items: %s", c.path, message))
			valid = false
		}
		value := strings.TrimSpace(c.items)
		if !c.hasItems || value == "" {
			report("missing membership list; use — for no work items")
		}
		seen := map[string]bool{}
		if value != "" && value != "—" {
			for _, part := range strings.Split(value, ",") {
				id := strings.TrimSpace(part)
				if seen[id] {
					report("duplicate reference " + id)
					continue
				}
				seen[id] = true
				p.Total++
				child, exists := known[id]
				switch {
				case !project.IsCardID(id):
					report(fmt.Sprintf("invalid card ID %q", id))
				case id == c.id:
					report("an epic cannot contain itself")
				case !exists:
					report(id + " is not on the board")
				case ambiguous[id]:
					report(id + " has duplicate briefs")
				case child.epic:
					report(id + " is a nested epic")
				case child.done && child.cancelled:
					p.Cancelled++
				case child.done:
					p.Delivered++
				}
			}
		}
		p.Remaining = p.Total - p.Delivered
		p.Complete = valid && p.Total > 0 && p.Remaining == 0
	}
	return progress, problems
}
