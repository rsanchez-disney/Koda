package team

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.disney.com/SANCR225/koda/internal/acp"
)

const planPrompt = `You are a team planning assistant. Given a goal, decompose it into parallel worker tasks.

Output ONLY valid JSON matching this schema (no markdown, no explanation):
{
  "name": "short-kebab-name",
  "description": "one line",
  "mergeStrategy": "rebase-chain",
  "baseBranch": "main",
  "workers": [
    {
      "id": "short-id",
      "role": "Role Name",
      "agentConfig": "agent_name",
      "trustLevel": "supervised",
      "dependsOn": [],
      "taskTemplate": "What this worker should do for {goal}",
      "outputFormat": "Summary + modified files"
    }
  ]
}

Available agents: orchestrator, backend, webapi, ui, flutter, test_automation_agent, code_review_agent, security_scanner_agent, architecture_agent, qa_orchestrator_agent.

Rules:
- 2-5 workers max
- Use dependsOn for sequential tasks (e.g., tests depend on implementation)
- Use "supervised" trust for code changes, "autonomous" for read-only tasks
- Keep taskTemplate specific and actionable
`

// GeneratePlan uses kiro-cli to decompose a goal into a TeamSpec.
func GeneratePlan(goal string) (TeamSpec, error) {
	fmt.Println("\U0001f9e0 Generating team plan...")

	client, err := acp.Spawn("")
	if err != nil {
		return TeamSpec{}, fmt.Errorf("failed to start kiro-cli: %w", err)
	}
	defer client.Close()

	if err := client.CreateSession(""); err != nil {
		return TeamSpec{}, fmt.Errorf("session failed: %w", err)
	}

	prompt := planPrompt + "\nGoal: " + goal
	if err := client.SendMessage(prompt); err != nil {
		return TeamSpec{}, err
	}

	// Collect response
	var buf strings.Builder
	for event := range client.Events {
		switch event.Type {
		case "MessageChunk":
			buf.WriteString(event.Chunk)
			fmt.Print(".")
		case "Complete":
			goto done
		}
	}
done:
	fmt.Println()

	// Parse JSON from response (strip markdown fences if present)
	raw := buf.String()
	raw = strings.TrimSpace(raw)
	if idx := strings.Index(raw, "{"); idx >= 0 {
		raw = raw[idx:]
	}
	if idx := strings.LastIndex(raw, "}"); idx >= 0 {
		raw = raw[:idx+1]
	}

	var spec TeamSpec
	if err := json.Unmarshal([]byte(raw), &spec); err != nil {
		return TeamSpec{}, fmt.Errorf("failed to parse plan: %w\nRaw: %s", err, raw[:min(len(raw), 500)])
	}
	spec.Version = "1.0"

	if err := ValidateDeps(spec); err != nil {
		return TeamSpec{}, fmt.Errorf("invalid plan: %w", err)
	}

	return spec, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
