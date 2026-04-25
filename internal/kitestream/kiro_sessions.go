package kitestream

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.disney.com/SANCR225/koda/internal/config"
)

type kiroSessionSummary struct {
	Name      string `json:"name"`
	Title     string `json:"title"`
	Agent     string `json:"agent"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
	Turns     int    `json:"turns"`
	Context   float64 `json:"contextUsage"`
}

type kiroSessionDetail struct {
	kiroSessionSummary
	Messages []kiroMessage `json:"messages"`
}

type kiroMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
	Time    int64  `json:"time,omitempty"`
}

func (s *Server) listKiroSessions() []kiroSessionSummary {
	dir := filepath.Join(config.KiroRoot(), "sessions")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return []kiroSessionSummary{}
	}

	var sessions []kiroSessionSummary
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		// Skip config files
		name := strings.TrimSuffix(e.Name(), ".json")
		if name == "cli" || name == "kite" || name == "mcp" || name == "kiro_cli_theme" {
			continue
		}

		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}

		var raw struct {
			Format   string `json:"format"`
			Metadata struct {
				Title        string `json:"title"`
				CreatedAt    string `json:"created_at"`
				UpdatedAt    string `json:"updated_at"`
				SessionState struct {
					AgentName              string `json:"agent_name"`
					ConversationMetadata   struct {
						UserTurnMetadatas []struct {
							ContextUsagePercentage float64 `json:"context_usage_percentage"`
						} `json:"user_turn_metadatas"`
					} `json:"conversation_metadata"`
				} `json:"session_state"`
			} `json:"metadata"`
			LogEntries []json.RawMessage `json:"log_entries"`
		}
		if json.Unmarshal(data, &raw) != nil || raw.Format != "kiro-session-export-v1" {
			continue
		}

		turns := 0
		var ctx float64
		metas := raw.Metadata.SessionState.ConversationMetadata.UserTurnMetadatas
		if len(metas) > 0 {
			turns = len(metas)
			ctx = metas[len(metas)-1].ContextUsagePercentage
		}

		title := name
		if raw.Metadata.Title != "" {
			title = name + " — " + raw.Metadata.Title
		}

		sessions = append(sessions, kiroSessionSummary{
			Name:      name,
			Title:     title,
			Agent:     raw.Metadata.SessionState.AgentName,
			CreatedAt: raw.Metadata.CreatedAt,
			UpdatedAt: raw.Metadata.UpdatedAt,
			Turns:     turns,
			Context:   ctx,
		})
	}

	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].UpdatedAt > sessions[j].UpdatedAt
	})
	return sessions
}

func (s *Server) loadKiroSession(name string) *kiroSessionDetail {
	if strings.ContainsAny(name, "/\\") || strings.Contains(name, "..") {
		return nil
	}
	path := filepath.Join(config.KiroRoot(), "sessions", name+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	var raw struct {
		Format   string `json:"format"`
		Metadata struct {
			Title        string `json:"title"`
			CreatedAt    string `json:"created_at"`
			UpdatedAt    string `json:"updated_at"`
			SessionState struct {
				AgentName string `json:"agent_name"`
				ConversationMetadata struct {
					UserTurnMetadatas []struct {
						ContextUsagePercentage float64 `json:"context_usage_percentage"`
					} `json:"user_turn_metadatas"`
				} `json:"conversation_metadata"`
			} `json:"session_state"`
		} `json:"metadata"`
		LogEntries []struct {
			Kind string          `json:"kind"`
			Data json.RawMessage `json:"data"`
		} `json:"log_entries"`
	}
	if json.Unmarshal(data, &raw) != nil || raw.Format != "kiro-session-export-v1" {
		return nil
	}

	var messages []kiroMessage
	for _, entry := range raw.LogEntries {
		switch entry.Kind {
		case "Prompt":
			msg := extractMessage(entry.Data, "user")
			if msg != nil {
				messages = append(messages, *msg)
			}
		case "AssistantMessage":
			msg := extractMessage(entry.Data, "assistant")
			if msg != nil {
				messages = append(messages, *msg)
			}
		}
	}

	turns := len(raw.Metadata.SessionState.ConversationMetadata.UserTurnMetadatas)
	var ctx float64
	if turns > 0 {
		ctx = raw.Metadata.SessionState.ConversationMetadata.UserTurnMetadatas[turns-1].ContextUsagePercentage
	}

	return &kiroSessionDetail{
		kiroSessionSummary: kiroSessionSummary{
			Name:      name,
			Title:     raw.Metadata.Title,
			Agent:     raw.Metadata.SessionState.AgentName,
			CreatedAt: raw.Metadata.CreatedAt,
			UpdatedAt: raw.Metadata.UpdatedAt,
			Turns:     turns,
			Context:   ctx,
		},
		Messages: messages,
	}
}

func extractMessage(data json.RawMessage, role string) *kiroMessage {
	var entry struct {
		Content []struct {
			Kind string `json:"kind"`
			Data interface{} `json:"data"`
		} `json:"content"`
		Meta struct {
			Timestamp int64 `json:"timestamp"`
		} `json:"meta"`
	}
	if json.Unmarshal(data, &entry) != nil {
		return nil
	}

	var text strings.Builder
	for _, c := range entry.Content {
		if c.Kind == "text" {
			if s, ok := c.Data.(string); ok {
				text.WriteString(s)
			}
		}
	}

	content := strings.TrimSpace(text.String())
	if content == "" {
		return nil
	}

	return &kiroMessage{
		Role:    role,
		Content: content,
		Time:    entry.Meta.Timestamp,
	}
}
