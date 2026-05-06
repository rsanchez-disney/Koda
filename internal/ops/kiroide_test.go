package ops

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// setupFakeSteerRoot creates a minimal steer-runtime layout for testing.
func setupFakeSteerRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()

	// Profile with steering-map.json (dev-dotnet style)
	mapProfile := filepath.Join(root, "profiles", "dev-dotnet")
	os.MkdirAll(filepath.Join(mapProfile, "context"), 0755)
	os.WriteFile(filepath.Join(mapProfile, "context", "aws_rules.md"), []byte("# AWS Rules\n\n- No hardcoded creds\n"), 0644)
	os.WriteFile(filepath.Join(mapProfile, "context", "testing.md"), []byte("# Testing\n\n- xUnit\n"), 0644)
	os.WriteFile(filepath.Join(mapProfile, "steering-map.json"), []byte(`{
  "mappings": [
    {"context": "aws_rules.md", "steering": "aws-rules.md", "inclusion": "always"},
    {"context": "testing.md", "steering": "testing-rules.md", "inclusion": "fileMatch", "fileMatchPattern": ["**/*.cs"]}
  ]
}`), 0644)

	// Profile with traditional steering/ dir (dev-core style)
	tradProfile := filepath.Join(root, "profiles", "dev-core")
	os.MkdirAll(filepath.Join(tradProfile, "steering"), 0755)
	os.WriteFile(filepath.Join(tradProfile, "steering", "foundation.md"), []byte("---\ninclusion: always\n---\n\n# Foundation\n"), 0644)

	// Skills: common flat + profile subdirectory
	os.MkdirAll(filepath.Join(root, "common", "skills"), 0755)
	os.WriteFile(filepath.Join(root, "common", "skills", "ship-it.md"), []byte("# Ship It\n"), 0644)
	os.WriteFile(filepath.Join(root, "common", "skills", "README.md"), []byte("# README\n"), 0644)

	os.MkdirAll(filepath.Join(mapProfile, "skills", "dotnet-api-skill", "references"), 0755)
	os.WriteFile(filepath.Join(mapProfile, "skills", "dotnet-api-skill", "SKILL.md"), []byte("# API Skill\n"), 0644)
	os.WriteFile(filepath.Join(mapProfile, "skills", "dotnet-api-skill", "references", "adapters.md"), []byte("# Adapters\n"), 0644)

	return root
}

func TestGenerateSteeringFromMap(t *testing.T) {
	root := setupFakeSteerRoot(t)
	dst := t.TempDir()

	profileDir := filepath.Join(root, "profiles", "dev-dotnet")
	mapFile := filepath.Join(profileDir, "steering-map.json")

	count, names := generateSteeringFromMap(profileDir, mapFile, dst)
	if count != 2 {
		t.Fatalf("expected 2 files generated, got %d", count)
	}
	if len(names) != 2 {
		t.Fatalf("expected 2 names returned, got %d", len(names))
	}

	// Check "always" inclusion
	data, err := os.ReadFile(filepath.Join(dst, "aws-rules.md"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.HasPrefix(content, "---\ninclusion: always\n---\n\n") {
		t.Error("aws-rules.md missing 'always' frontmatter")
	}
	if !strings.Contains(content, "# AWS Rules") {
		t.Error("aws-rules.md missing original content")
	}

	// Check "fileMatch" inclusion
	data, err = os.ReadFile(filepath.Join(dst, "testing-rules.md"))
	if err != nil {
		t.Fatal(err)
	}
	content = string(data)
	if !strings.Contains(content, "inclusion: fileMatch") {
		t.Error("testing-rules.md missing 'fileMatch' frontmatter")
	}
	if !strings.Contains(content, `"**/*.cs"`) {
		t.Error("testing-rules.md missing fileMatchPattern")
	}
	if !strings.Contains(content, "# Testing") {
		t.Error("testing-rules.md missing original content")
	}
}

func TestGenerateSteeringFromMap_MissingContext(t *testing.T) {
	root := t.TempDir()
	profileDir := filepath.Join(root, "profiles", "broken")
	os.MkdirAll(filepath.Join(profileDir, "context"), 0755)
	os.WriteFile(filepath.Join(profileDir, "steering-map.json"), []byte(`{
  "mappings": [{"context": "nonexistent.md", "steering": "out.md", "inclusion": "always"}]
}`), 0644)

	dst := t.TempDir()
	count, names := generateSteeringFromMap(profileDir, filepath.Join(profileDir, "steering-map.json"), dst)
	if count != 0 {
		t.Fatalf("expected 0 for missing context file, got %d", count)
	}
	if len(names) != 0 {
		t.Fatalf("expected 0 names for missing context file, got %d", len(names))
	}
}

func TestInstallSteering_MixedProfiles(t *testing.T) {
	root := setupFakeSteerRoot(t)
	target := t.TempDir()

	count := installSteering(root, target, []string{"dev-core", "dev-dotnet"})

	// dev-core: 1 traditional steering file
	// dev-dotnet: 2 from steering-map.json
	if count != 3 {
		t.Fatalf("expected 3 steering files, got %d", count)
	}

	// Traditional file copied as-is
	data, _ := os.ReadFile(filepath.Join(target, "steering", "foundation.md"))
	if !strings.Contains(string(data), "# Foundation") {
		t.Error("traditional steering file not copied")
	}

	// Map-generated file has frontmatter
	data, _ = os.ReadFile(filepath.Join(target, "steering", "aws-rules.md"))
	if !strings.HasPrefix(string(data), "---\n") {
		t.Error("map-generated file missing frontmatter")
	}
}

func TestInstallSkills_FlatAndDirs(t *testing.T) {
	root := setupFakeSteerRoot(t)
	target := t.TempDir()

	count := installSkills(root, target, []string{"dev-core", "dev-dotnet"})

	// common/skills: ship-it.md (README.md skipped) = 1
	// dev-dotnet/skills: dotnet-api-skill/ dir = 1
	// dev-core has no skills dir
	if count != 2 {
		t.Fatalf("expected 2 skills, got %d", count)
	}

	// Flat skill
	if _, err := os.Stat(filepath.Join(target, "skills", "ship-it.md")); err != nil {
		t.Error("flat skill ship-it.md not installed")
	}
	// README should be skipped
	if _, err := os.Stat(filepath.Join(target, "skills", "README.md")); err == nil {
		t.Error("README.md should not be installed")
	}
	// Skill directory with subdirectory
	if _, err := os.Stat(filepath.Join(target, "skills", "dotnet-api-skill", "SKILL.md")); err != nil {
		t.Error("skill dir SKILL.md not installed")
	}
	if _, err := os.Stat(filepath.Join(target, "skills", "dotnet-api-skill", "references", "adapters.md")); err != nil {
		t.Error("skill dir references/adapters.md not installed")
	}
}

func TestCopySkillDir(t *testing.T) {
	src := t.TempDir()
	os.MkdirAll(filepath.Join(src, "refs"), 0755)
	os.WriteFile(filepath.Join(src, "SKILL.md"), []byte("skill"), 0644)
	os.WriteFile(filepath.Join(src, "refs", "doc.md"), []byte("doc"), 0644)
	os.WriteFile(filepath.Join(src, "._hidden"), []byte("hidden"), 0644)

	dst := filepath.Join(t.TempDir(), "out")
	copySkillDir(src, dst)

	if _, err := os.Stat(filepath.Join(dst, "SKILL.md")); err != nil {
		t.Error("SKILL.md not copied")
	}
	if _, err := os.Stat(filepath.Join(dst, "refs", "doc.md")); err != nil {
		t.Error("refs/doc.md not copied")
	}
	if _, err := os.Stat(filepath.Join(dst, "._hidden")); err == nil {
		t.Error("._hidden should be skipped")
	}
}

func TestInstallSteering_CleansOrphans(t *testing.T) {
	root := setupFakeSteerRoot(t)
	target := t.TempDir()

	// First install with both profiles
	count := installSteering(root, target, []string{"dev-core", "dev-dotnet"})
	if count != 3 {
		t.Fatalf("expected 3 steering files, got %d", count)
	}

	// Verify all 3 files exist
	steeringDir := filepath.Join(target, "steering")
	for _, name := range []string{"foundation.md", "aws-rules.md", "testing-rules.md"} {
		if _, err := os.Stat(filepath.Join(steeringDir, name)); err != nil {
			t.Fatalf("expected %s to exist after full install", name)
		}
	}

	// Now sync with only dev-core — dev-dotnet files should be removed
	count = installSteering(root, target, []string{"dev-core"})
	if count != 1 {
		t.Fatalf("expected 1 steering file after narrowing profiles, got %d", count)
	}

	// foundation.md should still exist
	if _, err := os.Stat(filepath.Join(steeringDir, "foundation.md")); err != nil {
		t.Error("foundation.md should still exist")
	}
	// dev-dotnet files should be removed
	if _, err := os.Stat(filepath.Join(steeringDir, "aws-rules.md")); err == nil {
		t.Error("aws-rules.md should have been removed (orphan)")
	}
	if _, err := os.Stat(filepath.Join(steeringDir, "testing-rules.md")); err == nil {
		t.Error("testing-rules.md should have been removed (orphan)")
	}
}

func TestInstallSkills_CleansOrphans(t *testing.T) {
	root := setupFakeSteerRoot(t)
	target := t.TempDir()

	// Install with dev-dotnet (has skills)
	count := installSkills(root, target, []string{"dev-core", "dev-dotnet"})
	if count != 2 {
		t.Fatalf("expected 2 skills, got %d", count)
	}

	skillsDir := filepath.Join(target, "skills")
	if _, err := os.Stat(filepath.Join(skillsDir, "dotnet-api-skill", "SKILL.md")); err != nil {
		t.Fatal("dotnet-api-skill should exist after install")
	}

	// Now sync with only dev-core — dotnet skill dir should be removed
	count = installSkills(root, target, []string{"dev-core"})
	// Only common/skills/ship-it.md remains
	if count != 1 {
		t.Fatalf("expected 1 skill after narrowing profiles, got %d", count)
	}

	if _, err := os.Stat(filepath.Join(skillsDir, "ship-it.md")); err != nil {
		t.Error("ship-it.md (common) should still exist")
	}
	if _, err := os.Stat(filepath.Join(skillsDir, "dotnet-api-skill")); err == nil {
		t.Error("dotnet-api-skill dir should have been removed (orphan)")
	}
}

func TestCleanOrphans(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "keep.md"), []byte("keep"), 0644)
	os.WriteFile(filepath.Join(dir, "remove.md"), []byte("remove"), 0644)
	os.WriteFile(filepath.Join(dir, "other.txt"), []byte("other"), 0644)
	os.WriteFile(filepath.Join(dir, "local-my-rules.md"), []byte("local"), 0644)

	expected := map[string]bool{"keep.md": true}
	removed := cleanOrphans(dir, expected, ".md")

	if removed != 1 {
		t.Fatalf("expected 1 removed, got %d", removed)
	}
	if _, err := os.Stat(filepath.Join(dir, "keep.md")); err != nil {
		t.Error("keep.md should still exist")
	}
	if _, err := os.Stat(filepath.Join(dir, "remove.md")); err == nil {
		t.Error("remove.md should have been removed")
	}
	// .txt file should be untouched (suffix filter)
	if _, err := os.Stat(filepath.Join(dir, "other.txt")); err != nil {
		t.Error("other.txt should be untouched (different suffix)")
	}
	// local- prefix should be preserved
	if _, err := os.Stat(filepath.Join(dir, "local-my-rules.md")); err != nil {
		t.Error("local-my-rules.md should be preserved (local- prefix)")
	}
}

func TestCleanOrphanDirs(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "keep-dir"), 0755)
	os.MkdirAll(filepath.Join(dir, "remove-dir"), 0755)
	os.WriteFile(filepath.Join(dir, "remove-dir", "file.md"), []byte("x"), 0644)
	os.WriteFile(filepath.Join(dir, "file.md"), []byte("file"), 0644)

	expected := map[string]bool{"keep-dir": true}
	removed := cleanOrphanDirs(dir, expected)

	if removed != 1 {
		t.Fatalf("expected 1 dir removed, got %d", removed)
	}
	if _, err := os.Stat(filepath.Join(dir, "keep-dir")); err != nil {
		t.Error("keep-dir should still exist")
	}
	if _, err := os.Stat(filepath.Join(dir, "remove-dir")); err == nil {
		t.Error("remove-dir should have been removed")
	}
	// Regular file should be untouched
	if _, err := os.Stat(filepath.Join(dir, "file.md")); err != nil {
		t.Error("file.md should be untouched (not a dir)")
	}
}
