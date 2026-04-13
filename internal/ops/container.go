package ops

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.disney.com/SANCR225/koda/internal/config"
)

// ContainerRuntime returns the container runtime command (docker, nerdctl, podman).
// Checks CONTAINER_RUNTIME env var first, then auto-detects.
func ContainerRuntime() string {
	if rt := GetEnvVar("CONTAINER_RUNTIME"); rt != "" {
		return rt
	}
	for _, cmd := range []string{"docker", "nerdctl", "podman"} {
		if _, err := exec.LookPath(cmd); err == nil {
			return cmd
		}
	}
	return ""
}

// MCPMeta describes a docker-type MCP server manifest.
type MCPMeta struct {
	Type    string `json:"type"`
	Name    string `json:"name"`
	Compose string `json:"compose"`
	Port    int    `json:"port"`
	Health  string `json:"health"`
	MCPPath string `json:"mcp_path"`
}

// ReadMCPMeta reads mcp-meta.json from an MCP server directory.
func ReadMCPMeta(dir string) (*MCPMeta, error) {
	data, err := os.ReadFile(filepath.Join(dir, "mcp-meta.json"))
	if err != nil {
		return nil, err
	}
	var meta MCPMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, err
	}
	return &meta, nil
}

// WorkspaceMCPMeta describes a workspace-provided MCP server bundle.
type WorkspaceMCPMeta struct {
	Name    string            `json:"name"`
	Command string            `json:"command,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
}

// ReadWorkspaceMCPMeta reads mcp-meta.json from a workspace MCP server directory.
func ReadWorkspaceMCPMeta(dir string) (*WorkspaceMCPMeta, error) {
	data, err := os.ReadFile(filepath.Join(dir, "mcp-meta.json"))
	if err != nil {
		return nil, err
	}
	var meta WorkspaceMCPMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, err
	}
	if meta.Command == "" {
		meta.Command = "node"
	}
	return &meta, nil
}

// MemoryMCPDir returns the path to the memory-mcp bundle.
func MemoryMCPDir(targetDir string) string {
	return filepath.Join(targetDir, config.ToolsDir, "mcp-servers", "memory-mcp")
}

// MemoryMCPInstalled checks if memory-mcp bundle exists.
func MemoryMCPInstalled(targetDir string) bool {
	_, err := os.Stat(filepath.Join(MemoryMCPDir(targetDir), "mcp-meta.json"))
	return err == nil
}

// MemoryStart starts the memory-mcp Docker stack.
func MemoryStart(targetDir string) error {
	rt := ContainerRuntime()
	if rt == "" {
		return fmt.Errorf("no container runtime found (docker, nerdctl, or podman)")
	}
	dir := MemoryMCPDir(targetDir)
	if !MemoryMCPInstalled(targetDir) {
		return fmt.Errorf("memory-mcp not installed at %s", dir)
	}
	cmd := exec.Command(rt, "compose", "up", "-d", "--build")
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// MemoryStop stops the memory-mcp Docker stack.
func MemoryStop(targetDir string) error {
	rt := ContainerRuntime()
	if rt == "" {
		return fmt.Errorf("no container runtime found")
	}
	cmd := exec.Command(rt, "compose", "down")
	cmd.Dir = MemoryMCPDir(targetDir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// MemoryStatus returns the status of the memory-mcp stack.
type MemoryStatusInfo struct {
	Installed bool   `json:"installed"`
	Running   bool   `json:"running"`
	Runtime   string `json:"runtime"`
	Health    string `json:"health"`
	Port      int    `json:"port"`
}

func MemoryStatus(targetDir string) MemoryStatusInfo {
	info := MemoryStatusInfo{Port: 9377}
	info.Installed = MemoryMCPInstalled(targetDir)
	info.Runtime = ContainerRuntime()
	if !info.Installed || info.Runtime == "" {
		return info
	}

	// Check if containers are running
	cmd := exec.Command(info.Runtime, "compose", "ps", "--format", "json", "--status", "running")
	cmd.Dir = MemoryMCPDir(targetDir)
	out, err := cmd.Output()
	if err == nil && len(strings.TrimSpace(string(out))) > 2 {
		info.Running = true
	}

	// Check health endpoint
	if info.Running {
		info.Health = checkMemoryHealth(info.Port)
	}
	return info
}

func checkMemoryHealth(port int) string {
	out, err := exec.Command("curl", "-sf", fmt.Sprintf("http://localhost:%d/health", port)).Output()
	if err != nil {
		return "unreachable"
	}
	var resp map[string]string
	if json.Unmarshal(out, &resp) == nil {
		if resp["status"] == "ok" {
			return "ok"
		}
		return resp["status"]
	}
	return strings.TrimSpace(string(out))
}
