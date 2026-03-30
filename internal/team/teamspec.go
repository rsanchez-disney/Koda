package team

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// TeamSpec defines a reusable team template.
type TeamSpec struct {
	Version       string       `json:"version"`
	Name          string       `json:"name"`
	Description   string       `json:"description"`
	MergeStrategy string       `json:"mergeStrategy"` // rebase-chain, parallel-merge, pr-per-worker
	BaseBranch    string       `json:"baseBranch"`
	Workers       []WorkerSpec `json:"workers"`
}

// WorkerSpec defines one worker in a team.
type WorkerSpec struct {
	ID           string   `json:"id"`
	Role         string   `json:"role"`
	AgentConfig  string   `json:"agentConfig"`
	Model        string   `json:"model,omitempty"`
	TrustLevel   string   `json:"trustLevel"`
	DependsOn    []string `json:"dependsOn"`
	TaskTemplate string   `json:"taskTemplate"`
	OutputFormat string   `json:"outputFormat,omitempty"`
}

// Handoff is the structured payload sent as a worker's first prompt.
type Handoff struct {
	Role         string   `json:"role"`
	Task         string   `json:"task"`
	Context      []string `json:"context,omitempty"`
	Constraints  []string `json:"constraints,omitempty"`
	OutputFormat string   `json:"outputFormat,omitempty"`
}

// BuildHandoff creates the handoff prompt for a worker.
func BuildHandoff(spec WorkerSpec, goal string, priorResults map[string]string) string {
	task := spec.TaskTemplate
	if goal != "" {
		task = strings.ReplaceAll(task, "{goal}", goal)
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("# Team Worker: %s\n\n", spec.Role))
	b.WriteString(fmt.Sprintf("## Your Task\n%s\n\n", task))

	if spec.OutputFormat != "" {
		b.WriteString(fmt.Sprintf("## Output Format\n%s\n\n", spec.OutputFormat))
	}

	// Inject results from dependencies
	if len(spec.DependsOn) > 0 && len(priorResults) > 0 {
		b.WriteString("## Context from Prior Workers\n\n")
		for _, dep := range spec.DependsOn {
			if result, ok := priorResults[dep]; ok {
				b.WriteString(fmt.Sprintf("### %s\n%s\n\n", dep, result))
			}
		}
	}

	b.WriteString("## Completion\nWhen done, output the marker: [KODA_TEAM_DONE]\n")
	return b.String()
}

// LoadTeamSpec reads a TeamSpec from disk.
func LoadTeamSpec(path string) (TeamSpec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return TeamSpec{}, err
	}
	var spec TeamSpec
	if err := json.Unmarshal(data, &spec); err != nil {
		return TeamSpec{}, err
	}
	return spec, nil
}

// SaveTeamSpec writes a TeamSpec to disk.
func SaveTeamSpec(spec TeamSpec, dir string) error {
	os.MkdirAll(dir, 0755)
	path := filepath.Join(dir, spec.Name+".json")
	data, err := json.MarshalIndent(spec, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0644)
}

// ListTeamSpecs returns all .json files in the teams directory.
func ListTeamSpecs(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			names = append(names, strings.TrimSuffix(e.Name(), ".json"))
		}
	}
	return names
}
