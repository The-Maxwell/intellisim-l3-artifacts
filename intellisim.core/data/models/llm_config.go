package models

import (
	"intelli.sim.core/data/common"
)

// LLMConfig is the reusable (LLM target + permanent prompt context) pair. It is the
// building block of the model's three LLM configs — model generation (Model.LLMConfig),
// simulation generation (SimulationConfig.LLMSimulationGenerationConfig), and simulation
// execution (embedded in LLMSimulationExecutionConfig) — and is also embedded by the
// per-agent and per-shape override configs. All three top-level configs are optional;
// a config with an empty Target.ProviderUID means "no LLM is used for that purpose".
type LLMConfig struct {
	Target        common.LLMTarget // provider, model, effort (+ optional generation params / timeout)
	PromptContext string           // permanent context injected into prompts for this purpose
}

// LLMSimulationExecutionConfig is the Level-2 (simulation execution / run-engine) LLM
// config: the default LLM target + prompt context (embedded LLMConfig), the default
// per-agent runtime/spatial knobs (embedded LLMSimulationAgentConfig), the run-engine
// tuning below, and the DefaultStrategy that individual agents/archetypes inherit unless
// they override it. Lives at Model.SimulationConfig.LLMSimulationExecutionConfig; a
// simulation may override the target via Simulation.LLMExecutionTargetOverride.
type LLMSimulationExecutionConfig struct {
	LLMConfig
	LLMSimulationAgentConfig

	ParallelWorkers uint
	RetryTimeoutMs  uint // per-LLM-call timeout
	MaxRetries      uint
	StreamBatchSize uint // WebSocket chunking

	// DefaultStrategy is the model-wide default LLM strategy for agents/archetypes.
	DefaultStrategy LLMStrategy
}

// LLMAgentConfig holds the agent-level LLM target selection and the overridden strategy and
// runtime config of individual agents/archetypes.
type LLMAgentConfig struct {
	LLMConfig

	Strategy         LLMAgentStrategy
	SimulationConfig *LLMSimulationAgentConfig
}

// LLMShapeConfig holds the shape-level LLM target selection and the overridden strategy and
// runtime config of individual shapes.
type LLMShapeConfig struct {
	LLMConfig

	Strategy LLMShapeStrategy
}

// LLMSimulationAgentConfig is per-agent runtime/spatial config (overridable).
// FallbackPolicy is a pointer so "unset = inherit".
type LLMSimulationAgentConfig struct {
	PerceptionRadius  float64
	InteractionRadius float64
	MemorySize        int
	FallbackPolicy    *FallbackPolicy
}
