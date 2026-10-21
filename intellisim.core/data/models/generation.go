package models

// PropertyGenerationMode selects the strategy used by the simulation generator when
// producing the initial value for an AgentProperty.
type PropertyGenerationMode string

const (
	// GenModeDefault uses DefaultValue if set, otherwise falls back to a uniform
	// random draw within [MinValue..MaxValue] or a random choice from AllowedValues.
	GenModeDefault PropertyGenerationMode = "default"
	// GenModeRandom draws from the distribution described in PropertyGeneration.
	GenModeRandom PropertyGenerationMode = "random"
	// GenModeLLM asks the LLM to produce the initial value, using PromptContext as
	// the per-property hint injected into the prompt.
	GenModeLLM PropertyGenerationMode = "llm"
)

// DistributionType identifies the statistical distribution used in GenModeRandom.
type DistributionType string

const (
	// DistUniform draws uniformly from [Min..Max].
	DistUniform DistributionType = "uniform"
	// DistNormal draws from a Gaussian with Mean and StdDev, optionally clamped to [Min..Max].
	DistNormal DistributionType = "normal"
	// DistChoice samples one value from Choices according to their relative Weights.
	DistChoice DistributionType = "choice"
)

// ChoiceWeight pairs a candidate value with its relative sampling weight (must be > 0).
type ChoiceWeight struct {
	// Value is the candidate to sample. Canonical dynamic types (matching JSON
	// unmarshalling defaults): string, bool, or float64. Other scalar types are
	// tolerated but may not survive a BSON round-trip unchanged — use float64 for
	// all numeric choices.
	Value  interface{}
	Weight float64 // > 0
}

// PropertyGeneration tells the simulation generator how to produce the initial value
// for an AgentProperty at t=0.
//
// nil (absent) means: use DefaultValue if set, otherwise draw uniformly from
// [MinValue..MaxValue] or pick randomly from AllowedValues — identical to GenModeDefault.
type PropertyGeneration struct {
	Mode         PropertyGenerationMode
	Distribution DistributionType // GenModeRandom only
	Mean         *float64         // DistNormal
	StdDev       *float64         // DistNormal
	Min          *float64         // optional init-range override (overrides AgentProperty.MinValue)
	Max          *float64         // optional init-range override (overrides AgentProperty.MaxValue)
	Choices      []ChoiceWeight   // DistChoice
	PromptContext string           // GenModeLLM: per-property hint injected into the generation prompt
}

// PopulationCountMode selects how the simulation generator resolves the initial agent
// population count at simulation creation time.
type PopulationCountMode string

const (
	// CountFixed uses AgentInitialization.PopulationCount exactly.
	CountFixed PopulationCountMode = "fixed"
	// CountRandomRange draws uniformly from [MinPopulation..MaxPopulation].
	CountRandomRange PopulationCountMode = "random_range"
	// CountLLM asks the LLM to decide the initial population count.
	CountLLM PopulationCountMode = "llm"
)
