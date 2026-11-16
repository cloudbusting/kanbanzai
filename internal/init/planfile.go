package initcmd

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

// maxPlanBytes bounds a plan file; a real plan is a few kilobytes.
const maxPlanBytes = 1 << 20

// LoadPlan reads a plan from a file: either a bare plan or the report `init --dry-run --json` printed. The file
// is untrusted, so it is size-limited and parsed strictly; ValidatePlan then confines what it may do.
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
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(data, &probe); err != nil {
		return nil, refuse("--plan: %s is not a JSON object: %v", file, err)
	}
	if inner, ok := probe["plan"]; ok {
		data = inner
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

// ValidatePlan proves a plan is one init could have made for this checkout: right format and root, only known
// operations on paths of the right shape, all inside the root with no symlink on the way, content that matches
// its digest, and a config the loader reads. It does not look at the tree's current state (Preflight does).
func ValidatePlan(plan *Plan, d *Detected) error {
	if plan.Kind != PlanKind || plan.SchemaVersion != PlanVersion {
		return refuse("--plan: not a %s version %d plan", PlanKind, PlanVersion)
	}
	if plan.Root == "" || !samePath(plan.Root, d.Root) {
		return refuse("--plan: the plan is for %s, not this project (%s)", plan.Root, d.Root)
	}
	plan.Root = d.Root
	sawConfig := false
	bundled := len(plan.BundledPlugins) > 0
	bundledActions := map[string]bool{}
	for _, action := range plan.Actions {
		if action.What == WhatPluginReference {
			bundled = true
		}
		bundledActions[action.Path] = true
	}
	seenPlugins := map[string]bool{}
	for _, name := range plan.BundledPlugins {
		if seenPlugins[name] || name != "claude" && name != "opencode" {
			return refuse("--plan: invalid bundled plugin selection")
		}
		seenPlugins[name] = true
		if name == "claude" {
			source, err := bundledPluginSource()
			if err != nil || !bundledActions[".claude/.gitignore"] || !bundledActions[claudeLocalSettingsRel] {
				return refuse("--plan: bundled Claude settings are missing")
			}
			if ok, reason := bundledClaudeEligible(d.Root, source); !ok {
				return refuse("--plan: %s", reason)
			}
		} else if !bundledActions[pluginReferenceRel] || !bundledActions[openCodeSettingsRel] {
			return refuse("--plan: bundled OpenCode settings are missing")
		}
	}
	for index, action := range plan.Actions {
		bad := func(format string, args ...any) error {
			return refuse("--plan: action %d (%s %s): %s", index+1, action.Op, action.Path, fmt.Sprintf(format, args...))
		}
		if err := confined(d.Root, action.Path); err != nil {
			return bad("%v", err)
		}
		if len(action.Content) > 256<<10 {
			return bad("content is too large")
		}
		if action.Content != "" && digest(action.Content) != action.SHA256 {
			return bad("sha256 does not match the content")
		}
		base := path.Base(action.Path)
		switch action.Op {
		case OpMkdir, OpKeep, OpCreate, OpAppend, OpBackup, OpReplace, OpMerge:
		default:
			return bad("unknown operation")
		}
		switch action.What {
		case WhatFolder, WhatBatches:
			if action.Content != "" || action.Op != OpMkdir && action.Op != OpKeep {
				return bad("a folder holds no content")
			}
		case WhatGitkeep:
			if base != ".gitkeep" || action.Content != "" || action.Op != OpCreate && action.Op != OpKeep {
				return bad("a gitkeep is an empty .gitkeep file")
			}
		case WhatGitignore:
			if base != ".gitignore" || action.Op != OpCreate && action.Op != OpAppend && action.Op != OpKeep {
				return bad("an ignore entry is written to a .gitignore only")
			}
		case WhatTemplate:
			if action.Op != OpCreate && action.Op != OpKeep {
				return bad("a template is only ever created")
			}
		case WhatConfig:
			if action.Path != ConfigRel || action.Op != OpCreate && action.Op != OpKeep && action.Op != OpReplace {
				return bad("the config is %s and nothing else", ConfigRel)
			}
			sawConfig = true
			if action.Op != OpKeep {
				if _, err := config.ValidateText(d.Root, d.Toplevel, d.GitCommon, action.Content); err != nil {
					return bad("the config would not load: %v", err)
				}
			}
		case WhatPlugin:
			if err := validPluginAction(action); err != nil {
				return bad("%v", err)
			}
			if bundled && action.Path == claudeLocalSettingsRel && action.Op == OpMerge {
				fragment, _ := parseObject([]byte(action.Content))
				if markets, ok := fragment["extraKnownMarketplaces"].(map[string]any); ok {
					if market, ok := markets["kanbanzai"].(map[string]any); ok {
						if local, ok := market["source"].(map[string]any); ok {
							if path, ok := local["path"].(string); ok {
								source, err := bundledPluginSource()
								if err != nil || path != source {
									return bad("bundled Claude source differs from this command's installed bundle")
								}
							}
						}
					}
				}
			}
		case WhatPluginReference:
			source, err := bundledPluginSource()
			if err != nil || action.Path != pluginReferenceRel {
				return bad("invalid bundled plugin reference")
			}
			expected := filepath.Join(source, "opencode")
			if action.Op == OpCreate && action.Content != expected ||
				action.Op == OpKeep && (action.Content != "" || action.ExpectSHA256 != digest(expected)) ||
				action.Op != OpCreate && action.Op != OpKeep {
				return bad("invalid bundled plugin reference content")
			}
		case WhatBackup:
			if action.Op != OpBackup || !strings.HasPrefix(action.Path, ConfigRel+".bak-") {
				return bad("a backup is a %s.bak-* file", ConfigRel)
			}
		default:
			return bad("unknown kind")
		}
		if (action.Op == OpCreate || action.Op == OpAppend || action.Op == OpReplace || action.Op == OpBackup ||
			action.Op == OpMerge) &&
			action.Content == "" && action.What != WhatGitkeep {
			return bad("a write needs content")
		}
		if (action.Op == OpAppend || action.Op == OpReplace) && action.ExpectSHA256 == "" {
			return bad("expect_sha256 is missing")
		}
	}
	if !sawConfig {
		return refuse("--plan: the plan has no config action")
	}
	return nil
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
