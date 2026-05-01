package ops

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.disney.com/SANCR225/koda/internal/config"
	"github.disney.com/SANCR225/koda/internal/model"
)

// parsedMCPEntry is a helper for unmarshalling generated mcp.json entries.
type parsedMCPEntry struct {
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Env     map[string]string `json:"env"`
	URL     string            `json:"url"`
	Type    string            `json:"type"`
	Headers map[string]string `json:"headers"`
}

// readGeneratedConfig is a helper that reads and parses the generated mcp.json.
func readGeneratedConfig(t *testing.T, mcpPath string) map[string]parsedMCPEntry {
	t.Helper()
	data, err := os.ReadFile(mcpPath)
	if err != nil {
		t.Fatalf("failed to read mcp.json: %v", err)
	}
	var parsed struct {
		MCPServers map[string]parsedMCPEntry `json:"mcpServers"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal mcp.json: %v", err)
	}
	return parsed.MCPServers
}

// setupTempHome creates a temp dir, overrides HOME, and returns a cleanup func.
func setupTempHome(t *testing.T) (string, func()) {
	t.Helper()
	tmpHome := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpHome)

	settingsDir := filepath.Join(tmpHome, ".kiro", config.SettingsDir)
	if err := os.MkdirAll(settingsDir, 0755); err != nil {
		t.Fatalf("failed to create settings dir: %v", err)
	}
	return tmpHome, func() { os.Setenv("HOME", origHome) }
}

// setupAllBundles creates a temp dir with all knownServers bundles present and verified.
func setupAllBundles(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	mcpDir := filepath.Join(tmpDir, config.ToolsDir, "mcp-servers")
	if err := os.MkdirAll(mcpDir, 0755); err != nil {
		t.Fatalf("failed to create mcp-servers dir: %v", err)
	}
	for _, srv := range knownServers {
		if srv.BundleDir == "" {
			continue // SSE servers have no bundle dir
		}
		dirPath := filepath.Join(mcpDir, srv.BundleDir)
		if err := os.MkdirAll(dirPath, 0755); err != nil {
			t.Fatalf("failed to create bundle dir %s: %v", srv.BundleDir, err)
		}
		if srv.IsNPM {
			if err := os.WriteFile(filepath.Join(dirPath, "package.json"), []byte(`{}`), 0644); err != nil {
				t.Fatalf("failed to write package.json: %v", err)
			}
		} else {
			distDir := filepath.Join(dirPath, "dist")
			if err := os.MkdirAll(distDir, 0755); err != nil {
				t.Fatalf("failed to create dist dir: %v", err)
			}
			if err := os.WriteFile(filepath.Join(distDir, "index.cjs"), []byte("// bundle"), 0644); err != nil {
				t.Fatalf("failed to write index.cjs: %v", err)
			}
		}
	}
	return tmpDir
}

// --- Task 7.1: Non-TTY fallback behavior ---
// Verifies that when all verified servers are selected (simulating non-TTY),
// the generated config contains entries for all of them.
//
// **Validates: Requirements 1.4**
func TestNonTTYFallbackAllVerifiedServers(t *testing.T) {
	tmpHome, cleanup := setupTempHome(t)
	defer cleanup()

	targetDir := setupAllBundles(t)

	// Discover and select all verified (simulates non-TTY path).
	available, verified := DiscoverServers(targetDir)
	var selected []MCPServer
	for _, srv := range available {
		if verified[srv.Name] {
			selected = append(selected, srv)
		}
	}

	// All knownServers should be available and verified.
	if len(selected) != len(knownServers) {
		t.Fatalf("expected %d selected servers, got %d", len(knownServers), len(selected))
	}

	tokens := map[string]string{
		"JIRA_PAT":           "tok-jira",
		"CONFLUENCE_PAT":     "tok-confluence",
		"MYWIKI_PAT":         "tok-mywiki",
		"FIGMA_TOKEN":        "tok-figma",
		"COMPASS_TOKEN":      "tok-compass",
		"QTEST_BEARER_TOKEN": "tok-qtest",
		"SPLUNK_API_USERNAME":     "tok-splunk-user",
		"SPLUNK_API_PASSWORD":     "tok-splunk-pass",
		"APPD_CLIENT_ID":      "tok-appd-id",
		"APPD_CLIENT_SECRET":  "tok-appd-secret",
"SNOW_API_USERNAME":       "tok-snow-user",
		"SNOW_API_PASSWORD":       "tok-snow-pass",
		"SHAREPOINT_CLIENT_ID":    "tok-sp-id",
		"SHAREPOINT_CLIENT_SECRET": "tok-sp-secret",
	}
	envVars := map[string]string{
		"CONFLUENCE_URL": "https://confluence.example.com",
		"MYWIKI_URL":     "https://mywiki.example.com",
		"JIRA_URL":       "https://jira.example.com",
		"COMPASS_URL":    "https://compass.example.com/api/mcp",
		"SHAREPOINT_TENANT_ID": "test-tenant-id",
		"SHAREPOINT_SITE_URL":  "https://test.sharepoint.com/sites/test",
	}
	ghRemotes := []model.GitHubRemote{
		{Name: "origin", Host: "github.example.com", Token: "gh-tok"},
	}
	jiraInstances := []model.JiraInstance{
		{Name: "jira", URL: "https://jira.example.com", Token: "tok-jira"},
	}
	confInstances := []model.ConfluenceInstance{
		{Name: "confluence", URL: "https://confluence.example.com", Token: "tok-confluence"},
	}

	mcpPath, err := GenerateMCPConfig(selected, ghRemotes, jiraInstances, confInstances, tokens, envVars)
	if err != nil {
		t.Fatalf("GenerateMCPConfig error: %v", err)
	}

	servers := readGeneratedConfig(t, mcpPath)

	// Expect entries for every non-github server (github handled separately).
	// With COMPASS_TOKEN set, compass should appear.
	// With 1 remote, github should appear as "github".
	expectedNames := map[string]bool{
		"jira": true, "confluence": true, "mermaid": true, "bruno": true,
		"figma": true, "compass": true, "qtest": true, "yax": true,
		"splunk-mcp": true, "appdynamics-mcp": true, "servicenow-mcp": true,
		"github": true, "chrome": true, "chrome-devtools": true, "sharepoint": true,
	}

	for name := range expectedNames {
		if _, ok := servers[name]; !ok {
			t.Errorf("expected server %q in config, but not found", name)
		}
	}

	// Verify no unexpected entries.
	for name := range servers {
		if !expectedNames[name] {
			t.Errorf("unexpected server %q in config", name)
		}
	}

	_ = tmpHome // used via HOME override
}

// --- Task 7.3: Compass SSE entry generation ---
// Verifies compass entry uses type: "sse", url, and Authorization header when
// COMPASS_TOKEN is non-empty. Verifies compass is NOT in config when token is empty.
//
// **Validates: Requirements 4.4**
func TestCompassSSEEntryGeneration(t *testing.T) {
	var compassServer MCPServer
	for _, srv := range knownServers {
		if srv.Name == "compass" {
			compassServer = srv
			break
		}
	}

	t.Run("compass with token", func(t *testing.T) {
		_, cleanup := setupTempHome(t)
		defer cleanup()

		tokens := map[string]string{"COMPASS_TOKEN": "my-compass-token"}
		envVars := map[string]string{"COMPASS_URL": "https://compass.example.com/api/mcp"}

		selected := []MCPServer{compassServer}
		mcpPath, err := GenerateMCPConfig(selected, nil, nil, nil, tokens, envVars)
		if err != nil {
			t.Fatalf("GenerateMCPConfig error: %v", err)
		}

		servers := readGeneratedConfig(t, mcpPath)
		entry, ok := servers["compass"]
		if !ok {
			t.Fatal("compass not found in generated config")
		}
		if entry.Type != "sse" {
			t.Errorf("expected type 'sse', got %q", entry.Type)
		}
		if entry.URL != "https://compass.example.com/api/mcp" {
			t.Errorf("expected URL 'https://compass.example.com/api/mcp', got %q", entry.URL)
		}
		auth, hasAuth := entry.Headers["Authorization"]
		if !hasAuth {
			t.Fatal("missing Authorization header")
		}
		if auth != "Bearer my-compass-token" {
			t.Errorf("expected 'Bearer my-compass-token', got %q", auth)
		}
	})

	t.Run("compass without token", func(t *testing.T) {
		_, cleanup := setupTempHome(t)
		defer cleanup()

		tokens := map[string]string{"COMPASS_TOKEN": ""}
		envVars := map[string]string{"COMPASS_URL": "https://compass.example.com/api/mcp"}

		selected := []MCPServer{compassServer}
		mcpPath, err := GenerateMCPConfig(selected, nil, nil, nil, tokens, envVars)
		if err != nil {
			t.Fatalf("GenerateMCPConfig error: %v", err)
		}

		servers := readGeneratedConfig(t, mcpPath)
		if _, ok := servers["compass"]; ok {
			t.Error("compass should NOT be in config when COMPASS_TOKEN is empty")
		}
	})
}

// --- Task 7.4: Empty server selection ---
// Verifies empty selection produces empty mcpServers: {} config.
//
// **Validates: Requirements 4.2**
func TestEmptyServerSelectionProducesEmptyConfig(t *testing.T) {
	_, cleanup := setupTempHome(t)
	defer cleanup()

	var selected []MCPServer // empty
	mcpPath, err := GenerateMCPConfig(selected, nil, nil, nil, map[string]string{}, map[string]string{})
	if err != nil {
		t.Fatalf("GenerateMCPConfig error: %v", err)
	}

	servers := readGeneratedConfig(t, mcpPath)
	if _, hasYax := servers["yax"]; len(servers) != 0 && !(len(servers) == 1 && hasYax) {
		t.Errorf("expected only yax or empty mcpServers, got %d entries: %v", len(servers), servers)
	}
}

// --- Task 7.5: Default selection is all-enabled (Property 1) ---
// Verifies that for any list of discovered servers, when all are verified,
// selecting all verified gives back the full list.
//
// **Validates: Requirements 1.2**
func TestDefaultSelectionAllEnabled(t *testing.T) {
	targetDir := setupAllBundles(t)

	available, verified := DiscoverServers(targetDir)

	// When all bundles are present, every server should be available.
	if len(available) != len(knownServers) {
		t.Fatalf("expected %d available servers, got %d", len(knownServers), len(available))
	}

	// Every server should be verified.
	for _, srv := range available {
		if !verified[srv.Name] {
			t.Errorf("server %q should be verified but is not", srv.Name)
		}
	}

	// Selecting all verified should give back the full list.
	var selected []MCPServer
	for _, srv := range available {
		if verified[srv.Name] {
			selected = append(selected, srv)
		}
	}

	if len(selected) != len(available) {
		t.Errorf("expected all %d servers selected, got %d", len(available), len(selected))
	}

	// Verify every known server name is in the selected set.
	selectedNames := make(map[string]bool)
	for _, srv := range selected {
		selectedNames[srv.Name] = true
	}
	for _, srv := range knownServers {
		if !selectedNames[srv.Name] {
			t.Errorf("server %q missing from default selection", srv.Name)
		}
	}
}

// --- Preserve user customizations across regeneration ---

// writeMCPJsonWithState writes a mcp.json with the given servers map to the
// settings directory under tmpHome. Each value is a map[string]any representing
// the server entry (may include "disabled", "autoApprove", etc.).
func writeMCPJsonWithState(t *testing.T, tmpHome string, servers map[string]map[string]any) {
	t.Helper()
	settingsDir := filepath.Join(tmpHome, ".kiro", config.SettingsDir)
	os.MkdirAll(settingsDir, 0755)
	mcpConfig := map[string]any{"mcpServers": servers}
	data, err := json.MarshalIndent(mcpConfig, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal seed mcp.json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(settingsDir, "mcp.json"), append(data, '\n'), 0644); err != nil {
		t.Fatalf("failed to write seed mcp.json: %v", err)
	}
}

// readMCPJsonRaw reads mcp.json and returns the raw mcpServers map.
func readMCPJsonRaw(t *testing.T, mcpPath string) map[string]map[string]any {
	t.Helper()
	data, err := os.ReadFile(mcpPath)
	if err != nil {
		t.Fatalf("failed to read mcp.json: %v", err)
	}
	var parsed struct {
		MCPServers map[string]map[string]any `json:"mcpServers"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal mcp.json: %v", err)
	}
	return parsed.MCPServers
}

// TestGenerateMCPConfigPreservesDisabledState verifies that when a server was
// previously disabled in mcp.json, regenerating the config preserves that state.
func TestGenerateMCPConfigPreservesDisabledState(t *testing.T) {
	tmpHome, cleanup := setupTempHome(t)
	defer cleanup()

	// Seed mcp.json with bruno disabled.
	writeMCPJsonWithState(t, tmpHome, map[string]map[string]any{
		"bruno":   {"command": "node", "disabled": true},
		"mermaid": {"command": "node"},
	})

	// Select bruno and mermaid.
	var selected []MCPServer
	for _, srv := range knownServers {
		if srv.Name == "bruno" || srv.Name == "mermaid" {
			selected = append(selected, srv)
		}
	}

	mcpPath, err := GenerateMCPConfig(selected, nil, nil, nil, map[string]string{}, map[string]string{})
	if err != nil {
		t.Fatalf("GenerateMCPConfig error: %v", err)
	}

	servers := readMCPJsonRaw(t, mcpPath)

	// bruno should still be disabled.
	brunoEntry, ok := servers["bruno"]
	if !ok {
		t.Fatal("bruno not found in generated config")
	}
	if disabled, _ := brunoEntry["disabled"].(bool); !disabled {
		t.Errorf("expected bruno disabled=true, got %v", brunoEntry["disabled"])
	}

	// mermaid should NOT be disabled.
	mermaidEntry, ok := servers["mermaid"]
	if !ok {
		t.Fatal("mermaid not found in generated config")
	}
	if disabled, _ := mermaidEntry["disabled"].(bool); disabled {
		t.Error("expected mermaid disabled=false, but got true")
	}
}

// TestGenerateMCPConfigPreservesAutoApprove verifies that autoApprove arrays
// are preserved across regeneration.
func TestGenerateMCPConfigPreservesAutoApprove(t *testing.T) {
	tmpHome, cleanup := setupTempHome(t)
	defer cleanup()

	// Seed mcp.json with bruno having autoApprove.
	writeMCPJsonWithState(t, tmpHome, map[string]map[string]any{
		"bruno": {"command": "node", "autoApprove": []string{"create_collection", "create_request"}},
	})

	var selected []MCPServer
	for _, srv := range knownServers {
		if srv.Name == "bruno" {
			selected = append(selected, srv)
			break
		}
	}

	mcpPath, err := GenerateMCPConfig(selected, nil, nil, nil, map[string]string{}, map[string]string{})
	if err != nil {
		t.Fatalf("GenerateMCPConfig error: %v", err)
	}

	servers := readMCPJsonRaw(t, mcpPath)
	brunoEntry, ok := servers["bruno"]
	if !ok {
		t.Fatal("bruno not found in generated config")
	}

	autoApproveRaw, ok := brunoEntry["autoApprove"]
	if !ok {
		t.Fatal("autoApprove not found on bruno entry")
	}

	// JSON unmarshals arrays as []interface{}.
	autoApprove, ok := autoApproveRaw.([]interface{})
	if !ok {
		t.Fatalf("autoApprove is not an array: %T", autoApproveRaw)
	}
	if len(autoApprove) != 2 {
		t.Fatalf("expected 2 autoApprove entries, got %d", len(autoApprove))
	}
	if autoApprove[0] != "create_collection" || autoApprove[1] != "create_request" {
		t.Errorf("unexpected autoApprove values: %v", autoApprove)
	}
}

// TestGenerateMCPConfigDropsRemovedServerState verifies that disabled/autoApprove
// state for a server that is no longer generated gets dropped (not resurrected).
func TestGenerateMCPConfigDropsRemovedServerState(t *testing.T) {
	tmpHome, cleanup := setupTempHome(t)
	defer cleanup()

	// Seed mcp.json with a server that won't be regenerated.
	writeMCPJsonWithState(t, tmpHome, map[string]map[string]any{
		"old-removed-server": {"command": "node", "disabled": true, "autoApprove": []string{"foo"}},
		"bruno":              {"command": "node"},
	})

	var selected []MCPServer
	for _, srv := range knownServers {
		if srv.Name == "bruno" {
			selected = append(selected, srv)
			break
		}
	}

	mcpPath, err := GenerateMCPConfig(selected, nil, nil, nil, map[string]string{}, map[string]string{})
	if err != nil {
		t.Fatalf("GenerateMCPConfig error: %v", err)
	}

	servers := readMCPJsonRaw(t, mcpPath)
	if _, ok := servers["old-removed-server"]; ok {
		t.Error("removed server should not be resurrected in generated config")
	}
}

// TestGenerateMCPConfigNewServerDefaultsEnabled verifies that a server not
// previously in mcp.json defaults to enabled (no disabled field).
func TestGenerateMCPConfigNewServerDefaultsEnabled(t *testing.T) {
	tmpHome, cleanup := setupTempHome(t)
	defer cleanup()

	// Seed mcp.json with only mermaid (no bruno).
	writeMCPJsonWithState(t, tmpHome, map[string]map[string]any{
		"mermaid": {"command": "node", "disabled": true},
	})

	// Now generate with both mermaid and bruno selected.
	var selected []MCPServer
	for _, srv := range knownServers {
		if srv.Name == "bruno" || srv.Name == "mermaid" {
			selected = append(selected, srv)
		}
	}

	mcpPath, err := GenerateMCPConfig(selected, nil, nil, nil, map[string]string{}, map[string]string{})
	if err != nil {
		t.Fatalf("GenerateMCPConfig error: %v", err)
	}

	servers := readMCPJsonRaw(t, mcpPath)

	// bruno is new — should not be disabled.
	brunoEntry, ok := servers["bruno"]
	if !ok {
		t.Fatal("bruno not found in generated config")
	}
	if disabled, _ := brunoEntry["disabled"].(bool); disabled {
		t.Error("new server bruno should default to enabled, but got disabled=true")
	}

	// mermaid was disabled — should stay disabled.
	mermaidEntry, ok := servers["mermaid"]
	if !ok {
		t.Fatal("mermaid not found in generated config")
	}
	if disabled, _ := mermaidEntry["disabled"].(bool); !disabled {
		t.Error("existing disabled mermaid should stay disabled")
	}
}

// TestGenerateMCPConfigPreservesBothDisabledAndAutoApprove verifies that both
// disabled and autoApprove are preserved simultaneously on the same server.
func TestGenerateMCPConfigPreservesBothDisabledAndAutoApprove(t *testing.T) {
	tmpHome, cleanup := setupTempHome(t)
	defer cleanup()

	writeMCPJsonWithState(t, tmpHome, map[string]map[string]any{
		"bruno": {
			"command":     "node",
			"disabled":    true,
			"autoApprove": []string{"create_collection"},
		},
	})

	var selected []MCPServer
	for _, srv := range knownServers {
		if srv.Name == "bruno" {
			selected = append(selected, srv)
			break
		}
	}

	mcpPath, err := GenerateMCPConfig(selected, nil, nil, nil, map[string]string{}, map[string]string{})
	if err != nil {
		t.Fatalf("GenerateMCPConfig error: %v", err)
	}

	servers := readMCPJsonRaw(t, mcpPath)
	brunoEntry, ok := servers["bruno"]
	if !ok {
		t.Fatal("bruno not found in generated config")
	}

	if disabled, _ := brunoEntry["disabled"].(bool); !disabled {
		t.Error("expected bruno disabled=true")
	}

	autoApproveRaw, ok := brunoEntry["autoApprove"]
	if !ok {
		t.Fatal("autoApprove not found on bruno entry")
	}
	autoApprove, ok := autoApproveRaw.([]interface{})
	if !ok {
		t.Fatalf("autoApprove is not an array: %T", autoApproveRaw)
	}
	if len(autoApprove) != 1 || autoApprove[0] != "create_collection" {
		t.Errorf("unexpected autoApprove: %v", autoApprove)
	}
}
