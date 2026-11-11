// Package sessions reads what a claimed card's session is doing.
//
// The harnesses write the state: Claude Code keeps a status file per live session, and Kanbanzai's OpenCode plugin
// writes one small file per session under the project's state folder. The reader treats both as untrusted input:
// a shape it does not recognise, or a process that is not alive here, is skipped and never raises.
package sessions

import (
	"github.com/cloudbusting/kanbanzai/internal/config"
)

// Record is one live session's public state, as `board --json` and `sessions --json` carry it.
type Record struct {
	State   string `json:"state"`
	Since   int64  `json:"since"`
	Started *int64 `json:"started"`
	Name    string `json:"name"`
}

// ForClaims returns the session of every claimed card whose harness has one. The readers land with their own
// task; until then the board's session field is empty and no session state is invented.
func ForClaims(claims map[string]string, project *config.Project) map[string]Record {
	return nil
}
