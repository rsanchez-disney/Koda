package ops

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.disney.com/SANCR225/koda/internal/model"
)

func TestExpandAliases(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want []string
	}{
		{"dev expands", []string{"dev"}, []string{"dev-core", "dev-web", "dev-mobile"}},
		{"no alias", []string{"qa", "ops"}, []string{"qa", "ops"}},
		{"dedup", []string{"dev", "dev-core"}, []string{"dev-core", "dev-web", "dev-mobile"}},
		{"mixed", []string{"dev", "ba"}, []string{"dev-core", "dev-web", "dev-mobile", "ba"}},
		{"empty", []string{}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExpandAliases(tt.in)
			if len(got) != len(tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("index %d: got %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestMaskToken(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"", "not set"},
		{"short", "****"},
		{"abcdefghij", "****"},
		{"abcdefghijk", "abcdef...hijk"},
		{"a]very-long-token-value-here", "a]very...here"},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := MaskToken(tt.in); got != tt.want {
				t.Errorf("MaskToken(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestReadWriteTokens(t *testing.T) {
	// Use a temp dir as fake ~/.kiro
	tmp := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmp)
	defer os.Setenv("HOME", origHome)

	// Write
	tokens := map[string]string{
		"JIRA_PAT":           "jira-abc123",
		"CONFLUENCE_PAT":     "conf-xyz789",
		"GITHUB_TOKEN_disney": "",
	}
	if err := WriteTokens(tokens); err != nil {
		t.Fatal(err)
	}

	// Read back
	got := ReadTokens()
	if got["JIRA_PAT"] != "jira-abc123" {
		t.Errorf("JIRA_PAT = %q, want %q", got["JIRA_PAT"], "jira-abc123")
	}
	if got["CONFLUENCE_PAT"] != "conf-xyz789" {
		t.Errorf("CONFLUENCE_PAT = %q, want %q", got["CONFLUENCE_PAT"], "conf-xyz789")
	}
	if _, ok := got["GITHUB_TOKEN_disney"]; ok {
		t.Error("empty token should not be written")
	}
}

func TestListProfiles(t *testing.T) {
	// Create a fake steer-runtime with two profiles
	tmp := t.TempDir()
	for _, p := range []string{".kiro-alpha", ".kiro-beta"} {
		agentsDir := filepath.Join(tmp, p, "agents")
		os.MkdirAll(agentsDir, 0755)
		os.WriteFile(filepath.Join(agentsDir, "test_agent.json"), []byte(`{"name":"test_agent","description":"test"}`), 0644)
	}
	// Also need .kiro-dev-core for SteerRoot detection
	os.MkdirAll(filepath.Join(tmp, ".kiro-dev-core", "agents"), 0755)
	os.WriteFile(filepath.Join(tmp, ".kiro-dev-core", "agents", "orch.json"), []byte(`{"name":"orch","description":"test"}`), 0644)

	target := filepath.Join(tmp, "target")
	os.MkdirAll(filepath.Join(target, "agents"), 0755)

	profiles, err := ListProfiles(tmp, target)
	if err != nil {
		t.Fatal(err)
	}
	if len(profiles) != 3 {
		t.Fatalf("got %d profiles, want 3", len(profiles))
	}
	for _, p := range profiles {
		if p.AgentCount != 1 {
			t.Errorf("profile %s: got %d agents, want 1", p.ID, p.AgentCount)
		}
	}
}

func TestInstallRemoveProfile(t *testing.T) {
	tmp := t.TempDir()
	// Create fake profile
	agentsDir := filepath.Join(tmp, ".kiro-test", "agents")
	os.MkdirAll(agentsDir, 0755)
	os.WriteFile(filepath.Join(agentsDir, "my_agent.json"), []byte(`{"name":"my_agent","description":"d"}`), 0644)

	target := filepath.Join(tmp, "target")

	// Install
	count, err := InstallProfile(tmp, "test", target)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("installed %d, want 1", count)
	}
	if _, err := os.Stat(filepath.Join(target, "agents", "my_agent.json")); err != nil {
		t.Fatal("agent file not found after install")
	}

	// Remove
	removed, err := RemoveProfile(tmp, "test", target)
	if err != nil {
		t.Fatal(err)
	}
	if removed != 1 {
		t.Fatalf("removed %d, want 1", removed)
	}
	if _, err := os.Stat(filepath.Join(target, "agents", "my_agent.json")); err == nil {
		t.Fatal("agent file still exists after remove")
	}
}

func TestCheckInstallation(t *testing.T) {
	tmp := t.TempDir()
	os.MkdirAll(filepath.Join(tmp, ".kiro-dev-core", "agents"), 0755)
	os.WriteFile(filepath.Join(tmp, ".kiro-dev-core", "agents", "a.json"), []byte(`{"name":"a","description":"d"}`), 0644)

	target := filepath.Join(tmp, "target")
	os.MkdirAll(filepath.Join(target, "agents"), 0755)
	os.WriteFile(filepath.Join(target, "agents", "a.json"), []byte(`{"name":"a","description":"d"}`), 0644)
	os.WriteFile(filepath.Join(target, "agents", "bad.json"), []byte(`not json`), 0644)

	report := CheckInstallation(tmp, target)
	if !report.AgentsDir {
		t.Error("AgentsDir should be true")
	}
	if report.TotalAgents != 2 {
		t.Errorf("TotalAgents = %d, want 2", report.TotalAgents)
	}
	if len(report.InvalidAgents) != 1 {
		t.Errorf("InvalidAgents = %d, want 1", len(report.InvalidAgents))
	}
}

func TestWriteProfilesManifest(t *testing.T) {
	tmp := t.TempDir()
	os.MkdirAll(filepath.Join(tmp, ".kiro-test", "agents"), 0755)
	os.WriteFile(filepath.Join(tmp, ".kiro-test", "agents", "a.json"), []byte(`{"name":"a","description":"d"}`), 0644)

	target := filepath.Join(tmp, "target")
	os.MkdirAll(filepath.Join(target, "agents"), 0755)
	os.WriteFile(filepath.Join(target, "agents", "a.json"), []byte(`{"name":"a","description":"d"}`), 0644)

	if err := WriteProfilesManifest(tmp, target); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(target, "settings", "profiles.json"))
	if err != nil {
		t.Fatal("profiles.json not created")
	}
	if !strings.Contains(string(data), "\"test\"") {
		t.Error("manifest should contain profile id 'test'")
	}
}

func TestDiffSync(t *testing.T) {
	tmp := t.TempDir()
	// Source profile
	srcAgents := filepath.Join(tmp, ".kiro-p1", "agents")
	os.MkdirAll(srcAgents, 0755)
	os.WriteFile(filepath.Join(srcAgents, "a.json"), []byte(`{"name":"a","description":"v2"}`), 0644)
	os.WriteFile(filepath.Join(srcAgents, "b.json"), []byte(`{"name":"b","description":"new"}`), 0644)

	// Target with outdated a.json, no b.json, orphan c.json
	target := filepath.Join(tmp, "target")
	os.MkdirAll(filepath.Join(target, "agents"), 0755)
	os.WriteFile(filepath.Join(target, "agents", "a.json"), []byte(`{"name":"a","description":"v1"}`), 0644)
	os.WriteFile(filepath.Join(target, "agents", "c.json"), []byte(`{"name":"c","description":"orphan"}`), 0644)

	entries := DiffSync(tmp, target)
	actions := map[string]string{}
	for _, e := range entries {
		actions[e.Path] = e.Action
	}
	if actions["agents/a.json"] != "update" {
		t.Errorf("a.json should be 'update', got %q", actions["agents/a.json"])
	}
	if actions["agents/b.json"] != "add" {
		t.Errorf("b.json should be 'add', got %q", actions["agents/b.json"])
	}
	if actions["agents/c.json"] != "orphan" {
		t.Errorf("c.json should be 'orphan', got %q", actions["agents/c.json"])
	}
}

func TestListWorkspaces(t *testing.T) {
	tmp := t.TempDir()
	wsDir := filepath.Join(tmp, "workspaces", "team1")
	os.MkdirAll(wsDir, 0755)
	os.WriteFile(filepath.Join(wsDir, "workspace.json"), []byte(`{"name":"team1","description":"Test","profiles":["dev-core"]}`), 0644)

	workspaces, err := ListWorkspaces(tmp)
	if err != nil {
		t.Fatal(err)
	}
	if len(workspaces) != 1 {
		t.Fatalf("got %d workspaces, want 1", len(workspaces))
	}
	if workspaces[0].Name != "team1" {
		t.Errorf("name = %q, want %q", workspaces[0].Name, "team1")
	}
}

func TestApplyWorkspace(t *testing.T) {
	tmp := t.TempDir()
	// Create a profile
	os.MkdirAll(filepath.Join(tmp, ".kiro-dev-core", "agents"), 0755)
	os.WriteFile(filepath.Join(tmp, ".kiro-dev-core", "agents", "orch.json"), []byte(`{"name":"orch","description":"d"}`), 0644)
	// Create a rule
	os.MkdirAll(filepath.Join(tmp, "common", "rules"), 0755)
	os.WriteFile(filepath.Join(tmp, "common", "rules", "myrule.md"), []byte("# rule"), 0644)

	target := filepath.Join(tmp, "target")
	ws := model.Workspace{Name: "test", Profiles: []string{"dev-core"}, Rules: []string{"myrule"}}

	if err := ApplyWorkspace(tmp, target, ws); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(target, "agents", "orch.json")); err != nil {
		t.Error("agent not installed")
	}
	if _, err := os.Stat(filepath.Join(target, "rules", "myrule.md")); err != nil {
		t.Error("rule not installed")
	}
}