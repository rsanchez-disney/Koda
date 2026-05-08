package ops

import (
	"encoding/json"
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
		{"dev expands", []string{"dev"}, []string{"dev-core", "dev-web", "dev-mobile", "dev-python", "dev-ai", "dev-infra", "dev-dotnet", "dev-php", "dev-ui"}},
		{"no alias", []string{"qa", "ops"}, []string{"qa", "ops"}},
		{"dedup", []string{"dev", "dev-core"}, []string{"dev-core", "dev-web", "dev-mobile", "dev-python", "dev-ai", "dev-infra", "dev-dotnet", "dev-php", "dev-ui"}},
		{"mixed", []string{"dev", "ba"}, []string{"dev-core", "dev-web", "dev-mobile", "dev-python", "dev-ai", "dev-infra", "dev-dotnet", "dev-php", "dev-ui", "ba"}},
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
	for _, p := range []string{"profiles/alpha", "profiles/beta"} {
		agentsDir := filepath.Join(tmp, p, "agents")
		os.MkdirAll(agentsDir, 0755)
		os.WriteFile(filepath.Join(agentsDir, "test_agent.json"), []byte(`{"name":"test_agent","description":"test"}`), 0644)
	}
	// Also need profiles/dev-core for SteerRoot detection
	os.MkdirAll(filepath.Join(tmp, "profiles/dev-core", "agents"), 0755)
	os.WriteFile(filepath.Join(tmp, "profiles/dev-core", "agents", "orch.json"), []byte(`{"name":"orch","description":"test"}`), 0644)

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

func TestSiblingProfilesNotFalsePositive(t *testing.T) {
	tmp := t.TempDir()
	target := filepath.Join(tmp, "target")
	os.MkdirAll(filepath.Join(target, "agents"), 0755)

	// Two workspace profiles with an agent of the same name
	wsA := filepath.Join(tmp, "workspaces", "ws-a", "profiles", "ops")
	wsB := filepath.Join(tmp, "workspaces", "ws-b", "profiles", "ops")
	os.MkdirAll(filepath.Join(wsA, "agents"), 0755)
	os.MkdirAll(filepath.Join(wsB, "agents"), 0755)
	os.WriteFile(filepath.Join(wsA, "agents", "orchestrator.json"), []byte(`{"name":"orchestrator","description":"from A"}`), 0644)
	os.WriteFile(filepath.Join(wsB, "agents", "orchestrator.json"), []byte(`{"name":"orchestrator","description":"from B"}`), 0644)

	// Install only ws-a profile
	TrackProfileInstall("ops", wsA, target)
	os.WriteFile(filepath.Join(target, "agents", "orchestrator.json"), []byte(`{"name":"orchestrator","description":"from A"}`), 0644)

	// ws-a should be installed, ws-b should NOT
	if !isProfileInstalled("ops", wsA, target) {
		t.Error("ws-a/ops should be detected as installed")
	}
	if isProfileInstalled("ops", wsB, target) {
		t.Error("ws-b/ops should NOT be detected as installed (sibling false positive)")
	}
}

func TestProfileWithoutAgentsTracked(t *testing.T) {
	tmp := t.TempDir()
	target := filepath.Join(tmp, "target")
	os.MkdirAll(filepath.Join(target, "agents"), 0755)

	// Profile with no agents, only rules
	srcDir := filepath.Join(tmp, "workspaces", "myws", "profiles", "rules-only")
	os.MkdirAll(filepath.Join(srcDir, "rules"), 0755)
	os.WriteFile(filepath.Join(srcDir, "rules", "no-console.md"), []byte("# rule"), 0644)

	// Without tracking, should NOT be installed
	if isProfileInstalled("rules-only", srcDir, target) {
		t.Error("profile without agents should not be detected without manifest")
	}

	// After tracking, should be installed
	TrackProfileInstall("rules-only", srcDir, target)
	if !isProfileInstalled("rules-only", srcDir, target) {
		t.Error("profile without agents should be detected after TrackProfileInstall")
	}

	// After removal, should not be installed
	TrackProfileRemove("rules-only", srcDir, target)
	if isProfileInstalled("rules-only", srcDir, target) {
		t.Error("profile should not be detected after TrackProfileRemove")
	}
}

func TestSeedInstalledFromWorkspace(t *testing.T) {
	tmp := t.TempDir()
	target := filepath.Join(tmp, "target")
	os.MkdirAll(filepath.Join(target, "agents"), 0755)

	// Create a workspace with two profiles (one without agents)
	wsDir := filepath.Join(tmp, "workspaces", "test-ws")
	os.MkdirAll(filepath.Join(wsDir, "profiles", "dev-core", "agents"), 0755)
	os.WriteFile(filepath.Join(wsDir, "profiles", "dev-core", "agents", "a.json"), []byte(`{"name":"a"}`), 0644)
	os.MkdirAll(filepath.Join(wsDir, "profiles", "no-agents", "rules"), 0755)
	os.WriteFile(filepath.Join(wsDir, "profiles", "no-agents", "rules", "r.md"), []byte("# r"), 0644)

	// Create workspace JSON
	wsJSON := `{"name":"test-ws","profiles":["dev-core","no-agents"]}`
	os.WriteFile(filepath.Join(wsDir, "workspace.json"), []byte(wsJSON), 0644)

	// Need profiles/dev-core for SteerRoot detection
	os.MkdirAll(filepath.Join(tmp, "profiles/dev-core", "agents"), 0755)

	// Directly call TrackProfileInstall for both (simulates what seed does)
	wsDC := filepath.Join(wsDir, "profiles", "dev-core")
	wsNA := filepath.Join(wsDir, "profiles", "no-agents")
	TrackProfileInstall("dev-core", wsDC, target)
	TrackProfileInstall("no-agents", wsNA, target)

	if !isProfileInstalled("dev-core", wsDC, target) {
		t.Error("dev-core should be installed after tracking")
	}
	if !isProfileInstalled("no-agents", wsNA, target) {
		t.Error("no-agents profile should be installed after tracking")
	}
}

func TestInstallRemoveProfile(t *testing.T) {
	tmp := t.TempDir()
	// Create fake profile
	agentsDir := filepath.Join(tmp, "profiles/test", "agents")
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
	os.MkdirAll(filepath.Join(tmp, "profiles/dev-core", "agents"), 0755)
	os.WriteFile(filepath.Join(tmp, "profiles/dev-core", "agents", "a.json"), []byte(`{"name":"a","description":"d"}`), 0644)

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
	os.MkdirAll(filepath.Join(tmp, "profiles/test", "agents"), 0755)
	os.WriteFile(filepath.Join(tmp, "profiles/test", "agents", "a.json"), []byte(`{"name":"a","description":"d"}`), 0644)

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
	srcAgents := filepath.Join(tmp, "profiles/p1", "agents")
	os.MkdirAll(srcAgents, 0755)
	os.WriteFile(filepath.Join(srcAgents, "a.json"), []byte(`{"name":"a","description":"v2"}`), 0644)
	os.WriteFile(filepath.Join(srcAgents, "b.json"), []byte(`{"name":"b","description":"new"}`), 0644)

	// Target with outdated a.json, b.json present, orphan c.json
	target := filepath.Join(tmp, "target")
	os.MkdirAll(filepath.Join(target, "agents"), 0755)
	os.WriteFile(filepath.Join(target, "agents", "a.json"), []byte(`{"name":"a","description":"v1"}`), 0644)
	os.WriteFile(filepath.Join(target, "agents", "b.json"), []byte(`{"name":"b","description":"old"}`), 0644)
	os.WriteFile(filepath.Join(target, "agents", "c.json"), []byte(`{"name":"c","description":"orphan"}`), 0644)

	entries := DiffSync(tmp, target)
	actions := map[string]string{}
	for _, e := range entries {
		actions[e.Path] = e.Action
	}
	if actions["agents/a.json"] != "update" {
		t.Errorf("a.json should be 'update', got %q", actions["agents/a.json"])
	}
	if actions["agents/b.json"] != "update" {
		t.Errorf("b.json should be 'update', got %q", actions["agents/b.json"])
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
	os.MkdirAll(filepath.Join(tmp, "profiles/dev-core", "agents"), 0755)
	os.WriteFile(filepath.Join(tmp, "profiles/dev-core", "agents", "orch.json"), []byte(`{"name":"orch","description":"d"}`), 0644)
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

func TestWorkspaceManifest(t *testing.T) {
	tmp := t.TempDir()

	// Snapshot before: one existing file
	os.MkdirAll(filepath.Join(tmp, "agents"), 0755)
	os.WriteFile(filepath.Join(tmp, "agents", "existing.json"), []byte(`{}`), 0644)
	before := snapshotFiles(tmp)

	// Add new files (simulating workspace install)
	os.WriteFile(filepath.Join(tmp, "agents", "new_agent.json"), []byte(`{}`), 0644)
	os.MkdirAll(filepath.Join(tmp, "steering"), 0755)
	os.WriteFile(filepath.Join(tmp, "steering", "bugfix.md"), []byte("# bugfix"), 0644)
	after := snapshotFiles(tmp)

	os.MkdirAll(filepath.Join(tmp, "settings"), 0755)
	WriteWorkspaceManifest(tmp, before, after)

	// Manifest should exist and contain only the new files as relative paths
	data, err := os.ReadFile(filepath.Join(tmp, "settings", "workspace-files.json"))
	if err != nil {
		t.Fatal("manifest not written")
	}
	content := string(data)
	if !strings.Contains(content, "new_agent.json") {
		t.Error("manifest should contain new_agent.json")
	}
	if !strings.Contains(content, "bugfix.md") {
		t.Error("manifest should contain bugfix.md")
	}
	if strings.Contains(content, "existing.json") {
		t.Error("manifest should NOT contain pre-existing file")
	}
	// Paths must be relative (no absolute path prefix)
	if strings.Contains(content, tmp) {
		t.Error("manifest should store relative paths, not absolute")
	}
}

func TestRemoveWorkspaceFiles(t *testing.T) {
	tmp := t.TempDir()

	// Pre-existing file (should survive)
	os.MkdirAll(filepath.Join(tmp, "agents"), 0755)
	existing := filepath.Join(tmp, "agents", "existing.json")
	os.WriteFile(existing, []byte(`{}`), 0644)
	before := snapshotFiles(tmp)

	// New files added by workspace
	wsFile := filepath.Join(tmp, "agents", "ws_agent.json")
	os.WriteFile(wsFile, []byte(`{}`), 0644)
	after := snapshotFiles(tmp)

	os.MkdirAll(filepath.Join(tmp, "settings"), 0755)
	WriteWorkspaceManifest(tmp, before, after)
	RemoveWorkspaceFiles(tmp)

	if _, err := os.Stat(wsFile); err == nil {
		t.Error("workspace file should have been removed")
	}
	if _, err := os.Stat(existing); err != nil {
		t.Error("pre-existing file should NOT have been removed")
	}
}

func TestDeactivateWorkspace(t *testing.T) {
	tmp := t.TempDir()

	os.MkdirAll(filepath.Join(tmp, "settings"), 0755)
	os.WriteFile(filepath.Join(tmp, "settings", "workspace.json"), []byte(`{"name":"test"}`), 0644)

	// Write manifest with a relative path (as snapshotFiles now produces)
	wsFile := filepath.Join(tmp, "steering", "bugfix.md")
	os.MkdirAll(filepath.Join(tmp, "steering"), 0755)
	os.WriteFile(wsFile, []byte("# bugfix"), 0644)
	manifest := []string{"steering/bugfix.md"} // relative path
	data, _ := json.MarshalIndent(manifest, "", "  ")
	os.WriteFile(filepath.Join(tmp, "settings", "workspace-files.json"), data, 0644)

	DeactivateWorkspace(tmp)

	if _, err := os.Stat(wsFile); err == nil {
		t.Error("workspace file should have been removed by deactivate")
	}
	if _, err := os.Stat(filepath.Join(tmp, "settings", "workspace.json")); err == nil {
		t.Error("workspace snapshot should have been removed")
	}
}

func TestInstallWorkspaceCommon(t *testing.T) {
	tmp := t.TempDir()

	wsDir := filepath.Join(tmp, "workspaces", "myteam")
	os.MkdirAll(filepath.Join(wsDir, "common", "steering"), 0755)
	os.MkdirAll(filepath.Join(wsDir, "common", "rules"), 0755)
	os.WriteFile(filepath.Join(wsDir, "workspace.json"), []byte(`{"name":"myteam"}`), 0644)
	os.WriteFile(filepath.Join(wsDir, "common", "steering", "bugfix.md"), []byte("# bugfix"), 0644)
	os.WriteFile(filepath.Join(wsDir, "common", "rules", "git-flow.md"), []byte("# git"), 0644)

	target := filepath.Join(tmp, "target")
	os.MkdirAll(target, 0755)

	InstallWorkspaceCommon(tmp, target, []string{"myteam"})

	if _, err := os.Stat(filepath.Join(target, "steering", "bugfix-myteam.md")); err != nil {
		t.Error("steering file not installed with workspace suffix")
	}
	if _, err := os.Stat(filepath.Join(target, "rules", "git-flow-myteam.md")); err != nil {
		t.Error("rules file not installed with workspace suffix")
	}
	// Original name should NOT exist
	if _, err := os.Stat(filepath.Join(target, "steering", "bugfix.md")); err == nil {
		t.Error("file should be installed with suffix, not original name")
	}
}

