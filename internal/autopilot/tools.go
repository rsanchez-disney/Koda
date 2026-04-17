package autopilot

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.disney.com/SANCR225/koda/internal/pkg"
)

// Tool represents an autopilot action available to the Koda prompt agent.
type Tool struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Parameters  map[string]Param  `json:"parameters"`
}

type Param struct {
	Type        string `json:"type"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
}

// RegisteredTools returns autopilot tools if autopilot is installed.
// Returns nil if not installed (discovery-based: no autopilot = no tools).
func RegisteredTools() []Tool {
	if !pkg.IsInstalled("autopilot") {
		return nil
	}
	return []Tool{
		{
			Name:        "autopilot_run",
			Description: "Run an AI-SDLC pipeline for a Jira ticket",
			Parameters: map[string]Param{
				"jira_ticket": {Type: "string", Description: "Jira ticket ID", Required: true},
				"template":    {Type: "string", Description: "Pipeline template (feature-delivery, bug-fix, hotfix, spike). Omit for dynamic generation."},
			},
		},
		{
			Name:        "autopilot_status",
			Description: "Get pipeline status with stage details",
			Parameters: map[string]Param{
				"pipeline_id": {Type: "string", Description: "Pipeline ID. Omit to list all."},
			},
		},
		{
			Name:        "autopilot_approve_gate",
			Description: "Approve a pending gate",
			Parameters: map[string]Param{
				"gate_id": {Type: "string", Description: "Gate ID", Required: true},
			},
		},
		{
			Name:        "autopilot_reject_gate",
			Description: "Reject a gate with feedback",
			Parameters: map[string]Param{
				"gate_id":  {Type: "string", Description: "Gate ID", Required: true},
				"feedback": {Type: "string", Description: "Rejection feedback", Required: true},
				"route_to": {Type: "string", Description: "Stage to route back to"},
			},
		},
		{
			Name:        "autopilot_metrics",
			Description: "Get pipeline metrics and token usage",
		},
		{
			Name:        "autopilot_dashboard",
			Description: "Start the autopilot dashboard and open in browser",
		},
	}
}

// ExecuteTool runs an autopilot tool by name with the given arguments.
func ExecuteTool(_ context.Context, name string, args map[string]string) (string, error) {
	binPath := pkg.BinPath("autopilot")
	if !pkg.IsInstalled("autopilot") {
		return "", fmt.Errorf("autopilot is not installed. Run 'koda autopilot install'")
	}

	var cmdArgs []string
	switch name {
	case "autopilot_run":
		cmdArgs = []string{"run"}
		if t, ok := args["template"]; ok && t != "" {
			cmdArgs = append(cmdArgs, t)
		}
		if j, ok := args["jira_ticket"]; ok {
			cmdArgs = append(cmdArgs, "--jira", j)
		}
	case "autopilot_status":
		if id, ok := args["pipeline_id"]; ok && id != "" {
			cmdArgs = []string{"status", id}
		} else {
			cmdArgs = []string{"list"}
		}
	case "autopilot_approve_gate":
		cmdArgs = []string{"gate", "approve", args["gate_id"]}
	case "autopilot_reject_gate":
		cmdArgs = []string{"gate", "reject", args["gate_id"]}
		if fb, ok := args["feedback"]; ok {
			cmdArgs = append(cmdArgs, "--feedback", fb)
		}
		if rt, ok := args["route_to"]; ok && rt != "" {
			cmdArgs = append(cmdArgs, "--route-to", rt)
		}
	case "autopilot_metrics":
		cmdArgs = []string{"metrics"}
	case "autopilot_dashboard":
		cmdArgs = []string{"dashboard", "--open"}
	default:
		return "", fmt.Errorf("unknown tool: %s", name)
	}

	cmd := exec.Command(binPath, cmdArgs...)
	out, err := cmd.CombinedOutput()
	result := strings.TrimSpace(string(out))
	if err != nil {
		return result, fmt.Errorf("%s: %w", result, err)
	}
	return result, nil
}
