package simulations_test

import (
	"testing"

	"intelli.sim.core/data/common"
	"intelli.sim.core/data/simulations"
)

// TestAddRun_FoldsAllDimensions verifies that AddRun accumulates Total,
// ByPurpose, ByAgent, and a single ByRun entry across two same-target calls.
func TestAddRun_FoldsAllDimensions(t *testing.T) {
	u := &simulations.SimulationTokenUsage{}
	tgt := common.LLMTarget{ProviderUID: "openai", ModelUID: "gpt-4o-mini"}
	tu := common.TokenUsage{LLMTarget: tgt, PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15}
	u.AddRun(tu, "wolf", simulations.LLMUsageSimulationDecision, 0)
	u.AddRun(tu, "wolf", simulations.LLMUsageSimulationDecision, 0)
	if got := u.Total.TotalTokens; got != 30 {
		t.Fatalf("Total=%d want 30", got)
	}
	if len(u.ByRun) != 1 || u.ByRun[0].TotalTokens != 30 {
		t.Fatalf("ByRun=%+v want one entry of 30", u.ByRun)
	}
	if len(u.ByPurpose) == 0 {
		t.Fatal("ByPurpose not populated")
	}
	if len(u.ByAgent) == 0 {
		t.Fatal("ByAgent not populated")
	}
}

// TestAddRun_MixedTargetsBlankByRunTarget verifies that when one run mixes two
// distinct targets, the ByRun entry's LLMTarget is blanked.
func TestAddRun_MixedTargetsBlankByRunTarget(t *testing.T) {
	u := &simulations.SimulationTokenUsage{}
	a := common.TokenUsage{LLMTarget: common.LLMTarget{ProviderUID: "openai", ModelUID: "gpt-4o"}, TotalTokens: 5}
	b := common.TokenUsage{LLMTarget: common.LLMTarget{ProviderUID: "openai", ModelUID: "gpt-4o-mini"}, TotalTokens: 7}
	u.AddRun(a, "", simulations.LLMUsageSimulationMovement, 0)
	u.AddRun(b, "", simulations.LLMUsageSimulationMovement, 0)
	if u.ByRun[0].LLMTarget.ModelUID != "" {
		t.Fatalf("expected blanked target on mixed run, got %q", u.ByRun[0].LLMTarget.ModelUID)
	}
}

// makeUsage builds a TokenUsage with the given provider, model, effort, and
// token counts for compact test table construction.
func makeUsage(providerUID, modelUID string, effort common.LLMEffort, prompt, completion, total, calls uint64, latencyMs float64) common.TokenUsage {
	return common.TokenUsage{
		LLMTarget: common.LLMTarget{
			ProviderUID: providerUID,
			ModelUID:    modelUID,
			Effort:      effort,
		},
		PromptTokens:     prompt,
		CompletionTokens: completion,
		TotalTokens:      total,
		Calls:            calls,
		LatencyMs:        latencyMs,
	}
}

// TestAdd_TwoSameTargetCallsInOneRun verifies that two calls with the same
// (provider, model, effort) in the same generation run produce a single
// ByGeneration entry that keeps the LLMTarget and accumulates the numbers.
func TestAdd_TwoSameTargetCallsInOneRun(t *testing.T) {
	u := &simulations.SimulationTokenUsage{}
	u1 := makeUsage("p1", "m1", common.LLMEffortMedium, 10, 5, 15, 1, 100)
	u2 := makeUsage("p1", "m1", common.LLMEffortMedium, 20, 10, 30, 1, 200)

	u.Add(u1, "agent-a", simulations.LLMUsageSimulationGeneration, 0)
	u.Add(u2, "agent-a", simulations.LLMUsageSimulationGeneration, 0)

	// ByGeneration should have exactly one entry (runIdx=0).
	if got := len(u.ByGeneration); got != 1 {
		t.Fatalf("ByGeneration len = %d, want 1", got)
	}
	gen := u.ByGeneration[0]

	// Target should be preserved (same target for both calls).
	if gen.ProviderUID != "p1" || gen.ModelUID != "m1" || gen.Effort != common.LLMEffortMedium {
		t.Errorf("ByGeneration[0].LLMTarget = {%s %s %s}, want {p1 m1 medium}",
			gen.ProviderUID, gen.ModelUID, gen.Effort)
	}

	// Numbers must be summed.
	if gen.PromptTokens != 30 {
		t.Errorf("PromptTokens = %d, want 30", gen.PromptTokens)
	}
	if gen.TotalTokens != 45 {
		t.Errorf("TotalTokens = %d, want 45", gen.TotalTokens)
	}
	if gen.Calls != 2 {
		t.Errorf("Calls = %d, want 2", gen.Calls)
	}
}

// TestAdd_MixedTargetsInOneRun verifies that two calls with different
// (provider, model, effort) in the same run blank the ByGeneration entry's
// LLMTarget while still accumulating numeric fields.
func TestAdd_MixedTargetsInOneRun(t *testing.T) {
	u := &simulations.SimulationTokenUsage{}
	u1 := makeUsage("p1", "m1", common.LLMEffortLow, 10, 5, 15, 1, 50)
	u2 := makeUsage("p2", "m2", common.LLMEffortHigh, 20, 10, 30, 1, 100)

	u.Add(u1, "", simulations.LLMUsageSimulationGeneration, 0)
	u.Add(u2, "", simulations.LLMUsageSimulationGeneration, 0)

	if got := len(u.ByGeneration); got != 1 {
		t.Fatalf("ByGeneration len = %d, want 1", got)
	}
	gen := u.ByGeneration[0]

	// Target must be blanked because the two calls used different targets.
	if gen.ProviderUID != "" || gen.ModelUID != "" || gen.Effort != "" {
		t.Errorf("ByGeneration[0].LLMTarget should be zero, got {%s %s %s}",
			gen.ProviderUID, gen.ModelUID, gen.Effort)
	}

	// Numbers must still be accumulated.
	if gen.TotalTokens != 45 {
		t.Errorf("TotalTokens = %d, want 45", gen.TotalTokens)
	}
	if gen.Calls != 2 {
		t.Errorf("Calls = %d, want 2", gen.Calls)
	}
}

// TestAdd_TwoRuns verifies that two generation runs produce two separate
// ByGeneration entries at the correct indices.
func TestAdd_TwoRuns(t *testing.T) {
	u := &simulations.SimulationTokenUsage{}
	u1 := makeUsage("p1", "m1", common.LLMEffortMedium, 10, 5, 15, 1, 100)
	u2 := makeUsage("p1", "m1", common.LLMEffortMedium, 20, 10, 30, 1, 200)

	u.Add(u1, "", simulations.LLMUsageSimulationGeneration, 0) // run 1, idx=0
	u.Add(u2, "", simulations.LLMUsageSimulationGeneration, 1) // run 2, idx=1

	if got := len(u.ByGeneration); got != 2 {
		t.Fatalf("ByGeneration len = %d, want 2", got)
	}

	// Index 0 should carry run-1 numbers.
	if u.ByGeneration[0].TotalTokens != 15 {
		t.Errorf("ByGeneration[0].TotalTokens = %d, want 15", u.ByGeneration[0].TotalTokens)
	}

	// Index 1 should carry run-2 numbers.
	if u.ByGeneration[1].TotalTokens != 30 {
		t.Errorf("ByGeneration[1].TotalTokens = %d, want 30", u.ByGeneration[1].TotalTokens)
	}
}

// TestAdd_GrowthFromEmptyToIdx1 verifies that when the first reported call
// is for runIdx=1 (run-2), the ByGeneration slice is grown to length 2 with
// a zero entry at index 0.
func TestAdd_GrowthFromEmptyToIdx1(t *testing.T) {
	u := &simulations.SimulationTokenUsage{}
	usage := makeUsage("p1", "m1", common.LLMEffortLow, 5, 2, 7, 1, 30)

	u.Add(usage, "", simulations.LLMUsageSimulationGeneration, 1) // skip idx=0

	if got := len(u.ByGeneration); got != 2 {
		t.Fatalf("ByGeneration len = %d, want 2 (zero padding at 0)", got)
	}

	// Index 0 must be zero.
	zero := u.ByGeneration[0]
	if zero.TotalTokens != 0 || zero.Calls != 0 {
		t.Errorf("ByGeneration[0] should be zero, got %+v", zero)
	}

	// Index 1 must carry the reported usage.
	if u.ByGeneration[1].TotalTokens != 7 {
		t.Errorf("ByGeneration[1].TotalTokens = %d, want 7", u.ByGeneration[1].TotalTokens)
	}
}

// TestAdd_NegativeRunIdxLeavesGenerationUntouched verifies that passing a
// negative runIdx (non-generation call) does not grow ByGeneration.
func TestAdd_NegativeRunIdxLeavesGenerationUntouched(t *testing.T) {
	u := &simulations.SimulationTokenUsage{}
	usage := makeUsage("p1", "m1", common.LLMEffortLow, 5, 2, 7, 1, 30)

	u.Add(usage, "agent-x", simulations.LLMUsageSimulationDecision, -1)

	if got := len(u.ByGeneration); got != 0 {
		t.Errorf("ByGeneration len = %d, want 0 for non-generation call", got)
	}
}

// TestAdd_EmptyAgentUID verifies that a call with agentUID="" contributes to
// Total/ByModel/ByPurpose but does NOT create a ByAgent entry.
func TestAdd_EmptyAgentUID(t *testing.T) {
	u := &simulations.SimulationTokenUsage{}
	usage := makeUsage("p1", "m1", common.LLMEffortMedium, 10, 5, 15, 1, 100)

	u.Add(usage, "", simulations.LLMUsageSimulationProperties, -1)

	if got := len(u.ByAgent); got != 0 {
		t.Errorf("ByAgent len = %d, want 0 when agentUID is empty", got)
	}
	if u.Total.TotalTokens != 15 {
		t.Errorf("Total.TotalTokens = %d, want 15", u.Total.TotalTokens)
	}
}

// TestAdd_AllDimensionsConsistent verifies that after several calls the sum
// of ByPurpose totals equals the grand Total (same logic applies to ByModel).
func TestAdd_AllDimensionsConsistent(t *testing.T) {
	u := &simulations.SimulationTokenUsage{}

	calls := []struct {
		usage   common.TokenUsage
		agent   string
		purpose simulations.LLMUsagePurpose
		runIdx  int
	}{
		{makeUsage("p1", "m1", common.LLMEffortLow, 10, 5, 15, 1, 100), "agent-a", simulations.LLMUsageSimulationGeneration, 0},
		{makeUsage("p1", "m1", common.LLMEffortLow, 20, 10, 30, 1, 200), "agent-b", simulations.LLMUsageSimulationDecision, -1},
		{makeUsage("p2", "m2", common.LLMEffortHigh, 5, 3, 8, 1, 50), "agent-a", simulations.LLMUsageSimulationMovement, -1},
	}

	for _, c := range calls {
		u.Add(c.usage, c.agent, c.purpose, c.runIdx)
	}

	wantTotal := uint64(15 + 30 + 8)

	// Grand total must equal the sum of all calls.
	if u.Total.TotalTokens != wantTotal {
		t.Errorf("Total.TotalTokens = %d, want %d", u.Total.TotalTokens, wantTotal)
	}

	// Grand total LLMTarget must be zero.
	if u.Total.ProviderUID != "" || u.Total.ModelUID != "" {
		t.Errorf("Total.LLMTarget should be zero, got {%s %s}", u.Total.ProviderUID, u.Total.ModelUID)
	}

	// Sum of ByPurpose tokens must match grand total.
	var purposeSum uint64
	for _, p := range u.ByPurpose {
		purposeSum += p.TotalTokens
	}
	if purposeSum != wantTotal {
		t.Errorf("sum(ByPurpose.TotalTokens) = %d, want %d", purposeSum, wantTotal)
	}

	// Sum of ByModel tokens must match grand total.
	var modelSum uint64
	for _, m := range u.ByModel {
		modelSum += m.TotalTokens
	}
	if modelSum != wantTotal {
		t.Errorf("sum(ByModel.TotalTokens) = %d, want %d", modelSum, wantTotal)
	}

	// agent-a appears twice (runs 0 and analysis call), agent-b once.
	agentTotals := make(map[string]uint64)
	for _, a := range u.ByAgent {
		agentTotals[a.AgentUID] += a.TotalTokens
	}
	if agentTotals["agent-a"] != 15+8 {
		t.Errorf("ByAgent[agent-a].TotalTokens = %d, want %d", agentTotals["agent-a"], uint64(15+8))
	}
	if agentTotals["agent-b"] != 30 {
		t.Errorf("ByAgent[agent-b].TotalTokens = %d, want %d", agentTotals["agent-b"], uint64(30))
	}

	// ByGeneration must have one entry (only the first call had runIdx>=0).
	if got := len(u.ByGeneration); got != 1 {
		t.Errorf("ByGeneration len = %d, want 1", got)
	}
	if u.ByGeneration[0].TotalTokens != 15 {
		t.Errorf("ByGeneration[0].TotalTokens = %d, want 15", u.ByGeneration[0].TotalTokens)
	}
}
