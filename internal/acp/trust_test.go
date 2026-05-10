package acp

import "testing"

func TestIsDestructiveTool(t *testing.T) {
	cases := []struct {
		name     string
		expected bool
	}{
		{"fs_write", true},
		{"write", true},
		{"execute_bash", true},
		{"shell", true},
		{"delete_file", true},
		{"fs_read", false},
		{"grep", false},
		{"code", false},
		{"thinking", false},
		{"Read File", false},
		{"Write to disk", true},
	}
	for _, tc := range cases {
		got := isDestructiveTool(tc.name)
		if got != tc.expected {
			t.Errorf("isDestructiveTool(%q) = %v, want %v", tc.name, got, tc.expected)
		}
	}
}

func TestExtractToolName(t *testing.T) {
	cases := []struct {
		params   string
		expected string
	}{
		{`{"title":"fs_write"}`, "fs_write"},
		{`{"name":"grep"}`, "grep"},
		{`{}`, "unknown"},
		{`invalid`, "unknown"},
	}
	for _, tc := range cases {
		got := extractToolName([]byte(tc.params))
		if got != tc.expected {
			t.Errorf("extractToolName(%s) = %q, want %q", tc.params, got, tc.expected)
		}
	}
}
