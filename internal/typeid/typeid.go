// Package typeid defines a project's issue types and the canonical card id:
//
//	[TYPE-]<PREFIX>-<NUMBER>          no project key
//	[<PROJECT_KEY>-]<PREFIX>-<NUMBER> with a project key
//
// A type's prefix identifies the type; the number comes from one global sequence shared by every type, so BUG-7 is
// the seventh item overall, not the seventh bug. Type keys are lower-case identifiers; prefixes and the optional
// project key are upper-case letters and digits. IDs are formatted upper-case and zero-padded, and parsed
// case-insensitively.
package typeid

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var (
	keyRE        = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
	prefixRE     = regexp.MustCompile(`^[A-Z][A-Z0-9]*$`)
	colourRE     = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)
	digitsNumber = regexp.MustCompile(`^\d+$`)
)

// Type is one issue type.
type Type struct {
	Key         string
	Label       string
	Prefix      string
	Template    string
	Schema      string
	Colour      string
	Icon        string
	Description string
}

// Set is a project's validated types and the default selection.
type Set struct {
	Types      []Type
	Default    string // the default type's key
	ProjectKey string // empty: ids have no project component
	Digits     int    // minimum number of digits
}

// Validate builds a Set from raw types, rejecting ambiguous or malformed configuration rather than guessing.
func Validate(types []Type, defaultKey, projectKey string, digits int) (*Set, error) {
	if len(types) == 0 {
		return nil, fmt.Errorf("no types are configured")
	}
	if digits < 1 || digits > 18 {
		return nil, fmt.Errorf("digits must be an integer from 1 through 18")
	}
	clean := make([]Type, 0, len(types))
	byKey := map[string]string{}
	byPrefix := map[string]string{}
	for index, item := range types {
		number := index + 1
		item.Key = strings.TrimSpace(item.Key)
		item.Label = strings.TrimSpace(item.Label)
		item.Prefix = strings.TrimSpace(item.Prefix)
		item.Template = strings.TrimSpace(item.Template)
		item.Colour = strings.TrimSpace(item.Colour)
		item.Icon = strings.TrimSpace(item.Icon)
		item.Description = strings.TrimSpace(item.Description)
		if !keyRE.MatchString(item.Key) {
			return nil, fmt.Errorf("type %d: key must be a lower-case identifier starting with a letter, not %q",
				number, item.Key)
		}
		if item.Label == "" {
			return nil, fmt.Errorf("type %q: label is required", item.Key)
		}
		if !prefixRE.MatchString(item.Prefix) {
			return nil, fmt.Errorf("type %q: prefix must be upper-case letters and digits starting with a letter, "+
				"not %q", item.Key, item.Prefix)
		}
		if item.Colour != "" && !colourRE.MatchString(item.Colour) {
			return nil, fmt.Errorf("type %q: colour must be # plus six hexadecimal digits, not %q", item.Key,
				item.Colour)
		}
		lowerKey, upperPrefix := strings.ToLower(item.Key), strings.ToUpper(item.Prefix)
		if other, ok := byKey[lowerKey]; ok {
			return nil, fmt.Errorf("types %q and %q share the key %q (keys are unique ignoring case)", other,
				item.Key, item.Key)
		}
		if other, ok := byPrefix[upperPrefix]; ok {
			return nil, fmt.Errorf("types %q and %q share the prefix %q (prefixes are unique ignoring case)", other,
				item.Key, item.Prefix)
		}
		byKey[lowerKey] = item.Key
		byPrefix[upperPrefix] = item.Key
		item.Key = lowerKey
		item.Prefix = upperPrefix
		clean = append(clean, item)
	}

	projectKey = strings.TrimSpace(projectKey)
	if projectKey != "" && !prefixRE.MatchString(projectKey) {
		return nil, fmt.Errorf("project_key must be upper-case letters and digits starting with a letter, not %q",
			projectKey)
	}

	chosen := strings.ToLower(strings.TrimSpace(defaultKey))
	if chosen == "" {
		if _, ok := byKey["task"]; ok {
			chosen = "task"
		} else if len(clean) == 1 {
			chosen = clean[0].Key
		} else {
			return nil, fmt.Errorf("default_type is required when more than one type is configured (valid keys: %s)",
				strings.Join(keys(clean), ", "))
		}
	}
	if _, ok := byKey[chosen]; !ok {
		return nil, fmt.Errorf("default_type names %q, which is not a configured type (valid keys: %s)", chosen,
			strings.Join(keys(clean), ", "))
	}

	return &Set{Types: clean, Default: chosen, ProjectKey: projectKey, Digits: digits}, nil
}

func keys(types []Type) []string {
	out := make([]string, 0, len(types))
	for _, item := range types {
		out = append(out, item.Key)
	}
	sort.Strings(out)
	return out
}

// Enabled reports whether a set is configured.
func (s *Set) Enabled() bool { return s != nil && len(s.Types) > 0 }

// DefaultType is the type used when none is named.
func (s *Set) DefaultType() (Type, bool) {
	if s == nil {
		return Type{}, false
	}
	return s.ByKey(s.Default)
}

// ByKey finds a type by its key, ignoring case.
func (s *Set) ByKey(key string) (Type, bool) {
	if s == nil {
		return Type{}, false
	}
	for _, item := range s.Types {
		if strings.EqualFold(item.Key, strings.TrimSpace(key)) {
			return item, true
		}
	}
	return Type{}, false
}

// ByPrefix finds a type by its prefix, ignoring case.
func (s *Set) ByPrefix(prefix string) (Type, bool) {
	if s == nil {
		return Type{}, false
	}
	for _, item := range s.Types {
		if strings.EqualFold(item.Prefix, strings.TrimSpace(prefix)) {
			return item, true
		}
	}
	return Type{}, false
}

// Prefixes are the configured prefixes, upper-case, in configuration order.
func (s *Set) Prefixes() []string {
	if s == nil {
		return nil
	}
	out := make([]string, 0, len(s.Types))
	for _, item := range s.Types {
		out = append(out, item.Prefix)
	}
	return out
}

// Keys are the configured keys, in configuration order.
func (s *Set) Keys() []string {
	if s == nil {
		return nil
	}
	out := make([]string, 0, len(s.Types))
	for _, item := range s.Types {
		out = append(out, item.Key)
	}
	return out
}

// Format is a type and a number as its canonical id.
func (s *Set) Format(item Type, number int) string {
	prefix := item.Prefix
	if s != nil && s.ProjectKey != "" {
		prefix = s.ProjectKey + "-" + prefix
	}
	digits := 1
	if s != nil && s.Digits > 0 {
		digits = s.Digits
	}
	return fmt.Sprintf("%s-%0*d", prefix, digits, number)
}

// Parse reads a card id, ignoring case, and returns its type and numeric value. The number must be positive.
func (s *Set) Parse(id string) (Type, int, bool) {
	if s == nil {
		return Type{}, 0, false
	}
	parts := strings.Split(id, "-")
	if len(parts) < 2 {
		return Type{}, 0, false
	}
	numberPart := parts[len(parts)-1]
	prefix := parts[len(parts)-2]
	rest := parts[:len(parts)-2]
	switch len(rest) {
	case 0:
		if s.ProjectKey != "" {
			return Type{}, 0, false
		}
	case 1:
		if s.ProjectKey == "" || !strings.EqualFold(rest[0], s.ProjectKey) {
			return Type{}, 0, false
		}
	default:
		return Type{}, 0, false
	}
	if !digitsNumber.MatchString(numberPart) {
		return Type{}, 0, false
	}
	number, err := strconv.Atoi(numberPart)
	if err != nil || number <= 0 {
		return Type{}, 0, false
	}
	item, ok := s.ByPrefix(prefix)
	if !ok {
		return Type{}, 0, false
	}
	return item, number, true
}

// Canonical is an id in its canonical form, or false when it is not one of the configured types.
func (s *Set) Canonical(id string) (string, bool) {
	item, number, ok := s.Parse(id)
	if !ok {
		return "", false
	}
	return s.Format(item, number), true
}

// IDPattern is the regular expression body for a canonical id: the optional project key, one configured prefix and
// a number padded to at least Digits. It is unanchored, for embedding in CardRe, TitleRe and CardFileRe.
func (s *Set) IDPattern() string {
	return s.pattern(`\d{` + strconv.Itoa(s.digits()) + `,}`)
}

// LooseIDPattern is IDPattern with any number of digits, for finding an id inside a branch name or a path.
func (s *Set) LooseIDPattern() string {
	return s.pattern(`\d+`)
}

func (s *Set) digits() int {
	if s == nil || s.Digits < 1 {
		return 1
	}
	return s.Digits
}

func (s *Set) pattern(number string) string {
	if s == nil || len(s.Types) == 0 {
		return ""
	}
	prefixes := make([]string, 0, len(s.Types))
	for _, item := range s.Types {
		prefixes = append(prefixes, regexp.QuoteMeta(item.Prefix))
	}
	sort.SliceStable(prefixes, func(i, j int) bool { return len(prefixes[i]) > len(prefixes[j]) })
	body := "(?:" + strings.Join(prefixes, "|") + ")-" + number
	if s.ProjectKey != "" {
		body = regexp.QuoteMeta(s.ProjectKey) + "-" + body
	}
	return body
}

// Example is an id shape for a message, such as TASK-NN or KANZ-TASK-NN.
func (s *Set) Example() string {
	item, ok := s.DefaultType()
	if !ok {
		return "TYPE-NN"
	}
	prefix := item.Prefix
	if s.ProjectKey != "" {
		prefix = s.ProjectKey + "-" + prefix
	}
	return prefix + "-NN"
}

// TypeInfo is a type as the board and config JSON expose it.
type TypeInfo struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	Prefix      string `json:"prefix"`
	Template    string `json:"template,omitempty"`
	Colour      string `json:"colour,omitempty"`
	Icon        string `json:"icon,omitempty"`
	Description string `json:"description,omitempty"`
}

// Info is the type's JSON shape.
func (t Type) Info() TypeInfo {
	return TypeInfo{Key: t.Key, Label: t.Label, Prefix: t.Prefix, Template: t.Template, Colour: t.Colour,
		Icon: t.Icon, Description: t.Description}
}
