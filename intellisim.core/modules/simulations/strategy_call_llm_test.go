package simulations

import (
	"errors"
	"strings"
	"testing"

	"intelli.sim.core/data/common"
	"intelli.sim.core/data/models"
	datasims "intelli.sim.core/data/simulations"
)

// callLLMRuntime builds a minimal state whose single archetype carries the given
// strategy, for exercising applyCallLLM directly.
func callLLMRuntime(strat models.LLMStrategy) (*RuntimeState, *RuntimeAgent) {
	arch := &models.Agent{UID: "wolf", Type: models.AgentTypePlain, MoveType: models.AgentMoveTypeMovable, States: []string{"idle"}}
	if strat.Type != "" {
		arch.LLMConfig = &models.LLMAgentConfig{Strategy: models.LLMAgentStrategy{LLMStrategy: strat}}
	}
	ra := &RuntimeAgent{
		A:         &datasims.SimulationAgent{ID: 1, AgentUID: "wolf", State: "idle", Properties: map[string]any{}},
		Archetype: arch,
	}
	rs := &RuntimeState{
		Sim:    &datasims.Simulation{},
		Model:  &models.Model{Agents: []models.Agent{*arch}},
		Agents: []*RuntimeAgent{ra},
		ByID:   map[uint64]*RuntimeAgent{1: ra},
	}
	return rs, ra
}

// TestApplyCallLLM_GuardsAndFlag verifies the decision-capability guard and the
// pending-flag + memory event on success.
func TestApplyCallLLM_GuardsAndFlag(t *testing.T) {
	// No strategy at all -> warn + no flag.
	rs, ra := callLLMRuntime(models.LLMStrategy{})
	if evts := ApplyAction(rs, ra, models.Action{Type: models.ActionTypeCallLLM}); len(evts) != 0 || ra.llmCallPending {
		t.Fatalf("call_llm without a strategy must warn+skip; evts=%v pending=%v", evts, ra.llmCallPending)
	}
	if len(rs.warnings) == 0 || !strings.Contains(rs.warnings[0], "no decision-capable LLM strategy") {
		t.Fatalf("expected guard warning, got %v", rs.warnings)
	}

	// Hybrid strategy without UseForDecisions -> warn + no flag.
	rs, ra = callLLMRuntime(models.LLMStrategy{Type: models.LLMStrategyHybrid, UseForMovement: true})
	ApplyAction(rs, ra, models.Action{Type: models.ActionTypeCallLLM})
	if ra.llmCallPending {
		t.Fatal("call_llm must not set the flag without UseForDecisions")
	}

	// Decision-capable strategy -> flag + decision memory event.
	rs, ra = callLLMRuntime(models.LLMStrategy{Type: models.LLMStrategyDecision, UseForDecisions: true})
	evts := ApplyAction(rs, ra, models.Action{Type: models.ActionTypeCallLLM})
	if !ra.llmCallPending {
		t.Fatal("call_llm must set llmCallPending for a decision-capable agent")
	}
	if len(evts) != 1 || evts[0].Type != datasims.AgentMemoryDecision || evts[0].Summary != "call_llm requested" {
		t.Fatalf("unexpected memEvents: %+v", evts)
	}
}

// TestActionPurpose_CallLLMHasNoPurpose pins the fix for the inverted
// suppression: call_llm must NOT map to a purpose, so ApplyRules never
// suppresses it for LLM-owned agents and ApplyIntentActions drops it.
func TestActionPurpose_CallLLMHasNoPurpose(t *testing.T) {
	if p, ok := actionPurpose(models.ActionTypeCallLLM); ok {
		t.Fatalf("actionPurpose(call_llm) = %v, want no purpose", p)
	}
	if p, ok := actionPurpose(models.ActionTypeTransition); !ok || p != purposeDecision {
		t.Fatal("transition must keep purposeDecision")
	}
}

// TestCallLLM_OffSchedulePendingOnly_Individual verifies that on an off-schedule
// step the individual decision strategy calls the LLM for ONLY the pending
// members, clears their flags, and leaves the others untouched.
func TestCallLLM_OffSchedulePendingOnly_Individual(t *testing.T) {
	inv := &fakeInvoker{
		simpleQueue: []fakeReply{{
			text:  `{"agents":[{"instance":0,"label":"flee"}]}`,
			usage: common.TokenUsage{TotalTokens: 3, Calls: 1},
		}},
	}
	strat := models.LLMStrategy{
		Type:              models.LLMStrategyDecision,
		UseForDecisions:   true,
		BatchSize:         10,
		DecisionFrequency: 5,
	}
	sc, group := individualTestSetup(inv, strat, 3)
	sc.RS.Step = 3 // 3 % 5 != 0 -> frequency gate closed
	group.Members[1].llmCallPending = true

	res, err := (individualDecisionStrategy{}).Plan(sc, group)
	if err != nil {
		t.Fatalf("Plan error: %v", err)
	}
	if inv.simpleCalls != 1 {
		t.Fatalf("expected exactly 1 off-schedule call, got %d", inv.simpleCalls)
	}
	// Only the pending member (id 2) got a decision; instance 0 maps to it.
	if in := res.Intents[2]; in == nil || len(in.Decisions) == 0 || in.Decisions[0].Label != "flee" {
		t.Fatalf("pending agent 2 missing decision: %+v", res.Intents[2])
	}
	if res.Intents[1] != nil || res.Intents[3] != nil {
		t.Fatalf("non-pending agents must not receive off-schedule decisions: %+v", res.Intents)
	}
	// The prompt enumerated exactly one instance.
	if strings.Count(inv.lastSimple.Message, "- instance ") != 1 {
		t.Fatalf("prompt must enumerate only the pending member:\n%s", inv.lastSimple.Message)
	}
	for _, ra := range group.Members {
		if ra.llmCallPending {
			t.Fatalf("agent %d flag not consumed", ra.A.ID)
		}
	}

	// A fully idle off-schedule step issues no call at all.
	inv.simpleCalls = 0
	if _, err := (individualDecisionStrategy{}).Plan(sc, group); err != nil {
		t.Fatalf("idle Plan error: %v", err)
	}
	if inv.simpleCalls != 0 {
		t.Fatalf("idle off-schedule step must issue no calls, got %d", inv.simpleCalls)
	}
}

// TestCallLLM_FlagConsumedOnFailedCall verifies consume-once semantics: a failed
// off-schedule call clears the flag (no retry storm next step).
func TestCallLLM_FlagConsumedOnFailedCall(t *testing.T) {
	inv := &fakeInvoker{simpleQueue: []fakeReply{{err: errors.New("provider down")}}}
	strat := models.LLMStrategy{
		Type: models.LLMStrategyDecision, UseForDecisions: true,
		BatchSize: 10, DecisionFrequency: 5,
	}
	sc, group := individualTestSetup(inv, strat, 2)
	sc.RS.Step = 3
	group.Members[0].llmCallPending = true

	res, err := (individualDecisionStrategy{}).Plan(sc, group)
	if err != nil {
		t.Fatalf("Plan error: %v", err)
	}
	if group.Members[0].llmCallPending {
		t.Fatal("pending flag must be consumed even when the call fails")
	}
	if in := res.Intents[1]; in != nil && len(in.Decisions) > 0 {
		t.Fatalf("failed call without LastDecision must yield no decision, got %+v", in)
	}
}

// TestCallLLM_OffSchedulePendingOnly_Archetype verifies the archetype-scope
// analogue: the directive is fanned out ONLY to the pending members.
func TestCallLLM_OffSchedulePendingOnly_Archetype(t *testing.T) {
	inv := &fakeInvoker{
		simpleQueue: []fakeReply{{
			text:  `{"agents":[{"archetype_uid":"wolf","label":"flee"}]}`,
			usage: common.TokenUsage{TotalTokens: 4, Calls: 1},
		}},
	}
	strat := models.LLMStrategy{
		Type:              models.LLMStrategyDecision,
		UseForDecisions:   true,
		BatchSize:         1,
		DecisionFrequency: 5,
	}
	sc, groups := archetypeTestState(inv, []archSpec{{uid: "wolf", strat: strat, count: 3}})
	group := groups[0]
	sc.RS.Step = 3
	group.Members[2].llmCallPending = true

	res, err := (archetypeDecisionStrategy{}).Plan(sc, group)
	if err != nil {
		t.Fatalf("Plan error: %v", err)
	}
	if inv.simpleCalls != 1 {
		t.Fatalf("expected 1 off-schedule archetype call, got %d", inv.simpleCalls)
	}
	pendingID := group.Members[2].A.ID
	if in := res.Intents[pendingID]; in == nil || len(in.Decisions) == 0 || in.Decisions[0].Label != "flee" {
		t.Fatalf("pending member %d missing directive: %+v", pendingID, res.Intents[pendingID])
	}
	for _, ra := range group.Members[:2] {
		if in := res.Intents[ra.A.ID]; in != nil && len(in.Decisions) > 0 {
			t.Fatalf("non-pending member %d must not receive the off-schedule directive", ra.A.ID)
		}
		if ra.llmCallPending {
			t.Fatalf("member %d flag not consumed", ra.A.ID)
		}
	}
}

// TestCallLLM_FlagClonedAndCommitted verifies the synchronous working-copy path
// carries the flag from the rules phase back to the live agent.
func TestCallLLM_FlagClonedAndCommitted(t *testing.T) {
	rs, ra := callLLMRuntime(models.LLMStrategy{Type: models.LLMStrategyDecision, UseForDecisions: true})
	_ = rs
	w := cloneRuntimeAgent(ra)
	if w.llmCallPending {
		t.Fatal("clone must start with the source's flag (false)")
	}
	w.llmCallPending = true
	commitRuntimeAgent(ra, w)
	if !ra.llmCallPending {
		t.Fatal("commit must copy llmCallPending back to the live agent")
	}
}
