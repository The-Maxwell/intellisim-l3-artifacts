package models

import (
	"intelli.sim.core/data/models"
	mongocommon "intelli.sim.core/data/mongodb/common"
)

// LLMConfigDocument is the reusable (LLM target + permanent prompt context) pair —
// the building block of the model's model-generation, simulation-generation, and
// simulation-execution configs, and of the per-agent/per-shape override configs.
type LLMConfigDocument struct {
	Target        mongocommon.LLMTargetDocument `bson:"target"`
	PromptContext string                        `bson:"prompt_context,omitempty"`
}

// SimulationConfigDocument groups everything that governs simulations created from a
// model: the update mode plus the simulation-generation and simulation-execution LLM
// configs.
type SimulationConfigDocument struct {
	UpdateMode models.ModelUpdateMode `bson:"update_mode"`

	LLMSimulationGenerationConfig LLMConfigDocument                    `bson:"llm_simulation_generation_config"`
	LLMSimulationExecutionConfig  LLMSimulationExecutionConfigDocument `bson:"llm_simulation_execution_config"`

	// Termination is the default early-stop condition stamped onto simulations
	// created from this model (U2). Absent = run to Steps.
	Termination *ConditionDocument `bson:"termination,omitempty"`
}

// LLMAgentConfigDocument holds the agent-level LLM target/context (embedded LLMConfig)
// plus the overridden strategy and runtime config of individual agents/archetypes.
type LLMAgentConfigDocument struct {
	LLMConfigDocument `bson:",inline"`

	Strategy         LLMAgentStrategyDocument          `bson:"strategy"`
	SimulationConfig *LLMSimulationAgentConfigDocument `bson:"simulation_config,omitempty"`
}

// LLMShapeConfigDocument holds the shape-level LLM target/context (embedded LLMConfig)
// plus the overridden strategy of individual shapes.
type LLMShapeConfigDocument struct {
	LLMConfigDocument `bson:",inline"`

	Strategy LLMShapeStrategyDocument `bson:"strategy"`
}

// LLMSimulationAgentConfigDocument is per-agent runtime/spatial config (overridable).
// FallbackPolicy is a pointer so "unset = inherit".
type LLMSimulationAgentConfigDocument struct {
	PerceptionRadius  float64                 `bson:"perception_radius,omitempty"`
	InteractionRadius float64                 `bson:"interaction_radius,omitempty"`
	MemorySize        int                     `bson:"memory_size,omitempty"`
	FallbackPolicy    *FallbackPolicyDocument `bson:"fallback_policy,omitempty"`
}

// LLMSimulationExecutionConfigDocument is the Level-2 (run-engine) LLM config: the
// default target+context (embedded LLMConfig), the default per-agent runtime/spatial
// knobs (embedded LLMSimulationAgentConfig), run-engine tuning, and the DefaultStrategy.
type LLMSimulationExecutionConfigDocument struct {
	LLMConfigDocument                `bson:",inline"`
	LLMSimulationAgentConfigDocument `bson:",inline"`

	ParallelWorkers uint `bson:"parallel_workers,omitempty"`
	RetryTimeoutMs  uint `bson:"retry_timeout_ms,omitempty"` // per-LLM-call timeout
	MaxRetries      uint `bson:"max_retries,omitempty"`
	StreamBatchSize uint `bson:"stream_batch_size,omitempty"` // WebSocket chunking

	DefaultStrategy LLMStrategyDocument `bson:"default_strategy"`
}
