package initcmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cloudbusting/kanbanzai/internal/fsutil"
)

// Applied is what happened to one action.
type Applied struct {
	Op     string `json:"op"`
	Path   string `json:"path"`
	Status string `json:"status"`
}

// Result is what Apply did, in plan order.
type Result struct {
	Applied []Applied `json:"applied"`
	Error   string    `json:"error,omitempty"`
}

// StaleError means the tree no longer matches the plan; nothing was written. It exits 2.
type StaleError struct{ Message string }

func (e *StaleError) Error() string { return e.Message }

func stale(format string, args ...any) error {
	return &StaleError{Message: fmt.Sprintf(format, args...)}
}

// abs turns a validated plan path into a native path under root.
func abs(root, rel string) string { return filepath.Join(root, filepath.FromSlash(rel)) }

func fileDigest(path string) (string, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	return digest(string(data)), true
}

// Preflight checks every action against the tree without writing. A plan that fails is stale (or hostile) and is
// refused whole, so nothing is half applied for a reason known in advance.
func Preflight(plan *Plan) error {
	for _, action := range plan.Actions {
		path := abs(plan.Root, action.Path)
		info, err := os.Lstat(path)
		exists := err == nil
		switch action.Op {
		case OpMkdir:
			if exists && !info.IsDir() {
				return stale("%s exists and is not a folder: run init --dry-run again", action.Path)
			}
		case OpCreate:
			if exists {
				want := action.SHA256
				if action.Content == "" {
					want = digest("")
				}
				if sum, ok := fileDigest(path); !ok || sum != want {
					return stale("%s now exists with other content: run init --dry-run again", action.Path)
				}
			}
		case OpAppend, OpReplace:
			sum, ok := fileDigest(path)
			if !ok || sum != action.ExpectSHA256 {
				return stale("%s changed since the plan was made: run init --dry-run again", action.Path)
			}
		case OpBackup:
			if exists {
				return stale("%s already exists: run init --dry-run again", action.Path)
			}
			if sum, ok := fileDigest(abs(plan.Root, ConfigRel)); !ok || sum != action.SHA256 {
				return stale("%s changed since the plan was made: run init --dry-run again", ConfigRel)
			}
		case OpKeep:
		}
	}
	return nil
}

// Apply executes the plan, in order. After runs after each action (tests use it to stop part-way). A failure
// returns the result so far; the order of the plan puts the config last, so a re-run completes the job.
func Apply(plan *Plan, after func(index int) error) (*Result, error) {
	if err := Preflight(plan); err != nil {
		return nil, err
	}
	result := &Result{Applied: []Applied{}}
	for index, action := range plan.Actions {
		status, err := execute(plan.Root, action)
		if err != nil {
			result.Error = err.Error()
			return result, err
		}
		result.Applied = append(result.Applied, Applied{Op: action.Op, Path: action.Path, Status: status})
		if after != nil {
			if err := after(index); err != nil {
				result.Error = err.Error()
				return result, err
			}
		}
	}
	return result, nil
}

func execute(root string, action Action) (string, error) {
	path := abs(root, action.Path)
	fail := func(err error) (string, error) { return "", fmt.Errorf("%s %s: %v", action.Op, action.Path, err) }
	switch action.Op {
	case OpKeep:
		return "kept", nil
	case OpMkdir:
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			return "kept", nil
		}
		if err := os.MkdirAll(path, 0o755); err != nil {
			return fail(err)
		}
		return "made", nil
	case OpCreate:
		if _, err := os.Lstat(path); err == nil {
			return "kept", nil
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return fail(err)
		}
		if err := fsutil.WriteFileAtomic(path, []byte(action.Content), 0o644); err != nil {
			return fail(err)
		}
		return "made", nil
	case OpAppend:
		existing, err := os.ReadFile(path)
		if err != nil {
			return fail(err)
		}
		text := string(existing)
		if text != "" && !strings.HasSuffix(text, "\n") {
			text += "\n"
		}
		if err := fsutil.WriteFileAtomic(path, []byte(text+action.Content), 0o644); err != nil {
			return fail(err)
		}
		return "appended", nil
	case OpReplace:
		if err := fsutil.WriteFileAtomic(path, []byte(action.Content), 0o644); err != nil {
			return fail(err)
		}
		return "replaced", nil
	case OpBackup:
		file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if err != nil {
			return fail(err)
		}
		_, werr := file.WriteString(action.Content)
		if serr := file.Sync(); werr == nil {
			werr = serr
		}
		if cerr := file.Close(); werr == nil {
			werr = cerr
		}
		if werr != nil {
			_ = os.Remove(path)
			return fail(werr)
		}
		return "backed up", nil
	}
	return fail(fmt.Errorf("unknown operation"))
}
