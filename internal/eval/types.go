package eval

import "time"

// Fixture is an input scenario for an agent eval.
type Fixture struct {
	Agent       string `yaml:"agent"`
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Timeout     int    `yaml:"timeout"` // seconds
	Tags        []string `yaml:"tags"`
	Prompt      string // body after frontmatter
	Path        string // source file path
}

// Rubric defines scoring criteria for an agent.
type Rubric struct {
	Agent            string             `yaml:"agent"`
	Description      string             `yaml:"description"`
	StructuralChecks []StructuralCheck  `yaml:"structural_checks"`
	QualityDimensions []QualityDimension `yaml:"quality_dimensions"`
	Thresholds       Thresholds         `yaml:"thresholds"`
}

// StructuralCheck is a regex-based check on agent output.
type StructuralCheck struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Pattern     string `yaml:"pattern"`
	Required    bool   `yaml:"required"`
	Expect      string `yaml:"expect"` // "present" (default) or "absent"
}

// QualityDimension is an LLM-scored quality axis.
type QualityDimension struct {
	Name        string `yaml:"name"`
	Weight      int    `yaml:"weight"`
	Description string `yaml:"description"`
}

// Thresholds defines pass/fail cutoffs.
type Thresholds struct {
	Structural int `yaml:"structural"` // percent of required checks that must pass
	Quality    int `yaml:"quality"`    // minimum composite quality score
}

// StructuralResult is the outcome of one structural check.
type StructuralResult struct {
	Name    string `json:"name"`
	Passed  bool   `json:"passed"`
	Required bool  `json:"required"`
}

// DimensionScore is the LLM judge's score for one quality dimension.
type DimensionScore struct {
	Name      string `json:"name"`
	Score     int    `json:"score"`
	Reasoning string `json:"reasoning"`
}

// EvalResult is the complete result for one fixture run.
type EvalResult struct {
	Agent            string             `json:"agent"`
	Fixture          string             `json:"fixture"`
	Output           string             `json:"output,omitempty"`
	Duration         time.Duration      `json:"duration"`
	Error            string             `json:"error,omitempty"`
	Structural       []StructuralResult `json:"structural"`
	StructuralPass   bool               `json:"structural_pass"`
	Quality          []DimensionScore   `json:"quality,omitempty"`
	QualityComposite int                `json:"quality_composite,omitempty"`
	QualityPass      bool               `json:"quality_pass"`
	Pass             bool               `json:"pass"`
}

// Config is the top-level eval configuration.
type Config struct {
	Defaults ConfigDefaults `yaml:"defaults"`
	Judge    JudgeConfig    `yaml:"judge"`
	Profiles map[string][]string `yaml:"profiles"`
}

// ConfigDefaults holds default eval settings.
type ConfigDefaults struct {
	Timeout             int `yaml:"timeout"`
	ThresholdStructural int `yaml:"threshold_structural"`
	ThresholdQuality    int `yaml:"threshold_quality"`
}

// JudgeConfig holds LLM-as-judge settings.
type JudgeConfig struct {
	Agent  string `yaml:"agent"`
	Prompt string `yaml:"prompt"`
}
