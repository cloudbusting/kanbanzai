package config

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

// CurrentVersion versions the source configuration, not the command or the JSON transport.
const CurrentVersion = 1

// IsolationModes are the values of flow.isolation: how a card's change is kept apart from the board.
var IsolationModes = map[string]bool{"worktree": true, "branch": true, "shared": true, "external": true}

// schemaKeys is the single allowlist for all supported configuration tables. Step names are user-defined.
func schemaKeys() map[string]map[string]string {
	return map[string]map[string]string{
		"": {"config_version": "version", "config_allow_unknown": "strings", "project": "table", "claims": "table", "flow": "table", "columns": "columns", "steps": "steps", "types": "types", "schemas": "schemas"},
		"project": {
			"prefix": "string", "digits": "digits", "owner": "string", "home": "string", "work": "string", "state": "string", "claims": "string", "numbers": "string", "batches": "string", "template": "string", "worktree": "string", "main_branch": "string", "branch": "string", "batch_branch": "string", "context": "strings", "default_type": "string", "project_key": "string",
		},
		"claims":       {"lease_minutes": "positive number", "owner_env": "string"},
		"flow":         {"wip_limit": "positive integer", "stale_days": "positive integer", "move_grace_minutes": "positive number", "pull_order": "strings", "isolation": "isolation"},
		"columns":      {"key": "string", "label": "string", "words": "strings", "folder": "string", "kind": "string", "started": "started", "next": "string", "settled_next": "string", "title": "string", "settled_title": "string", "worktree": "boolean", "waits_for_owner": "boolean", "lane": "string", "doing": "string", "gate": "string", "gate_name": "string", "gate_after": "string", "gate_until": "string"},
		"types":        {"key": "string", "label": "string", "prefix": "string", "template": "string", "schema": "string", "colour": "string", "icon": "string", "description": "string"},
		"brief_schema": {"required_fields": "strings", "optional_fields": "strings", "required_sections": "strings", "optional_sections": "strings", "repeatable_sections": "strings", "section_order": "strings", "final_section": "string"},
		"step":         {"into": "string", "during": "string", "skill": "string", "extend": "strings", "context": "strings", "checks": "strings", "skip_field": "string", "skip_values": "strings"},
	}
}

func sourceName(path string) string {
	if path == "" {
		return "defaults/config.toml"
	}
	return path
}

func validateVersion(raw map[string]any, source string) error {
	value, exists := raw["config_version"]
	if !exists {
		return errf("%s: root key config_version is missing (version 0); run `kanbanzai config upgrade`, then `kanbanzai config upgrade --apply`", source)
	}
	n, ok := value.(int64)
	if !ok || n < 0 {
		return errf("%s: root key config_version must be a non-negative integer; use %d", source, CurrentVersion)
	}
	if n < CurrentVersion {
		return errf("%s: root key config_version is %d, expected %d; run `kanbanzai config upgrade`, then `kanbanzai config upgrade --apply`", source, n, CurrentVersion)
	}
	if n > CurrentVersion {
		return errf("%s: root key config_version is %d, newer than supported %d; install a newer kanbanzai binary (do not downgrade the file)", source, n, CurrentVersion)
	}
	return nil
}

// validateSchema checks before merge, so even an unused or overwritten setting cannot hide a typo or wrong type.
func validateSchema(raw map[string]any, source string) ([]string, error) {
	allowed := map[string]bool{}
	if value, ok := raw["config_allow_unknown"]; ok {
		if !validType(value, "strings") {
			return nil, errf("%s: root key config_allow_unknown must be a list of strings (exact dotted paths)", source)
		}
		for _, item := range value.([]any) {
			path := item.(string)
			if path == "" || strings.ContainsAny(path, "*?[]") || strings.TrimSpace(path) != path || strings.Contains(path, "..") || strings.HasPrefix(path, ".") || strings.HasSuffix(path, ".") {
				return nil, errf("%s: config_allow_unknown entry %q must be an exact dotted path, without wildcards", source, path)
			}
			allowed[path] = true
		}
	}
	keys := schemaKeys()
	ignored := []string{}
	var table func(map[string]any, string, string) error
	table = func(values map[string]any, schema, path string) error {
		for _, key := range sortedKeys(values) {
			value := values[key]
			full := key
			if path != "" {
				full = path + "." + key
			}
			typ, known := keys[schema][key]
			if !known {
				if allowed[full] {
					ignored = append(ignored, full)
					continue
				}
				label := "root"
				if path != "" {
					label = "[" + path + "]"
				}
				return errf("%s: %s unknown key %q; nearest valid key: %q; correct it or explicitly list %q in config_allow_unknown", source, label, key, nearestKey(key, keys[schema]), full)
			}
			if !validType(value, typ) {
				if typ == "table" || typ == "steps" {
					return errf("%s: [%s] must be a table; replace the value with a table", source, full)
				}
				if typ == "types" {
					return errf("%s: types must be a list of [[types]] tables", source)
				}
				want := typ
				switch typ {
				case "strings":
					want = "a list of strings"
				case "digits":
					want = "an integer from 1 through 18"
				case "positive integer":
					want = "an integer greater than zero"
				case "positive number":
					want = "a finite number greater than zero, at most 153722867 minutes (the duration limit)"
				case "started":
					want = `a boolean or "claimed"`
				case "isolation":
					want = `one of "worktree", "branch", "shared" or "external"`
				}
				return errf("%s: %s must be %s; correct the value in the config", source, full, want)
			}
			switch typ {
			case "table":
				if err := table(value.(map[string]any), key, full); err != nil {
					return err
				}
			case "steps":
				for _, name := range sortedKeys(value.(map[string]any)) {
					entry, ok := value.(map[string]any)[name].(map[string]any)
					if !ok {
						return errf("%s: [steps.%s] must be a table", source, name)
					}
					if err := table(entry, "step", "steps."+name); err != nil {
						return err
					}
				}
			case "schemas":
				for _, name := range sortedKeys(value.(map[string]any)) {
					entry, ok := value.(map[string]any)[name].(map[string]any)
					if !ok {
						return errf("%s: [schemas.%s] must be a table", source, name)
					}
					if err := table(entry, "brief_schema", "schemas."+name); err != nil {
						return err
					}
				}
			case "columns":
				for _, item := range value.([]any) {
					if err := table(item.(map[string]any), "columns", "columns"); err != nil {
						return err
					}
				}
			case "types":
				for _, item := range value.([]any) {
					if err := table(item.(map[string]any), "types", "types"); err != nil {
						return err
					}
				}
			}
		}
		return nil
	}
	err := table(raw, "", "")
	sort.Strings(ignored)
	return uniqueStrings(ignored), err
}

func validType(value any, typ string) bool {
	switch typ {
	case "version":
		_, ok := value.(int64)
		return ok
	case "string":
		_, ok := value.(string)
		return ok
	case "boolean":
		_, ok := value.(bool)
		return ok
	case "isolation":
		text, ok := value.(string)
		return ok && IsolationModes[text]
	case "started":
		if _, ok := value.(bool); ok {
			return true
		}
		return value == "claimed"
	case "table", "steps", "schemas":
		_, ok := value.(map[string]any)
		return ok
	case "strings":
		list, ok := value.([]any)
		if !ok {
			return false
		}
		for _, item := range list {
			if _, ok := item.(string); !ok {
				return false
			}
		}
		return true
	case "columns":
		list, ok := value.([]any)
		if !ok || len(list) == 0 {
			return false
		}
		for _, item := range list {
			if _, ok := item.(map[string]any); !ok {
				return false
			}
		}
		return true
	case "types":
		list, ok := value.([]any)
		if !ok {
			return false
		}
		for _, item := range list {
			if _, ok := item.(map[string]any); !ok {
				return false
			}
		}
		return true
	case "digits", "positive integer":
		n, ok := value.(int64)
		if !ok || n <= 0 || uint64(n) > uint64(^uint(0)>>1) {
			return false
		}
		return typ != "digits" || n <= 18
	case "positive number":
		var n float64
		switch v := value.(type) {
		case int64:
			n = float64(v)
		case float64:
			n = v
		default:
			return false
		}
		return n > 0 && !math.IsNaN(n) && !math.IsInf(n, 0) && n <= float64(math.MaxInt64/int64(time.Minute))
	}
	return false
}

func sortedKeys(values map[string]any) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func nearestKey(key string, choices map[string]string) string {
	best, distance := "", int(^uint(0)>>1)
	for _, candidate := range sortedKeysString(choices) {
		// Levenshtein distance, bounded by the short schema keys, no fuzzy application of settings.
		previous := make([]int, len(candidate)+1)
		for j := range previous {
			previous[j] = j
		}
		for i := 0; i < len(key); i++ {
			current := make([]int, len(candidate)+1)
			current[0] = i + 1
			for j := 0; j < len(candidate); j++ {
				cost := 0
				if key[i] != candidate[j] {
					cost = 1
				}
				current[j+1] = min(current[j]+1, previous[j+1]+1, previous[j]+cost)
			}
			previous = current
		}
		if previous[len(candidate)] < distance {
			best = candidate
			distance = previous[len(candidate)]
		}
	}
	return best
}

func sortedKeysString(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func qualifyError(source string, err error) error {
	if err == nil {
		return nil
	}
	return errf("%s: %s; correct the named key or reference in the config", source, fmt.Sprint(err))
}
