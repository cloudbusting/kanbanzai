// Package sessions reads what a claimed card's session is doing.
//
// The harnesses write the state: Claude Code keeps a status file per live session, and Kanbanzai's OpenCode plugin
// writes one small file per session under the project's state folder. The reader treats both as untrusted input: a
// shape it does not recognise, a file that is not a small regular file, or a process that is not alive here is
// skipped and never raises.
package sessions

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/cloudbusting/kanbanzai/internal/config"
)

// States are the three states a session can be in.
var States = map[string]bool{"busy": true, "idle": true, "waiting": true}

const (
	claudePrefix   = "claude:"
	opencodePrefix = "opencode:"
	// MaxBytes is the largest status file the reader will open. A file near this is not one.
	MaxBytes = 1024 * 1024
)

// Record is one live session's public state, as `board --json` and `sessions --json` carry it.
type Record struct {
	State   string `json:"state"`
	Since   int64  `json:"since"`
	Started *int64 `json:"started"`
	Name    string `json:"name"`
}

type fileRecord struct {
	ID        string
	Updated   int64
	Record    Record
	Directory string
}

// SessionsDir is $CLAUDE_CONFIG_DIR/sessions, else ~/.claude/sessions.
func SessionsDir() string {
	base := strings.TrimSpace(os.Getenv("CLAUDE_CONFIG_DIR"))
	if base == "" {
		home := strings.TrimSpace(os.Getenv("HOME"))
		if home == "" {
			if value, err := os.UserHomeDir(); err == nil {
				home = value
			}
		}
		base = filepath.Join(home, ".claude")
	}
	return filepath.Join(base, "sessions")
}

// OpenCodeDir is <state>/sessions/opencode: where the OpenCode plugin writes its sessions' state.
func OpenCodeDir(state string) string {
	return filepath.Join(state, "sessions", "opencode")
}

// ReadAll is {session id: {state, since, started, name}} for Claude Code's live sessions; never raises.
func ReadAll() map[string]Record {
	return public(readFolder(SessionsDir(), claudeShape))
}

// ReadOpenCode is the OpenCode sessions this project's plugin wrote; never raises.
func ReadOpenCode(state, root string) map[string]Record {
	found := readFolder(OpenCodeDir(state), opencodeShape)
	canonical := canonicalPath(root)
	out := map[string]Record{}
	for id, record := range found {
		if record.Directory == "" || canonicalPath(record.Directory) == canonical {
			out[id] = record.Record
		}
	}
	return out
}

// UnclaimedAsking is {owner label: session} for the live sessions of this project that are waiting for the owner and
// hold no live claim in it, so no card can show them asking (BUG-64). `owners` is {card: owner label} for the live
// claims. A Claude Code session belongs to the project when the folder it started in is the project root, inside it,
// or inside the folder the worktree setting puts card worktrees in; a file with no folder is not placed. An OpenCode
// session belongs to it as ReadOpenCode decides. Only waiting sessions are returned, never busy or idle ones. It
// never raises and the result is never nil.
func UnclaimedAsking(owners map[string]string, project *config.Project) (out map[string]Record) {
	out = map[string]Record{}
	defer func() {
		if recover() != nil {
			out = map[string]Record{}
		}
	}()
	if project == nil {
		return out
	}
	held := map[string]bool{}
	for _, owner := range owners {
		held[owner] = true
	}
	folders := projectFolders(project)
	for id, record := range readFolder(SessionsDir(), claudeShape) {
		label := claudePrefix + id
		if record.Record.State != "waiting" || held[label] || !within(record.Directory, folders) {
			continue
		}
		out[label] = record.Record
	}
	for id, record := range ReadOpenCode(project.State, project.Root) {
		label := opencodePrefix + id
		if record.State != "waiting" || held[label] {
			continue
		}
		out[label] = record
	}
	return out
}

// projectFolders is the project root and the folder card worktrees are made in: where a session's folder must be.
func projectFolders(project *config.Project) []string {
	folders := []string{canonicalPath(project.Root)}
	// The setting's last element holds {id}; its parent is the folder every card worktree lives in.
	// With {type} in a folder above it, each type has its own folder, so one is taken for every configured type.
	if strings.Contains(filepath.Base(project.WorktreePattern), "{id}") {
		samples := []string{"card"}
		for _, item := range project.Types {
			samples = append(samples, project.TypeSet.Format(item, 1))
		}
		for _, sample := range samples {
			if path := project.WorktreePath(sample); path != "" {
				folder := canonicalPath(filepath.Dir(path))
				if !slices.Contains(folders, folder) {
					folders = append(folders, folder)
				}
			}
		}
	}
	return folders
}

// within is true when directory is one of the folders or inside one; an empty directory is within none.
func within(directory string, folders []string) bool {
	if directory == "" {
		return false
	}
	path := canonicalPath(directory)
	for _, folder := range folders {
		if folder == "" {
			continue
		}
		if path == folder || strings.HasPrefix(path, folder+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

// OfOwner is the session of a claim's owner label, or nil: the label's harness decides which folder it may come from.
func OfOwner(owner string, claude, opencode map[string]Record) *Record {
	if strings.HasPrefix(owner, claudePrefix) {
		if record, ok := claude[owner[len(claudePrefix):]]; ok {
			return &record
		}
		return nil
	}
	if opencode != nil && strings.HasPrefix(owner, opencodePrefix) {
		if record, ok := opencode[owner[len(opencodePrefix):]]; ok {
			return &record
		}
	}
	return nil
}

// ForClaims is {card: session} for the claims ({card: owner label}) that have one.
//
// Only the folders a claim's harness needs are read: a project with no OpenCode claim never touches the OpenCode
// folder, and a machine whose claims are all OpenCode never reads Claude Code's. It never raises.
func ForClaims(claims map[string]string, project *config.Project) (out map[string]Record) {
	out = map[string]Record{}
	defer func() {
		if recover() != nil {
			out = map[string]Record{}
		}
	}()
	if len(claims) == 0 {
		return out
	}
	wantsClaude := false
	wantsOpenCode := false
	for _, owner := range claims {
		if strings.HasPrefix(owner, claudePrefix) {
			wantsClaude = true
		}
		if project != nil && strings.HasPrefix(owner, opencodePrefix) {
			wantsOpenCode = true
		}
	}
	if !wantsClaude && !wantsOpenCode {
		return out
	}
	var claude map[string]Record
	var opencode map[string]Record
	if wantsClaude {
		claude = ReadAll()
	}
	if wantsOpenCode {
		opencode = ReadOpenCode(project.State, project.Root)
	}
	for card, owner := range claims {
		if session := OfOwner(owner, claude, opencode); session != nil {
			out[card] = *session
		}
	}
	return out
}

func claudeShape(data map[string]any) *fileRecord {
	session, ok := data["sessionId"].(string)
	if !ok || session == "" {
		return nil
	}
	state, ok := data["status"].(string)
	if !ok || !States[state] {
		return nil
	}
	pid, ok := integerOf(data["pid"])
	if !ok {
		return nil
	}
	since, ok := integerOf(data["statusUpdatedAt"])
	if !ok {
		return nil
	}
	if !alive(int(pid), data["procStart"]) {
		return nil
	}
	name, _ := data["name"].(string)
	updated, ok := integerOf(data["updatedAt"])
	if !ok {
		updated = since
	}
	var started *int64
	if value, ok := integerOf(data["startedAt"]); ok {
		started = &value
	}
	directory, _ := data["cwd"].(string)
	return &fileRecord{
		ID: session, Updated: updated, Directory: directory,
		Record: Record{State: state, Since: since, Started: started, Name: name},
	}
}

func opencodeShape(data map[string]any) *fileRecord {
	session, ok := data["sessionId"].(string)
	if !ok || session == "" {
		return nil
	}
	state, ok := data["state"].(string)
	if !ok || !States[state] {
		return nil
	}
	pid, ok := integerOf(data["pid"])
	if !ok {
		return nil
	}
	since, ok := integerOf(data["since"])
	if !ok {
		return nil
	}
	if !alive(int(pid), data["procStart"]) {
		return nil
	}
	name, _ := data["name"].(string)
	updated, ok := integerOf(data["updated"])
	if !ok {
		updated = since
	}
	var started *int64
	if value, ok := integerOf(data["started"]); ok {
		started = &value
	}
	directory, _ := data["directory"].(string)
	return &fileRecord{
		ID: session, Updated: updated, Directory: directory,
		Record: Record{State: state, Since: since, Started: started, Name: name},
	}
}

func readFolder(folder string, shape func(map[string]any) *fileRecord) map[string]*fileRecord {
	found := map[string]*fileRecord{}
	entries, err := os.ReadDir(folder)
	if err != nil {
		return found
	}
	names := make([]string, 0)
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".json") {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	for _, name := range names {
		data, ok := jsonObject(filepath.Join(folder, name))
		if !ok {
			continue
		}
		record := shape(data)
		if record == nil {
			continue
		}
		if previous, ok := found[record.ID]; !ok || record.Updated > previous.Updated {
			found[record.ID] = record
		}
	}
	return found
}

func jsonObject(path string) (map[string]any, bool) {
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() > MaxBytes {
		return nil, false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return nil, false
	}
	object, ok := value.(map[string]any)
	return object, ok
}

func public(found map[string]*fileRecord) map[string]Record {
	out := map[string]Record{}
	for id, record := range found {
		out[id] = record.Record
	}
	return out
}

func integerOf(value any) (int64, bool) {
	switch typed := value.(type) {
	case float64:
		if typed == math.Trunc(typed) {
			return int64(typed), true
		}
	case int:
		return int64(typed), true
	case int64:
		return typed, true
	case json.Number:
		parsed, err := typed.Int64()
		if err == nil {
			return parsed, true
		}
	}
	return 0, false
}

func canonicalPath(path string) string {
	if path == "" {
		return ""
	}
	value, err := filepath.Abs(path)
	if err != nil {
		value = filepath.Clean(path)
	}
	if resolved, err := filepath.EvalSymlinks(value); err == nil {
		return resolved
	}
	return value
}
