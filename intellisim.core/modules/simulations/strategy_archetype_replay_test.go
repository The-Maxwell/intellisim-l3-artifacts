package simulations

import (
	"errors"
	"testing"

	"intelli.sim.core/data/models"
)

// TestArchetypeDecision_ReplaysLastDecisionOnFailedCall verifies the
// archetype-scope last-decision fallback: when the decision call fails, each
// member with a prior LastDecision replays ITS OWN label (per-member — exactly
// what distribution mode recorded), members without one are left to the rules.
func TestArchetypeDecision_ReplaysLastDecisionOnFailedCall(t *testing.T) {
	inv := &fakeInvoker{simpleQueue: []fakeReply{{err: errors.New("provider down")}}}
	strat := models.LLMStrategy{
		Type:            models.LLMStrategyDecision,
		UseForDecisions: true,
		BatchSize:       1,
	}
	sc, groups := archetypeTestState(inv, []archSpec{{uid: "wolf", strat: strat, count: 3}})
	group := groups[0]
	group.Members[0].LastDecision = &DecisionIntent{Label: "search_food"}
	group.Members[1].LastDecision = &DecisionIntent{Label: "flee"}
	// Members[2] has no prior decision.

	res, err := (archetypeDecisionStrategy{}).Plan(sc, group)
	if err != nil {
		t.Fatalf("Plan error: %v", err)
	}
	if in := res.Intents[group.Members[0].A.ID]; in == nil || len(in.Decisions) == 0 || in.Decisions[0].Label != "search_food" {
		t.Fatalf("member 0 must replay its own LastDecision, got %+v", in)
	}
	if in := res.Intents[group.Members[1].A.ID]; in == nil || len(in.Decisions) == 0 || in.Decisions[0].Label != "flee" {
		t.Fatalf("member 1 must replay its own LastDecision, got %+v", in)
	}
	if in := res.Intents[group.Members[2].A.ID]; in != nil && len(in.Decisions) > 0 {
		t.Fatalf("member 2 has no LastDecision and must be left to the rules, got %+v", in)
	}
}

// TestArchetypeFull_NoReplayOnFailedCall pins the intentional asymmetry: the
// full kind does NOT replay decisions on a failed call (parity with
// individual_full).
func TestArchetypeFull_NoReplayOnFailedCall(t *testing.T) {
	inv := &fakeInvoker{simpleQueue: []fakeReply{{err: errors.New("provider down")}}}
	strat := models.LLMStrategy{
		Type:            models.LLMStrategyFull,
		UseForDecisions: true,
		BatchSize:       1,
	}
	sc, groups := archetypeTestState(inv, []archSpec{{uid: "wolf", strat: strat, count: 2}})
	group := groups[0]
	group.Members[0].LastDecision = &DecisionIntent{Label: "search_food"}

	res, err := (archetypeFullStrategy{}).Plan(sc, group)
	if err != nil {
		t.Fatalf("Plan error: %v", err)
	}
	for _, ra := range group.Members {
		if in := res.Intents[ra.A.ID]; in != nil && len(in.Decisions) > 0 {
			t.Fatalf("full kind must not replay decisions, member %d got %+v", ra.A.ID, in)
		}
	}
}
