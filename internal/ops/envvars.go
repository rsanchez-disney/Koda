package ops

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.disney.com/SANCR225/koda/internal/config"
	"github.disney.com/SANCR225/koda/internal/model"
)

// ReadEnvVars reads key=value pairs from ~/.kiro/env.vars.
// Missing known keys are filled with defaults.
func ReadEnvVars() map[string]string {
	vars := map[string]string{}
	for _, e := range model.KnownEnvVars {
		vars[e.Key] = e.Default
	}

	path := filepath.Join(config.KiroRoot(), config.EnvVarsFile)
	f, err := os.Open(path)
	if err != nil {
		return vars
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if k, v, ok := strings.Cut(line, "="); ok {
			vars[k] = v
		}
	}
	return vars
}

// WriteEnvVars writes env vars to ~/.kiro/env.vars.
func WriteEnvVars(vars map[string]string) error {
	path := filepath.Join(config.KiroRoot(), config.EnvVarsFile)
	os.MkdirAll(filepath.Dir(path), 0755)

	var lines []string
	lines = append(lines, "# Koda Environment Variables")
	// Write known vars first
	written := map[string]bool{}
	for _, e := range model.KnownEnvVars {
		if v, ok := vars[e.Key]; ok && v != "" {
			lines = append(lines, fmt.Sprintf("%s=%s", e.Key, v))
			written[e.Key] = true
		}
	}
	// Write custom vars
	for k, v := range vars {
		if !written[k] && v != "" {
			lines = append(lines, fmt.Sprintf("%s=%s", k, v))
		}
	}
	return os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0644)
}

// GetEnvVar returns a single env var value, falling back to default.
func GetEnvVar(key string) string {
	vars := ReadEnvVars()
	if v, ok := vars[key]; ok && v != "" {
		return v
	}
	for _, e := range model.KnownEnvVars {
		if e.Key == key {
			return e.Default
		}
	}
	return ""
}
