package simulations

import "intelli.sim.core/data/common"

// LLMUsagePurpose classifies what an LLM call was used for, so token spend
// can be attributed across the three integration levels and the runtime
// operations.
type LLMUsagePurpose string

const (
	// LLMUsageSimulationGeneration is the initial-state generation of a
	// simulation from its model (Level-1 seeding pipeline).
	LLMUsageSimulationGeneration LLMUsagePurpose = "simulation_generation"

	// LLMUsageSimulationMovement is a Level-2 runtime call deciding agent
	// movement during execution.
	LLMUsageSimulationMovement LLMUsagePurpose = "simulation_movement"

	// LLMUsageSimulationProperties is a Level-2 runtime call updating agent
	// property values during execution.
	LLMUsageSimulationProperties LLMUsagePurpose = "simulation_properties"

	// LLMUsageSimulationDecision is a Level-2 runtime call producing an agent
	// decision during execution.
	LLMUsageSimulationDecision LLMUsagePurpose = "simulation_decision"

	// LLMUsageSimulationInteraction is a Level-2 runtime call mediating an
	// agent interaction during execution.
	LLMUsageSimulationInteraction LLMUsagePurpose = "simulation_interaction"

	// LLMUsagePurposeSimulationShape is a Level-2 runtime call updating shape
	// property values during execution (shape-LLM phase, Task 19).
	LLMUsagePurposeSimulationShape LLMUsagePurpose = "simulation_shape"
)

// AgentTokenUsage is token spend attributed to one agent/archetype definition
// (by AgentUID).
type AgentTokenUsage struct {
	AgentUID string `json:"agent_uid"`
	common.TokenUsage
}

// PurposeTokenUsage is token spend attributed to one LLM usage purpose.
type PurposeTokenUsage struct {
	Purpose LLMUsagePurpose `json:"purpose"`
	common.TokenUsage
}

// SimulationTokenUsage is the extended, multi-dimensional token accounting for
// a simulation: the grand total plus breakdowns by model, by agent/archetype,
// by usage purpose, and by generation run. This makes it possible to see how
// much was spent, by whom, and for what at every stage of the lifecycle.
type SimulationTokenUsage struct {
	Total     common.TokenUsage   `json:"total"`
	ByModel   []common.TokenUsage `json:"by_model,omitempty"`   // per (provider, model, effort)
	ByAgent   []AgentTokenUsage   `json:"by_agent,omitempty"`   // per AgentUID (agent/archetype type)
	ByPurpose []PurposeTokenUsage `json:"by_purpose,omitempty"` // per LLMUsagePurpose
	// ByGeneration is indexed by generation run Seq-1. The LLMTarget of an
	// entry is blanked when the run mixed multiple distinct targets.
	ByGeneration []common.TokenUsage `json:"by_generation,omitempty"`
	// ByRun is indexed by execution run Seq-1 (ExecutionRun.Seq-1). The
	// LLMTarget of an entry is blanked when the run mixed multiple distinct
	// targets.
	ByRun []common.TokenUsage `json:"by_run,omitempty"`
}

// Add folds one completed LLM call into every dimension. runIdx is the
// 0-based generation-run index (Seq-1); pass a negative runIdx for
// non-generation calls. agentUID may be "" when the call is not attributable
// to one agent definition.
//
// Accumulation rules:
//
//   - Total: counts and latency are summed; Total.LLMTarget is always zero.
//   - ByModel: entries are keyed on (ProviderUID, ModelUID, Effort); a new
//     entry is appended when no match is found; matched entries accumulate.
//   - ByAgent: when agentUID != "", the entry keyed on AgentUID is found or
//     appended; its TokenUsage.LLMTarget is always zero.
//   - ByPurpose: the entry keyed on Purpose is found or appended; its
//     TokenUsage.LLMTarget is always zero.
//   - ByGeneration: when runIdx >= 0, the slice is grown (zero-padded) so
//     that len > runIdx; the first call into an entry sets its LLMTarget from
//     t; a subsequent call with a different (ProviderUID, ModelUID, Effort)
//     blanks the entry's LLMTarget while still accumulating numeric fields.
func (u *SimulationTokenUsage) Add(t common.TokenUsage, agentUID string, purpose LLMUsagePurpose, runIdx int) {
	// ── Total ─────────────────────────────────────────────────────────────
	common.Accumulate(&u.Total, t)
	u.Total.LLMTarget = common.LLMTarget{}

	// ── ByModel ───────────────────────────────────────────────────────────
	matched := false
	for i := range u.ByModel {
		m := u.ByModel[i].LLMTarget
		if m.ProviderUID == t.ProviderUID && m.ModelUID == t.ModelUID && m.Effort == t.Effort {
			common.Accumulate(&u.ByModel[i], t)
			matched = true
			break
		}
	}
	if !matched {
		u.ByModel = append(u.ByModel, t)
	}

	// ── ByAgent ───────────────────────────────────────────────────────────
	if agentUID != "" {
		agentMatched := false
		for i := range u.ByAgent {
			if u.ByAgent[i].AgentUID == agentUID {
				common.Accumulate(&u.ByAgent[i].TokenUsage, t)
				u.ByAgent[i].LLMTarget = common.LLMTarget{}
				agentMatched = true
				break
			}
		}
		if !agentMatched {
			entry := AgentTokenUsage{AgentUID: agentUID}
			common.Accumulate(&entry.TokenUsage, t)
			entry.LLMTarget = common.LLMTarget{}
			u.ByAgent = append(u.ByAgent, entry)
		}
	}

	// ── ByPurpose ─────────────────────────────────────────────────────────
	purposeMatched := false
	for i := range u.ByPurpose {
		if u.ByPurpose[i].Purpose == purpose {
			common.Accumulate(&u.ByPurpose[i].TokenUsage, t)
			u.ByPurpose[i].LLMTarget = common.LLMTarget{}
			purposeMatched = true
			break
		}
	}
	if !purposeMatched {
		entry := PurposeTokenUsage{Purpose: purpose}
		common.Accumulate(&entry.TokenUsage, t)
		entry.LLMTarget = common.LLMTarget{}
		u.ByPurpose = append(u.ByPurpose, entry)
	}

	// ── ByGeneration ──────────────────────────────────────────────────────
	if runIdx >= 0 {
		accumulateInto(&u.ByGeneration, runIdx, t)
	}
}

// AddRun folds one completed runtime call into Total, ByModel, ByAgent,
// ByPurpose, and the ByRun entry at index runIdx (0-based; ExecutionRun.Seq-1).
// agentUID may be "" (not agent-scoped). ByRun entries keep LLMTarget only while
// every call of that run shares one target; on a mix the entry's LLMTarget is
// reset to zero. The Total/ByModel/ByAgent/ByPurpose accumulation is identical
// to Add; only the per-run dimension differs.
func (u *SimulationTokenUsage) AddRun(t common.TokenUsage, agentUID string, purpose LLMUsagePurpose, runIdx int) {
	// ── Total ─────────────────────────────────────────────────────────────
	common.Accumulate(&u.Total, t)
	u.Total.LLMTarget = common.LLMTarget{}

	// ── ByModel ───────────────────────────────────────────────────────────
	matched := false
	for i := range u.ByModel {
		m := u.ByModel[i].LLMTarget
		if m.ProviderUID == t.ProviderUID && m.ModelUID == t.ModelUID && m.Effort == t.Effort {
			common.Accumulate(&u.ByModel[i], t)
			matched = true
			break
		}
	}
	if !matched {
		u.ByModel = append(u.ByModel, t)
	}

	// ── ByAgent ───────────────────────────────────────────────────────────
	if agentUID != "" {
		agentMatched := false
		for i := range u.ByAgent {
			if u.ByAgent[i].AgentUID == agentUID {
				common.Accumulate(&u.ByAgent[i].TokenUsage, t)
				u.ByAgent[i].LLMTarget = common.LLMTarget{}
				agentMatched = true
				break
			}
		}
		if !agentMatched {
			entry := AgentTokenUsage{AgentUID: agentUID}
			common.Accumulate(&entry.TokenUsage, t)
			entry.LLMTarget = common.LLMTarget{}
			u.ByAgent = append(u.ByAgent, entry)
		}
	}

	// ── ByPurpose ─────────────────────────────────────────────────────────
	purposeMatched := false
	for i := range u.ByPurpose {
		if u.ByPurpose[i].Purpose == purpose {
			common.Accumulate(&u.ByPurpose[i].TokenUsage, t)
			u.ByPurpose[i].LLMTarget = common.LLMTarget{}
			purposeMatched = true
			break
		}
	}
	if !purposeMatched {
		entry := PurposeTokenUsage{Purpose: purpose}
		common.Accumulate(&entry.TokenUsage, t)
		entry.LLMTarget = common.LLMTarget{}
		u.ByPurpose = append(u.ByPurpose, entry)
	}

	// ── ByRun ─────────────────────────────────────────────────────────────
	if runIdx >= 0 {
		accumulateInto(&u.ByRun, runIdx, t)
	}
}

// accumulateInto folds one call into a per-run dimension (ByGeneration / ByRun)
// at index idx. It grows the slice with zero entries until len > idx, adopts the
// call's LLMTarget on the first call into the slot, and blanks the slot's
// LLMTarget when a later call uses a different (ProviderUID, ModelUID, Effort)
// while still accumulating the numeric fields. "First call" is detected by the
// slot being freshly created here, so zero-Calls usages are handled correctly.
func accumulateInto(dimension *[]common.TokenUsage, idx int, t common.TokenUsage) {
	// A pristine slot has never been written: it has no recorded calls and no
	// adopted target. Detecting first-write this way (rather than by Calls
	// alone) is robust to usages that report zero Calls and to slots that were
	// merely zero-padded while writing a higher index. LLMTarget is not
	// comparable (Generation is a map), so compare its identity fields.
	isFirstCall := true
	if idx < len(*dimension) {
		e := (*dimension)[idx]
		isFirstCall = e.Calls == 0 &&
			e.PromptTokens == 0 && e.CompletionTokens == 0 && e.TotalTokens == 0 &&
			e.ProviderUID == "" && e.ModelUID == "" && e.Effort == ""
	}
	// Grow with zero entries until the slice is long enough.
	for len(*dimension) <= idx {
		*dimension = append(*dimension, common.TokenUsage{})
	}
	entry := &(*dimension)[idx]
	if isFirstCall {
		// First call into this slot: adopt the target.
		entry.LLMTarget = t.LLMTarget
	} else {
		// Subsequent call: blank the target if it differs.
		cur := entry.LLMTarget
		if cur.ProviderUID != t.ProviderUID || cur.ModelUID != t.ModelUID || cur.Effort != t.Effort {
			entry.LLMTarget = common.LLMTarget{}
		}
	}
	common.Accumulate(entry, t)
}
