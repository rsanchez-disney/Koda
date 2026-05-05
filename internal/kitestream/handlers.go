package kitestream

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.disney.com/SANCR225/koda/internal/config"
	"github.disney.com/SANCR225/koda/internal/ops"
)

func (s *Server) handleAPI(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api")

	switch {
	// Sessions
	case r.Method == "GET" && path == "/sessions":
		writeJSON(w, 200, s.bridge.ListSessions())

	case r.Method == "POST" && path == "/sessions":
		var body struct {
			AgentID     string `json:"agentId"`
			WorkspaceID string `json:"workspaceId"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		if body.AgentID == "" {
			body.AgentID = ResolveBestAgent(s.steerRoot, s.targetDir)
		}
		id := fmt.Sprintf("ks-%d", time.Now().UnixMilli())
		sess, err := s.bridge.CreateSession(id, body.AgentID, body.WorkspaceID)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 201, sess)

	case r.Method == "POST" && strings.HasSuffix(path, "/message"):
		sid := extractID(path, "/sessions/", "/message")
		var body struct{ Content string `json:"content"` }
		json.NewDecoder(r.Body).Decode(&body)
		if err := s.bridge.SendMessage(sid, body.Content); err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 200, map[string]string{"status": "sent"})

	case r.Method == "POST" && strings.HasSuffix(path, "/abort"):
		sid := extractID(path, "/sessions/", "/abort")
		s.bridge.CloseSession(sid)
		writeJSON(w, 200, map[string]string{"status": "aborted"})

	case r.Method == "GET" && strings.HasSuffix(path, "/messages"):
		// Messages are streamed via WebSocket, return empty for now
		writeJSON(w, 200, []interface{}{})

	case r.Method == "GET" && strings.HasPrefix(path, "/sessions/"):
		sid := strings.TrimPrefix(path, "/sessions/")
		sess := s.bridge.GetSession(sid)
		if sess != nil {
			writeJSON(w, 200, sess)
		} else {
			writeJSON(w, 404, map[string]string{"error": "not found"})
		}

	// Pipelines (stub)
	case r.Method == "GET" && path == "/pipelines":
		writeJSON(w, 200, []interface{}{})

	case r.Method == "POST" && path == "/pipelines":
		writeJSON(w, 201, map[string]string{"id": fmt.Sprintf("pl-%d", time.Now().UnixMilli()), "status": "pending"})

	// Prompts (stub)
	case r.Method == "GET" && path == "/prompts":
		writeJSON(w, 200, []interface{}{})

	// Agents
	case r.Method == "GET" && path == "/agents":
		writeJSON(w, 200, s.listAgents())

	case r.Method == "POST" && path == "/agents/switch":
		var body struct {
			AgentID   string `json:"agentId"`
			Workspace string `json:"workspaceId"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		if body.Workspace == "" {
			st := config.ReadSteerSettings()
			body.Workspace = st.ActiveWorkspace
		}
		id := fmt.Sprintf("ks-%d", time.Now().UnixMilli())
		sess, err := s.bridge.SwitchAgent(id, body.AgentID, body.Workspace)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 200, sess)

	// Workspaces
	case r.Method == "GET" && path == "/workspaces":
		writeJSON(w, 200, s.listWorkspaces())

	case r.Method == "POST" && path == "/workspaces/switch":
		var body struct{ WorkspaceID string `json:"workspaceId"` }
		json.NewDecoder(r.Body).Decode(&body)
		if err := s.switchWorkspace(body.WorkspaceID); err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 200, map[string]string{"status": "switched", "workspaceId": body.WorkspaceID})

	// Settings
	case r.Method == "GET" && path == "/settings":
		writeJSON(w, 200, config.ReadSteerSettings())

	// File Explorer
	case r.Method == "GET" && path == "/files":
		root := r.URL.Query().Get("path")
		if root == "" {
			root = s.targetDir
		}
		writeJSON(w, 200, s.listFiles(root))

	case r.Method == "GET" && path == "/files/read":
		filePath := r.URL.Query().Get("path")
		if filePath == "" {
			writeJSON(w, 400, map[string]string{"error": "path required"})
			return
		}
		content, err := os.ReadFile(filePath)
		if err != nil {
			writeJSON(w, 404, map[string]string{"error": "file not found"})
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Write(content)

	// Kiro Sessions (from ~/.kiro/sessions/)
	case r.Method == "GET" && path == "/kiro-sessions":
		writeJSON(w, 200, s.listKiroSessions())

	case r.Method == "GET" && strings.HasPrefix(path, "/kiro-sessions/"):
		name := strings.TrimPrefix(path, "/kiro-sessions/")
		data := s.loadKiroSession(name)
		if data != nil {
			writeJSON(w, 200, data)
		} else {
			writeJSON(w, 404, map[string]string{"error": "not found"})
		}

	// Scoring
	case r.Method == "POST" && path == "/score":
		var body struct {
			Prompt    string `json:"prompt"`
			SessionID string `json:"sessionId"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		result := ScorePrompt(body.Prompt, body.SessionID)
		writeJSON(w, 200, result)

	case r.Method == "GET" && strings.HasPrefix(path, "/tokens/"):
		sid := strings.TrimPrefix(path, "/tokens/")
		writeJSON(w, 200, map[string]int{"tokens": GetSessionTokens(sid)})

	case r.Method == "DELETE" && strings.HasPrefix(path, "/tokens/"):
		sid := strings.TrimPrefix(path, "/tokens/")
		ResetSessionTokens(sid)
		writeJSON(w, 200, map[string]string{"status": "reset"})

	default:
		writeJSON(w, 404, map[string]string{"error": "not found"})
	}
}

type agentInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Profile     string `json:"profile"`
}

func (s *Server) listAgents() []agentInfo {
	agentsDir := filepath.Join(s.targetDir, config.AgentsDir)
	entries, err := os.ReadDir(agentsDir)
	if err != nil {
		return nil
	}
	agents := make([]agentInfo, 0)
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") || strings.HasPrefix(e.Name(), "._") {
			continue
		}
		name := strings.TrimSuffix(e.Name(), ".json")
		data, err := os.ReadFile(filepath.Join(agentsDir, e.Name()))
		if err != nil {
			continue
		}
		var raw struct {
			Name        string `json:"name"`
			Description string `json:"description"`
		}
		json.Unmarshal(data, &raw)
		desc := raw.Description
		if desc == "" {
			desc = raw.Name
		}
		agents = append(agents, agentInfo{ID: name, Name: name, Description: desc, Profile: detectProfile(name, s.steerRoot)})
	}
	return agents
}

type workspaceInfo struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Team         string   `json:"team"`
	JiraPrefix   string   `json:"jiraPrefix"`
	Profiles     []string `json:"profiles"`
	DefaultAgent string   `json:"defaultAgent"`
}

func (s *Server) listWorkspaces() []workspaceInfo {
	wsList, err := ops.ListWorkspaces(s.steerRoot)
	if err != nil {
		return nil
	}
	out := make([]workspaceInfo, 0)
	for _, ws := range wsList {
		out = append(out, workspaceInfo{
			ID: ws.Name, Name: ws.Name,
			Profiles:     ws.Profiles,
			DefaultAgent: ws.DefaultAgent,
		})
	}
	return out
}

func (s *Server) switchWorkspace(wsName string) error {
	ws, err := ops.GetWorkspace(s.steerRoot, wsName)
	if err != nil {
		return err
	}
	return ops.ApplyWorkspace(s.steerRoot, s.targetDir, ws)
}

func detectProfile(agentName, steerRoot string) string {
	dirs, _ := config.ProfileDirs(steerRoot)
	for _, d := range dirs {
		agentFile := filepath.Join(d, config.AgentsDir, agentName+".json")
		if _, err := os.Stat(agentFile); err == nil {
			return filepath.Base(d)
		}
	}
	return ""
}

func extractID(path, prefix, suffix string) string {
	path = strings.TrimPrefix(path, prefix)
	path = strings.TrimSuffix(path, suffix)
	return path
}

type fileEntry struct {
	Name     string       `json:"name"`
	Path     string       `json:"path"`
	IsDir    bool         `json:"isDir"`
	Size     int64        `json:"size,omitempty"`
	Children []fileEntry  `json:"children,omitempty"`
}

var ignoreDirs = map[string]bool{
	"node_modules": true, ".git": true, "dist": true, "build": true,
	".DS_Store": true, "__pycache__": true, ".cache": true, "target": true,
	".next": true, ".nuxt": true, "coverage": true, ".vite": true,
}

func (s *Server) listFiles(root string) []fileEntry {
	entries, err := os.ReadDir(root)
	if err != nil {
		return []fileEntry{}
	}
	var result []fileEntry
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") && name != ".kiro" {
			continue
		}
		if ignoreDirs[name] {
			continue
		}
		info, _ := e.Info()
		entry := fileEntry{
			Name:  name,
			Path:  filepath.Join(root, name),
			IsDir: e.IsDir(),
		}
		if info != nil && !e.IsDir() {
			entry.Size = info.Size()
		}
		result = append(result, entry)
	}
	// Sort: dirs first, then files
	sort.Slice(result, func(i, j int) bool {
		if result[i].IsDir != result[j].IsDir {
			return result[i].IsDir
		}
		return result[i].Name < result[j].Name
	})
	return result
}
