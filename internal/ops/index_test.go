package ops

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBuildContextIndex(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "api.md"), []byte("# API Standards\nUse REST endpoints with JSON responses. Pagination via cursor."), 0644)
	os.WriteFile(filepath.Join(dir, "security.md"), []byte("# Security\nNever store passwords in code. Use environment variables for secrets."), 0644)

	err := BuildContextIndex(dir)
	if err != nil {
		t.Fatal(err)
	}

	// Verify index file created
	indexPath := filepath.Join(dir, "_index.json")
	if _, err := os.Stat(indexPath); err != nil {
		t.Fatal("index file not created")
	}

	// Query for API-related content
	results, err := QueryContextIndex(dir, "REST API pagination", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 {
		t.Fatal("expected results for API query")
	}
	if results[0].File != "api.md" {
		t.Fatalf("expected api.md as top result, got %s", results[0].File)
	}

	// Query for security content
	results, err = QueryContextIndex(dir, "passwords secrets environment", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 {
		t.Fatal("expected results for security query")
	}
	if results[0].File != "security.md" {
		t.Fatalf("expected security.md as top result, got %s", results[0].File)
	}
}

func TestBuildContextIndex_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	err := BuildContextIndex(dir)
	if err != nil {
		t.Fatal(err)
	}
	// Should create index with 0 chunks
	results, err := QueryContextIndex(dir, "anything", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 {
		t.Fatalf("expected 0 results from empty index, got %d", len(results))
	}
}

func TestQueryContextIndex_MissingIndex(t *testing.T) {
	dir := t.TempDir()
	_, err := QueryContextIndex(dir, "test", 5)
	if err == nil {
		t.Fatal("expected error for missing index")
	}
}

func TestTokenize(t *testing.T) {
	tokens := tokenize("Hello World! This is a test-123 of tokenization.")
	// Should filter out words ≤2 chars ("is", "a", "of")
	for _, tok := range tokens {
		if len(tok) <= 2 {
			t.Errorf("token %q should have been filtered (≤2 chars)", tok)
		}
	}
	// Should contain "hello", "world", "test", "123", "tokenization"
	found := map[string]bool{}
	for _, tok := range tokens {
		found[tok] = true
	}
	for _, expected := range []string{"hello", "world", "this", "test", "123", "tokenization"} {
		if !found[expected] {
			t.Errorf("expected token %q not found in %v", expected, tokens)
		}
	}
}
