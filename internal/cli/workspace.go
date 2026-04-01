package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.disney.com/SANCR225/koda/internal/config"
	"github.disney.com/SANCR225/koda/internal/ops"
)

var workspaceCmd = &cobra.Command{
	Use:   "workspace",
	Short: "Manage team workspaces",
}

var wsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available workspaces",
	RunE: func(cmd *cobra.Command, args []string) error {
		if steerRoot == "" {
			return fmt.Errorf("steer-runtime not found")
		}
		workspaces, _ := ops.ListWorkspaces(steerRoot)
		if jsonOutput {
			return json.NewEncoder(os.Stdout).Encode(workspaces)
		}
		if len(workspaces) == 0 {
			fmt.Println("No workspaces found.")
			return nil
		}
		fmt.Println("\U0001f4cb Team workspaces:")
		fmt.Println()
		for _, ws := range workspaces {
			fmt.Printf("  \u2022 %-20s %s\n", ws.Name, ws.Description)
			if len(ws.Profiles) > 0 {
				fmt.Printf("    Profiles: %s\n", strings.Join(ws.Profiles, ", "))
			}
		}
		return nil
	},
}

var wsShowCmd = &cobra.Command{
	Use:   "show [name]",
	Short: "Show workspace details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if steerRoot == "" {
			return fmt.Errorf("steer-runtime not found")
		}
		ws, err := ops.GetWorkspace(steerRoot, args[0])
		if err != nil {
			return err
		}
		if jsonOutput {
			return json.NewEncoder(os.Stdout).Encode(ws)
		}
		ops.PrintWorkspace(ws)
		return nil
	},
}

var wsApplyCmd = &cobra.Command{
	Use:   "apply [name]",
	Short: "Apply a workspace configuration",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if steerRoot == "" {
			return fmt.Errorf("steer-runtime not found")
		}
		ws, err := ops.GetWorkspace(steerRoot, args[0])
		if err != nil {
			return err
		}
		target := config.TargetDir(projectDir)
		fmt.Printf("\U0001f680 Applying workspace: %s\n", ws.Name)
		if err := ops.ApplyWorkspace(steerRoot, target, ws); err != nil {
			return err
		}
		fmt.Printf("\u2705 Workspace '%s' applied\n", ws.Name)
		if ws.DefaultAgent != "" {
			fmt.Printf("   Default agent: %s\n", ws.DefaultAgent)
		}
		return nil
	},
}

var wsCreateLocal bool

var wsCreateCmd = &cobra.Command{
	Use:   "create [name]",
	Short: "Scaffold a new team workspace",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if steerRoot == "" {
			return fmt.Errorf("steer-runtime not found")
		}
		name := args[0]
		wsPath := filepath.Join(steerRoot, config.WorkspacesDir, name)
		if _, err := os.Stat(wsPath); err == nil {
			return fmt.Errorf("workspace already exists: %s", name)
		}

		fmt.Printf("\U0001f3d7 Creating workspace: %s\n", name)
		for _, sub := range []string{"rules", "context", "memory-banks"} {
			os.MkdirAll(filepath.Join(wsPath, sub), 0755)
		}

		wsJSON := fmt.Sprintf(`{
  "name": "%s",
  "description": "",
  "team": "",
  "profiles": ["dev-core", "dev-web"],
  "default_agent": "orchestrator",
  "projects": [],
  "rules": ["conventional_commit"],
  "enable_tools": true,
  "jira_prefix": ""
}
`, name)
		os.WriteFile(filepath.Join(wsPath, "workspace.json"), []byte(wsJSON), 0644)
		fmt.Printf("\u2705 Workspace scaffolded at workspaces/%s/\n", name)

		if !wsCreateLocal {
			fmt.Println("\U0001f4e4 Publishing to repository...")
			exec.Command("git", "-C", steerRoot, "add", "workspaces/"+name+"/").Run()
			if err := exec.Command("git", "-C", steerRoot, "commit", "-m", "feat: add "+name+" team workspace", "--quiet").Run(); err == nil {
				if err := exec.Command("git", "-C", steerRoot, "push", "--quiet").Run(); err == nil {
					fmt.Println("  \u2713 Committed and pushed")
				} else {
					fmt.Println("  \u26a0 Committed locally, push manually")
				}
			}
		}

		fmt.Printf("\nNext: edit workspaces/%s/workspace.json, then koda workspace apply %s\n", name, name)
		return nil
	},
}

var wsSyncPush bool

var wsSyncCmd = &cobra.Command{
	Use:   "sync [name]",
	Short: "Git pull/push across workspace repos",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if steerRoot == "" {
			return fmt.Errorf("steer-runtime not found")
		}
		ws, err := ops.GetWorkspace(steerRoot, args[0])
		if err != nil {
			return err
		}
		fmt.Printf("\U0001f504 Syncing workspace: %s\n\n", ws.Name)
		for _, proj := range ws.Projects {
			path := proj.Path
			if strings.HasPrefix(path, "../") {
				path = filepath.Join(filepath.Dir(steerRoot), strings.TrimPrefix(path, "../"))
			}
			name := filepath.Base(path)
			if _, err := os.Stat(filepath.Join(path, ".git")); err != nil {
				fmt.Printf("  \u23ed %s (not found)\n", name)
				continue
			}
			if wsSyncPush {
				if err := exec.Command("git", "-C", path, "push", "--quiet").Run(); err == nil {
					fmt.Printf("  \u2713 %s (pushed)\n", name)
				} else {
					fmt.Printf("  \u26a0 %s (push failed)\n", name)
				}
			} else {
				exec.Command("git", "-C", path, "fetch", "--all", "--quiet").Run()
				if err := exec.Command("git", "-C", path, "pull", "--rebase", "--quiet").Run(); err == nil {
					fmt.Printf("  \u2713 %s (pulled)\n", name)
				} else {
					fmt.Printf("  \u26a0 %s (pull failed)\n", name)
				}
			}
		}
		fmt.Println("\n\u2705 Sync complete")
		return nil
	},
}


var wsEditCmd = &cobra.Command{
	Use:   "edit [name]",
	Short: "Edit a workspace in ",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if steerRoot == "" {
			return fmt.Errorf("steer-runtime not found")
		}
		wsFile := filepath.Join(steerRoot, config.WorkspacesDir, args[0], "workspace.json")
		if _, err := os.Stat(wsFile); err != nil {
			return fmt.Errorf("workspace not found: %s", args[0])
		}
		c := ops.EditorCmd(wsFile)
		c.Stdin = os.Stdin
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		return c.Run()
	},
}

func init() {
	wsCreateCmd.Flags().BoolVar(&wsCreateLocal, "local", false, "Scaffold only, skip git commit/push")
	wsSyncCmd.Flags().BoolVar(&wsSyncPush, "push", false, "Push instead of pull")
	workspaceCmd.AddCommand(wsListCmd)
	workspaceCmd.AddCommand(wsShowCmd)
	workspaceCmd.AddCommand(wsApplyCmd)
	workspaceCmd.AddCommand(wsCreateCmd)
	workspaceCmd.AddCommand(wsSyncCmd)
	workspaceCmd.AddCommand(wsEditCmd)
}
