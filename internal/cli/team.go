package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.disney.com/SANCR225/koda/internal/team"
	"github.disney.com/SANCR225/koda/internal/tui"
)

var teamGoal string
var teamTUI bool

var teamCmd = &cobra.Command{
	Use:   "team",
	Short: "Manage agent teams",
}

var teamRunCmd = &cobra.Command{
	Use:   "run [spec-file]",
	Short: "Launch a team from a TeamSpec file",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		spec, err := team.LoadTeamSpec(args[0])
		if err != nil {
			return fmt.Errorf("failed to load spec: %w", err)
		}

		repoRoot, _ := os.Getwd()

		if err := team.ValidateDeps(spec); err != nil {
			return fmt.Errorf("invalid team spec: %w", err)
		}
		teamID := fmt.Sprintf("%s-%s", spec.Name, time.Now().Format("20060102-150405"))

		PrintBanner(appVersion)
		fmt.Printf("\U0001f3af Team: %s (%d workers)\n", spec.Name, len(spec.Workers))
		fmt.Printf("   Goal: %s\n", teamGoal)
		fmt.Printf("   Merge: %s\n", spec.MergeStrategy)
		fmt.Printf("   Base: %s\n\n", spec.BaseBranch)

		for _, ws := range spec.Workers {
			deps := "none"
			if len(ws.DependsOn) > 0 {
				deps = fmt.Sprintf("%v", ws.DependsOn)
			}
			fmt.Printf("   \u25b8 %-20s agent=%-20s trust=%-12s deps=%s\n", ws.Role, ws.AgentConfig, ws.TrustLevel, deps)
		}
		fmt.Println()

		// TUI mode
		if teamTUI {
			return tui.RunTeamDashboard(spec, teamGoal, repoRoot)
		}

		t := team.NewTeam(teamID, spec, teamGoal, repoRoot)

		// Stream events in background
		go func() {
			for event := range t.Events {
				switch event.Type {
				case "StateChange":
					fmt.Printf("   [%s] %s\n", event.WorkerID, event.Data)
				case "ToolCall":
					fmt.Printf("   [%s] \u2699 %s\n", event.WorkerID, event.Data)
				case "Complete":
					fmt.Printf("   [%s] \u2713 Done\n", event.WorkerID)
				}
			}
		}()

		if err := t.Run(); err != nil {
			return err
		}

		fmt.Println("\n" + t.Status())
		fmt.Println("\u2705 Team run complete")
		return nil
	},
}

var teamStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show running team status",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("No team currently running. Use 'koda team run <spec>' to start.")
	},
}

var teamListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available TeamSpec templates",
	Run: func(cmd *cobra.Command, args []string) {
		dir := filepath.Join(".", ".koda", "teams")
		specs := team.ListTeamSpecs(dir)
		if len(specs) == 0 {
			fmt.Println("No team specs found in .koda/teams/")
			fmt.Println("Create one with: koda team init <name>")
			return
		}
		fmt.Println("\U0001f4cb Team specs:")
		for _, name := range specs {
			spec, err := team.LoadTeamSpec(filepath.Join(dir, name+".json"))
			if err != nil {
				continue
			}
			fmt.Printf("  \u2022 %-20s %d workers  %s\n", name, len(spec.Workers), spec.Description)
		}
	},
}

var teamInitCmd = &cobra.Command{
	Use:   "init [name]",
	Short: "Create a sample TeamSpec template",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		spec := team.TeamSpec{
			Version:       "1.0",
			Name:          name,
			Description:   "Describe your team's purpose",
			MergeStrategy: "rebase-chain",
			BaseBranch:    "main",
			Workers: []team.WorkerSpec{
				{
					ID:           "backend",
					Role:         "Backend Engineer",
					AgentConfig:  "backend",
					Model:        "claude-sonnet-4",
					TrustLevel:   "supervised",
					DependsOn:    []string{},
					TaskTemplate: "Implement {goal} in the backend layer.",
					OutputFormat: "Summary + list of modified files.",
				},
				{
					ID:           "tests",
					Role:         "QA Engineer",
					AgentConfig:  "test_automation_agent",
					Model:        "claude-sonnet-4",
					TrustLevel:   "autonomous",
					DependsOn:    []string{"backend"},
					TaskTemplate: "Write tests for {goal} covering the backend changes.",
					OutputFormat: "List test files and coverage.",
				},
			},
		}

		dir := filepath.Join(".", ".koda", "teams")
		if err := team.SaveTeamSpec(spec, dir); err != nil {
			return err
		}
		fmt.Printf("\u2705 Created %s/%s.json\n", dir, name)
		fmt.Println("Edit the file, then run: koda team run .koda/teams/" + name + ".json --goal \"your goal\"")
		return nil
	},
}

func init() {
	teamRunCmd.Flags().StringVar(&teamGoal, "goal", "", "High-level goal for the team")
	teamRunCmd.Flags().BoolVar(&teamTUI, "tui", false, "Launch interactive TUI dashboard")
	teamCmd.AddCommand(teamRunCmd)
	teamCmd.AddCommand(teamStatusCmd)
	teamCmd.AddCommand(teamListCmd)
	teamCmd.AddCommand(teamInitCmd)
}

var teamPlanCmd = &cobra.Command{
	Use:   "plan",
	Short: "AI-generate a TeamSpec from a goal",
	RunE: func(cmd *cobra.Command, args []string) error {
		if teamGoal == "" {
			return fmt.Errorf("--goal is required")
		}
		spec, err := team.GeneratePlan(teamGoal)
		if err != nil {
			return err
		}

		fmt.Printf("\n📋 Generated plan: %s (%d workers)\n\n", spec.Name, len(spec.Workers))
		for _, ws := range spec.Workers {
			deps := "none"
			if len(ws.DependsOn) > 0 {
				deps = fmt.Sprintf("%v", ws.DependsOn)
			}
			fmt.Printf("  ▸ %-20s agent=%-20s deps=%s\n", ws.Role, ws.AgentConfig, deps)
			fmt.Printf("    %s\n\n", ws.TaskTemplate)
		}

		dir := filepath.Join(".", ".koda", "teams")
		if err := team.SaveTeamSpec(spec, dir); err != nil {
			return err
		}
		fmt.Printf("✅ Saved to %s/%s.json\n", dir, spec.Name)
		fmt.Printf("Review and run: koda team run %s/%s.json --goal %q\n", dir, spec.Name, teamGoal)
		return nil
	},
}

var teamMergeCmd = &cobra.Command{
	Use:   "merge [spec-file]",
	Short: "Merge completed workers' changes",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		spec, err := team.LoadTeamSpec(args[0])
		if err != nil {
			return err
		}
		repoRoot, _ := os.Getwd()

		if err := team.ValidateDeps(spec); err != nil {
			return fmt.Errorf("invalid team spec: %w", err)
		}
		// Find the most recent team run for this spec
		t := team.NewTeam("merge", spec, "", repoRoot)
		t.Worktrees = team.NewWorktreeManager(repoRoot)

		// Check conflicts first
		report := team.DetectConflicts(t)
		if len(report.Overlaps) > 0 {
			fmt.Printf("⚠ %d file conflicts detected:\n\n", len(report.Overlaps))
			for _, o := range report.Overlaps {
				fmt.Printf("  %s → %v\n", o.Path, o.Workers)
			}
			fmt.Println("\nResolve conflicts before merging.")
			return nil
		}

		fmt.Printf("🔀 Merging with strategy: %s\n\n", spec.MergeStrategy)
		if err := team.Merge(t); err != nil {
			return err
		}

		team.CleanupAfterMerge(t)
		fmt.Println("\n✅ Merge complete")
		return nil
	},
}

func init() {
	teamPlanCmd.Flags().StringVar(&teamGoal, "goal", "", "Goal to decompose")
	teamCmd.AddCommand(teamPlanCmd)
	teamCmd.AddCommand(teamMergeCmd)
}
