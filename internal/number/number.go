// Package number allocates the next card number, so two sessions never choose the same one.
//
// A number is the highest already in the project or the ledger, plus one. Every issued number stays in the ledger
// for good, so closing a card does not bring the number back for a different item. The briefs' file names in every
// worktree are scanned too, so numbers written before the tool, or on another branch, stay taken.
//
// State lives in the project's numbers folder, shared by every worktree and never in git. Exit codes: 0 a number or
// a clean command; 1 an error; 2 another prefix was named.
package number

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/cloudbusting/kanbanzai/internal/claim"
	"github.com/cloudbusting/kanbanzai/internal/config"
	"github.com/cloudbusting/kanbanzai/internal/fsutil"
	"github.com/cloudbusting/kanbanzai/internal/gitutil"
	"github.com/cloudbusting/kanbanzai/internal/timeutil"
)

// NumberError is a number that cannot be allocated, released or marked.
type NumberError struct{ Message string }

func (e *NumberError) Error() string { return e.Message }

func numberErr(format string, args ...any) *NumberError {
	return &NumberError{Message: fmt.Sprintf(format, args...)}
}

// StateDir is the project's numbers folder, made when missing.
func StateDir(project *config.Project) (string, error) {
	if err := os.MkdirAll(project.Numbers, 0o700); err != nil {
		return "", err
	}
	return project.Numbers, nil
}

// StatePath is the project's ledger file.
func StatePath(project *config.Project) (string, error) {
	dir, err := StateDir(project)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, project.Prefix+".json"), nil
}

func lock(project *config.Project) (*fsutil.Lock, error) {
	dir, err := StateDir(project)
	if err != nil {
		return nil, err
	}
	return fsutil.OpenLock(filepath.Join(dir, project.Prefix+".lock"))
}

// Roots is the main checkout and every worktree of it.
func Roots(project *config.Project) []string {
	found := []string{project.Root}
	listing, err := gitutil.GitRaw(project.Root, "worktree", "list", "--porcelain")
	if err != nil {
		return found
	}
	for _, line := range strings.Split(listing, "\n") {
		if !strings.HasPrefix(line, "worktree ") {
			continue
		}
		path := strings.TrimSpace(strings.TrimPrefix(line, "worktree "))
		resolved := absolutePath(path)
		info, err := os.Stat(resolved)
		if err != nil || !info.IsDir() {
			continue
		}
		if !contains(found, resolved) {
			found = append(found, resolved)
		}
	}
	return found
}

// RepoTaken is every number already recorded in any worktree: only a brief's file name counts.
func RepoTaken(project *config.Project) map[int]bool {
	found := map[int]bool{}
	pattern := regexp.MustCompile(`^` + regexp.QuoteMeta(project.Prefix) + `-(\d{` +
		strconv.Itoa(project.Digits) + `,})-.+\.md$`)
	inside, err := filepath.Rel(project.Root, project.Work)
	external := err != nil || inside == ".." || strings.HasPrefix(inside, ".."+string(filepath.Separator))
	for _, root := range Roots(project) {
		briefs := project.Work
		if !external {
			briefs = filepath.Join(root, inside)
		}
		if info, err := os.Stat(briefs); err != nil || !info.IsDir() {
			continue
		}
		_ = filepath.WalkDir(briefs, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil || entry.IsDir() {
				return nil
			}
			match := pattern.FindStringSubmatch(entry.Name())
			if match == nil {
				return nil
			}
			value, err := strconv.Atoi(match[1])
			if err == nil {
				found[value] = true
			}
			return nil
		})
	}
	return found
}

// Load is the ledger: every number ever issued, and the reservations still worth showing.
func Load(project *config.Project) ([]int, []map[string]any) {
	path, err := StatePath(project)
	if err != nil {
		return nil, nil
	}
	data, ok := fsutil.ReadJSON(path)
	if !ok {
		return nil, nil
	}
	issued := []int{}
	if items, ok := data["issued"].([]any); ok {
		for _, item := range items {
			if value, ok := numberOf(item); ok {
				issued = append(issued, value)
			}
		}
	}
	reserved := []map[string]any{}
	if items, ok := data["reserved"].([]any); ok {
		for _, item := range items {
			record, ok := item.(map[string]any)
			if !ok {
				continue
			}
			value, ok := numberOf(record["number"])
			if !ok {
				continue
			}
			record["number"] = value
			reserved = append(reserved, record)
		}
	}
	return issued, reserved
}

func save(project *config.Project, issued []int, reserved []map[string]any) error {
	path, err := StatePath(project)
	if err != nil {
		return err
	}
	sorted := uniqueSorted(issued)
	return fsutil.WriteJSON(path, map[string]any{"issued": sorted, "reserved": reserved})
}

// StillReserved is the reservations whose number has not landed on the board yet.
func StillReserved(records []map[string]any, taken map[int]bool) []map[string]any {
	out := make([]map[string]any, 0, len(records))
	for _, record := range records {
		value, ok := numberOf(record["number"])
		if ok && !taken[value] {
			out = append(out, record)
		}
	}
	return out
}

// NextNumber is the highest taken, issued or reserved number, plus one.
func NextNumber(taken map[int]bool, issued []int, reserved []map[string]any) int {
	next := 0
	for value := range taken {
		if value > next {
			next = value
		}
	}
	for _, value := range issued {
		if value > next {
			next = value
		}
	}
	for _, record := range reserved {
		if value, ok := numberOf(record["number"]); ok && value > next {
			next = value
		}
	}
	return next + 1
}

// ParseItem is a card id as a number, or an error naming what it was for.
func ParseItem(project *config.Project, item, what string) (int, error) {
	pattern := regexp.MustCompile(`(?i)^` + regexp.QuoteMeta(project.Prefix) + `-(\d+)$`)
	match := pattern.FindStringSubmatch(strings.TrimSpace(item))
	if match == nil {
		return 0, numberErr("not a number to %s: '%s' (like %s)", what, item, project.FormatID(45))
	}
	value, err := strconv.Atoi(match[1])
	if err != nil {
		return 0, numberErr("not a number to %s: '%s' (like %s)", what, item, project.FormatID(45))
	}
	return value, nil
}

// Allocate is the next free number, issued to this session unless peeking.
func Allocate(project *config.Project, peek bool, note string) (string, error) {
	now := timeutil.Now()
	lock, err := lock(project)
	if err != nil {
		return "", err
	}
	defer lock.Close()
	taken := RepoTaken(project)
	issued, reserved := Load(project)
	reserved = StillReserved(reserved, taken)
	value := NextNumber(taken, issued, reserved)
	item := project.FormatID(value)
	if !peek {
		issued = append(issued, value)
		reserved = append(reserved, map[string]any{
			"number": value, "item": item, "owner": claim.OwnerLabel(project), "at": timeutil.ToISO(now),
			"note": note,
		})
		if err := save(project, issued, reserved); err != nil {
			return "", err
		}
	}
	return item, nil
}

// Release gives an abandoned number back.
func Release(project *config.Project, item string) (string, error) {
	value, err := ParseItem(project, item, "release")
	if err != nil {
		return "", err
	}
	lock, err := lock(project)
	if err != nil {
		return "", err
	}
	defer lock.Close()
	issued, reserved := Load(project)
	keptIssued := make([]int, 0, len(issued))
	for _, number := range issued {
		if number != value {
			keptIssued = append(keptIssued, number)
		}
	}
	keptReserved := make([]map[string]any, 0, len(reserved))
	for _, record := range reserved {
		if number, ok := numberOf(record["number"]); !ok || number != value {
			keptReserved = append(keptReserved, record)
		}
	}
	if err := save(project, keptIssued, keptReserved); err != nil {
		return "", err
	}
	return project.FormatID(value), nil
}

// Mark records a number chosen by hand, so it is never handed to another item.
func Mark(project *config.Project, item string) (string, error) {
	value, err := ParseItem(project, item, "mark")
	if err != nil {
		return "", err
	}
	lock, err := lock(project)
	if err != nil {
		return "", err
	}
	defer lock.Close()
	issued, reserved := Load(project)
	found := false
	for _, number := range issued {
		if number == value {
			found = true
			break
		}
	}
	if !found {
		issued = append(issued, value)
	}
	if err := save(project, issued, reserved); err != nil {
		return "", err
	}
	return project.FormatID(value), nil
}

// Status prints the ledger, the reservations and the next free number.
func Status(project *config.Project, out io.Writer) error {
	dir, err := StateDir(project)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "state: %s\n", dir)
	lock, err := lock(project)
	if err != nil {
		return err
	}
	defer lock.Close()
	taken := RepoTaken(project)
	issued, reserved := Load(project)
	reserved = StillReserved(reserved, taken)
	if err := save(project, issued, reserved); err != nil {
		return err
	}
	next := NextNumber(taken, issued, reserved)
	fmt.Fprintf(out, "%s: next %s; %d issued\n", project.Prefix, project.FormatID(next), len(issued))
	sort.SliceStable(reserved, func(i, j int) bool {
		left, _ := numberOf(reserved[i]["number"])
		right, _ := numberOf(reserved[j]["number"])
		return left < right
	})
	for _, record := range reserved {
		item := recordString(record, "item", "")
		if item == "" {
			value, _ := numberOf(record["number"])
			item = strconv.Itoa(value)
		}
		fmt.Fprintf(out, "  %-6s %-32s %s  %s\n", item, recordString(record, "owner", "?"),
			recordString(record, "at", ""), recordString(record, "note", ""))
	}
	return nil
}

// Options is one number command.
type Options struct {
	Prefix  string
	Peek    bool
	Release string
	Mark    string
	Status  bool
	Note    string
	Out     io.Writer
	Err     io.Writer
}

// Run is the number command.
func Run(project *config.Project, opts Options) int {
	out, errOut := writers(opts.Out, opts.Err)
	if opts.Status {
		if err := Status(project, out); err != nil {
			fmt.Fprintf(errOut, "number: %v\n", err)
			return 1
		}
		return 0
	}
	if opts.Release != "" {
		item, err := Release(project, opts.Release)
		if err != nil {
			fmt.Fprintf(errOut, "number: %v\n", err)
			return 1
		}
		fmt.Fprintln(out, item)
		return 0
	}
	if opts.Mark != "" {
		item, err := Mark(project, opts.Mark)
		if err != nil {
			fmt.Fprintf(errOut, "number: %v\n", err)
			return 1
		}
		fmt.Fprintln(out, item)
		return 0
	}
	if opts.Prefix != "" && !strings.EqualFold(opts.Prefix, project.Prefix) {
		fmt.Fprintf(errOut, "number: this project's numbers are %s-NN, not '%s'\n", project.Prefix, opts.Prefix)
		return 2
	}
	item, err := Allocate(project, opts.Peek, opts.Note)
	if err != nil {
		fmt.Fprintf(errOut, "number: %v\n", err)
		return 1
	}
	fmt.Fprintln(out, item)
	return 0
}

func numberOf(value any) (int, bool) {
	switch typed := value.(type) {
	case int:
		return typed, true
	case int64:
		return int(typed), true
	case float64:
		if typed == math.Trunc(typed) {
			return int(typed), true
		}
	case json.Number:
		parsed, err := typed.Int64()
		if err == nil {
			return int(parsed), true
		}
	}
	return 0, false
}

func uniqueSorted(values []int) []int {
	seen := map[int]bool{}
	out := make([]int, 0, len(values))
	for _, value := range values {
		if !seen[value] {
			seen[value] = true
			out = append(out, value)
		}
	}
	sort.Ints(out)
	return out
}

func recordString(record map[string]any, key, fallback string) string {
	value, ok := record[key]
	if !ok || value == nil {
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

func absolutePath(path string) string {
	value, err := filepath.Abs(path)
	if err != nil {
		return filepath.Clean(path)
	}
	if resolved, err := filepath.EvalSymlinks(value); err == nil {
		return resolved
	}
	return filepath.Clean(value)
}

func contains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
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
