package model

import (
	"encoding/json"
	"strings"
)

// StringOrSlice holds a value that can be unmarshalled from either a JSON string or a JSON array of strings.
// This enables backward-compatible evolution of fields like jira_prefix from string to string[].
type StringOrSlice []string

// ParseStringOrSlice converts a comma-separated string into a StringOrSlice.
func ParseStringOrSlice(s string) StringOrSlice {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	if !strings.Contains(s, ",") {
		return StringOrSlice{s}
	}
	parts := strings.Split(s, ",")
	out := make(StringOrSlice, 0, len(parts))
	for _, p := range parts {
		if v := strings.TrimSpace(p); v != "" {
			out = append(out, v)
		}
	}
	return out
}

func (s *StringOrSlice) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		*s = nil
		return nil
	}
	var single string
	if err := json.Unmarshal(data, &single); err == nil {
		*s = StringOrSlice{single}
		return nil
	}
	var slice []string
	if err := json.Unmarshal(data, &slice); err != nil {
		return err
	}
	*s = slice
	return nil
}

func (s StringOrSlice) MarshalJSON() ([]byte, error) {
	if len(s) == 1 {
		return json.Marshal(s[0])
	}
	return json.Marshal([]string(s))
}

// String returns a comma-separated display representation.
func (s StringOrSlice) String() string {
	switch len(s) {
	case 0:
		return ""
	case 1:
		return s[0]
	default:
		out := s[0]
		for _, v := range s[1:] {
			out += ", " + v
		}
		return out
	}
}

// IsEmpty returns true if no prefixes are set.
func (s StringOrSlice) IsEmpty() bool {
	return len(s) == 0 || (len(s) == 1 && s[0] == "")
}
