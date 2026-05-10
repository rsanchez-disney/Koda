package cli

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.disney.com/SANCR225/koda/internal/config"
	"github.disney.com/SANCR225/koda/internal/ops"
)

var evalContextCmd = &cobra.Command{
	Use:   "context-retrieval",
	Short: "Evaluate RAG context retrieval quality",
	Long:  "Runs predefined queries against the context index and reports precision.",
	RunE: func(cmd *cobra.Command, args []string) error {
		contextDir := filepath.Join(config.KiroRoot(), config.ContextDir)

		testCases := []struct {
			Query    string
			Expected string
		}{
			{"estimate story points sizing", "ccv-estimation.md"},
			{"token cost drift budget", "drift-estimation.md"},
			{"splunk index query SPL", "splunk_indexes.md"},
			{"servicenow incident INC", "servicenow_reference.md"},
			{"REST API pagination cursor", "api_standards.md"},
			{"golden rules backward compatible", "golden_rules.md"},
			{"email send recipients HTML", "email_guidelines.md"},
			{"Harness SonarQube AWS region", "ops_guidelines.md"},
			{"GLX Galaxy commerce platform", "domain_glossary.md"},
			{"ECS container Datadog monitoring", "enterprise_architecture.md"},
		}

		fmt.Println("🔍 Context Retrieval Evaluation")
		fmt.Println()

		hits := 0
		for _, tc := range testCases {
			results, err := ops.QueryContextIndex(contextDir, tc.Query, 1)
			if err != nil {
				fmt.Printf("  ✗ %q → ERROR: %v\n", tc.Query, err)
				continue
			}
			if len(results) == 0 {
				fmt.Printf("  ✗ %q → no results (expected %s)\n", tc.Query, tc.Expected)
				continue
			}
			if results[0].File == tc.Expected {
				fmt.Printf("  ✓ %q → %s\n", tc.Query, results[0].File)
				hits++
			} else {
				fmt.Printf("  ✗ %q → %s (expected %s)\n", tc.Query, results[0].File, tc.Expected)
			}
		}

		precision := float64(hits) / float64(len(testCases))
		fmt.Printf("\nPrecision: %d/%d (%.0f%%)\n", hits, len(testCases), precision*100)

		if precision < 0.7 {
			fmt.Println("⚠ Below 70% threshold — index quality needs improvement")
			return fmt.Errorf("precision %.0f%% < 70%%", precision*100)
		}
		fmt.Println("✓ Passes 70% threshold")
		return nil
	},
}

func init() {
	evalCmd.AddCommand(evalContextCmd)
}
