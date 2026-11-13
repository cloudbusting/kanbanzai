package flow

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/cloudbusting/kanbanzai/internal/config"
)

// maxPlanBytes bounds a plan file; it holds the new config and one line per migrated card.
const maxPlanBytes = 8 << 20

// Refusal is a plan, or a state of the board, apply will not work with. It exits 2 and says why.
type Refusal struct{ Message string }

func (r *Refusal) Error() string { return r.Message }

func refuse(format string, args ...any) error { return &Refusal{Message: fmt.Sprintf(format, args...)} }

// JSON is the plan as it is written to a file.
func (plan *Plan) JSON() ([]byte, error) {
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(plan); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// LoadPlan reads a plan from a file. The file is untrusted, so it is size-limited and parsed strictly;
// ValidatePlan then confines what it may do.
func LoadPlan(file string) (*Plan, error) {
	handle, err := os.Open(file)
	if err != nil {
		return nil, refuse("--plan: %v", err)
	}
	defer handle.Close()
	data, err := io.ReadAll(io.LimitReader(handle, maxPlanBytes+1))
	if err != nil {
		return nil, refuse("--plan: %v", err)
	}
	if len(data) > maxPlanBytes {
		return nil, refuse("--plan: %s is larger than %d bytes", file, maxPlanBytes)
	}
	plan := &Plan{}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(plan); err != nil {
		return nil, refuse("--plan: %s: %v", file, err)
	}
	if decoder.More() {
		return nil, refuse("--plan: %s has trailing data", file)
	}
	return plan, nil
}

// ValidatePlan proves a plan is one BuildPlan could have made for this project: the right format and root, no
// blocker, paths of the right shape inside the root with no symlink on the way, texts that match their digests.
// It does not look at the tree's current state (Preflight does).
func ValidatePlan(plan *Plan, p *config.Project) error {
	if plan.Kind != PlanKind || plan.SchemaVersion != PlanVersion {
		return refuse("--plan: not a %s version %d plan", PlanKind, PlanVersion)
	}
	if !samePath(plan.Root, p.Root) {
		return refuse("--plan: it is for %s, not this project (%s)", plan.Root, p.Root)
	}
	if len(plan.Blockers) > 0 {
		return refuse("--plan: the plan has %d blocker(s); fix them and plan again:\n  %s", len(plan.Blockers), strings.Join(plan.Blockers, "\n  "))
	}
	if plan.Config.Path != slash(p.Root, ConfigPath(p)) {
		return refuse("--plan: the config is %s, not %s", slash(p.Root, ConfigPath(p)), plan.Config.Path)
	}
	if err := confined(p.Root, plan.Config.Path); err != nil {
		return refuse("--plan: %s: %v", plan.Config.Path, err)
	}
	if Digest(plan.Config.Content) != plan.Config.AfterSHA256 {
		return refuse("--plan: the config text does not match its digest")
	}
	if _, err := config.ValidateText(p.Root, p.Toplevel, p.GitCommon, plan.Config.Content); err != nil {
		return refuse("--plan: the new config cannot be loaded: %v", err)
	}
	work := slash(p.Root, p.Work)
	for _, folder := range plan.Folders {
		if !within(work, folder) {
			return refuse("--plan: folder %s is outside the cards folder %s", folder, work)
		}
		if err := confined(p.Root, folder); err != nil {
			return refuse("--plan: %s: %v", folder, err)
		}
	}
	seen := map[string]bool{}
	for _, c := range plan.Cards {
		if !p.CardRe.MatchString(c.ID) || seen[c.ID] {
			return refuse("--plan: card %q is not a card id, or is listed twice", c.ID)
		}
		seen[c.ID] = true
		for _, rel := range []string{c.Path, c.NewPath} {
			if !within(work, rel) || !p.CardFileRe.MatchString(path.Base(rel)) {
				return refuse("--plan: %s is not a brief inside the cards folder %s", rel, work)
			}
			if err := confined(p.Root, rel); err != nil {
				return refuse("--plan: %s: %v", rel, err)
			}
		}
		if path.Base(c.Path) != path.Base(c.NewPath) {
			return refuse("--plan: %s would be renamed; a migration only changes a brief's folder", c.Path)
		}
		if c.ExpectSHA256 == "" || c.AfterSHA256 == "" || c.StageAfter == "" || c.Updated == "" || c.Progress == "" {
			return refuse("--plan: card %s is incomplete", c.ID)
		}
		if strings.ContainsAny(c.StageAfter+c.Updated+c.Progress, "\r\n") {
			return refuse("--plan: card %s has a line break in a single-line field", c.ID)
		}
	}
	allowed := map[string]bool{}
	if !p.TemplateIsShipped {
		allowed[slash(p.Root, p.Template)] = true
	}
	for _, item := range p.Types {
		if item.Template != "" {
			allowed[slash(p.Root, item.Template)] = true
		}
	}
	paths := map[string]bool{}
	for _, template := range plan.AllTemplates() {
		if !allowed[template.Path] || paths[template.Path] {
			return refuse("--plan: the template change %s is not for one of this project's templates, or is listed twice", template.Path)
		}
		paths[template.Path] = true
		if err := confined(p.Root, template.Path); err != nil {
			return refuse("--plan: %s: %v", template.Path, err)
		}
		if Digest(template.Content) != template.AfterSHA256 {
			return refuse("--plan: the template text for %s does not match its digest", template.Path)
		}
	}
	return nil
}

func within(dir, rel string) bool {
	return rel != dir && strings.HasPrefix(rel, dir+"/") && path.Clean(rel) == rel
}

func samePath(a, b string) bool {
	if a == b {
		return true
	}
	ra, ea := filepath.EvalSymlinks(a)
	rb, eb := filepath.EvalSymlinks(b)
	return ea == nil && eb == nil && ra == rb
}

// confined rejects a plan path that is absolute, climbs out, names a drive or device, or passes through a
// symlink (an existing component that is a link).
func confined(root, rel string) error {
	if rel == "" || strings.ContainsAny(rel, "\\:\x00") || strings.HasPrefix(rel, "/") {
		return fmt.Errorf("the path must be relative and use forward slashes")
	}
	clean := path.Clean(rel)
	if clean != rel || clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return fmt.Errorf("the path must be clean and stay inside the project")
	}
	current := root
	for _, part := range strings.Split(rel, "/") {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err != nil {
			break
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("%s is a symbolic link", strings.TrimPrefix(current, root+string(filepath.Separator)))
		}
	}
	return nil
}
