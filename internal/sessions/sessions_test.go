package sessions

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/cloudbusting/kanbanzai/internal/config"
	"github.com/cloudbusting/kanbanzai/internal/testutil"
)

const (
	SID = "0f5cdd50-0cf7-4d6d-8513-8a2e02940987"
	OC  = "ses_oc1"
)

func myStart(t *testing.T) any {
	t.Helper()
	if runtime.GOOS != "linux" {
		return nil
	}
	data, err := os.ReadFile("/proc/self/stat")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	end := strings.LastIndex(text, ")")
	if end < 0 {
		t.Fatal("no stat fields")
	}
	fields := strings.Fields(text[end+1:])
	if len(fields) < 20 {
		t.Fatalf("stat fields = %d", len(fields))
	}
	return fields[19]
}

func claudeDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "sessions"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CLAUDE_CONFIG_DIR", dir)
	return dir
}

func writeSession(t *testing.T, dir string, name string, over map[string]any) {
	t.Helper()
	record := map[string]any{
		"pid": os.Getpid(), "sessionId": SID, "startedAt": 1794343666976, "procStart": myStart(t),
		"name": name, "status": "waiting", "waitingFor": "input needed", "updatedAt": 1794343821264,
		"statusUpdatedAt": 1794343821000,
	}
	for key, value := range over {
		record[key] = value
	}
	for key, value := range record {
		if value == nil {
			delete(record, key)
		}
	}
	data, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sessions", name+".json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func testProject(t *testing.T) *config.Project {
	t.Helper()
	t.Setenv("KANBANZAI_PROJECT", "")
	root := testutil.Repo(t, "[project]\nowner = \"Sam\"\n")
	project, err := config.Find(root)
	if err != nil {
		t.Fatal(err)
	}
	return project
}

func writeOpenCode(t *testing.T, project *config.Project, name string, over map[string]any) {
	t.Helper()
	folder := OpenCodeDir(project.State)
	if err := os.MkdirAll(folder, 0o755); err != nil {
		t.Fatal(err)
	}
	record := map[string]any{
		"sessionId": OC, "state": "waiting", "since": 1794343821000, "started": 1794343666976,
		"name": name, "pid": os.Getpid(), "procStart": myStart(t), "updated": 1794343821000,
		"directory": project.Root,
	}
	for key, value := range over {
		record[key] = value
	}
	for key, value := range record {
		if value == nil {
			delete(record, key)
		}
	}
	data, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(folder, name+".json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestAWaitingSessionGivesItsRecord(t *testing.T) {
	dir := claudeDir(t)
	writeSession(t, dir, "proj-7b", nil)
	records := ReadAll()
	record, ok := records[SID]
	if !ok {
		t.Fatalf("records = %#v", records)
	}
	if record.State != "waiting" || record.Since != 1794343821000 || record.Name != "proj-7b" {
		t.Fatalf("record = %#v", record)
	}
	if record.Started == nil || *record.Started != 1794343666976 {
		t.Fatalf("started = %#v", record.Started)
	}
}

func TestBusyAndIdleAreStates(t *testing.T) {
	dir := claudeDir(t)
	for _, state := range []string{"busy", "idle"} {
		writeSession(t, dir, "proj-7b", map[string]any{"status": state, "waitingFor": nil})
		record, ok := ReadAll()[SID]
		if !ok || record.State != state {
			t.Fatalf("%s: %#v", state, record)
		}
	}
}

func TestADeadProcessGivesNone(t *testing.T) {
	dir := claudeDir(t)
	writeSession(t, dir, "dead", map[string]any{"pid": 4194999})
	if records := ReadAll(); len(records) != 0 {
		t.Fatalf("records = %#v", records)
	}
}

func TestAProcessThatIsNotTheRecordedOneGivesNone(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("the start-time comparison is the Linux liveness path")
	}
	dir := claudeDir(t)
	writeSession(t, dir, "wrong-start", map[string]any{"procStart": "1"})
	if records := ReadAll(); len(records) != 0 {
		t.Fatalf("wrong start: %#v", records)
	}
	writeSession(t, dir, "no-start", map[string]any{"procStart": nil})
	if records := ReadAll(); len(records) != 0 {
		t.Fatalf("no start: %#v", records)
	}
}

func TestJunkGivesNoneAndRaisesNothing(t *testing.T) {
	dir := claudeDir(t)
	writeSession(t, dir, "good", nil)
	folder := filepath.Join(dir, "sessions")
	if err := os.WriteFile(filepath.Join(folder, "a.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(folder, "b.json"), []byte("[1, 2]"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(folder, "d.json"), 0o755); err != nil {
		t.Fatal(err)
	}
	makeFIFO(t, filepath.Join(folder, "f.json"))
	big, err := os.Create(filepath.Join(folder, "big.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := big.Truncate(MaxBytes + 1); err != nil {
		t.Fatal(err)
	}
	big.Close()
	if runtime.GOOS != "windows" {
		if err := os.Symlink(filepath.Join(t.TempDir(), "nowhere"), filepath.Join(folder, "l.json")); err != nil {
			t.Fatal(err)
		}
	}
	for _, over := range []map[string]any{
		{"sessionId": nil}, {"status": "asleep"}, {"status": 7}, {"statusUpdatedAt": "soon"},
		{"pid": "x"}, {"statusUpdatedAt": true},
	} {
		writeSession(t, dir, "bad", over)
	}
	records := ReadAll()
	if len(records) != 1 {
		t.Fatalf("records = %#v", records)
	}
	if _, ok := records[SID]; !ok {
		t.Fatalf("the good session was lost: %#v", records)
	}
}

func TestTheLaterUpdatedFileWins(t *testing.T) {
	dir := claudeDir(t)
	writeSession(t, dir, "old", map[string]any{"status": "busy", "updatedAt": 1000, "waitingFor": nil})
	writeSession(t, dir, "new", map[string]any{"updatedAt": 2000})
	record, ok := ReadAll()[SID]
	if !ok || record.State != "waiting" || record.Name != "new" {
		t.Fatalf("record = %#v, %v", record, ok)
	}
}

func TestNoSessionsFolderGivesNone(t *testing.T) {
	t.Setenv("CLAUDE_CONFIG_DIR", filepath.Join(t.TempDir(), "nowhere"))
	if records := ReadAll(); len(records) != 0 {
		t.Fatalf("records = %#v", records)
	}
}

func TestTheConfigDirIsReadFromTheEnvironmentElseTheHome(t *testing.T) {
	dir := claudeDir(t)
	if got := SessionsDir(); got != filepath.Join(dir, "sessions") {
		t.Fatalf("sessions dir = %q", got)
	}
	home := filepath.Join(t.TempDir(), "home")
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	t.Setenv("HOME", home)
	if got := SessionsDir(); got != filepath.Join(home, ".claude", "sessions") {
		t.Fatalf("home sessions dir = %q", got)
	}
}

func TestANameThatIsNotTextIsEmpty(t *testing.T) {
	dir := claudeDir(t)
	writeSession(t, dir, "noname", map[string]any{"name": nil})
	record, ok := ReadAll()[SID]
	if !ok || record.Name != "" {
		t.Fatalf("record = %#v", record)
	}
}

func TestOpenCodeSessionsAreFilteredToTheProject(t *testing.T) {
	project := testProject(t)
	writeOpenCode(t, project, "proj-oc", nil)
	records := ReadOpenCode(project.State, project.Root)
	record, ok := records[OC]
	if !ok || record.State != "waiting" || record.Name != "proj-oc" {
		t.Fatalf("records = %#v", records)
	}
	if record.Started == nil || *record.Started != 1794343666976 {
		t.Fatalf("started = %#v", record.Started)
	}

	writeOpenCode(t, project, "foreign", map[string]any{"sessionId": "ses_foreign",
		"directory": filepath.Join(t.TempDir(), "elsewhere")})
	records = ReadOpenCode(project.State, project.Root)
	if _, ok := records["ses_foreign"]; ok {
		t.Fatalf("a foreign session was included: %#v", records)
	}
	if _, ok := records[OC]; !ok {
		t.Fatalf("the project's own session was lost: %#v", records)
	}
}

func TestForClaimsReadsEachHarness(t *testing.T) {
	dir := claudeDir(t)
	writeSession(t, dir, "proj-7b", nil)
	project := testProject(t)
	writeOpenCode(t, project, "proj-oc", nil)
	claims := map[string]string{"W-99": "claude:" + SID, "W-10": "opencode:" + OC, "W-11": "me@host"}
	records := ForClaims(claims, project)
	if len(records) != 2 {
		t.Fatalf("records = %#v", records)
	}
	if records["W-99"].State != "waiting" || records["W-10"].Name != "proj-oc" {
		t.Fatalf("records = %#v", records)
	}
	if _, ok := records["W-11"]; ok {
		t.Fatal("a session for a non-harness owner")
	}
}

func TestForClaimsWithNoClaimsIsEmpty(t *testing.T) {
	project := testProject(t)
	records := ForClaims(map[string]string{}, project)
	if records == nil || len(records) != 0 {
		t.Fatalf("records = %#v", records)
	}
}

func TestOfOwnerUsesThePrefixes(t *testing.T) {
	claude := map[string]Record{"a": {State: "idle", Name: "claude"}}
	opencode := map[string]Record{"b": {State: "busy", Name: "opencode"}}
	if got := OfOwner("claude:a", claude, opencode); got == nil || got.Name != "claude" {
		t.Fatalf("claude = %#v", got)
	}
	if got := OfOwner("opencode:b", claude, opencode); got == nil || got.Name != "opencode" {
		t.Fatalf("opencode = %#v", got)
	}
	if OfOwner("claude:missing", claude, opencode) != nil || OfOwner("me@host", claude, opencode) != nil {
		t.Fatal("an unknown owner got a session")
	}
}

func TestOpenCodeJunkIsSkipped(t *testing.T) {
	project := testProject(t)
	writeOpenCode(t, project, "proj-oc", nil)
	folder := OpenCodeDir(project.State)
	if err := os.WriteFile(filepath.Join(folder, "a.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(folder, "c.json"), 0o755); err != nil {
		t.Fatal(err)
	}
	makeFIFO(t, filepath.Join(folder, "d.json"))
	big, err := os.Create(filepath.Join(folder, "e.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := big.Truncate(MaxBytes + 1); err != nil {
		t.Fatal(err)
	}
	big.Close()
	for _, over := range []map[string]any{
		{"pid": 4194999}, {"directory": filepath.Join(t.TempDir(), "elsewhere")}, {"state": "asleep"},
		{"state": 7}, {"sessionId": nil}, {"since": "soon"}, {"since": true}, {"pid": "x"},
	} {
		bad := map[string]any{"sessionId": "ses_bad"}
		for key, value := range over {
			bad[key] = value
		}
		writeOpenCode(t, project, "bad", bad)
		if records := ReadOpenCode(project.State, project.Root); len(records) != 1 {
			t.Fatalf("junk %v: %#v", over, records)
		}
	}
}

func TestOpenCodeLaterUpdatedWins(t *testing.T) {
	project := testProject(t)
	writeOpenCode(t, project, "old", map[string]any{"state": "busy"})
	writeOpenCode(t, project, "new", map[string]any{"updated": 1794343900000})
	records := ReadOpenCode(project.State, project.Root)
	if records[OC].State != "waiting" || records[OC].Name != "new" {
		t.Fatalf("records = %#v", records)
	}
}

func TestASessionFileMustBeRegularAndSmall(t *testing.T) {
	dir := claudeDir(t)
	writeSession(t, dir, "good", nil)
	if err := os.WriteFile(filepath.Join(dir, "sessions", "small.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if records := ReadAll(); len(records) != 1 {
		t.Fatalf("records = %#v", records)
	}
}
