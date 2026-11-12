package initcmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/cloudbusting/kanbanzai/internal/fsutil"
)

// W-17: `init --plugins` puts the project's Claude Code and OpenCode plugin settings in the plan. Each is a `merge`
// action that carries only the JSON the file lacks, guarded by a digest of the file it merges into; a file that
// is not plain JSON is left alone and the fragment to add by hand goes in the warnings.

const (
	// PluginsGitHubRepo is the marketplace Claude Code fetches the plugin from when no --plugin-source is given.
	PluginsGitHubRepo = "cloudbusting/kanbanzai"

	claudeSettingsRel      = ".claude/settings.json"
	claudeLocalSettingsRel = ".claude/settings.local.json"
	openCodeSettingsRel    = ".opencode/opencode.json"

	pluginKey = "kanbanzai@kanbanzai"
)

// pluginFiles are the only paths a merge may write.
var pluginFiles = map[string]bool{claudeSettingsRel: true, claudeLocalSettingsRel: true, openCodeSettingsRel: true}

// checkPlugins validates --plugins and --plugin-source. A source is an absolute path to a Kanbanzai checkout that
// holds the plugin of every harness asked for.
func checkPlugins(opts Options) (agents []string, source string, err error) {
	seen := map[string]bool{}
	for _, name := range opts.Plugins {
		if name != "claude" && name != "opencode" {
			return nil, "", refuse("--plugins '%s': name claude, opencode or both (claude,opencode)", name)
		}
		if seen[name] {
			return nil, "", refuse("--plugins names %s twice", name)
		}
		seen[name] = true
		agents = append(agents, name)
	}
	if opts.PluginSource == "" {
		return agents, "", nil
	}
	if len(agents) == 0 {
		return nil, "", refuse("--plugin-source needs --plugins")
	}
	if !filepath.IsAbs(opts.PluginSource) {
		return nil, "", refuse("--plugin-source '%s': give the absolute path of a Kanbanzai checkout", opts.PluginSource)
	}
	need := map[string]string{"claude": filepath.Join(".claude-plugin", "plugin.json"),
		"opencode": filepath.Join("opencode", "server.js")}
	for _, name := range agents {
		if info, statErr := os.Stat(filepath.Join(opts.PluginSource, need[name])); statErr != nil || !info.Mode().IsRegular() {
			return nil, "", refuse("--plugin-source %s is not a Kanbanzai checkout for %s (no %s)", opts.PluginSource,
				name, filepath.ToSlash(need[name]))
		}
	}
	return agents, filepath.Clean(opts.PluginSource), nil
}

// pluginActions makes the plan's plugin actions and the warnings that go with them.
func pluginActions(root string, agents []string, source string) ([]Action, []string, error) {
	var actions []Action
	var warnings []string
	add := func(rel string, want map[string]any, note string) error {
		action, warn, err := mergeAction(root, rel, want)
		if err != nil {
			return err
		}
		if action != nil {
			actions = append(actions, *action)
		}
		warnings = append(warnings, warn...)
		if note != "" && action != nil && action.Op == OpMerge {
			warnings = append(warnings, note)
		}
		return nil
	}
	for _, name := range agents {
		switch name {
		case "claude":
			rel, marketplace := claudeSettingsRel, map[string]any{"source": "github", "repo": PluginsGitHubRepo}
			note := claudeSettingsRel + " is committed with the project: every teammate's Claude Code will offer the " +
				"Kanbanzai plugin from github.com/" + PluginsGitHubRepo
			if source != "" {
				rel, marketplace = claudeLocalSettingsRel, map[string]any{"source": "directory", "path": source}
				note = claudeLocalSettingsRel + " holds a machine path (your Kanbanzai checkout): it is yours alone, do not commit it"
			}
			want := map[string]any{
				"extraKnownMarketplaces": map[string]any{"kanbanzai": map[string]any{"source": marketplace}},
				"enabledPlugins":         map[string]any{pluginKey: true},
			}
			if err := add(rel, want, note); err != nil {
				return nil, nil, err
			}
		case "opencode":
			if source == "" {
				warnings = append(warnings, "the OpenCode plugin is not published as a package yet: pass --plugin-source "+
					"<a Kanbanzai checkout> to add it to "+openCodeSettingsRel)
				continue
			}
			want := map[string]any{"plugins": []any{filepath.Join(source, "opencode")}}
			note := openCodeSettingsRel + " will hold a machine path (your Kanbanzai checkout): do not commit it as it is"
			if err := add(openCodeSettingsRel, want, note); err != nil {
				return nil, nil, err
			}
		}
	}
	return actions, warnings, nil
}

// mergeAction compares what a settings file should hold with what it holds. It returns nil (and a warning) when
// the file cannot be merged safely, a keep when nothing is missing, and a merge of just the missing JSON otherwise.
func mergeAction(root, rel string, want map[string]any) (*Action, []string, error) {
	abs := filepath.Join(root, filepath.FromSlash(rel))
	info, err := os.Lstat(abs)
	var existing map[string]any
	expect := ""
	switch {
	case err != nil && !os.IsNotExist(err):
		return nil, nil, fmt.Errorf("%s: %v", rel, err)
	case err == nil && !info.Mode().IsRegular():
		return nil, []string{fmt.Sprintf("%s is not a regular file: init left it alone; add this by hand: %s", rel,
			compactJSON(want))}, nil
	case err == nil:
		data, readErr := os.ReadFile(abs)
		if readErr != nil {
			return nil, nil, fmt.Errorf("%s: %v", rel, readErr)
		}
		parsed, parseErr := parseObject(data)
		if parseErr != nil {
			return nil, []string{fmt.Sprintf("%s is not plain JSON (%v): init left it alone; add this by hand: %s", rel,
				parseErr, compactJSON(want))}, nil
		}
		existing, expect = parsed, digest(string(data))
	}
	missing, conflicts := missingJSON(existing, want, "")
	warnings := []string{}
	for _, conflict := range conflicts {
		warnings = append(warnings, rel+": "+conflict)
	}
	disabled := false
	for _, conflict := range conflicts {
		disabled = disabled || strings.Contains(conflict, "plugin is disabled")
	}
	if len(missing) == 0 || disabled { // a plugin the owner switched off is left exactly as they left it
		return &Action{Op: OpKeep, Path: rel, What: WhatPlugin}, warnings, nil
	}
	text, err := json.MarshalIndent(missing, "", "  ")
	if err != nil {
		return nil, nil, err
	}
	content := string(text) + "\n"
	return &Action{Op: OpMerge, Path: rel, What: WhatPlugin, Content: content, SHA256: digest(content),
		ExpectSHA256: expect}, warnings, nil
}

// parseObject reads a JSON object, keeping numbers as written. Anything else (comments, trailing commas, an array,
// trailing text) is an error.
func parseObject(data []byte) (map[string]any, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var value map[string]any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	if value == nil {
		return nil, fmt.Errorf("it is not a JSON object")
	}
	if decoder.More() {
		return nil, fmt.Errorf("it has trailing data")
	}
	return value, nil
}

func compactJSON(value any) string {
	data, _ := json.Marshal(value)
	return string(data)
}

// missingJSON is the part of want that existing lacks, and a sentence for each place where existing holds
// something different (which init never overwrites). Arrays are compared element by element, as a union.
func missingJSON(existing, want map[string]any, prefix string) (map[string]any, []string) {
	missing := map[string]any{}
	var conflicts []string
	keys := make([]string, 0, len(want))
	for key := range want {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		wanted, here := want[key], existing[key]
		path := prefix + key
		if _, present := existing[key]; !present {
			missing[key] = wanted
			continue
		}
		switch w := wanted.(type) {
		case map[string]any:
			have, ok := here.(map[string]any)
			if !ok {
				conflicts = append(conflicts, fmt.Sprintf("%s is already set to something that is not an object: left as is", path))
				continue
			}
			sub, subConflicts := missingJSON(have, w, path+".")
			conflicts = append(conflicts, subConflicts...)
			if len(sub) > 0 {
				missing[key] = sub
			}
		case []any:
			have, ok := here.([]any)
			if !ok {
				conflicts = append(conflicts, fmt.Sprintf("%s is already set to something that is not a list: left as is", path))
				continue
			}
			var extra []any
			for _, item := range w {
				if !containsJSON(have, item) {
					extra = append(extra, item)
				}
			}
			if len(extra) > 0 {
				missing[key] = extra
			}
		default:
			if compactJSON(here) != compactJSON(wanted) {
				if key == pluginKey && here == false {
					conflicts = append(conflicts, fmt.Sprintf("the Kanbanzai plugin is disabled (%s is false): left as is", path))
				} else {
					conflicts = append(conflicts, fmt.Sprintf("%s is already %s, not %s: left as is", path, compactJSON(here),
						compactJSON(wanted)))
				}
			}
		}
	}
	return missing, conflicts
}

func containsJSON(list []any, item any) bool {
	want := compactJSON(item)
	for _, other := range list {
		if compactJSON(other) == want {
			return true
		}
	}
	return false
}

// mergeJSON adds the fragment to the object: missing keys are added, objects merge, lists gain the elements they
// lack. Nothing existing is overwritten.
func mergeJSON(into, fragment map[string]any) {
	for key, value := range fragment {
		have, present := into[key]
		if !present {
			into[key] = value
			continue
		}
		switch v := value.(type) {
		case map[string]any:
			if h, ok := have.(map[string]any); ok {
				mergeJSON(h, v)
			}
		case []any:
			if h, ok := have.([]any); ok {
				for _, item := range v {
					if !containsJSON(h, item) {
						h = append(h, item)
					}
				}
				into[key] = h
			}
		}
	}
}

// applyMerge writes the merged settings file atomically, keeping the file's mode.
func applyMerge(path string, action Action) (string, error) {
	fragment, err := parseObject([]byte(action.Content))
	if err != nil {
		return "", fmt.Errorf("the fragment is not a JSON object: %v", err)
	}
	merged, status, mode := map[string]any{}, "made", os.FileMode(0o644)
	if info, statErr := os.Lstat(path); statErr == nil {
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return "", readErr
		}
		if merged, err = parseObject(data); err != nil {
			return "", fmt.Errorf("%s is no longer plain JSON: %v", action.Path, err)
		}
		status, mode = "merged", info.Mode().Perm()
	}
	mergeJSON(merged, fragment)
	text, err := json.MarshalIndent(merged, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	if err := fsutil.WriteFileAtomic(path, append(text, '\n'), mode); err != nil {
		return "", err
	}
	return status, nil
}

// validPluginAction is ValidatePlan's rule for a plugin action: a merge or keep on one of the three settings
// files, and a merge carries a JSON object.
func validPluginAction(action Action) error {
	if !pluginFiles[action.Path] {
		return fmt.Errorf("plugin settings are written to %s, %s or %s only", claudeSettingsRel, claudeLocalSettingsRel,
			openCodeSettingsRel)
	}
	switch action.Op {
	case OpKeep:
		if action.Content != "" {
			return fmt.Errorf("a keep holds no content")
		}
	case OpMerge:
		if _, err := parseObject([]byte(action.Content)); err != nil {
			return fmt.Errorf("a merge carries a JSON object: %v", err)
		}
		if strings.TrimSpace(action.Content) == "" || action.SHA256 == "" {
			return fmt.Errorf("a merge needs content")
		}
	default:
		return fmt.Errorf("plugin settings are only ever merged")
	}
	return nil
}
