package doctor

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// adapterCheck aggregates declarations, not effective harness discovery or runtime health.
type adapterCheck struct {
	errors, warnings, checked []string
}

type unverified string

func (e unverified) Error() string { return string(e) }

func (a *adapterCheck) record(prefix string, err error) {
	var limitation unverified
	if errors.As(err, &limitation) {
		a.warnings = append(a.warnings, prefix+err.Error()+"; verify this declaration manually.")
	} else {
		a.errors = append(a.errors, prefix+err.Error())
	}
}

func (a *adapterCheck) finish(r *Report, name string) {
	status, fix := "pass", ""
	parts := append([]string{}, a.checked...)
	if len(a.warnings) > 0 {
		status = "warning"
		fix = "Verify the actual harness installation and configuration separately."
		parts = append(parts, a.warnings...)
	}
	if len(a.errors) > 0 {
		status = "error"
		fix = "Correct the named repository-local adapter declarations or restore their local files."
		parts = append(parts, a.errors...)
	}
	if len(parts) == 0 {
		status = "warning"
		parts = append(parts, "No repository-local Kanbanzai adapter declaration found; CLI-only use is valid.")
		fix = "Enable the adapter only if needed; verify external configuration separately."
	}
	parts = append(parts, "Only repository-root/current-checkout declarations were inspected; ancestor, nested-package, global, managed and live harness configuration are not certified.")
	r.add("adapter."+name, status, strings.Join(parts, " "), fix)
}

func readObject(path string) (map[string]json.RawMessage, error) {
	if err := readableFile(path); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var value map[string]json.RawMessage
	if err := json.Unmarshal(data, &value); err != nil {
		return nil, fmt.Errorf("%s: invalid JSON: %w", path, err)
	}
	if value == nil {
		return nil, fmt.Errorf("%s: expected a JSON object", path)
	}
	return value, nil
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func (r *Report) claude(bases []string) {
	a := adapterCheck{}
	enabled := map[string]bool{}
	type marketplace struct {
		raw  json.RawMessage
		base string
	}
	markets := map[string]marketplace{}
	for _, base := range bases {
		for _, name := range []string{"settings.json", "settings.local.json"} {
			path := filepath.Join(base, ".claude", name)
			doc, err := readObject(path)
			if os.IsNotExist(err) {
				continue
			}
			if err != nil {
				a.errors = append(a.errors, err.Error())
				continue
			}
			if raw, ok := doc["enabledPlugins"]; ok {
				var entries map[string]bool
				if err := json.Unmarshal(raw, &entries); err != nil || entries == nil {
					a.errors = append(a.errors, path+": enabledPlugins must be an object of booleans.")
				} else {
					for key, value := range entries {
						enabled[key] = value
					}
				}
			}
			if raw, ok := doc["extraKnownMarketplaces"]; ok {
				var entries map[string]json.RawMessage
				if err := json.Unmarshal(raw, &entries); err != nil || entries == nil {
					a.errors = append(a.errors, path+": extraKnownMarketplaces must be an object.")
				} else {
					for key, value := range entries {
						markets[key] = marketplace{value, base}
					}
				}
			}
		}
	}
	for _, key := range sortedKeys(enabled) {
		if !enabled[key] || !strings.HasPrefix(key, "kanbanzai@") {
			continue
		}
		marketName := strings.TrimPrefix(key, "kanbanzai@")
		entry, ok := markets[marketName]
		if !ok {
			a.warnings = append(a.warnings, "Enabled "+key+" has no local marketplace source declaration; cached/remote installs cannot be certified.")
			continue
		}
		var declaration struct {
			Source struct {
				Kind string `json:"source"`
				Path string `json:"path"`
			} `json:"source"`
		}
		if err := json.Unmarshal(entry.raw, &declaration); err != nil || declaration.Source.Kind == "" {
			a.errors = append(a.errors, "Marketplace "+marketName+" has an invalid source declaration.")
			continue
		}
		if declaration.Source.Kind != "directory" && declaration.Source.Kind != "file" {
			a.warnings = append(a.warnings, "Marketplace "+marketName+" uses a remote/external source; its installation cannot be certified.")
			continue
		}
		if declaration.Source.Path == "" {
			a.errors = append(a.errors, "Marketplace "+marketName+" has no local source path.")
			continue
		}
		path := localPath(entry.base, declaration.Source.Path)
		if strings.Contains(path, "${") || strings.Contains(path, "{env:") || strings.HasPrefix(declaration.Source.Path, "~") {
			a.warnings = append(a.warnings, "Marketplace "+marketName+" uses an unexpanded path; not evaluated.")
			continue
		}
		manifest := path
		if declaration.Source.Kind == "directory" {
			manifest = filepath.Join(path, ".claude-plugin", "marketplace.json")
		}
		if err := checkClaudeMarketplace(manifest); err != nil {
			a.record("", err)
		} else {
			a.checked = append(a.checked, "Enabled "+key+": local marketplace/plugin manifests are readable (adapter code not executed).")
		}
	}
	a.finish(r, "claude")
}

func checkClaudeMarketplace(path string) error {
	doc, err := readObject(path)
	if err != nil {
		return fmt.Errorf("Local Claude marketplace %s: %w", path, err)
	}
	var plugins []struct {
		Name   string          `json:"name"`
		Source json.RawMessage `json:"source"`
	}
	if err := json.Unmarshal(doc["plugins"], &plugins); err != nil {
		return fmt.Errorf("%s: invalid plugins list", path)
	}
	for _, plugin := range plugins {
		if plugin.Name != "kanbanzai" {
			continue
		}
		var source string
		if err := json.Unmarshal(plugin.Source, &source); err != nil {
			return unverified(path + ": Kanbanzai plugin source cannot be inspected as a local path")
		}
		if source == "" {
			return fmt.Errorf("%s: Kanbanzai plugin has no inspectable local source", path)
		}
		if !strings.HasPrefix(source, ".") && !filepath.IsAbs(source) {
			return unverified(path + ": Kanbanzai plugin source is not an explicit local path")
		}
		// Directory marketplaces root relative sources at the directory containing .claude-plugin.
		base := filepath.Dir(path)
		if filepath.Base(base) == ".claude-plugin" {
			base = filepath.Dir(base)
		}
		pluginRoot := localPath(base, source)
		manifest := filepath.Join(pluginRoot, ".claude-plugin", "plugin.json")
		metadata, err := readObject(manifest)
		if err != nil {
			return err
		}
		var name string
		if err := json.Unmarshal(metadata["name"], &name); err != nil || name != "kanbanzai" {
			return fmt.Errorf("%s: expected plugin name kanbanzai", manifest)
		}
		// A present hooks file must be readable JSON, but commands are never interpreted or executed.
		hooks := filepath.Join(pluginRoot, "hooks", "hooks.json")
		if _, err := readObject(hooks); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	return fmt.Errorf("%s: no Kanbanzai plugin entry", path)
}

func localPath(base, path string) string {
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return filepath.Join(base, path)
}

func (r *Report) opencode(bases []string) {
	a := adapterCheck{}
	for _, base := range bases {
		for _, dir := range []string{base, filepath.Join(base, ".opencode")} {
			jsonc := filepath.Join(dir, "opencode.jsonc")
			if _, err := os.Stat(jsonc); err == nil {
				a.warnings = append(a.warnings, jsonc+": JSONC inspection is unsupported; this declaration was not parsed.")
			} else if !os.IsNotExist(err) {
				a.errors = append(a.errors, jsonc+": "+err.Error())
			}
			path := filepath.Join(dir, "opencode.json")
			doc, err := readObject(path)
			if os.IsNotExist(err) {
				continue
			}
			if err != nil {
				a.errors = append(a.errors, err.Error())
				continue
			}
			raw, ok := doc["plugins"]
			if !ok {
				continue
			}
			var entries []json.RawMessage
			if err := json.Unmarshal(raw, &entries); err != nil || string(raw) == "null" {
				a.errors = append(a.errors, path+": plugins must be an array.")
				continue
			}
			for _, entry := range entries {
				var spec string
				if err := json.Unmarshal(entry, &spec); err != nil {
					var object struct {
						Package string `json:"package"`
					}
					if err := json.Unmarshal(entry, &object); err != nil {
						a.errors = append(a.errors, path+": invalid plugin declaration.")
						continue
					}
					spec = object.Package
				}
				if spec == "" {
					a.errors = append(a.errors, path+": plugin must name a package or local directory.")
					continue
				}
				if strings.Contains(spec, "{env:") || strings.Contains(spec, "{file:") || strings.HasPrefix(spec, "~") {
					a.warnings = append(a.warnings, path+": plugin path substitution was not evaluated.")
					continue
				}
				local := filepath.IsAbs(spec) || strings.HasPrefix(spec, ".") || strings.HasPrefix(spec, "file:")
				if !local {
					if strings.Contains(strings.ToLower(spec), "kanbanzai") {
						a.warnings = append(a.warnings, path+": remote Kanbanzai package is declared; installed package and runtime are not certified.")
					}
					continue
				}
				candidate := spec
				if strings.HasPrefix(spec, "file:") {
					u, err := url.Parse(spec)
					if err != nil || (u.Host != "" && u.Host != "localhost") {
						a.warnings = append(a.warnings, path+": nonlocal/unsupported file URL was not inspected.")
						continue
					}
					candidate = filepath.FromSlash(u.Path)
					if runtimeWindowsDrive(candidate) {
						candidate = candidate[1:]
					}
				}
				candidate = localPath(dir, candidate)
				if err := checkOpenCodePlugin(candidate); err != nil {
					a.record(path+": ", err)
				} else {
					a.checked = append(a.checked, path+": local plugin files readable at "+candidate+" (identity and code not executed).")
				}
			}
		}
	}
	a.finish(r, "opencode")
}

func runtimeWindowsDrive(path string) bool {
	return filepath.Separator == '\\' && len(path) > 2 && path[0] == '/' && path[2] == ':'
}

func checkOpenCodePlugin(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("local plugin %s: %w", path, err)
	}
	if !info.IsDir() {
		return readableFile(path)
	}
	packageFile := filepath.Join(path, "package.json")
	if doc, err := readObject(packageFile); err == nil {
		// Export maps are declarative: check all local target files, never import them.
		if raw, ok := doc["exports"]; ok {
			var exports any
			if err := json.Unmarshal(raw, &exports); err != nil {
				return fmt.Errorf("%s: invalid exports", packageFile)
			}
			targets := exportTargets(exports)
			if len(targets) == 0 {
				return unverified(packageFile + ": no inspectable export targets")
			}
			for _, target := range targets {
				if !strings.HasPrefix(target, "./") {
					return fmt.Errorf("%s: unsupported export target %q", packageFile, target)
				}
				if strings.Contains(target, "*") {
					return unverified(packageFile + ": wildcard export targets cannot be certified")
				}
				if err := readableFile(filepath.Join(path, target)); err != nil {
					return err
				}
			}
			return nil
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	for _, stem := range []string{"server", "index"} {
		for _, ext := range []string{".ts", ".tsx", ".js"} {
			candidate := filepath.Join(path, stem+ext)
			if _, err := os.Stat(candidate); err == nil {
				return readableFile(candidate)
			}
		}
	}
	return fmt.Errorf("%s: no readable server/index entrypoint or package exports", path)
}

func exportTargets(value any) []string {
	switch v := value.(type) {
	case string:
		return []string{v}
	case []any:
		var result []string
		for _, child := range v {
			result = append(result, exportTargets(child)...)
		}
		return result
	case map[string]any:
		var result []string
		for _, key := range sortedKeys(v) {
			result = append(result, exportTargets(v[key])...)
		}
		return result
	}
	return nil
}

func readableFile(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%s is not a regular file", path)
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	return f.Close()
}
