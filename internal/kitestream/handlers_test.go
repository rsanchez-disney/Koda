package kitestream

import (
	"testing"
)

func TestExtractID(t *testing.T) {
	tests := []struct {
		path, prefix, suffix, want string
	}{
		{"/sessions/abc123/message", "/sessions/", "/message", "abc123"},
		{"/sessions/xyz/abort", "/sessions/", "/abort", "xyz"},
		{"/pipelines/p1/stream", "/pipelines/", "/stream", "p1"},
	}
	for _, tt := range tests {
		got := extractID(tt.path, tt.prefix, tt.suffix)
		if got != tt.want {
			t.Errorf("extractID(%q, %q, %q) = %q, want %q", tt.path, tt.prefix, tt.suffix, got, tt.want)
		}
	}
}
