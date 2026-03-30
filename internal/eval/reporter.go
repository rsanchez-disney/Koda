package eval

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// PrintResult renders an EvalResult to the terminal.
func PrintResult(r EvalResult) {
	fmt.Printf("\n  %s / %s\n", r.Agent, r.Fixture)
	fmt.Println("  " + strings.Repeat("─", 45))

	if r.Error != "" {
		fmt.Printf("  \u2717 Error: %s\n", r.Error)
		return
	}

	// Structural
	passed, total := 0, 0
	for _, s := range r.Structural {
		total++
		icon := "\u2717"
		if s.Passed {
			icon = "\u2713"
			passed++
		}
		req := ""
		if !s.Required {
			req = " (optional)"
		}
		fmt.Printf("    %s %s%s\n", icon, s.Name, req)
	}
	fmt.Printf("    %d/%d passed\n", passed, total)

	// Quality
	if len(r.Quality) > 0 {
		fmt.Println()
		for _, q := range r.Quality {
			fmt.Printf("    %-22s %3d/100  %q\n", q.Name+":", q.Score, q.Reasoning)
		}
		fmt.Println("    " + strings.Repeat("─", 45))
		label := "\u2713 PASS"
		if !r.QualityPass {
			label = "\u2717 FAIL"
		}
		fmt.Printf("    Composite:            %3d/100  %s\n", r.QualityComposite, label)
	}

	fmt.Printf("\n  Duration: %s\n", r.Duration.Round(time.Second))
	if r.Pass {
		fmt.Println("  Result: \u2713 PASS")
	} else {
		fmt.Println("  Result: \u2717 FAIL")
	}
}

// PrintSummary renders a summary of all results.
func PrintSummary(results []EvalResult) {
	passed, failed := 0, 0
	for _, r := range results {
		if r.Pass {
			passed++
		} else {
			failed++
		}
	}
	fmt.Printf("\n  Summary: %d passed, %d failed, %d total\n", passed, failed, len(results))
}

// SaveResults writes results as JSON to evals/results/<timestamp>/.
func SaveResults(evalsDir string, results []EvalResult) (string, error) {
	ts := time.Now().Format("2006-01-02T15-04-05")
	dir := filepath.Join(evalsDir, "results", ts)
	os.MkdirAll(dir, 0755)

	// Strip output from summary to keep it small
	type summaryEntry struct {
		Agent            string `json:"agent"`
		Fixture          string `json:"fixture"`
		Pass             bool   `json:"pass"`
		StructuralPass   bool   `json:"structural_pass"`
		QualityComposite int    `json:"quality_composite,omitempty"`
		QualityPass      bool   `json:"quality_pass"`
		Error            string `json:"error,omitempty"`
		DurationSec      int    `json:"duration_sec"`
	}
	var summary []summaryEntry
	for _, r := range results {
		summary = append(summary, summaryEntry{
			Agent: r.Agent, Fixture: r.Fixture, Pass: r.Pass,
			StructuralPass: r.StructuralPass, QualityComposite: r.QualityComposite,
			QualityPass: r.QualityPass, Error: r.Error,
			DurationSec: int(r.Duration.Seconds()),
		})

		// Full result per agent
		data, _ := json.MarshalIndent(r, "", "  ")
		os.WriteFile(filepath.Join(dir, r.Agent+"_"+r.Fixture+".json"), data, 0644)
	}

	data, _ := json.MarshalIndent(summary, "", "  ")
	os.WriteFile(filepath.Join(dir, "summary.json"), data, 0644)
	return dir, nil
}

// ResultsJSON returns all results as a JSON byte slice.
func ResultsJSON(results []EvalResult) []byte {
	data, _ := json.MarshalIndent(results, "", "  ")
	return data
}
