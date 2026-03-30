package eval

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.disney.com/SANCR225/koda/internal/acp"
)

// RunStructuralChecks evaluates output against regex-based checks.
func RunStructuralChecks(output string, checks []StructuralCheck) []StructuralResult {
	results := make([]StructuralResult, len(checks))
	for i, check := range checks {
		re, err := regexp.Compile(check.Pattern)
		if err != nil {
			results[i] = StructuralResult{Name: check.Name, Passed: false, Required: check.Required}
			continue
		}
		found := re.MatchString(output)
		expectAbsent := strings.EqualFold(check.Expect, "absent")
		passed := found
		if expectAbsent {
			passed = !found
		}
		results[i] = StructuralResult{Name: check.Name, Passed: passed, Required: check.Required}
	}
	return results
}

// StructuralPassed returns true if all required checks passed.
func StructuralPassed(results []StructuralResult) bool {
	for _, r := range results {
		if r.Required && !r.Passed {
			return false
		}
	}
	return true
}

// RunQualityScoring sends output to an LLM judge and returns per-dimension scores.
func RunQualityScoring(evalsDir string, fixture Fixture, rubric Rubric, output string) ([]DimensionScore, error) {
	judgePrompt, err := loadJudgePrompt(evalsDir)
	if err != nil {
		return nil, err
	}

	// Build dimension descriptions
	var dims strings.Builder
	for _, d := range rubric.QualityDimensions {
		fmt.Fprintf(&dims, "- %s (weight %d%%): %s\n", d.Name, d.Weight, d.Description)
	}

	prompt := fmt.Sprintf(`%s

## Fixture (what the agent was asked)
%s

## Dimensions to score
%s

## Agent output to evaluate
%s

Return ONLY valid JSON: {"dimensions": [{"name": "...", "score": N, "reasoning": "..."}]}`,
		judgePrompt, fixture.Prompt, dims.String(), truncate(output, 12000))

	client, err := acp.Spawn("")
	if err != nil {
		return nil, fmt.Errorf("judge spawn: %w", err)
	}
	defer client.Close()

	if err := client.CreateSession(""); err != nil {
		return nil, fmt.Errorf("judge session: %w", err)
	}
	if err := client.SendMessage(prompt); err != nil {
		return nil, fmt.Errorf("judge send: %w", err)
	}

	var buf strings.Builder
	for event := range client.Events {
		switch event.Type {
		case "MessageChunk":
			buf.WriteString(event.Chunk)
		case "Complete":
			goto done
		}
	}
done:

	return parseJudgeResponse(buf.String())
}

// CompositeScore calculates the weighted average quality score.
func CompositeScore(scores []DimensionScore, dimensions []QualityDimension) int {
	if len(scores) == 0 {
		return 0
	}
	weightMap := map[string]int{}
	for _, d := range dimensions {
		weightMap[d.Name] = d.Weight
	}
	totalWeight, weighted := 0, 0
	for _, s := range scores {
		w := weightMap[s.Name]
		if w == 0 {
			w = 100 / len(scores) // equal weight fallback
		}
		weighted += s.Score * w
		totalWeight += w
	}
	if totalWeight == 0 {
		return 0
	}
	return weighted / totalWeight
}

func loadJudgePrompt(evalsDir string) (string, error) {
	data, err := os.ReadFile(filepath.Join(evalsDir, "judge.md"))
	if err != nil {
		return defaultJudgePrompt, nil
	}
	return string(data), nil
}

const defaultJudgePrompt = `You are an evaluation judge for AI agent outputs. Score the output on each dimension from 0 to 100. Be strict — 70 is "acceptable", 90 is "excellent". For each dimension, return a score and one sentence of reasoning.`

func parseJudgeResponse(raw string) ([]DimensionScore, error) {
	// Extract JSON from potential markdown fences
	s := strings.TrimSpace(raw)
	if idx := strings.Index(s, "{"); idx >= 0 {
		s = s[idx:]
	}
	if idx := strings.LastIndex(s, "}"); idx >= 0 {
		s = s[:idx+1]
	}

	var resp struct {
		Dimensions []DimensionScore `json:"dimensions"`
	}
	if err := json.Unmarshal([]byte(s), &resp); err != nil {
		return nil, fmt.Errorf("judge parse: %w\nraw: %s", err, truncate(s, 300))
	}
	return resp.Dimensions, nil
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "\n... (truncated)"
}
