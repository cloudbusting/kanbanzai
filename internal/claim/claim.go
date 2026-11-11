// Package claim owns a card for one session, with a lease.
//
// A claim is the card's only owner record: a JSON file per card under <claims>/cards, written through the shared
// lock and readable by every worktree. The lease is `claims.lease_minutes` (two hours by default); a hook renews it
// while the session works, so a crashed session frees its card without anyone guessing. Exit codes: 0 free, mine or
// done; 1 an error; 3 held by another live owner; 4 the claim expired and needs --takeover.
package claim

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/cloudbusting/kanbanzai/internal/config"
	"github.com/cloudbusting/kanbanzai/internal/fsutil"
	"github.com/cloudbusting/kanbanzai/internal/gitutil"
	"github.com/cloudbusting/kanbanzai/internal/timeutil"
)

const (
	OK      = 0
	Error   = 1
	Held    = 3
	Expired = 4
)

var ownerRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.:@/+~-]{0,63}$`)

// ClaimError is a claim that cannot be made, renewed or released.
type ClaimError struct{ Message string }

func (e *ClaimError) Error() string { return e.Message }

func claimErr(format string, args ...any) *ClaimError {
	return &ClaimError{Message: fmt.Sprintf(format, args...)}
}

// parentProcess is the Claude Code process above this shell, when there is one. Tests replace it.
var parentProcess = visibleParentProcess

// Store is one card per file, with the lock around every read-modify-write.
type Store struct {
	Directory string
	Prefix    string
}

// NewStore makes the claims folder and returns its store.
func NewStore(project *config.Project) (*Store, error) {
	if err := os.MkdirAll(filepath.Join(project.Claims, "cards"), 0o700); err != nil {
		return nil, err
	}
	return &Store{Directory: project.Claims, Prefix: project.Prefix}, nil
}

// Lock takes the store's advisory lock.
func (s *Store) Lock() (*fsutil.Lock, error) {
	return fsutil.OpenLock(filepath.Join(s.Directory, "lock"))
}

// Path is one card's claim file.
func (s *Store) Path(card string) string {
	return filepath.Join(s.Directory, "cards", card+".json")
}

// Load reads a card's claim.
func (s *Store) Load(card string) (map[string]any, bool) {
	return fsutil.ReadJSON(s.Path(card))
}

// Records reads every claim in the store.
func (s *Store) Records() map[string]map[string]any {
	found := map[string]map[string]any{}
	paths, _ := filepath.Glob(filepath.Join(s.Directory, "cards", s.Prefix+"-*.json"))
	sort.Strings(paths)
	for _, path := range paths {
		record, ok := fsutil.ReadJSON(path)
		if !ok {
			continue
		}
		card := stringOf(record["card"], strings.TrimSuffix(filepath.Base(path), ".json"))
		found[card] = record
	}
	return found
}

// Save writes a card's claim atomically.
func (s *Store) Save(record map[string]any) error {
	card := stringOf(record["card"], "")
	if card == "" {
		return claimErr("a claim record has no card")
	}
	return fsutil.WriteJSON(s.Path(card), record)
}

// Remove deletes a card's claim.
func (s *Store) Remove(card string) error {
	err := os.Remove(s.Path(card))
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// History appends one event to the claim history.
func (s *Store) History(record map[string]any) error {
	return fsutil.AppendJSONLine(filepath.Join(s.Directory, "history.jsonl"), record)
}

// ReadRecords reads every claim without creating anything.
func ReadRecords(project *config.Project) map[string]map[string]any {
	store := &Store{Directory: project.Claims, Prefix: project.Prefix}
	return store.Records()
}

// Live reports whether a claim has not expired.
func Live(record map[string]any, now time.Time) bool {
	expires, ok := timeutil.Parse(record["expires_at"])
	return ok && expires.After(now)
}

// LeaseLeft is the seconds left on a lease; negative when expired, zero when it cannot be read.
func LeaseLeft(record map[string]any, now time.Time) float64 {
	expires, ok := timeutil.Parse(record["expires_at"])
	if !ok {
		return 0
	}
	return expires.Sub(now).Seconds()
}

// When is a claim timestamp as the command prints it.
func When(value any) string {
	parsed, ok := timeutil.Parse(value)
	if !ok {
		return "unknown"
	}
	return parsed.Format("2006-01-02 15:04 UTC")
}

// Remaining is the human lease left, or how long ago it expired.
func Remaining(record map[string]any, now time.Time) string {
	expires, ok := timeutil.Parse(record["expires_at"])
	if !ok {
		return "unknown"
	}
	seconds := int(expires.Sub(now).Seconds())
	if seconds <= 0 {
		seconds = -seconds
		if seconds >= 3600 {
			return fmt.Sprintf("expired %dh", seconds/3600)
		}
		return fmt.Sprintf("expired %dm", seconds/60)
	}
	hours := seconds / 3600
	return fmt.Sprintf("%dh %dm left", hours, (seconds%3600)/60)
}

// Describe is one claim in one line, from me's point of view.
func Describe(record map[string]any, me string) string {
	card := stringOf(record["card"], "?")
	owner := stringOf(record["owner"], "unknown")
	mine := owner
	if owner == me {
		mine = "you"
	}
	now := timeutil.Now()
	if Live(record, now) {
		return fmt.Sprintf("%s: held by %s since %s, until %s (%s)", card, mine,
			When(record["claimed_at"]), When(record["expires_at"]), Remaining(record, now))
	}
	return fmt.Sprintf("%s: EXPIRED claim by %s since %s (%s); take it over with --takeover <reason>",
		card, mine, When(record["claimed_at"]), Remaining(record, now))
}

// CheckOwner is a label that fits one word: letters, digits and _.:@/+~-.
func CheckOwner(value string) (string, error) {
	if !ownerRe.MatchString(value) {
		return "", claimErr("owner must be one word of letters, digits and _.:@/+~- (no spaces or '|'): '%s'", value)
	}
	return value, nil
}

// CheckCard is an id that looks like the project's W-NN.
func CheckCard(project *config.Project, value string) (string, error) {
	if !project.CardRe.MatchString(value) {
		return "", claimErr("card must look like %s-NN: '%s'", project.Prefix, value)
	}
	return value, nil
}

// BranchName is the branch of the worktree this session is in.
func BranchName(project *config.Project) string {
	branch, err := gitutil.Git(project.Toplevel, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return ""
	}
	return branch
}

// MinutesOf is the lease length for a command, or the configured default.
func MinutesOf(project *config.Project, minutes *float64) (float64, error) {
	value := project.LeaseMinutes
	if minutes != nil {
		value = *minutes
	}
	if value <= 0 {
		return 0, claimErr("--minutes must be greater than zero")
	}
	return value, nil
}

// Lease is the timestamp part of a claim record.
func Lease(now time.Time, minutes float64) map[string]any {
	return map[string]any{
		"renewed_at": timeutil.ToISO(now),
		"expires_at": timeutil.ToISO(now.Add(time.Duration(minutes * float64(time.Minute)))),
		"minutes":    minutes,
	}
}

func updateLease(record map[string]any, now time.Time, minutes float64) {
	for key, value := range Lease(now, minutes) {
		record[key] = value
	}
}

// OwnerLabel is this session's label: the configured variable, KANBANZAI_OWNER, the harness's session id,
// the Claude Code process above this shell (only when the shell has no session id), then user@host.
func OwnerLabel(project *config.Project) string {
	keys := []string{project.OwnerEnv}
	if project.OwnerEnv != "KANBANZAI_OWNER" {
		keys = append(keys, "KANBANZAI_OWNER")
	}
	for _, key := range keys {
		if key == "" {
			continue
		}
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
	}
	if session := strings.TrimSpace(os.Getenv("OPENCODE_SESSION_ID")); session != "" {
		return "opencode:" + session
	}
	for _, key := range []string{"CLAUDE_CODE_SESSION_ID", "CLAUDE_SESSION_ID"} {
		if session := strings.TrimSpace(os.Getenv(key)); session != "" {
			return "claude:" + session
		}
	}
	if parent := parentProcess(); parent != "" {
		return parent
	}
	name := ""
	for _, key := range []string{"LOGNAME", "USER", "LNAME", "USERNAME"} {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			name = value
			break
		}
	}
	if name == "" {
		if current, err := user.Current(); err == nil {
			name = current.Username
		}
	}
	if name == "" {
		name = "unknown"
	}
	host, err := os.Hostname()
	if err != nil || host == "" {
		host = "unknown"
	}
	return name + "@" + host
}

// HookSession takes the Claude Code session id from the hook's JSON on stdin, when the environment has none.
func HookSession(in io.Reader) {
	if strings.TrimSpace(os.Getenv("CLAUDE_CODE_SESSION_ID")) != "" || in == nil {
		return
	}
	if file, ok := in.(*os.File); ok {
		if info, err := file.Stat(); err == nil && info.Mode()&os.ModeCharDevice != 0 {
			return
		}
	}
	data, err := io.ReadAll(in)
	if err != nil {
		return
	}
	payload, ok := readObject(data)
	if !ok {
		return
	}
	session, ok := payload["session_id"].(string)
	if !ok {
		return
	}
	session = strings.TrimSpace(session)
	if session == "" || !ownerRe.MatchString("claude:"+session) {
		return
	}
	_ = os.Setenv("CLAUDE_CODE_SESSION_ID", session)
}

// Options for the claim command.
type ClaimOptions struct {
	Card     string
	Owner    string
	Minutes  *float64
	Note     string
	Branch   *string
	NoMove   bool
	Takeover string
	Start    func(card, owner string) (string, error)
	Out      io.Writer
	Err      io.Writer
}

// Owner prints this session's label and the state folder.
func Owner(project *config.Project, owner string, out io.Writer) int {
	label, err := labelFor(project, owner)
	if err != nil {
		return printErr(out, err)
	}
	fmt.Fprintln(out, label)
	fmt.Fprintf(out, "state: %s\n", project.Claims)
	return OK
}

// Status prints one card's claim, or all claims.
func Status(project *config.Project, owner, card string, out, errOut io.Writer) int {
	me, err := labelFor(project, owner)
	if err != nil {
		return printErr(errOut, err)
	}
	store, err := NewStore(project)
	if err != nil {
		return printErr(errOut, err)
	}
	if card != "" {
		card, err = CheckCard(project, card)
		if err != nil {
			return printErr(errOut, err)
		}
		record, ok := store.Load(card)
		if !ok {
			fmt.Fprintf(out, "%s: free\n", card)
			return OK
		}
		fmt.Fprintln(out, Describe(record, me))
		if Live(record, timeutil.Now()) {
			if stringOf(record["owner"], "") == me {
				return OK
			}
			return Held
		}
		return Expired
	}
	records := store.Records()
	if len(records) == 0 {
		fmt.Fprintln(out, "no claims")
		return OK
	}
	keys := make([]string, 0, len(records))
	for key := range records {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		fmt.Fprintln(out, Describe(records[key], me))
	}
	return OK
}

// Claim takes a card, renews this session's own claim, or takes over with a reason.
func Claim(project *config.Project, opts ClaimOptions) int {
	out, errOut := writers(opts.Out, opts.Err)
	me, err := labelFor(project, opts.Owner)
	if err != nil {
		return printErr(errOut, err)
	}
	card, err := CheckCard(project, opts.Card)
	if err != nil {
		return printErr(errOut, err)
	}
	minutes, err := MinutesOf(project, opts.Minutes)
	if err != nil {
		return printErr(errOut, err)
	}
	now := timeutil.Now()
	store, err := NewStore(project)
	if err != nil {
		return printErr(errOut, err)
	}
	lock, err := store.Lock()
	if err != nil {
		return printErr(errOut, err)
	}
	defer lock.Close()

	record, ok := store.Load(card)
	if ok {
		recordOwner := stringOf(record["owner"], "unknown")
		if recordOwner == me {
			updateLease(record, now, minutes)
			if opts.Note != "" {
				record["note"] = opts.Note
			}
			if opts.NoMove {
				record["step"] = false
			} else {
				delete(record, "step")
			}
			if err := store.Save(record); err != nil {
				return printErr(errOut, err)
			}
			if err := store.History(map[string]any{"at": timeutil.ToISO(now), "card": card, "event": "renew",
				"owner": me}); err != nil {
				return printErr(errOut, err)
			}
			fmt.Fprintf(out, "renewed %s for you until %s (%s)\n", card, When(record["expires_at"]),
				Remaining(record, now))
			if !opts.NoMove {
				start(out, opts.Start, card, me)
			}
			return OK
		}
		if opts.Takeover == "" {
			fmt.Fprintf(errOut, "claim: %s\n", Describe(record, me))
			if Live(record, now) {
				return Held
			}
			return Expired
		}
		if err := store.History(map[string]any{"at": timeutil.ToISO(now), "card": card, "event": "takeover",
			"owner": me, "previous_owner": recordOwner, "reason": opts.Takeover}); err != nil {
			return printErr(errOut, err)
		}
	}
	record = map[string]any{
		"card":       card,
		"owner":      me,
		"claimed_at": timeutil.ToISO(now),
		"branch":     branchOrCurrent(project, opts.Branch),
		"where":      project.Toplevel,
	}
	for key, value := range Lease(now, minutes) {
		record[key] = value
	}
	if opts.Note != "" {
		record["note"] = opts.Note
	}
	if opts.NoMove {
		record["step"] = false
	}
	if err := store.Save(record); err != nil {
		return printErr(errOut, err)
	}
	if err := store.History(map[string]any{"at": timeutil.ToISO(now), "card": card, "event": "claim",
		"owner": me}); err != nil {
		return printErr(errOut, err)
	}
	fmt.Fprintf(out, "claimed %s by %s until %s (%s)\n", card, me, When(record["expires_at"]),
		Remaining(record, now))
	fmt.Fprintf(out, "owner: %s\n", me)
	if !opts.NoMove {
		start(out, opts.Start, card, me)
	}
	return OK
}

// Takeover is the takeover command: a claim with a reason.
func Takeover(project *config.Project, opts ClaimOptions, reason string) int {
	opts.Takeover = reason
	return Claim(project, opts)
}

// Renew extends this session's claim.
func Renew(project *config.Project, owner, card string, minutes *float64, out, errOut io.Writer) int {
	me, err := labelFor(project, owner)
	if err != nil {
		return printErr(errOut, err)
	}
	card, err = CheckCard(project, card)
	if err != nil {
		return printErr(errOut, err)
	}
	length, err := MinutesOf(project, minutes)
	if err != nil {
		return printErr(errOut, err)
	}
	now := timeutil.Now()
	store, err := NewStore(project)
	if err != nil {
		return printErr(errOut, err)
	}
	lock, err := store.Lock()
	if err != nil {
		return printErr(errOut, err)
	}
	defer lock.Close()
	record, ok := store.Load(card)
	if !ok {
		return printErr(errOut, claimErr("%s has no claim to renew", card))
	}
	if stringOf(record["owner"], "") != me {
		fmt.Fprintf(errOut, "claim: %s\n", Describe(record, me))
		if Live(record, now) {
			return Held
		}
		return Expired
	}
	updateLease(record, now, length)
	if err := store.Save(record); err != nil {
		return printErr(errOut, err)
	}
	if err := store.History(map[string]any{"at": timeutil.ToISO(now), "card": card, "event": "renew",
		"owner": me}); err != nil {
		return printErr(errOut, err)
	}
	fmt.Fprintf(out, "renewed %s until %s (%s)\n", card, When(record["expires_at"]), Remaining(record, now))
	return OK
}

// Release gives a card back; anyone may release an expired claim.
func Release(project *config.Project, owner, card string, out, errOut io.Writer) int {
	me, err := labelFor(project, owner)
	if err != nil {
		return printErr(errOut, err)
	}
	card, err = CheckCard(project, card)
	if err != nil {
		return printErr(errOut, err)
	}
	now := timeutil.Now()
	store, err := NewStore(project)
	if err != nil {
		return printErr(errOut, err)
	}
	lock, err := store.Lock()
	if err != nil {
		return printErr(errOut, err)
	}
	defer lock.Close()
	record, ok := store.Load(card)
	if !ok {
		fmt.Fprintf(out, "%s is not claimed\n", card)
		return OK
	}
	recordOwner := stringOf(record["owner"], "unknown")
	if recordOwner != me && Live(record, now) {
		fmt.Fprintf(errOut, "claim: %s\n", Describe(record, me))
		return Held
	}
	event := "release"
	if recordOwner != me {
		event = "release-expired"
	}
	if err := store.Remove(card); err != nil {
		return printErr(errOut, err)
	}
	if err := store.History(map[string]any{"at": timeutil.ToISO(now), "card": card, "event": event,
		"owner": me, "previous_owner": recordOwner}); err != nil {
		return printErr(errOut, err)
	}
	suffix := ""
	if recordOwner != me {
		suffix = fmt.Sprintf(" (expired claim by %s)", recordOwner)
	}
	fmt.Fprintf(out, "released %s%s\n", card, suffix)
	return OK
}

// HeartbeatOptions is what a hook passes to the heartbeat.
type HeartbeatOptions struct {
	Owner   string
	Minutes *float64
	Hook    bool
	In      io.Reader
}

// Heartbeat renews every claim this session holds that has used half its lease, in silence and without failing.
func Heartbeat(project *config.Project, opts HeartbeatOptions) int {
	defer func() { _ = recover() }()
	if opts.Hook {
		HookSession(opts.In)
	}
	me, err := labelFor(project, opts.Owner)
	if err != nil {
		return OK
	}
	minutes := project.LeaseMinutes
	if opts.Minutes != nil {
		minutes = *opts.Minutes
	}
	now := timeutil.Now()
	store, err := NewStore(project)
	if err != nil {
		return OK
	}
	due := make([]string, 0)
	for card, record := range store.Records() {
		if stringOf(record["owner"], "") == me && LeaseLeft(record, now) < minutes*30 {
			due = append(due, card)
		}
	}
	if len(due) == 0 {
		return OK
	}
	lock, err := store.Lock()
	if err != nil {
		return OK
	}
	defer lock.Close()
	for _, card := range due {
		record, ok := store.Load(card)
		if !ok || stringOf(record["owner"], "") != me {
			continue
		}
		updateLease(record, now, minutes)
		_ = store.Save(record)
	}
	return OK
}

func labelFor(project *config.Project, owner string) (string, error) {
	if owner == "" {
		owner = OwnerLabel(project)
	}
	return CheckOwner(owner)
}

func branchOrCurrent(project *config.Project, branch *string) string {
	if branch != nil {
		return *branch
	}
	return BranchName(project)
}

func start(out io.Writer, starter func(card, owner string) (string, error), card, owner string) {
	if starter == nil {
		return
	}
	message, err := starter(card, owner)
	if err != nil {
		fmt.Fprintf(out, "not moved: %v\n", err)
		return
	}
	if message != "" {
		fmt.Fprintln(out, message)
	}
}

func printErr(out io.Writer, err error) int {
	if out == nil {
		out = io.Discard
	}
	fmt.Fprintf(out, "claim: %v\n", err)
	return Error
}

func writers(out, errOut io.Writer) (io.Writer, io.Writer) {
	if out == nil {
		out = io.Discard
	}
	if errOut == nil {
		errOut = io.Discard
	}
	return out, errOut
}

func stringOf(value any, fallback string) string {
	if value == nil {
		return fallback
	}
	if text, ok := value.(string); ok {
		if text == "" {
			return fallback
		}
		return text
	}
	return fmt.Sprint(value)
}

func readObject(data []byte) (map[string]any, bool) {
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return nil, false
	}
	object, ok := value.(map[string]any)
	return object, ok
}
