package simulations

import (
	mongocommon "intelli.sim.core/data/mongodb/common"
	"intelli.sim.core/data/simulations"
)

// AgentTokenUsageDocument is token spend attributed to one agent/archetype
// definition (by AgentUID).
type AgentTokenUsageDocument struct {
	AgentUID                       string `bson:"agent_uid"`
	mongocommon.TokenUsageDocument `bson:",inline"`
}

// PurposeTokenUsageDocument is token spend attributed to one LLM usage purpose.
type PurposeTokenUsageDocument struct {
	Purpose                        simulations.LLMUsagePurpose `bson:"purpose"`
	mongocommon.TokenUsageDocument `bson:",inline"`
}

// SimulationTokenUsageDocument is the extended, multi-dimensional token
// accounting for a simulation: grand total plus breakdowns by model, agent,
// purpose, and generation run.
type SimulationTokenUsageDocument struct {
	Total        mongocommon.TokenUsageDocument   `bson:"total"`
	ByModel      []mongocommon.TokenUsageDocument `bson:"by_model,omitempty"`      // per (provider, model, effort)
	ByAgent      []AgentTokenUsageDocument        `bson:"by_agent,omitempty"`      // per AgentUID
	ByPurpose    []PurposeTokenUsageDocument      `bson:"by_purpose,omitempty"`    // per LLMUsagePurpose
	ByGeneration []mongocommon.TokenUsageDocument `bson:"by_generation,omitempty"` // indexed by generation run Seq-1
	ByRun        []mongocommon.TokenUsageDocument `bson:"by_run,omitempty"`        // indexed by execution run Seq-1
}
