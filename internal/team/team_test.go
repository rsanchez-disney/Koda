package team

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateDeps_Valid(t *testing.T) {
	spec := TeamSpec{
		Workers: []WorkerSpec{
			{ID: "a", Role: "backend", TrustLevel: "autonomous", OnFailure: "skip"},
			{ID: "b", Role: "ui", DependsOn: []string{"a"}, TrustLevel: "supervised"},
		},
	}
	if err := ValidateDeps(spec); err != nil {
		t.Fatal(err)
	}
}

func TestValidateDeps_Cycle(t *testing.T) {
	spec := TeamSpec{
		Workers: []WorkerSpec{
			{ID: "a", DependsOn: []string{"b"}},
			{ID: "b", DependsOn: []string{"a"}},
		},
	}
	if err := ValidateDeps(spec); err == nil {
		t.Fatal("expected cycle error")
	}
}

func TestValidateDeps_MissingDep(t *testing.T) {
	spec := TeamSpec{
		Workers: []WorkerSpec{
			{ID: "a", DependsOn: []string{"nonexistent"}},
		},
	}
	if err := ValidateDeps(spec); err == nil {
		t.Fatal("expected missing dep error")
	}
}

func TestValidateDeps_InvalidTrust(t *testing.T) {
	spec := TeamSpec{
		Workers: []WorkerSpec{
			{ID: "a", TrustLevel: "invalid_level"},
		},
	}
	err := ValidateDeps(spec)
	if err == nil || !strings.Contains(err.Error(), "invalid trustLevel") {
		t.Fatalf("expected trustLevel error, got: %v", err)
	}
}

func TestValidateDeps_InvalidOnFailure(t *testing.T) {
	spec := TeamSpec{
		Workers: []WorkerSpec{
			{ID: "a", OnFailure: "explode"},
		},
	}
	err := ValidateDeps(spec)
	if err == nil || !strings.Contains(err.Error(), "invalid onFailure") {
		t.Fatalf("expected onFailure error, got: %v", err)
	}
}

func TestInitBlackboard(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "blackboard.md")

	workers := []WorkerSpec{
		{ID: "w1", Role: "backend", DependsOn: []string{}},
		{ID: "w2", Role: "ui", DependsOn: []string{"w1"}},
	}
	initBlackboard(path, "Build feature X", workers)

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.Contains(content, "Build feature X") {
		t.Error("blackboard missing goal")
	}
	if !strings.Contains(content, "w1: backend") {
		t.Error("blackboard missing worker w1")
	}
	if !strings.Contains(content, "w2: ui (depends: w1)") {
		t.Error("blackboard missing worker w2 with deps")
	}
}

func TestBuildHandoffWithBlackboard(t *testing.T) {
	spec := WorkerSpec{ID: "w1", Role: "backend", TaskTemplate: "Do {goal}"}
	handoff := BuildHandoffWithBlackboard(spec, "the thing", nil, "some shared notes")
	if !strings.Contains(handoff, "## Shared Blackboard") {
		t.Error("handoff missing blackboard section")
	}
	if !strings.Contains(handoff, "some shared notes") {
		t.Error("handoff missing blackboard content")
	}
}

func TestBuildHandoffWithBlackboard_Empty(t *testing.T) {
	spec := WorkerSpec{ID: "w1", Role: "backend", TaskTemplate: "Do {goal}"}
	handoff := BuildHandoffWithBlackboard(spec, "the thing", nil, "")
	if strings.Contains(handoff, "Blackboard") {
		t.Error("empty blackboard should not appear in handoff")
	}
}
