package cli

import "testing"

func TestParseToggleInput(t *testing.T) {
	tests := []struct {
		name  string
		input string
		max   int
		want  []int
	}{
		{"single number", "3", 9, []int{2}},
		{"comma separated", "1,3,5", 9, []int{0, 2, 4}},
		{"with spaces", "1, 3, 5", 9, []int{0, 2, 4}},
		{"out of range high", "10", 9, nil},
		{"out of range zero", "0", 9, nil},
		{"negative", "-1", 9, nil},
		{"non-numeric", "abc", 9, nil},
		{"mixed valid and invalid", "1,abc,3,99", 9, []int{0, 2}},
		{"empty string", "", 9, nil},
		{"just commas", ",,", 9, nil},
		{"boundary max", "9", 9, []int{8}},
		{"boundary one", "1", 9, []int{0}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseToggleInput(tt.input, tt.max)
			if len(got) != len(tt.want) {
				t.Fatalf("parseToggleInput(%q, %d) = %v, want %v", tt.input, tt.max, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("index %d: got %d, want %d", i, got[i], tt.want[i])
				}
			}
		})
	}
}
