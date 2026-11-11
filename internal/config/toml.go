// The focused TOML parser: enough of TOML 1.0 for Kanbanzai's config, with an error rather than a guess for
// anything it does not support.
//
// Supported: comments, bare/quoted/dotted keys, tables and arrays of tables, basic and literal strings (including
// multiline), integers (decimal, hex, octal, binary, underscores), floats (including inf/nan), booleans, arrays
// and inline tables. Dates and times are not part of the config schema and are rejected with a clear message; so
// is every other syntax the parser cannot prove it understands. Duplicate keys and tables are errors.
package config

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"
)

type tomlParser struct {
	src  string
	pos  int
	line int
}

// parseTOML parses a whole document.
func parseTOML(src string) (map[string]any, error) {
	p := &tomlParser{src: strings.ReplaceAll(src, "\r\n", "\n"), line: 1}
	root := map[string]any{}
	current := root
	explicit := map[string]bool{}
	for {
		p.skipBlank()
		if p.eof() {
			return root, nil
		}
		if p.peek() == '[' {
			array := p.peekAt(1) == '['
			p.pos++
			if array {
				p.pos++
			}
			p.skipSpace()
			path, err := p.keyPath()
			if err != nil {
				return nil, err
			}
			p.skipSpace()
			if !p.take(']') {
				return nil, p.errorf("expected ']' after a table name")
			}
			if array && !p.take(']') {
				return nil, p.errorf("expected ']]' after an array-of-tables name")
			}
			if err := p.endOfLine(); err != nil {
				return nil, err
			}
			key := strings.Join(path, ".")
			if array {
				current, err = p.appendTable(root, path, key)
				if err != nil {
					return nil, err
				}
				continue
			}
			current, err = p.defineTable(root, path, key, explicit)
			if err != nil {
				return nil, err
			}
			continue
		}
		path, err := p.keyPath()
		if err != nil {
			return nil, err
		}
		p.skipSpace()
		if !p.take('=') {
			return nil, p.errorf("expected '=' after a key")
		}
		p.skipSpace()
		value, err := p.value()
		if err != nil {
			return nil, err
		}
		if err := setPath(current, path, value); err != nil {
			return nil, p.wrap(err)
		}
		if err := p.endOfLine(); err != nil {
			return nil, err
		}
	}
}

func (p *tomlParser) eof() bool { return p.pos >= len(p.src) }

func (p *tomlParser) peek() byte {
	if p.eof() {
		return 0
	}
	return p.src[p.pos]
}

func (p *tomlParser) peekAt(n int) byte {
	if p.pos+n >= len(p.src) {
		return 0
	}
	return p.src[p.pos+n]
}

func (p *tomlParser) take(ch byte) bool {
	if p.peek() != ch {
		return false
	}
	p.pos++
	return true
}

func (p *tomlParser) skipSpace() {
	for {
		switch p.peek() {
		case ' ', '\t':
			p.pos++
		default:
			return
		}
	}
}

// skipBlank skips whitespace, newlines and comments: everything between statements and array elements.
func (p *tomlParser) skipBlank() {
	for {
		switch p.peek() {
		case ' ', '\t', '\n':
			if p.peek() == '\n' {
				p.line++
			}
			p.pos++
		case '#':
			for !p.eof() && p.peek() != '\n' {
				p.pos++
			}
		default:
			return
		}
	}
}

func (p *tomlParser) endOfLine() error {
	p.skipSpace()
	if p.peek() == '#' {
		for !p.eof() && p.peek() != '\n' {
			p.pos++
		}
	}
	if p.eof() {
		return nil
	}
	if p.peek() == '\n' {
		p.pos++
		p.line++
		return nil
	}
	return p.errorf("expected the end of the line")
}

func (p *tomlParser) errorf(format string, args ...any) error {
	return fmt.Errorf("line %d: %s", p.line, fmt.Sprintf(format, args...))
}

func (p *tomlParser) wrap(err error) error { return p.errorf("%s", err) }

// keyPath parses a key or dotted key: a.b.c, with basic or literal quoted segments.
func (p *tomlParser) keyPath() ([]string, error) {
	var path []string
	for {
		p.skipSpace()
		part, err := p.keyPart()
		if err != nil {
			return nil, err
		}
		path = append(path, part)
		p.skipSpace()
		if !p.take('.') {
			return path, nil
		}
	}
}

func (p *tomlParser) keyPart() (string, error) {
	if p.eof() {
		return "", p.errorf("expected a key")
	}
	switch p.peek() {
	case '"':
		return p.basicString(false)
	case '\'':
		return p.literalString(false)
	}
	start := p.pos
	for !p.eof() {
		ch := p.peek()
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_' || ch == '-' {
			p.pos++
			continue
		}
		break
	}
	if p.pos == start {
		return "", p.errorf("expected a key, not %q", string(p.peek()))
	}
	return p.src[start:p.pos], nil
}

// value parses one TOML value.
func (p *tomlParser) value() (any, error) {
	if p.eof() {
		return nil, p.errorf("expected a value")
	}
	switch p.peek() {
	case '"':
		if p.peekAt(1) == '"' && p.peekAt(2) == '"' {
			return p.multilineBasicString()
		}
		return p.basicString(false)
	case '\'':
		if p.peekAt(1) == '\'' && p.peekAt(2) == '\'' {
			return p.multilineLiteralString()
		}
		return p.literalString(false)
	case '[':
		return p.array()
	case '{':
		return p.inlineTable()
	}
	start := p.pos
	for !p.eof() {
		ch := p.peek()
		if ch == ' ' || ch == '\t' || ch == '\n' || ch == ',' || ch == ']' || ch == '}' || ch == '#' {
			break
		}
		p.pos++
	}
	word := p.src[start:p.pos]
	if word == "" {
		return nil, p.errorf("expected a value")
	}
	switch word {
	case "true":
		return true, nil
	case "false":
		return false, nil
	case "inf", "+inf":
		return math.Inf(1), nil
	case "-inf":
		return math.Inf(-1), nil
	case "nan", "+nan", "-nan":
		return math.NaN(), nil
	}
	if strings.ContainsAny(word, "Tt:Zz") && strings.Contains(word, "-") {
		return nil, p.errorf("date and time values are not supported in the config")
	}
	clean := strings.ReplaceAll(word, "_", "")
	if strings.HasPrefix(clean, "0x") || strings.HasPrefix(clean, "0X") ||
		strings.HasPrefix(clean, "0o") || strings.HasPrefix(clean, "0O") ||
		strings.HasPrefix(clean, "0b") || strings.HasPrefix(clean, "0B") {
		if n, err := strconv.ParseInt(clean, 0, 64); err == nil {
			return n, nil
		}
	}
	if !strings.ContainsAny(clean, ".eE") {
		if n, err := strconv.ParseInt(clean, 10, 64); err == nil {
			return n, nil
		}
	}
	if f, err := strconv.ParseFloat(clean, 64); err == nil {
		return f, nil
	}
	return nil, p.errorf("invalid value %q", word)
}

func (p *tomlParser) basicString(allowNewline bool) (string, error) {
	p.pos++ // "
	var b strings.Builder
	for !p.eof() {
		ch := p.peek()
		if ch == '"' {
			p.pos++
			return b.String(), nil
		}
		if ch == '\n' {
			if !allowNewline {
				return "", p.errorf("a basic string cannot span lines")
			}
			p.line++
		}
		if ch == '\\' {
			p.pos++
			if p.eof() {
				return "", p.errorf("an escape needs a character")
			}
			esc := p.peek()
			p.pos++
			switch esc {
			case 'b':
				b.WriteByte('\b')
			case 't':
				b.WriteByte('\t')
			case 'n':
				b.WriteByte('\n')
			case 'f':
				b.WriteByte('\f')
			case 'r':
				b.WriteByte('\r')
			case '"':
				b.WriteByte('"')
			case '\\':
				b.WriteByte('\\')
			case 'u', 'U':
				width := 4
				if esc == 'U' {
					width = 8
				}
				if p.pos+width > len(p.src) {
					return "", p.errorf("a Unicode escape is too short")
				}
				hex := p.src[p.pos : p.pos+width]
				value, err := strconv.ParseUint(hex, 16, 32)
				if err != nil || !utf8.ValidRune(rune(value)) {
					return "", p.errorf("invalid Unicode escape \\%c%s", esc, hex)
				}
				b.WriteRune(rune(value))
				p.pos += width
			default:
				return "", p.errorf("unsupported escape \\%c", esc)
			}
			continue
		}
		b.WriteByte(ch)
		p.pos++
	}
	return "", p.errorf("unterminated basic string")
}

func (p *tomlParser) multilineBasicString() (string, error) {
	p.pos += 3
	if p.peek() == '\n' {
		p.pos++
		p.line++
	}
	var b strings.Builder
	for !p.eof() {
		if p.peek() == '"' && p.peekAt(1) == '"' && p.peekAt(2) == '"' {
			p.pos += 3
			return b.String(), nil
		}
		ch := p.peek()
		if ch == '\n' {
			p.line++
			b.WriteByte(ch)
			p.pos++
			continue
		}
		if ch == '\\' {
			// A line-ending backslash trims whitespace and newlines up to the next non-space character.
			if p.peekAt(1) == '\n' || p.peekAt(1) == ' ' || p.peekAt(1) == '\t' || p.peekAt(1) == '\r' {
				p.pos++
				for !p.eof() {
					switch p.peek() {
					case ' ', '\t', '\n':
						if p.peek() == '\n' {
							p.line++
						}
						p.pos++
					default:
						goto escaped
					}
				}
			escaped:
				continue
			}
			p.pos++
			esc := p.peek()
			p.pos++
			switch esc {
			case 'b':
				b.WriteByte('\b')
			case 't':
				b.WriteByte('\t')
			case 'n':
				b.WriteByte('\n')
			case 'f':
				b.WriteByte('\f')
			case 'r':
				b.WriteByte('\r')
			case '"':
				b.WriteByte('"')
			case '\\':
				b.WriteByte('\\')
			case 'u', 'U':
				width := 4
				if esc == 'U' {
					width = 8
				}
				if p.pos+width > len(p.src) {
					return "", p.errorf("a Unicode escape is too short")
				}
				hex := p.src[p.pos : p.pos+width]
				value, err := strconv.ParseUint(hex, 16, 32)
				if err != nil || !utf8.ValidRune(rune(value)) {
					return "", p.errorf("invalid Unicode escape \\%c%s", esc, hex)
				}
				b.WriteRune(rune(value))
				p.pos += width
			default:
				return "", p.errorf("unsupported escape \\%c", esc)
			}
			continue
		}
		b.WriteByte(ch)
		p.pos++
	}
	return "", p.errorf("unterminated multiline basic string")
}

func (p *tomlParser) literalString(allowNewline bool) (string, error) {
	p.pos++ // '
	start := p.pos
	for !p.eof() {
		switch p.peek() {
		case '\'':
			value := p.src[start:p.pos]
			p.pos++
			return value, nil
		case '\n':
			if !allowNewline {
				return "", p.errorf("a literal string cannot span lines")
			}
			p.line++
		}
		p.pos++
	}
	return "", p.errorf("unterminated literal string")
}

func (p *tomlParser) multilineLiteralString() (string, error) {
	p.pos += 3
	if p.peek() == '\n' {
		p.pos++
		p.line++
	}
	start := p.pos
	for !p.eof() {
		if p.peek() == '\'' && p.peekAt(1) == '\'' && p.peekAt(2) == '\'' {
			value := p.src[start:p.pos]
			p.pos += 3
			return value, nil
		}
		if p.peek() == '\n' {
			p.line++
		}
		p.pos++
	}
	return "", p.errorf("unterminated multiline literal string")
}

func (p *tomlParser) array() ([]any, error) {
	p.pos++ // [
	var out []any
	for {
		p.skipBlank()
		if p.take(']') {
			return out, nil
		}
		if p.eof() {
			return nil, p.errorf("unterminated array")
		}
		value, err := p.value()
		if err != nil {
			return nil, err
		}
		out = append(out, value)
		p.skipBlank()
		if p.eof() {
			return nil, p.errorf("unterminated array")
		}
		if p.take(',') {
			continue
		}
		if p.take(']') {
			return out, nil
		}
		return nil, p.errorf("expected ',' or ']' in an array")
	}
}

func (p *tomlParser) inlineTable() (map[string]any, error) {
	p.pos++ // {
	out := map[string]any{}
	for {
		p.skipSpace()
		if p.take('}') {
			return out, nil
		}
		if p.eof() {
			return nil, p.errorf("unterminated inline table")
		}
		path, err := p.keyPath()
		if err != nil {
			return nil, err
		}
		p.skipSpace()
		if !p.take('=') {
			return nil, p.errorf("expected '=' in an inline table")
		}
		p.skipSpace()
		value, err := p.value()
		if err != nil {
			return nil, err
		}
		if err := setPath(out, path, value); err != nil {
			return nil, p.wrap(err)
		}
		p.skipSpace()
		if p.eof() || p.peek() == '\n' {
			return nil, p.errorf("unterminated inline table")
		}
		if p.take(',') {
			continue
		}
		if p.take('}') {
			return out, nil
		}
		return nil, p.errorf("expected ',' or '}' in an inline table")
	}
}

// defineTable navigates or creates the table at path and returns it.
func (p *tomlParser) defineTable(root map[string]any, path []string, key string, explicit map[string]bool) (map[string]any, error) {
	if len(path) == 0 {
		return nil, p.errorf("a table needs a name")
	}
	current := root
	for _, part := range path[:len(path)-1] {
		next, err := descend(current, part, true)
		if err != nil {
			return nil, p.wrap(err)
		}
		current = next
	}
	last := path[len(path)-1]
	value, ok := current[last]
	if !ok {
		table := map[string]any{}
		current[last] = table
		explicit[key] = true
		return table, nil
	}
	switch typed := value.(type) {
	case map[string]any:
		if explicit[key] {
			return nil, p.errorf("table %q is defined twice", key)
		}
		explicit[key] = true
		return typed, nil
	case []any:
		if len(typed) == 0 {
			return nil, p.wrap(fmt.Errorf("table %q is not an array of tables", key))
		}
		lastMap, ok := typed[len(typed)-1].(map[string]any)
		if !ok {
			return nil, p.wrap(fmt.Errorf("table %q is not an array of tables", key))
		}
		explicit[key] = true
		return lastMap, nil
	default:
		return nil, p.errorf("table %q is not a table", key)
	}
}

// appendTable appends a new table to the array of tables at path.
func (p *tomlParser) appendTable(root map[string]any, path []string, key string) (map[string]any, error) {
	if len(path) == 0 {
		return nil, p.errorf("an array of tables needs a name")
	}
	current := root
	for _, part := range path[:len(path)-1] {
		next, err := descend(current, part, true)
		if err != nil {
			return nil, p.wrap(err)
		}
		current = next
	}
	last := path[len(path)-1]
	value, ok := current[last]
	if !ok {
		value = []any{}
		current[last] = value
	}
	tables, ok := value.([]any)
	if !ok {
		return nil, p.errorf("%q is already a value, not an array of tables", key)
	}
	table := map[string]any{}
	current[last] = append(tables, table)
	return table, nil
}

// descend returns the map at key, creating it when create is true.
func descend(current map[string]any, key string, create bool) (map[string]any, error) {
	value, ok := current[key]
	if !ok {
		if !create {
			return nil, fmt.Errorf("%q is not a table", key)
		}
		table := map[string]any{}
		current[key] = table
		return table, nil
	}
	switch typed := value.(type) {
	case map[string]any:
		return typed, nil
	case []any:
		if len(typed) == 0 {
			return nil, fmt.Errorf("%q is an empty array, not an array of tables", key)
		}
		last, ok := typed[len(typed)-1].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("%q is not an array of tables", key)
		}
		return last, nil
	default:
		return nil, fmt.Errorf("%q is a value, not a table", key)
	}
}

// setPath assigns a value at a dotted key path, refusing a duplicate.
func setPath(current map[string]any, path []string, value any) error {
	if len(path) == 0 {
		return fmt.Errorf("a key needs a name")
	}
	for _, part := range path[:len(path)-1] {
		next, err := descend(current, part, true)
		if err != nil {
			return err
		}
		current = next
	}
	last := path[len(path)-1]
	if _, ok := current[last]; ok {
		return fmt.Errorf("key %q is defined twice", last)
	}
	current[last] = value
	return nil
}
