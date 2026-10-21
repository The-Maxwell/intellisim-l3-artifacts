package simulations

import (
	"strings"
	"testing"
	"time"

	"intelli.sim.core/data/models"
	datasims "intelli.sim.core/data/simulations"
)

var fixedNow = time.Date(2026, 6, 13, 12, 0, 0, 0, time.UTC)

func TestAppendMemory_FIFOEvictsAtCap(t *testing.T) {
	ra := &RuntimeAgent{A: &datasims.SimulationAgent{}}
	for i := 0; i < 7; i++ {
		AppendMemory(ra, uint64(i), datasims.AgentMemoryDecision, "d", nil, 5, fixedNow)
	}
	if len(ra.A.Memory) != 5 {
		t.Fatalf("len=%d want 5 (cap)", len(ra.A.Memory))
	}
	if ra.A.Memory[0].Step != 2 {
		t.Fatalf("oldest step=%d want 2 (evicted 0,1)", ra.A.Memory[0].Step)
	}
	if ra.A.Memory[len(ra.A.Memory)-1].Step != 6 {
		t.Fatalf("newest step=%d want 6", ra.A.Memory[len(ra.A.Memory)-1].Step)
	}
	if !ra.A.Memory[0].Timestamp.Equal(fixedNow) {
		t.Fatalf("timestamp=%v want injected %v", ra.A.Memory[0].Timestamp, fixedNow)
	}
}

func TestAppendMemory_DefaultCapWhenNonPositive(t *testing.T) {
	ra := &RuntimeAgent{A: &datasims.SimulationAgent{}}
	for i := 0; i < 25; i++ {
		AppendMemory(ra, uint64(i), datasims.AgentMemoryProperty, "p", nil, 0, fixedNow)
	}
	if len(ra.A.Memory) != 20 {
		t.Fatalf("len=%d want 20 (default cap)", len(ra.A.Memory))
	}
	if ra.A.Memory[0].Step != 5 {
		t.Fatalf("oldest step=%d want 5 (evicted 0..4)", ra.A.Memory[0].Step)
	}
}

func TestSummarizeMemory_RecentFirstWithinBudget(t *testing.T) {
	ra := &RuntimeAgent{A: &datasims.SimulationAgent{Memory: []datasims.AgentMemory{
		{Step: 1, Type: datasims.AgentMemoryDecision, Summary: "a"},
		{Step: 2, Type: datasims.AgentMemoryInteraction, Summary: "b"},
	}}}
	s := SummarizeMemory(ra, 10)
	if !strings.Contains(s, "b") || !strings.Contains(s, "a") {
		t.Fatalf("summary missing events: %q", s)
	}
	// Newest first: the step-2 event must appear before the step-1 event.
	if strings.Index(s, "step 2") > strings.Index(s, "step 1") {
		t.Fatalf("expected newest-first ordering, got: %q", s)
	}
}

func TestSummarizeMemory_RespectsMaxEvents(t *testing.T) {
	ra := &RuntimeAgent{A: &datasims.SimulationAgent{Memory: []datasims.AgentMemory{
		{Step: 1, Type: datasims.AgentMemoryDecision, Summary: "first"},
		{Step: 2, Type: datasims.AgentMemoryDecision, Summary: "second"},
		{Step: 3, Type: datasims.AgentMemoryDecision, Summary: "third"},
	}}}
	s := SummarizeMemory(ra, 2)
	if !strings.Contains(s, "third") || !strings.Contains(s, "second") {
		t.Fatalf("expected the two newest events, got: %q", s)
	}
	if strings.Contains(s, "first") {
		t.Fatalf("expected oldest event dropped by budget, got: %q", s)
	}
}

func TestSummarizeMemory_EmptyReturnsEmpty(t *testing.T) {
	ra := &RuntimeAgent{A: &datasims.SimulationAgent{}}
	if got := SummarizeMemory(ra, 10); got != "" {
		t.Fatalf("empty memory summary=%q want empty", got)
	}
}

func TestMemoryMatch_ByTypeAndRecency(t *testing.T) {
	ra := &RuntimeAgent{A: &datasims.SimulationAgent{Memory: []datasims.AgentMemory{
		{Step: 5, Type: datasims.AgentMemoryInteraction, Summary: "fought wolf"},
	}}}
	if !MemoryMatch(ra, datasims.AgentMemoryInteraction, "wolf", 0) {
		t.Fatal("should match recent interaction containing 'wolf'")
	}
}

func TestMemoryMatch_WrongTypeDoesNotMatch(t *testing.T) {
	ra := &RuntimeAgent{A: &datasims.SimulationAgent{Memory: []datasims.AgentMemory{
		{Step: 5, Type: datasims.AgentMemoryDecision, Summary: "fought wolf"},
	}}}
	if MemoryMatch(ra, datasims.AgentMemoryInteraction, "wolf", 0) {
		t.Fatal("type mismatch must not match")
	}
}

func TestMemoryMatch_SubstrMiss(t *testing.T) {
	ra := &RuntimeAgent{A: &datasims.SimulationAgent{Memory: []datasims.AgentMemory{
		{Step: 5, Type: datasims.AgentMemoryInteraction, Summary: "fought sheep"},
	}}}
	if MemoryMatch(ra, datasims.AgentMemoryInteraction, "wolf", 0) {
		t.Fatal("substring miss must not match")
	}
}

func TestMemoryMatch_EmptySubstrMatchesAnyOfType(t *testing.T) {
	ra := &RuntimeAgent{A: &datasims.SimulationAgent{Memory: []datasims.AgentMemory{
		{Step: 5, Type: datasims.AgentMemoryInteraction, Summary: "anything"},
	}}}
	if !MemoryMatch(ra, datasims.AgentMemoryInteraction, "", 0) {
		t.Fatal("empty substring should match any event of that type")
	}
}

func TestMemoryMatch_WithinWindow(t *testing.T) {
	ra := &RuntimeAgent{A: &datasims.SimulationAgent{Memory: []datasims.AgentMemory{
		{Step: 2, Type: datasims.AgentMemoryInteraction, Summary: "old fight wolf"},
		{Step: 10, Type: datasims.AgentMemoryDecision, Summary: "search_food"},
	}}}
	// Current step is the newest recorded (10). within=3 => only steps >= 8 qualify,
	// so the step-2 interaction is excluded.
	if MemoryMatch(ra, datasims.AgentMemoryInteraction, "wolf", 3) {
		t.Fatal("event older than the recency window must not match")
	}
	// within large enough includes step 2.
	if !MemoryMatch(ra, datasims.AgentMemoryInteraction, "wolf", 100) {
		t.Fatal("event inside the recency window should match")
	}
}

func TestSummarizeMemoryByType_Filters(t *testing.T) {
	ra := &RuntimeAgent{A: &datasims.SimulationAgent{Memory: []datasims.AgentMemory{
		{Step: 1, Type: datasims.AgentMemoryDecision, Summary: "chose_action"},
		{Step: 2, Type: datasims.AgentMemoryRule, Summary: "rule_fired"},
		{Step: 3, Type: datasims.AgentMemoryLLM, Summary: "llm_said_go"},
		{Step: 4, Type: datasims.AgentMemoryInteraction, Summary: "met_agent"},
		{Step: 5, Type: datasims.AgentMemoryRule, Summary: "rule_cooldown"},
	}}}

	// Only AgentMemoryRule entries should appear.
	got := SummarizeMemoryByType(ra, []datasims.AgentMemoryType{datasims.AgentMemoryRule}, 10)
	if !strings.Contains(got, "rule_fired") {
		t.Fatalf("expected rule_fired in output, got: %q", got)
	}
	if !strings.Contains(got, "rule_cooldown") {
		t.Fatalf("expected rule_cooldown in output, got: %q", got)
	}
	if strings.Contains(got, "chose_action") || strings.Contains(got, "llm_said_go") || strings.Contains(got, "met_agent") {
		t.Fatalf("non-rule types must be filtered out, got: %q", got)
	}

	// Newest first: step 5 must appear before step 2.
	if strings.Index(got, "step 5") > strings.Index(got, "step 2") {
		t.Fatalf("expected newest-first ordering, got: %q", got)
	}

	// Max cap: only 1 entry allowed, so only the newest rule (step 5) appears.
	capped := SummarizeMemoryByType(ra, []datasims.AgentMemoryType{datasims.AgentMemoryRule}, 1)
	if !strings.Contains(capped, "rule_cooldown") {
		t.Fatalf("expected only newest rule in capped output, got: %q", capped)
	}
	if strings.Contains(capped, "rule_fired") {
		t.Fatalf("older rule must be excluded by cap, got: %q", capped)
	}

	// AgentMemoryLLM type filter.
	llmGot := SummarizeMemoryByType(ra, []datasims.AgentMemoryType{datasims.AgentMemoryLLM}, 10)
	if !strings.Contains(llmGot, "llm_said_go") {
		t.Fatalf("expected llm_said_go in llm-filtered output, got: %q", llmGot)
	}
	if strings.Contains(llmGot, "rule_fired") || strings.Contains(llmGot, "chose_action") {
		t.Fatalf("non-llm types must be absent, got: %q", llmGot)
	}

	// Empty result when no types match.
	empty := SummarizeMemoryByType(ra, []datasims.AgentMemoryType{datasims.AgentMemoryMovement}, 10)
	if empty != "" {
		t.Fatalf("expected empty string when no type matches, got: %q", empty)
	}

	// Nil agent returns empty.
	if SummarizeMemoryByType(nil, []datasims.AgentMemoryType{datasims.AgentMemoryRule}, 10) != "" {
		t.Fatal("nil ra must return empty string")
	}

	// max<=0 returns empty.
	if SummarizeMemoryByType(ra, []datasims.AgentMemoryType{datasims.AgentMemoryRule}, 0) != "" {
		t.Fatal("max<=0 must return empty string")
	}
}

// TestResolveMemoryCap verifies the three-level resolution chain:
//   - agent SimulationConfig.MemorySize wins when > 0
//   - model LLMSimulationExecutionConfig.MemorySize is the fallback when > 0
//   - both zero (or nil config) → 0 (AppendMemory applies its floor of 20)
func TestResolveMemoryCap(t *testing.T) {
	agentMemSize := func(n int) *models.Agent {
		return &models.Agent{
			LLMConfig: &models.LLMAgentConfig{
				SimulationConfig: &models.LLMSimulationAgentConfig{
					MemorySize: n,
				},
			},
		}
	}
	modelDefSize := func(n int) *models.Model {
		return &models.Model{
			SimulationConfig: models.SimulationConfig{
				LLMSimulationExecutionConfig: models.LLMSimulationExecutionConfig{
					LLMSimulationAgentConfig: models.LLMSimulationAgentConfig{
						MemorySize: n,
					},
				},
			},
		}
	}

	t.Run("agent override wins", func(t *testing.T) {
		arch := agentMemSize(10)
		model := modelDefSize(30)
		if got := resolveMemoryCap(model, arch); got != 10 {
			t.Fatalf("resolveMemoryCap=%d want 10 (agent override)", got)
		}
	})

	t.Run("model default fallback", func(t *testing.T) {
		arch := agentMemSize(0) // agent not set
		model := modelDefSize(15)
		if got := resolveMemoryCap(model, arch); got != 15 {
			t.Fatalf("resolveMemoryCap=%d want 15 (model default)", got)
		}
	})

	t.Run("both zero returns 0", func(t *testing.T) {
		arch := agentMemSize(0)
		model := modelDefSize(0)
		if got := resolveMemoryCap(model, arch); got != 0 {
			t.Fatalf("resolveMemoryCap=%d want 0 (AppendMemory applies floor)", got)
		}
	})

	t.Run("nil arch returns model default", func(t *testing.T) {
		model := modelDefSize(7)
		if got := resolveMemoryCap(model, nil); got != 7 {
			t.Fatalf("resolveMemoryCap=%d want 7 (nil arch → model default)", got)
		}
	})

	t.Run("nil arch and nil model default returns 0", func(t *testing.T) {
		model := &models.Model{} // no LLMSimulationExecutionConfig.MemorySize
		if got := resolveMemoryCap(model, nil); got != 0 {
			t.Fatalf("resolveMemoryCap=%d want 0", got)
		}
	})

	t.Run("nil model returns 0", func(t *testing.T) {
		arch := agentMemSize(5)
		if got := resolveMemoryCap(nil, arch); got != 5 {
			t.Fatalf("resolveMemoryCap=%d want 5 (agent set, nil model)", got)
		}
	})

	t.Run("nil arch LLMConfig falls back to model", func(t *testing.T) {
		arch := &models.Agent{} // LLMConfig is nil
		model := modelDefSize(12)
		if got := resolveMemoryCap(model, arch); got != 12 {
			t.Fatalf("resolveMemoryCap=%d want 12 (arch LLMConfig nil)", got)
		}
	})

	t.Run("nil arch SimulationConfig falls back to model", func(t *testing.T) {
		arch := &models.Agent{LLMConfig: &models.LLMAgentConfig{}} // SimulationConfig is nil
		model := modelDefSize(8)
		if got := resolveMemoryCap(model, arch); got != 8 {
			t.Fatalf("resolveMemoryCap=%d want 8 (arch SimulationConfig nil)", got)
		}
	})
}

func TestSummarizeMemory_CollapsesConsecutiveRepeats(t *testing.T) {
	ra := &RuntimeAgent{A: &datasims.SimulationAgent{ID: 1}}
	now := time.Now()
	// oldest-first: moved x3 (steps 1-3), set energy (4), moved x2 (5-6).
	for step := uint64(1); step <= 3; step++ {
		AppendMemory(ra, step, datasims.AgentMemoryMovement, "moved", nil, 0, now)
	}
	AppendMemory(ra, 4, datasims.AgentMemoryProperty, "set energy", nil, 0, now)
	for step := uint64(5); step <= 6; step++ {
		AppendMemory(ra, step, datasims.AgentMemoryMovement, "moved", nil, 0, now)
	}

	got := SummarizeMemory(ra, 10)
	want := "- step 6 [movement]: moved ×2\n" +
		"- step 4 [property]: set energy\n" +
		"- step 3 [movement]: moved ×3"
	if got != want {
		t.Fatalf("collapsed summary mismatch:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestSummarizeMemory_CollapsedRunConsumesOneBudgetLine(t *testing.T) {
	ra := &RuntimeAgent{A: &datasims.SimulationAgent{ID: 1}}
	now := time.Now()
	for step := uint64(1); step <= 5; step++ {
		AppendMemory(ra, step, datasims.AgentMemoryMovement, "moved", nil, 0, now)
	}
	AppendMemory(ra, 6, datasims.AgentMemoryProperty, "set energy", nil, 0, now)

	// Budget of 2 lines fits BOTH the property line and the whole collapsed run.
	got := SummarizeMemory(ra, 2)
	want := "- step 6 [property]: set energy\n" +
		"- step 5 [movement]: moved ×5"
	if got != want {
		t.Fatalf("budget summary mismatch:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestSummarizeMemoryByType_CollapsesFilteredAdjacency(t *testing.T) {
	ra := &RuntimeAgent{A: &datasims.SimulationAgent{ID: 1}}
	now := time.Now()
	// llm notes interleaved with movement: adjacency is judged on the FILTERED
	// stream, so the two identical llm notes collapse despite the movement between.
	AppendMemory(ra, 1, datasims.AgentMemoryLLM, "chose: flee", nil, 0, now)
	AppendMemory(ra, 2, datasims.AgentMemoryMovement, "moved", nil, 0, now)
	AppendMemory(ra, 3, datasims.AgentMemoryLLM, "chose: flee", nil, 0, now)

	got := SummarizeMemoryByType(ra, []datasims.AgentMemoryType{datasims.AgentMemoryLLM}, 5)
	want := "- step 3 [llm_response]: chose: flee ×2"
	if got != want {
		t.Fatalf("filtered collapse mismatch:\ngot:\n%s\nwant:\n%s", got, want)
	}
}
