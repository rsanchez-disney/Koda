package ops

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"testing/quick"

	"github.disney.com/SANCR225/koda/internal/model"
)

// randAlphanumString generates a random alphanumeric string of length n using the given rand source.
func randAlphanumString(r *rand.Rand, n int) string {
	const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = chars[r.Intn(len(chars))]
	}
	return string(b)
}

// -----------------------------------------------------------------------
// Feature: mcp-install-command, Property 5: Token update round trip
// -----------------------------------------------------------------------

// tokenRoundTripInput holds a randomly chosen token key from model.KnownTokens
// and a random new value. When NewValue is empty the test verifies that the
// original value is preserved.
type tokenRoundTripInput struct {
	KeyIndex int    // index into model.KnownTokens
	NewValue string // empty means "user pressed Enter"
	OldValue string // pre-existing value in tokens.env
}

// Generate implements quick.Generator for tokenRoundTripInput.
func (tokenRoundTripInput) Generate(r *rand.Rand, size int) reflect.Value {
	idx := r.Intn(len(model.KnownTokens))
	// Decide whether the new value is empty (keep existing) or non-empty (update).
	var newVal string
	if r.Intn(3) > 0 { // ~67% chance of providing a new value
		newVal = randAlphanumString(r, 5+r.Intn(20))
	}
	oldVal := randAlphanumString(r, 5+r.Intn(20))
	return reflect.ValueOf(tokenRoundTripInput{
		KeyIndex: idx,
		NewValue: newVal,
		OldValue: oldVal,
	})
}

// TestTokenUpdateRoundTrip verifies that for any token key from model.KnownTokens
// and a non-empty new value, writing via WriteTokens and reading back via ReadTokens
// returns the same value. When the new value is empty, the original value is preserved.
//
// **Validates: Requirements 2.3, 2.4**
func TestTokenUpdateRoundTrip(t *testing.T) {
	prop := func(input tokenRoundTripInput) bool {
		// Override HOME to isolate each iteration.
		tmpHome := t.TempDir()
		origHome := os.Getenv("HOME")
		os.Setenv("HOME", tmpHome)
		defer os.Setenv("HOME", origHome)

		key := model.KnownTokens[input.KeyIndex].Key

		// 1. Seed the tokens.env with the old value.
		initial := map[string]string{key: input.OldValue}
		if err := WriteTokens(initial); err != nil {
			t.Logf("WriteTokens (seed) error: %v", err)
			return false
		}

		// 2. Simulate the user action: if NewValue is non-empty, write it;
		//    otherwise leave the file as-is (user pressed Enter).
		if input.NewValue != "" {
			updated := ReadTokens()
			updated[key] = input.NewValue
			if err := WriteTokens(updated); err != nil {
				t.Logf("WriteTokens (update) error: %v", err)
				return false
			}
		}

		// 3. Read back and verify.
		got := ReadTokens()
		expected := input.OldValue
		if input.NewValue != "" {
			expected = input.NewValue
		}

		if got[key] != expected {
			t.Logf("key=%s: got %q, want %q (old=%q, new=%q)",
				key, got[key], expected, input.OldValue, input.NewValue)
			return false
		}
		return true
	}

	cfg := &quick.Config{MaxCount: 100}
	if err := quick.Check(prop, cfg); err != nil {
		t.Errorf("Property 5 failed: %v", err)
	}
}

// -----------------------------------------------------------------------
// Feature: mcp-install-command, Property 6: GitHub remote persistence round trip
// -----------------------------------------------------------------------

// ghRemoteRoundTripInput holds a randomly generated GitHubRemote.
type ghRemoteRoundTripInput struct {
	Remote model.GitHubRemote
}

// Generate implements quick.Generator for ghRemoteRoundTripInput.
func (ghRemoteRoundTripInput) Generate(r *rand.Rand, size int) reflect.Value {
	name := randAlphanumString(r, 3+r.Intn(8))
	host := fmt.Sprintf("github%s.example.com", randAlphanumString(r, 3))
	token := randAlphanumString(r, 10+r.Intn(20))
	var apiPath string
	if r.Intn(2) == 1 {
		apiPath = "/api/v3"
	}
	return reflect.ValueOf(ghRemoteRoundTripInput{
		Remote: model.GitHubRemote{
			Name:    name,
			Host:    host,
			Token:   token,
			APIPath: apiPath,
		},
	})
}

// TestGitHubRemotePersistenceRoundTrip verifies that for any valid GitHubRemote
// (non-empty name, host, token, and optional API path), writing it via
// WriteGitHubRemote and reading back via ReadGitHubRemotes produces a remote
// with identical name, host, token, and API path fields.
//
// **Validates: Requirements 3.3**
func TestGitHubRemotePersistenceRoundTrip(t *testing.T) {
	prop := func(input ghRemoteRoundTripInput) bool {
		tmpHome := t.TempDir()
		origHome := os.Getenv("HOME")
		os.Setenv("HOME", tmpHome)
		defer os.Setenv("HOME", origHome)

		// Ensure the .kiro directory exists.
		os.MkdirAll(filepath.Join(tmpHome, ".kiro"), 0755)

		r := input.Remote
		WriteGitHubRemote(r)

		remotes := ReadGitHubRemotes()

		// Find the remote by name.
		var found *model.GitHubRemote
		for i := range remotes {
			if remotes[i].Name == r.Name {
				found = &remotes[i]
				break
			}
		}
		if found == nil {
			t.Logf("remote %q not found after write", r.Name)
			return false
		}
		if found.Host != r.Host {
			t.Logf("host mismatch: got %q, want %q", found.Host, r.Host)
			return false
		}
		if found.Token != r.Token {
			t.Logf("token mismatch: got %q, want %q", found.Token, r.Token)
			return false
		}
		if found.APIPath != r.APIPath {
			t.Logf("apiPath mismatch: got %q, want %q", found.APIPath, r.APIPath)
			return false
		}
		return true
	}

	cfg := &quick.Config{MaxCount: 100}
	if err := quick.Check(prop, cfg); err != nil {
		t.Errorf("Property 6 failed: %v", err)
	}
}

// -----------------------------------------------------------------------
// Feature: mcp-install-command, Property 7: GitHub remote upsert preserves count
// -----------------------------------------------------------------------

// ghUpsertInput holds a set of existing remotes and a new remote whose name
// matches one of the existing remotes.
type ghUpsertInput struct {
	Existing []model.GitHubRemote
	Updated  model.GitHubRemote
}

// Generate implements quick.Generator for ghUpsertInput.
func (ghUpsertInput) Generate(r *rand.Rand, size int) reflect.Value {
	count := 1 + r.Intn(4) // 1 to 4 existing remotes
	existing := make([]model.GitHubRemote, count)
	for i := range existing {
		existing[i] = model.GitHubRemote{
			Name:  randAlphanumString(r, 3+r.Intn(6)),
			Host:  fmt.Sprintf("github%s.example.com", randAlphanumString(r, 3)),
			Token: randAlphanumString(r, 10+r.Intn(10)),
		}
		if r.Intn(2) == 1 {
			existing[i].APIPath = "/api/v3"
		}
	}

	// Ensure unique names among existing remotes.
	seen := map[string]bool{}
	unique := existing[:0]
	for _, rem := range existing {
		if !seen[rem.Name] {
			seen[rem.Name] = true
			unique = append(unique, rem)
		}
	}
	existing = unique

	if len(existing) == 0 {
		// Fallback: ensure at least one existing remote.
		existing = []model.GitHubRemote{{
			Name:  randAlphanumString(r, 5),
			Host:  "github.fallback.com",
			Token: randAlphanumString(r, 12),
		}}
	}

	// Pick one existing name to update.
	targetIdx := r.Intn(len(existing))
	updated := model.GitHubRemote{
		Name:  existing[targetIdx].Name,
		Host:  fmt.Sprintf("newhost%s.example.com", randAlphanumString(r, 3)),
		Token: randAlphanumString(r, 10+r.Intn(10)),
	}
	if r.Intn(2) == 1 {
		updated.APIPath = "/api/v4"
	}

	return reflect.ValueOf(ghUpsertInput{
		Existing: existing,
		Updated:  updated,
	})
}

// TestGitHubRemoteUpsertPreservesCount verifies that for any existing set of
// GitHub remotes and a new remote whose name matches an existing remote, after
// calling WriteGitHubRemote the total number of remotes remains unchanged and
// the matching remote's host/token/API path reflect the new values.
//
// **Validates: Requirements 3.5**
func TestGitHubRemoteUpsertPreservesCount(t *testing.T) {
	prop := func(input ghUpsertInput) bool {
		tmpHome := t.TempDir()
		origHome := os.Getenv("HOME")
		os.Setenv("HOME", tmpHome)
		defer os.Setenv("HOME", origHome)

		os.MkdirAll(filepath.Join(tmpHome, ".kiro"), 0755)

		// Seed all existing remotes.
		for _, r := range input.Existing {
			WriteGitHubRemote(r)
		}

		countBefore := len(ReadGitHubRemotes())

		// Upsert the updated remote (same name as one existing).
		WriteGitHubRemote(input.Updated)

		remotes := ReadGitHubRemotes()
		countAfter := len(remotes)

		// Count must not change.
		if countAfter != countBefore {
			t.Logf("count changed: before=%d, after=%d (updated name=%q)",
				countBefore, countAfter, input.Updated.Name)
			return false
		}

		// Find the updated remote and verify fields.
		var found *model.GitHubRemote
		for i := range remotes {
			if remotes[i].Name == input.Updated.Name {
				found = &remotes[i]
				break
			}
		}
		if found == nil {
			t.Logf("remote %q not found after upsert", input.Updated.Name)
			return false
		}
		if found.Host != input.Updated.Host {
			t.Logf("host mismatch after upsert: got %q, want %q", found.Host, input.Updated.Host)
			return false
		}
		if found.Token != input.Updated.Token {
			t.Logf("token mismatch after upsert: got %q, want %q", found.Token, input.Updated.Token)
			return false
		}
		if found.APIPath != input.Updated.APIPath {
			t.Logf("apiPath mismatch after upsert: got %q, want %q", found.APIPath, input.Updated.APIPath)
			return false
		}
		return true
	}

	cfg := &quick.Config{MaxCount: 100}
	if err := quick.Check(prop, cfg); err != nil {
		t.Errorf("Property 7 failed: %v", err)
	}
}

// -----------------------------------------------------------------------
// Feature: mcp-install-command, Property 11: GitHub tool reference expansion
// -----------------------------------------------------------------------

// ghToolExpansionInput holds a list of 2+ GitHub remotes and a set of
// non-github tool entries to include alongside "@github/*" in the agent JSON.
type ghToolExpansionInput struct {
	Remotes    []model.GitHubRemote
	OtherTools []string // additional tool entries that should be preserved
}

// Generate implements quick.Generator for ghToolExpansionInput.
func (ghToolExpansionInput) Generate(r *rand.Rand, size int) reflect.Value {
	count := 2 + r.Intn(4) // 2 to 5 remotes
	remotes := make([]model.GitHubRemote, count)
	seen := map[string]bool{}
	for i := range remotes {
		var name string
		for {
			name = randAlphanumString(r, 3+r.Intn(6))
			if !seen[name] {
				seen[name] = true
				break
			}
		}
		remotes[i] = model.GitHubRemote{
			Name:  name,
			Host:  fmt.Sprintf("github%s.example.com", randAlphanumString(r, 3)),
			Token: randAlphanumString(r, 12),
		}
	}

	// Generate 0-3 other tool entries.
	otherCount := r.Intn(4)
	others := make([]string, otherCount)
	for i := range others {
		others[i] = fmt.Sprintf("@%s/*", randAlphanumString(r, 4+r.Intn(6)))
	}

	return reflect.ValueOf(ghToolExpansionInput{
		Remotes:    remotes,
		OtherTools: others,
	})
}

// TestGitHubToolReferenceExpansion verifies that for any list of 2+ GitHub
// remotes and any agent JSON file containing "@github/*" in its tools array,
// after expansion the tools array contains "@github-<name>/*" for each remote
// and does not contain the original "@github/*" entry.
//
// **Validates: Requirements 6.2**
func TestGitHubToolReferenceExpansion(t *testing.T) {
	prop := func(input ghToolExpansionInput) bool {
		tmpDir := t.TempDir()

		// Build the tools array: other tools + "@github/*".
		tools := make([]string, 0, len(input.OtherTools)+1)
		tools = append(tools, input.OtherTools...)
		tools = append(tools, "@github/*")

		// Create a minimal agent JSON file.
		agentData := map[string]interface{}{
			"name":  "test-agent",
			"tools": tools,
		}
		agentBytes, err := json.MarshalIndent(agentData, "", "  ")
		if err != nil {
			t.Logf("marshal error: %v", err)
			return false
		}

		agentPath := filepath.Join(tmpDir, "agent.json")
		if err := os.WriteFile(agentPath, agentBytes, 0644); err != nil {
			t.Logf("write error: %v", err)
			return false
		}

		// Build the expansion map (same logic as InjectAgentTokens).
		expansions := map[string][]string{}
		for _, r := range input.Remotes {
			expansions["@github/*"] = append(expansions["@github/*"], "@github-"+r.Name+"/*")
		}

		// Call the unexported expandToolRefs directly (same package).
		expandToolRefs(agentPath, expansions)

		// Read back and parse.
		data, err := os.ReadFile(agentPath)
		if err != nil {
			t.Logf("read error: %v", err)
			return false
		}

		var parsed map[string]json.RawMessage
		if err := json.Unmarshal(data, &parsed); err != nil {
			t.Logf("unmarshal error: %v", err)
			return false
		}

		var resultTools []string
		if err := json.Unmarshal(parsed["tools"], &resultTools); err != nil {
			t.Logf("unmarshal tools error: %v", err)
			return false
		}

		// Build a set of result tools for easy lookup.
		resultSet := map[string]bool{}
		for _, tool := range resultTools {
			resultSet[tool] = true
		}

		// 1. "@github/*" must NOT be present.
		if resultSet["@github/*"] {
			t.Logf("@github/* still present after expansion")
			return false
		}

		// 2. "@github-<name>/*" must be present for each remote.
		for _, r := range input.Remotes {
			expected := "@github-" + r.Name + "/*"
			if !resultSet[expected] {
				t.Logf("missing expanded tool ref %q", expected)
				return false
			}
		}

		// 3. All other tools must be preserved.
		for _, other := range input.OtherTools {
			if !resultSet[other] {
				t.Logf("other tool %q lost after expansion", other)
				return false
			}
		}

		// 4. Total count should be len(OtherTools) + len(Remotes).
		expectedCount := len(input.OtherTools) + len(input.Remotes)
		if len(resultTools) != expectedCount {
			t.Logf("tool count mismatch: got %d, want %d", len(resultTools), expectedCount)
			return false
		}

		return true
	}

	cfg := &quick.Config{MaxCount: 100}
	if err := quick.Check(prop, cfg); err != nil {
		t.Errorf("Property 11 failed: %v", err)
	}
}
