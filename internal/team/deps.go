package team

import (
	"fmt"
	"strings"
)

// ValidateDeps checks for cycles and missing dependencies in a TeamSpec.
func ValidateDeps(spec TeamSpec) error {
	ids := map[string]bool{}
	for _, w := range spec.Workers {
		ids[w.ID] = true
	}

	// Check for missing deps
	for _, w := range spec.Workers {
		for _, dep := range w.DependsOn {
			if !ids[dep] {
				return fmt.Errorf("worker %q depends on %q which does not exist", w.ID, dep)
			}
		}
	}

	// Cycle detection via topological sort (Kahn's algorithm)
	inDegree := map[string]int{}
	for _, w := range spec.Workers {
		if _, ok := inDegree[w.ID]; !ok {
			inDegree[w.ID] = 0
		}
		for _, dep := range w.DependsOn {
			inDegree[w.ID]++
			_ = dep
		}
	}

	queue := []string{}
	for id, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, id)
		}
	}

	visited := 0
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		visited++

		for _, w := range spec.Workers {
			for _, dep := range w.DependsOn {
				if dep == cur {
					inDegree[w.ID]--
					if inDegree[w.ID] == 0 {
						queue = append(queue, w.ID)
					}
				}
			}
		}
	}

	if visited != len(spec.Workers) {
		var cycled []string
		for id, deg := range inDegree {
			if deg > 0 {
				cycled = append(cycled, id)
			}
		}
		return fmt.Errorf("dependency cycle detected among: %s", strings.Join(cycled, ", "))
	}

	return nil
}

// ExtractResult parses a worker's output for the [KODA_TEAM_DONE] sentinel
// and returns the summary text before it.
func ExtractResult(output string) string {
	if idx := strings.Index(output, "[KODA_TEAM_DONE]"); idx >= 0 {
		return strings.TrimSpace(output[:idx])
	}
	// No sentinel — return last 500 chars as summary
	if len(output) > 500 {
		return output[len(output)-500:]
	}
	return output
}
