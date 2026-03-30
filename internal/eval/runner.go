package eval

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.disney.com/SANCR225/koda/internal/acp"
	"gopkg.in/yaml.v3"
)

// RunFixture executes an agent against a fixture and returns the raw output.
func RunFixture(fixture Fixture) EvalResult {
	start := time.Now()
	timeout := time.Duration(fixture.Timeout) * time.Second
	if timeout == 0 {
		timeout = 120 * time.Second
	}

	client, err := acp.Spawn(fixture.Agent)
	if err != nil {
		return EvalResult{
			Agent: fixture.Agent, Fixture: fixture.Name,
			Duration: time.Since(start), Error: fmt.Sprintf("spawn: %v", err),
		}
	}
	defer client.Close()

	if err := client.CreateSession(fixture.Agent); err != nil {
		return EvalResult{
			Agent: fixture.Agent, Fixture: fixture.Name,
			Duration: time.Since(start), Error: fmt.Sprintf("session: %v", err),
		}
	}

	if err := client.SendMessage(fixture.Prompt); err != nil {
		return EvalResult{
			Agent: fixture.Agent, Fixture: fixture.Name,
			Duration: time.Since(start), Error: fmt.Sprintf("send: %v", err),
		}
	}

	// Collect output with timeout
	var buf strings.Builder
	timer := time.NewTimer(timeout)
	defer timer.Stop()

loop:
	for {
		select {
		case event, ok := <-client.Events:
			if !ok {
				break loop
			}
			switch event.Type {
			case "MessageChunk":
				buf.WriteString(event.Chunk)
			case "Complete":
				break loop
			}
		case <-timer.C:
			return EvalResult{
				Agent: fixture.Agent, Fixture: fixture.Name, Output: buf.String(),
				Duration: time.Since(start), Error: "timeout",
			}
		}
	}

	return EvalResult{
		Agent: fixture.Agent, Fixture: fixture.Name,
		Output: buf.String(), Duration: time.Since(start),
	}
}

// LoadFixtures reads all fixture files for an agent from evalsDir/fixtures/<agent>/.
func LoadFixtures(evalsDir, agent string) ([]Fixture, error) {
	dir := filepath.Join(evalsDir, "fixtures", agent)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("no fixtures for %s: %w", agent, err)
	}
	var fixtures []Fixture
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		f, err := parseFixture(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", e.Name(), err)
		}
		fixtures = append(fixtures, f)
	}
	return fixtures, nil
}

// LoadRubric reads a rubric YAML for an agent.
func LoadRubric(evalsDir, agent string) (Rubric, error) {
	path := filepath.Join(evalsDir, "rubrics", agent+".yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return Rubric{}, fmt.Errorf("no rubric for %s: %w", agent, err)
	}
	var r Rubric
	if err := yaml.Unmarshal(data, &r); err != nil {
		return Rubric{}, err
	}
	return r, nil
}

// LoadConfig reads the eval config.
func LoadConfig(evalsDir string) (Config, error) {
	path := filepath.Join(evalsDir, "config.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{
			Defaults: ConfigDefaults{Timeout: 120, ThresholdStructural: 100, ThresholdQuality: 70},
		}, nil
	}
	var c Config
	if err := yaml.Unmarshal(data, &c); err != nil {
		return Config{}, err
	}
	return c, nil
}

// EvalsDir returns the evals directory under steerRoot.
func EvalsDir(steerRoot string) string {
	return filepath.Join(steerRoot, "evals")
}

// parseFixture splits YAML frontmatter from markdown body.
func parseFixture(path string) (Fixture, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Fixture{}, err
	}
	content := string(data)

	var f Fixture
	f.Path = path

	if strings.HasPrefix(content, "---\n") {
		end := strings.Index(content[4:], "\n---")
		if end >= 0 {
			frontmatter := content[4 : 4+end]
			if err := yaml.Unmarshal([]byte(frontmatter), &f); err != nil {
				return Fixture{}, fmt.Errorf("frontmatter: %w", err)
			}
			f.Prompt = strings.TrimSpace(content[4+end+4:])
		}
	}

	if f.Prompt == "" {
		f.Prompt = strings.TrimSpace(content)
	}
	if f.Name == "" {
		base := filepath.Base(path)
		f.Name = strings.TrimSuffix(base, ".md")
	}

	return f, nil
}
