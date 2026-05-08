package model

import (
	"encoding/json"
	"testing"
)

func TestStringOrSlice_UnmarshalString(t *testing.T) {
	var s StringOrSlice
	if err := json.Unmarshal([]byte(`"DPAY-"`), &s); err != nil {
		t.Fatal(err)
	}
	if len(s) != 1 || s[0] != "DPAY-" {
		t.Fatalf("expected [DPAY-], got %v", s)
	}
}

func TestStringOrSlice_UnmarshalArray(t *testing.T) {
	var s StringOrSlice
	if err := json.Unmarshal([]byte(`["AEXP-", "IEXP-", "COREEXP-"]`), &s); err != nil {
		t.Fatal(err)
	}
	if len(s) != 3 || s[0] != "AEXP-" || s[2] != "COREEXP-" {
		t.Fatalf("expected [AEXP- IEXP- COREEXP-], got %v", s)
	}
}

func TestStringOrSlice_MarshalSingle(t *testing.T) {
	s := StringOrSlice{"DPAY-"}
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != `"DPAY-"` {
		t.Fatalf("expected \"DPAY-\", got %s", b)
	}
}

func TestStringOrSlice_MarshalMultiple(t *testing.T) {
	s := StringOrSlice{"AEXP-", "IEXP-"}
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != `["AEXP-","IEXP-"]` {
		t.Fatalf("expected array JSON, got %s", b)
	}
}

func TestStringOrSlice_String(t *testing.T) {
	tests := []struct {
		in   StringOrSlice
		want string
	}{
		{nil, ""},
		{StringOrSlice{"DPAY-"}, "DPAY-"},
		{StringOrSlice{"AEXP-", "IEXP-", "COREEXP-"}, "AEXP-, IEXP-, COREEXP-"},
	}
	for _, tt := range tests {
		if got := tt.in.String(); got != tt.want {
			t.Errorf("StringOrSlice(%v).String() = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestStringOrSlice_IsEmpty(t *testing.T) {
	tests := []struct {
		in   StringOrSlice
		want bool
	}{
		{nil, true},
		{StringOrSlice{}, true},
		{StringOrSlice{""}, true},
		{StringOrSlice{"DPAY-"}, false},
		{StringOrSlice{"A-", "B-"}, false},
	}
	for _, tt := range tests {
		if got := tt.in.IsEmpty(); got != tt.want {
			t.Errorf("StringOrSlice(%v).IsEmpty() = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestParseStringOrSlice(t *testing.T) {
	tests := []struct {
		in   string
		want StringOrSlice
	}{
		{"", nil},
		{"DPAY-", StringOrSlice{"DPAY-"}},
		{"AEXP-, IEXP-, COREEXP-", StringOrSlice{"AEXP-", "IEXP-", "COREEXP-"}},
		{"AEXP-,IEXP-", StringOrSlice{"AEXP-", "IEXP-"}},
	}
	for _, tt := range tests {
		got := ParseStringOrSlice(tt.in)
		if len(got) != len(tt.want) {
			t.Errorf("ParseStringOrSlice(%q) = %v, want %v", tt.in, got, tt.want)
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("ParseStringOrSlice(%q)[%d] = %q, want %q", tt.in, i, got[i], tt.want[i])
			}
		}
	}
}

func TestStringOrSlice_UnmarshalNull(t *testing.T) {
	var s StringOrSlice
	if err := json.Unmarshal([]byte(`null`), &s); err != nil {
		t.Fatal(err)
	}
	if !s.IsEmpty() {
		t.Fatalf("expected empty after null unmarshal, got %v", s)
	}
}

func TestStringOrSlice_UnmarshalEmptyArray(t *testing.T) {
	var s StringOrSlice
	if err := json.Unmarshal([]byte(`[]`), &s); err != nil {
		t.Fatal(err)
	}
	if !s.IsEmpty() {
		t.Fatalf("expected empty after [] unmarshal, got %v", s)
	}
}

func TestStringOrSlice_MarshalNil(t *testing.T) {
	var s StringOrSlice
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != `null` {
		t.Fatalf("expected null, got %s", b)
	}
}

func TestWorkspace_UnmarshalJiraPrefix(t *testing.T) {
	// Verify full workspace unmarshal works with both formats
	single := `{"name":"test","jira_prefix":"DPAY-","profiles":["dev-core"]}`
	var ws1 Workspace
	if err := json.Unmarshal([]byte(single), &ws1); err != nil {
		t.Fatal(err)
	}
	if ws1.JiraPrefix.String() != "DPAY-" {
		t.Fatalf("expected DPAY-, got %s", ws1.JiraPrefix.String())
	}

	multi := `{"name":"test","jira_prefix":["AEXP-","IEXP-"],"profiles":["dev-core"]}`
	var ws2 Workspace
	if err := json.Unmarshal([]byte(multi), &ws2); err != nil {
		t.Fatal(err)
	}
	if ws2.JiraPrefix.String() != "AEXP-, IEXP-" {
		t.Fatalf("expected AEXP-, IEXP-, got %s", ws2.JiraPrefix.String())
	}
}
