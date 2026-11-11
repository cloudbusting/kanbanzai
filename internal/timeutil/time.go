// Package timeutil holds the timestamp format the state files share.
//
// It is deliberately the Python core's format: `2026-11-11T06:34:00+00:00`, whole seconds. A board written by one
// implementation and read by the other must not lose a claim to a different spelling of the same moment.
package timeutil

import "time"

// Now is the current UTC time, to the second.
func Now() time.Time {
	return time.Now().UTC().Truncate(time.Second)
}

// ToISO formats a time as the state files write it.
func ToISO(value time.Time) string {
	return value.UTC().Format("2006-01-02T15:04:05+00:00")
}

// Parse reads a timestamp from JSON, accepting the RFC 3339 forms Python's fromisoformat accepts. A value without
// an offset is UTC, as Python treats it.
func Parse(value any) (time.Time, bool) {
	text, ok := value.(string)
	if !ok || text == "" {
		return time.Time{}, false
	}
	layouts := []string{
		"2006-01-02T15:04:05.999999999Z07:00",
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02T15:04:05.999999999-0700",
		"2006-01-02T15:04:05-0700",
		"2006-01-02T15:04:05.999999999",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02 15:04:05-07:00",
		"2006-01-02 15:04:05.999999999",
		"2006-01-02 15:04:05",
		"2006-01-02",
	}
	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, text); err == nil {
			return parsed, true
		}
	}
	return time.Time{}, false
}
