// Package movelog reads the move log the move command writes: one JSON line per move.
package movelog

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cloudbusting/kanbanzai/internal/config"
	"github.com/cloudbusting/kanbanzai/internal/timeutil"
)

// Path is the project's move log.
func Path(project *config.Project) string {
	return filepath.Join(project.State, "moves.jsonl")
}

// Last is when a card was last moved, or false when it never was. The newest line for the card wins.
func Last(project *config.Project, card string) (time.Time, bool) {
	data, err := os.ReadFile(Path(project))
	if err != nil {
		return time.Time{}, false
	}
	lines := strings.Split(string(data), "\n")
	for index := len(lines) - 1; index >= 0; index-- {
		line := strings.TrimSpace(lines[index])
		if line == "" {
			continue
		}
		var entry map[string]any
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}
		if entry["card"] != card {
			continue
		}
		at, ok := timeutil.Parse(entry["at"])
		return at, ok
	}
	return time.Time{}, false
}
