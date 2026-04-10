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

	"github.disney.com/SANCR225/koda/internal/config"
	"github.disney.com/SANCR225/koda/internal/model"
)

// Feature: mcp-install-command, Property 2: Discovery matches bundle directories

// discoverInput represents a randomised filesystem layout for property testing.
type discoverInput struct {
	// DirPresent[i] indicates whether knownServers[i].BundleDir should be created.
	DirPresent []bool
	// BundlePresent[i] indicates whether dist/index.cjs (regular) or package.json (NPM) exists.
	BundlePresent []bool
}

// Generate implements quick.Generator so testing/quick can produce random inputs.
func (discoverInput) Generate(rand *rand.Rand, size int) reflect.Value {
	n := len(knownServers)
	d := discoverInput{
		DirPresent:    make([]bool, n),
		BundlePresent: make([]bool, n),
	}
	for i := 0; i < n; i++ {
		d.DirPresent[i] = rand.Intn(2) == 1
		d.BundlePresent[i] = rand.Intn(2) == 1
	}
	return reflect.ValueOf(d)
}

// TestDiscoverServersMatchesBundleDirs is a property-based test verifying that
// DiscoverServers returns exactly the servers whose BundleDir exists on disk
// (plus SSE servers which are always present), with correct verification status.
//
// **Validates: Requirements 1.5**
func TestDiscoverServersMatchesBundleDirs(t *testing.T) {
	prop := func(input discoverInput) bool {
		// --- Setup temp directory tree ---
		tmpDir := t.TempDir()
		mcpDir := filepath.Join(tmpDir, config.ToolsDir, "mcp-servers")
		if err := os.MkdirAll(mcpDir, 0755); err != nil {
			t.Logf("setup error: %v", err)
			return false
		}

		// Create directories and optional bundle files based on random input.
		for i, srv := range knownServers {
			if srv.BundleDir == "" {
				continue // SSE servers have no bundle dir
			}
			if !input.DirPresent[i] {
				continue
			}
			dirPath := filepath.Join(mcpDir, srv.BundleDir)
			if err := os.MkdirAll(dirPath, 0755); err != nil {
				t.Logf("mkdir error: %v", err)
				return false
			}

			if input.BundlePresent[i] {
				if srv.IsNPM {
					// NPM servers: create package.json
					pkgPath := filepath.Join(dirPath, "package.json")
					if err := os.WriteFile(pkgPath, []byte(`{}`), 0644); err != nil {
						t.Logf("write error: %v", err)
						return false
					}
				} else {
					// Regular servers: create dist/index.cjs
					distDir := filepath.Join(dirPath, "dist")
					if err := os.MkdirAll(distDir, 0755); err != nil {
						t.Logf("mkdir error: %v", err)
						return false
					}
					cjsPath := filepath.Join(distDir, "index.cjs")
					if err := os.WriteFile(cjsPath, []byte("// bundle"), 0644); err != nil {
						t.Logf("write error: %v", err)
						return false
					}
				}
			}
		}

		// --- Call function under test ---
		available, verified := DiscoverServers(tmpDir)

		// --- Build expected sets ---
		availableSet := make(map[string]bool)
		for _, srv := range available {
			availableSet[srv.Name] = true
		}

		for i, srv := range knownServers {
			if srv.IsSSE {
				// SSE servers must always be available and verified.
				if !availableSet[srv.Name] {
					t.Logf("SSE server %q missing from available", srv.Name)
					return false
				}
				if !verified[srv.Name] {
					t.Logf("SSE server %q not verified", srv.Name)
					return false
				}
				continue
			}

			if input.DirPresent[i] {
				// Directory exists → server must be available.
				if !availableSet[srv.Name] {
					t.Logf("server %q dir present but not in available", srv.Name)
					return false
				}

				// Check verification status.
				if srv.IsNPM {
					// NPM: verified iff package.json exists.
					if verified[srv.Name] != input.BundlePresent[i] {
						t.Logf("NPM server %q verified=%v, want %v", srv.Name, verified[srv.Name], input.BundlePresent[i])
						return false
					}
				} else {
					// Regular: verified iff dist/index.cjs exists.
					if verified[srv.Name] != input.BundlePresent[i] {
						t.Logf("server %q verified=%v, want %v", srv.Name, verified[srv.Name], input.BundlePresent[i])
						return false
					}
				}
			} else {
				// Directory absent → server must NOT be available.
				if availableSet[srv.Name] {
					t.Logf("server %q dir absent but found in available", srv.Name)
					return false
				}
			}
		}

		// Verify no unexpected servers appear.
		knownNames := make(map[string]bool)
		for _, srv := range knownServers {
			knownNames[srv.Name] = true
		}
		for _, srv := range available {
			if !knownNames[srv.Name] {
				t.Logf("unexpected server %q in available", srv.Name)
				return false
			}
		}

		return true
	}

	cfg := &quick.Config{MaxCount: 200}
	if err := quick.Check(prop, cfg); err != nil {
		t.Errorf("Property 2 failed: %v", err)
	}
}

// Feature: mcp-install-command, Property 4: Token prompting set equals selected server requirements

// tokenSubsetInput uses a bitmask to select a random subset of knownServers.
type tokenSubsetInput struct {
	Mask uint16 // bit i selects knownServers[i]
}

// Generate implements quick.Generator for random bitmask generation.
func (tokenSubsetInput) Generate(r *rand.Rand, size int) reflect.Value {
	return reflect.ValueOf(tokenSubsetInput{Mask: uint16(r.Intn(1 << len(knownServers)))})
}

// TestRequiredTokensEqualsSelectedServerRequirements verifies that for any subset
// of knownServers, RequiredTokens returns tokens whose keys equal the deduplicated
// union of TokenKeys across the selected servers, preserving first-appearance order.
// When no selected server has TokenKeys, the result should be empty.
//
// **Validates: Requirements 2.1, 2.6, 2.7**
func TestRequiredTokensEqualsSelectedServerRequirements(t *testing.T) {
	// Build a lookup from model.KnownTokens so we know which keys have metadata.
	knownTokenMap := make(map[string]model.Token, len(model.KnownTokens))
	for _, tok := range model.KnownTokens {
		knownTokenMap[tok.Key] = tok
	}

	prop := func(input tokenSubsetInput) bool {
		// 1. Use bitmask to select a subset of knownServers.
		var selected []MCPServer
		for i, srv := range knownServers {
			if input.Mask&(1<<uint(i)) != 0 {
				selected = append(selected, srv)
			}
		}

		// 2. Manually compute the expected deduplicated union of TokenKeys.
		seen := make(map[string]bool)
		var expectedKeys []string
		for _, srv := range selected {
			for _, k := range srv.TokenKeys {
				if !seen[k] {
					seen[k] = true
					expectedKeys = append(expectedKeys, k)
				}
			}
		}

		// 3. Call RequiredTokens with the selected subset.
		got := RequiredTokens(selected)

		// 4. Extract keys from the returned tokens.
		var gotKeys []string
		for _, tok := range got {
			gotKeys = append(gotKeys, tok.Key)
		}

		// 5. Filter expected keys to only those present in model.KnownTokens
		//    (RequiredTokens only returns tokens that have metadata in KnownTokens).
		var expectedFiltered []string
		for _, k := range expectedKeys {
			if _, ok := knownTokenMap[k]; ok {
				expectedFiltered = append(expectedFiltered, k)
			}
		}

		// 6. When no servers are selected or none have TokenKeys, verify empty result.
		if len(expectedFiltered) == 0 {
			if len(gotKeys) != 0 {
				t.Logf("expected empty tokens, got %v (mask=%b)", gotKeys, input.Mask)
				return false
			}
			return true
		}

		// 7. Verify same length.
		if len(gotKeys) != len(expectedFiltered) {
			t.Logf("length mismatch: got %d, want %d (mask=%b)", len(gotKeys), len(expectedFiltered), input.Mask)
			return false
		}

		// 8. Verify same keys in same order (first-appearance order).
		for i := range expectedFiltered {
			if gotKeys[i] != expectedFiltered[i] {
				t.Logf("key mismatch at index %d: got %q, want %q (mask=%b)", i, gotKeys[i], expectedFiltered[i], input.Mask)
				return false
			}
		}

		return true
	}

	cfg := &quick.Config{MaxCount: 200}
	if err := quick.Check(prop, cfg); err != nil {
		t.Errorf("Property 4 failed: %v", err)
	}
}

// Feature: mcp-install-command, Property 3: Generated config contains exactly selected and verified servers

// configInput drives Property 3 by randomly selecting a subset of knownServers,
// choosing a random number of GitHub remotes (0-2), and toggling whether
// COMPASS_TOKEN is non-empty.
type configInput struct {
	Mask          uint16 // bit i selects knownServers[i]
	GitHubRemotes uint8  // 0, 1, or 2 remotes
	CompassToken  bool   // whether COMPASS_TOKEN is non-empty
}

// Generate implements quick.Generator for configInput.
func (configInput) Generate(r *rand.Rand, size int) reflect.Value {
	return reflect.ValueOf(configInput{
		Mask:          uint16(r.Intn(1 << len(knownServers))),
		GitHubRemotes: uint8(r.Intn(3)), // 0, 1, or 2
		CompassToken:  r.Intn(2) == 1,
	})
}

// TestGeneratedConfigContainsExactlySelectedServers verifies that for any subset
// of selected servers and any GitHub remote configuration, the generated mcp.json
// contains entries for exactly those servers that are both selected AND should
// appear (compass only if token non-empty, github based on remote count), and
// no others.
//
// **Validates: Requirements 1.3, 4.2, 5.1, 5.2**
func TestGeneratedConfigContainsExactlySelectedServers(t *testing.T) {
	prop := func(input configInput) bool {
		// --- Override HOME to a temp dir ---
		tmpHome := t.TempDir()
		origHome := os.Getenv("HOME")
		os.Setenv("HOME", tmpHome)
		defer os.Setenv("HOME", origHome)

		// Create the settings directory that GenerateMCPConfig writes to.
		settingsDir := filepath.Join(tmpHome, ".kiro", config.SettingsDir)
		if err := os.MkdirAll(settingsDir, 0755); err != nil {
			t.Logf("setup error: %v", err)
			return false
		}

		// --- Select servers from bitmask ---
		var selected []MCPServer
		for i, srv := range knownServers {
			if input.Mask&(1<<uint(i)) != 0 {
				selected = append(selected, srv)
			}
		}

		// --- Build dummy tokens and envVars for all possible keys ---
		tokens := map[string]string{
			"JIRA_PAT":       "tok-jira",
			"CONFLUENCE_PAT": "tok-confluence",
			"MYWIKI_PAT":     "tok-mywiki",
			"FIGMA_TOKEN":    "tok-figma",
			"COMPASS_TOKEN":  "",
			"HARNESS_API_KEY": "tok-harness",
			"SONARQUBE_TOKEN": "tok-sonar",
		}
		if input.CompassToken {
			tokens["COMPASS_TOKEN"] = "tok-compass"
		}

		envVars := map[string]string{
			"CONFLUENCE_URL": "https://confluence.example.com",
			"MYWIKI_URL":     "https://mywiki.example.com",
			"JIRA_URL":       "https://jira.example.com",
			"COMPASS_URL":    "https://compass.example.com/api/mcp",
		}

		// --- Build GitHub remotes ---
		var ghRemotes []model.GitHubRemote
		for j := uint8(0); j < input.GitHubRemotes; j++ {
			ghRemotes = append(ghRemotes, model.GitHubRemote{
				Name:  fmt.Sprintf("remote%d", j),
				Host:  fmt.Sprintf("github%d.example.com", j),
				Token: fmt.Sprintf("gh-tok-%d", j),
			})
		}

		// --- Call GenerateMCPConfig ---
		mcpPath, err := GenerateMCPConfig(selected, ghRemotes, tokens, envVars)
		if err != nil {
			t.Logf("GenerateMCPConfig error: %v", err)
			return false
		}

		// --- Read back and parse the generated mcp.json ---
		data, err := os.ReadFile(mcpPath)
		if err != nil {
			t.Logf("read mcp.json error: %v", err)
			return false
		}

		var parsed struct {
			MCPServers map[string]json.RawMessage `json:"mcpServers"`
		}
		if err := json.Unmarshal(data, &parsed); err != nil {
			t.Logf("unmarshal error: %v", err)
			return false
		}

		// --- Build expected key set ---
		expectedKeys := make(map[string]bool)

		githubSelected := false
		for _, srv := range selected {
			switch {
			case srv.Name == "github":
				githubSelected = true
				// GitHub keys depend on remote count; handled below.

			case srv.IsSSE:
				// Compass: only if COMPASS_TOKEN is non-empty.
				if input.CompassToken {
					expectedKeys[srv.Name] = true
				}

			default:
				// Regular and NPM servers: included by name.
				expectedKeys[srv.Name] = true
			}
		}

		// GitHub entries based on remote count.
		if githubSelected && len(ghRemotes) > 0 {
			if len(ghRemotes) == 1 {
				expectedKeys["github"] = true
			} else {
				for _, r := range ghRemotes {
					expectedKeys["github-"+r.Name] = true
				}
			}
		}

		// --- Collect actual keys ---
		actualKeys := make(map[string]bool)
		for k := range parsed.MCPServers {
			actualKeys[k] = true
		}

		// --- Compare ---
		// Check no missing keys.
		for k := range expectedKeys {
			if !actualKeys[k] {
				t.Logf("missing key %q in mcp.json (mask=%b, ghRemotes=%d, compassToken=%v)",
					k, input.Mask, input.GitHubRemotes, input.CompassToken)
				return false
			}
		}

		// Check no extra keys.
		for k := range actualKeys {
			if !expectedKeys[k] {
				t.Logf("unexpected key %q in mcp.json (mask=%b, ghRemotes=%d, compassToken=%v)",
					k, input.Mask, input.GitHubRemotes, input.CompassToken)
				return false
			}
		}

		return true
	}

	cfg := &quick.Config{MaxCount: 200}
	if err := quick.Check(prop, cfg); err != nil {
		t.Errorf("Property 3 failed: %v", err)
	}
}

// Feature: mcp-install-command, Property 8: GitHub config naming depends on remote count

// ghNamingInput generates a random list of 1-5 GitHubRemote entries for property testing.
type ghNamingInput struct {
	Remotes []model.GitHubRemote
}

// Generate implements quick.Generator for ghNamingInput.
func (ghNamingInput) Generate(r *rand.Rand, size int) reflect.Value {
	count := 1 + r.Intn(5) // 1 to 5 remotes
	remotes := make([]model.GitHubRemote, count)
	for i := 0; i < count; i++ {
		nameLen := 3 + r.Intn(6) // 3-8 chars
		name := make([]byte, nameLen)
		for j := range name {
			const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
			name[j] = chars[r.Intn(len(chars))]
		}
		remotes[i] = model.GitHubRemote{
			Name:  string(name),
			Host:  fmt.Sprintf("github-%s.com", string(name)),
			Token: fmt.Sprintf("ghp_%d", r.Int()),
		}
		// Optionally add APIPath
		if r.Intn(2) == 1 {
			remotes[i].APIPath = "/api/v3"
		}
	}
	return reflect.ValueOf(ghNamingInput{Remotes: remotes})
}

// TestGitHubConfigNamingDependsOnRemoteCount verifies that for any single
// GitHubRemote the generated config contains a key named "github", and for
// any list of 2+ GitHubRemote entries the config contains "github-<name>"
// keys for each remote and no plain "github" key.
//
// **Validates: Requirements 3.7, 3.8**
func TestGitHubConfigNamingDependsOnRemoteCount(t *testing.T) {
	// Build a selected list that always includes the github server.
	var githubServer MCPServer
	for _, srv := range knownServers {
		if srv.Name == "github" {
			githubServer = srv
			break
		}
	}
	selected := []MCPServer{githubServer}

	prop := func(input ghNamingInput) bool {
		// --- Override HOME to a temp dir ---
		tmpHome := t.TempDir()
		origHome := os.Getenv("HOME")
		os.Setenv("HOME", tmpHome)
		defer os.Setenv("HOME", origHome)

		// Create the settings directory that GenerateMCPConfig writes to.
		settingsDir := filepath.Join(tmpHome, ".kiro", config.SettingsDir)
		if err := os.MkdirAll(settingsDir, 0755); err != nil {
			t.Logf("setup error: %v", err)
			return false
		}

		// Tokens and envVars are not relevant for GitHub naming; provide empty maps.
		tokens := map[string]string{}
		envVars := map[string]string{}

		// --- Call GenerateMCPConfig ---
		mcpPath, err := GenerateMCPConfig(selected, input.Remotes, tokens, envVars)
		if err != nil {
			t.Logf("GenerateMCPConfig error: %v", err)
			return false
		}

		// --- Read back and parse the generated mcp.json ---
		data, err := os.ReadFile(mcpPath)
		if err != nil {
			t.Logf("read mcp.json error: %v", err)
			return false
		}

		var parsed struct {
			MCPServers map[string]json.RawMessage `json:"mcpServers"`
		}
		if err := json.Unmarshal(data, &parsed); err != nil {
			t.Logf("unmarshal error: %v", err)
			return false
		}

		if len(input.Remotes) == 1 {
			// Single remote: expect "github" key, no "github-<name>" keys.
			if _, ok := parsed.MCPServers["github"]; !ok {
				t.Logf("single remote: expected 'github' key, got keys: %v", keysOf(parsed.MCPServers))
				return false
			}
			for k := range parsed.MCPServers {
				if len(k) > len("github") && k[:7] == "github-" {
					t.Logf("single remote: unexpected prefixed key %q", k)
					return false
				}
			}
		} else {
			// 2+ remotes: expect "github-<name>" for each, no plain "github".
			if _, ok := parsed.MCPServers["github"]; ok {
				t.Logf("multi remote: unexpected plain 'github' key")
				return false
			}
			for _, r := range input.Remotes {
				expectedKey := "github-" + r.Name
				if _, ok := parsed.MCPServers[expectedKey]; !ok {
					t.Logf("multi remote: missing key %q, got keys: %v", expectedKey, keysOf(parsed.MCPServers))
					return false
				}
			}
		}

		return true
	}

	cfg := &quick.Config{MaxCount: 100}
	if err := quick.Check(prop, cfg); err != nil {
		t.Errorf("Property 8 failed: %v", err)
	}
}

// keysOf returns the keys of a map for diagnostic logging.
func keysOf(m map[string]json.RawMessage) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// Feature: mcp-install-command, Property 9: Server entry structure correctness

// parsedEntry represents a single server entry parsed from the generated mcp.json.
type parsedEntry struct {
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Env     map[string]string `json:"env"`
	URL     string            `json:"url"`
	Type    string            `json:"type"`
	Headers map[string]string `json:"headers"`
}

// entryStructureInput uses a bitmask to select a random subset of knownServers
// (excluding github) for property testing of server entry structure.
type entryStructureInput struct {
	Mask uint16 // bit i selects knownServers[i] (github bit is ignored)
}

// Generate implements quick.Generator for entryStructureInput.
func (entryStructureInput) Generate(r *rand.Rand, size int) reflect.Value {
	return reflect.ValueOf(entryStructureInput{Mask: uint16(r.Intn(1 << len(knownServers)))})
}

// TestServerEntryStructureCorrectness verifies that for any subset of selected
// servers (excluding github), the generated config entries have the correct
// structure:
//   - Non-SSE, non-NPM: command=="node", args contains bundle path with BundleDir, env has all TokenKeys and EnvKeys
//   - NPM (context7): command=="npx", args contains "-y" and "@upstash/context7-mcp"
//   - SSE (compass): type=="sse", url is set, headers has "Authorization" starting with "Bearer "
//
// **Validates: Requirements 4.3, 4.4**
func TestServerEntryStructureCorrectness(t *testing.T) {
	// Identify the github server index so we can skip it.
	githubIdx := -1
	for i, srv := range knownServers {
		if srv.Name == "github" {
			githubIdx = i
			break
		}
	}

	prop := func(input entryStructureInput) bool {
		// --- Override HOME to a temp dir ---
		tmpHome := t.TempDir()
		origHome := os.Getenv("HOME")
		os.Setenv("HOME", tmpHome)
		defer os.Setenv("HOME", origHome)

		settingsDir := filepath.Join(tmpHome, ".kiro", config.SettingsDir)
		if err := os.MkdirAll(settingsDir, 0755); err != nil {
			t.Logf("setup error: %v", err)
			return false
		}

		// --- Select servers from bitmask, excluding github ---
		var selected []MCPServer
		for i, srv := range knownServers {
			if i == githubIdx {
				continue // skip github to simplify; it's tested in Property 8
			}
			if input.Mask&(1<<uint(i)) != 0 {
				selected = append(selected, srv)
			}
		}

		// If nothing selected, the property trivially holds.
		if len(selected) == 0 {
			return true
		}

		// --- Provide dummy tokens and envVars for all possible keys ---
		tokens := map[string]string{
			"JIRA_PAT":        "tok-jira",
			"CONFLUENCE_PAT":  "tok-confluence",
			"MYWIKI_PAT":      "tok-mywiki",
			"FIGMA_TOKEN":     "tok-figma",
			"COMPASS_TOKEN":   "tok-compass",
			"HARNESS_API_KEY":  "tok-harness",
			"SONARQUBE_TOKEN":  "tok-sonar",
		}

		envVars := map[string]string{
			"CONFLUENCE_URL": "https://confluence.example.com",
			"MYWIKI_URL":     "https://mywiki.example.com",
			"JIRA_URL":       "https://jira.example.com",
			"COMPASS_URL":    "https://compass.example.com/api/mcp",
		}

		// No GitHub remotes (github excluded from selection).
		var ghRemotes []model.GitHubRemote

		// --- Call GenerateMCPConfig ---
		mcpPath, err := GenerateMCPConfig(selected, ghRemotes, tokens, envVars)
		if err != nil {
			t.Logf("GenerateMCPConfig error: %v", err)
			return false
		}

		// --- Read back and parse the generated mcp.json ---
		data, err := os.ReadFile(mcpPath)
		if err != nil {
			t.Logf("read mcp.json error: %v", err)
			return false
		}

		var parsed struct {
			MCPServers map[string]parsedEntry `json:"mcpServers"`
		}
		if err := json.Unmarshal(data, &parsed); err != nil {
			t.Logf("unmarshal error: %v", err)
			return false
		}

		// --- Verify each selected server's entry structure ---
		for _, srv := range selected {
			entry, ok := parsed.MCPServers[srv.Name]
			if !ok {
				// Compass without token won't appear, but we always provide COMPASS_TOKEN above.
				t.Logf("missing entry for server %q", srv.Name)
				return false
			}

			switch {
			case srv.IsSSE:
				// SSE (compass): type=="sse", url is set, headers has "Authorization" starting with "Bearer "
				if entry.Type != "sse" {
					t.Logf("SSE server %q: expected type 'sse', got %q", srv.Name, entry.Type)
					return false
				}
				if entry.URL == "" {
					t.Logf("SSE server %q: url is empty", srv.Name)
					return false
				}
				authHeader, hasAuth := entry.Headers["Authorization"]
				if !hasAuth {
					t.Logf("SSE server %q: missing Authorization header", srv.Name)
					return false
				}
				if len(authHeader) < 7 || authHeader[:7] != "Bearer " {
					t.Logf("SSE server %q: Authorization header %q does not start with 'Bearer '", srv.Name, authHeader)
					return false
				}

			case srv.IsNPM:
				// NPM (context7): command=="npx", args contains "-y" and "@upstash/context7-mcp"
				if entry.Command != "npx" {
					t.Logf("NPM server %q: expected command 'npx', got %q", srv.Name, entry.Command)
					return false
				}
				hasY := false
				hasPkg := false
				for _, arg := range entry.Args {
					if arg == "-y" {
						hasY = true
					}
					if arg == "@upstash/context7-mcp" {
						hasPkg = true
					}
				}
				if !hasY {
					t.Logf("NPM server %q: args %v missing '-y'", srv.Name, entry.Args)
					return false
				}
				if !hasPkg {
					t.Logf("NPM server %q: args %v missing '@upstash/context7-mcp'", srv.Name, entry.Args)
					return false
				}

			default:
				// Regular node-based server: command=="node", args contains bundle path with BundleDir, env has all TokenKeys and EnvKeys
				if entry.Command != "node" {
					t.Logf("server %q: expected command 'node', got %q", srv.Name, entry.Command)
					return false
				}

				// Verify args contains a path with the BundleDir.
				expectedPathSuffix := filepath.Join("mcp-servers", srv.BundleDir, "dist", "index.cjs")
				foundBundle := false
				for _, arg := range entry.Args {
					if len(arg) >= len(expectedPathSuffix) &&
						arg[len(arg)-len(expectedPathSuffix):] == expectedPathSuffix {
						foundBundle = true
						break
					}
				}
				if !foundBundle {
					t.Logf("server %q: args %v missing bundle path containing %q", srv.Name, entry.Args, expectedPathSuffix)
					return false
				}

				// Verify env contains all TokenKeys.
				for _, tk := range srv.TokenKeys {
					if _, ok := entry.Env[tk]; !ok {
						t.Logf("server %q: env missing token key %q", srv.Name, tk)
						return false
					}
				}

				// Verify env contains all EnvKeys.
				for _, ek := range srv.EnvKeys {
					if _, ok := entry.Env[ek]; !ok {
						t.Logf("server %q: env missing env key %q", srv.Name, ek)
						return false
					}
				}
			}
		}

		return true
	}

	cfg := &quick.Config{MaxCount: 100}
	if err := quick.Check(prop, cfg); err != nil {
		t.Errorf("Property 9 failed: %v", err)
	}
}

// Feature: mcp-install-command, Property 10: Config generation is idempotent

// TestConfigGenerationIsIdempotent verifies that for any set of selected servers,
// tokens, env vars, and GitHub remotes, calling GenerateMCPConfig twice with the
// same inputs produces byte-identical output files.
//
// **Validates: Requirements 4.5**
func TestConfigGenerationIsIdempotent(t *testing.T) {
	prop := func(input configInput) bool {
		// --- Override HOME to a temp dir ---
		tmpHome := t.TempDir()
		origHome := os.Getenv("HOME")
		os.Setenv("HOME", tmpHome)
		defer os.Setenv("HOME", origHome)

		settingsDir := filepath.Join(tmpHome, ".kiro", config.SettingsDir)
		if err := os.MkdirAll(settingsDir, 0755); err != nil {
			t.Logf("setup error: %v", err)
			return false
		}

		// --- Select servers from bitmask ---
		var selected []MCPServer
		for i, srv := range knownServers {
			if input.Mask&(1<<uint(i)) != 0 {
				selected = append(selected, srv)
			}
		}

		// --- Build tokens and envVars ---
		tokens := map[string]string{
			"JIRA_PAT":        "tok-jira",
			"CONFLUENCE_PAT":  "tok-confluence",
			"MYWIKI_PAT":      "tok-mywiki",
			"FIGMA_TOKEN":     "tok-figma",
			"COMPASS_TOKEN":   "",
			"HARNESS_API_KEY":  "tok-harness",
			"SONARQUBE_TOKEN":  "tok-sonar",
		}
		if input.CompassToken {
			tokens["COMPASS_TOKEN"] = "tok-compass"
		}

		envVars := map[string]string{
			"CONFLUENCE_URL": "https://confluence.example.com",
			"MYWIKI_URL":     "https://mywiki.example.com",
			"JIRA_URL":       "https://jira.example.com",
			"COMPASS_URL":    "https://compass.example.com/api/mcp",
		}

		// --- Build GitHub remotes (0, 1, or 2) ---
		var ghRemotes []model.GitHubRemote
		for j := uint8(0); j < input.GitHubRemotes; j++ {
			ghRemotes = append(ghRemotes, model.GitHubRemote{
				Name:  fmt.Sprintf("remote%d", j),
				Host:  fmt.Sprintf("github%d.example.com", j),
				Token: fmt.Sprintf("gh-tok-%d", j),
			})
		}

		// --- First call ---
		mcpPath1, err := GenerateMCPConfig(selected, ghRemotes, tokens, envVars)
		if err != nil {
			t.Logf("first GenerateMCPConfig error: %v", err)
			return false
		}
		data1, err := os.ReadFile(mcpPath1)
		if err != nil {
			t.Logf("first read error: %v", err)
			return false
		}

		// --- Second call with identical inputs ---
		mcpPath2, err := GenerateMCPConfig(selected, ghRemotes, tokens, envVars)
		if err != nil {
			t.Logf("second GenerateMCPConfig error: %v", err)
			return false
		}
		data2, err := os.ReadFile(mcpPath2)
		if err != nil {
			t.Logf("second read error: %v", err)
			return false
		}

		// --- Paths must be the same ---
		if mcpPath1 != mcpPath2 {
			t.Logf("paths differ: %q vs %q", mcpPath1, mcpPath2)
			return false
		}

		// --- File contents must be byte-identical ---
		if len(data1) != len(data2) {
			t.Logf("length mismatch: %d vs %d (mask=%b, ghRemotes=%d, compassToken=%v)",
				len(data1), len(data2), input.Mask, input.GitHubRemotes, input.CompassToken)
			return false
		}
		for i := range data1 {
			if data1[i] != data2[i] {
				t.Logf("byte mismatch at offset %d (mask=%b, ghRemotes=%d, compassToken=%v)",
					i, input.Mask, input.GitHubRemotes, input.CompassToken)
				return false
			}
		}

		return true
	}

	cfg := &quick.Config{MaxCount: 100}
	if err := quick.Check(prop, cfg); err != nil {
		t.Errorf("Property 10 failed: %v", err)
	}
}
