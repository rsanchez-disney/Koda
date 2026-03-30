package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.disney.com/SANCR225/koda/internal/eval"
)

var (
	evalDeep       bool
	evalStructural bool
	evalFixture    string
	evalProfile    string
	evalThreshold  int
	evalAll        bool
	evalList       bool
	evalSave       bool
)

var evalCmd = &cobra.Command{
	Use:   "eval [agent]",
	Short: "Evaluate agent quality with fixtures and rubrics",
	Long: `Run fixtures against agents and score their output.

  koda eval orchestrator              # structural checks (default)
  koda eval orchestrator --deep       # structural + LLM quality scoring
  koda eval --all                     # all agents with fixtures
  koda eval --profile critical        # agents in the critical profile
  koda eval --list                    # list available fixtures`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if steerRoot == "" {
			return fmt.Errorf("steer-runtime not found")
		}
		evalsDir := eval.EvalsDir(steerRoot)

		cfg, err := eval.LoadConfig(evalsDir)
		if err != nil {
			return err
		}

		threshold := evalThreshold
		if threshold == 0 {
			threshold = cfg.Defaults.ThresholdQuality
		}
		if threshold == 0 {
			threshold = 70
		}

		// Determine which agents to eval
		agents, err := resolveAgents(evalsDir, cfg, args)
		if err != nil {
			return err
		}

		if evalList {
			return listFixtures(evalsDir, agents)
		}

		fmt.Printf("\U0001f9ea Koda Eval — %d agent(s)\n", len(agents))

		var allResults []eval.EvalResult
		exitCode := 0

		for _, agent := range agents {
			fixtures, err := eval.LoadFixtures(evalsDir, agent)
			if err != nil {
				fmt.Printf("  \u26a0 %s: %v\n", agent, err)
				continue
			}
			rubric, err := eval.LoadRubric(evalsDir, agent)
			if err != nil {
				fmt.Printf("  \u26a0 %s: %v\n", agent, err)
				continue
			}

			for _, fixture := range fixtures {
				if evalFixture != "" && fixture.Name != evalFixture {
					continue
				}
				if fixture.Timeout == 0 && cfg.Defaults.Timeout > 0 {
					fixture.Timeout = cfg.Defaults.Timeout
				}

				fmt.Printf("\n  \u25b6 Running %s / %s ...\n", agent, fixture.Name)

				// Run agent
				result := eval.RunFixture(fixture)

				if result.Error != "" {
					result.Pass = false
					eval.PrintResult(result)
					allResults = append(allResults, result)
					exitCode = 1
					continue
				}

				// Structural scoring
				result.Structural = eval.RunStructuralChecks(result.Output, rubric.StructuralChecks)
				result.StructuralPass = eval.StructuralPassed(result.Structural)
				result.Pass = result.StructuralPass

				// LLM quality scoring
				if evalDeep && !evalStructural {
					fmt.Printf("    \U0001f9d1\u200d\u2696\ufe0f Running LLM judge...\n")
					scores, err := eval.RunQualityScoring(evalsDir, fixture, rubric, result.Output)
					if err != nil {
						fmt.Printf("    \u26a0 Judge error: %v\n", err)
					} else {
						result.Quality = scores
						result.QualityComposite = eval.CompositeScore(scores, rubric.QualityDimensions)
						result.QualityPass = result.QualityComposite >= threshold
						result.Pass = result.StructuralPass && result.QualityPass
					}
				}

				if !result.Pass {
					exitCode = 1
				}

				eval.PrintResult(result)
				allResults = append(allResults, result)
			}
		}

		eval.PrintSummary(allResults)

		// Save results
		if evalSave || jsonOutput {
			if jsonOutput {
				os.Stdout.Write(eval.ResultsJSON(allResults))
			}
			dir, err := eval.SaveResults(evalsDir, allResults)
			if err == nil && !jsonOutput {
				fmt.Printf("  Results saved to %s\n", dir)
			}
		}

		if exitCode != 0 {
			os.Exit(1)
		}
		return nil
	},
}

func resolveAgents(evalsDir string, cfg eval.Config, args []string) ([]string, error) {
	if len(args) > 0 {
		return args, nil
	}
	if evalProfile != "" {
		agents, ok := cfg.Profiles[evalProfile]
		if !ok {
			return nil, fmt.Errorf("unknown profile: %s", evalProfile)
		}
		return agents, nil
	}
	if evalAll || evalList {
		// Discover all agents with fixtures
		entries, err := os.ReadDir(fmt.Sprintf("%s/fixtures", evalsDir))
		if err != nil {
			return nil, fmt.Errorf("no fixtures directory: %w", err)
		}
		var agents []string
		for _, e := range entries {
			if e.IsDir() {
				agents = append(agents, e.Name())
			}
		}
		return agents, nil
	}
	return nil, fmt.Errorf("specify an agent name, --all, or --profile")
}

func listFixtures(evalsDir string, agents []string) error {
	fmt.Println("\U0001f4cb Available fixtures:")
	for _, agent := range agents {
		fixtures, err := eval.LoadFixtures(evalsDir, agent)
		if err != nil {
			continue
		}
		for _, f := range fixtures {
			desc := f.Description
			if desc == "" {
				desc = f.Name
			}
			fmt.Printf("  %-25s %s\n", agent+"/"+f.Name, desc)
		}
	}
	return nil
}

func init() {
	evalCmd.Flags().BoolVar(&evalDeep, "deep", false, "Run LLM quality scoring (costs tokens)")
	evalCmd.Flags().BoolVar(&evalStructural, "structural", false, "Run only structural checks")
	evalCmd.Flags().StringVar(&evalFixture, "fixture", "", "Run specific fixture by name")
	evalCmd.Flags().StringVar(&evalProfile, "profile", "", "Run agents in a config profile")
	evalCmd.Flags().IntVar(&evalThreshold, "threshold", 0, "Quality score threshold (default: from config)")
	evalCmd.Flags().BoolVar(&evalAll, "all", false, "Run all agents with fixtures")
	evalCmd.Flags().BoolVar(&evalList, "list", false, "List available fixtures")
	evalCmd.Flags().BoolVar(&evalSave, "save", false, "Save results to evals/results/")
}
