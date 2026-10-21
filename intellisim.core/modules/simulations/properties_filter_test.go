package simulations

import (
	"encoding/json"
	"testing"

	"intelli.sim.core/data/common"
	"intelli.sim.core/data/models"
	datasims "intelli.sim.core/data/simulations"
)

// The runtime property prompts promise "unknown or immutable UIDs are dropped".
// These tests pin that promise at the LLM seam: an LLM-returned values map must
// be filtered to the archetype's DECLARED, MUTABLE properties BEFORE it enters
// the intent (rule-engine semantics stay untouched).

// filterWolf builds a wolf archetype with one mutable number ("energy") and one
// immutable string ("tag"), plus a runtime agent instance of it.
func filterWolf() (*models.Agent, *RuntimeAgent) {
	arch := &models.Agent{
		UID: "wolf",
		Properties: []models.AgentProperty{
			{UID: "energy", Type: models.PropertyTypeNumber},
			{UID: "tag", Type: models.PropertyTypeString, Immutable: true},
		},
	}
	ra := &RuntimeAgent{
		A: &datasims.SimulationAgent{
			ID: 1, AgentUID: "wolf", State: "idle",
			Properties: map[string]any{"energy": 50.0, "tag": "alpha"},
		},
		Archetype: arch,
	}
	return arch, ra
}

func mustPropsResp(t *testing.T, raw string) propsResp {
	t.Helper()
	var out propsResp
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		t.Fatalf("bad fixture JSON: %v", err)
	}
	return out
}

func TestDemuxProperties_DropsUndeclaredAndImmutable(t *testing.T) {
	_, ra := filterWolf()
	out := mustPropsResp(t, `{"agents":[{"instance":0,"values":{"energy":42,"tag":"hacked","ghost":1}}]}`)

	res := PlanResult{Intents: map[uint64]*AgentIntent{}}
	demuxProperties([]*RuntimeAgent{ra}, out, &res)

	in := res.Intents[1]
	if in == nil || in.Properties == nil {
		t.Fatalf("expected an intent with properties, got %+v", in)
	}
	if v, ok := in.Properties["energy"]; !ok || v != float64(42) {
		t.Fatalf("declared mutable property must pass through, got %v (ok=%v)", v, ok)
	}
	if _, ok := in.Properties["ghost"]; ok {
		t.Fatalf("undeclared UID reached the intent: %+v", in.Properties)
	}
	if _, ok := in.Properties["tag"]; ok {
		t.Fatalf("immutable UID reached the intent: %+v", in.Properties)
	}
}

func TestDemuxProperties_AllValuesDropped_NoIntent(t *testing.T) {
	_, ra := filterWolf()
	out := mustPropsResp(t, `{"agents":[{"instance":0,"values":{"ghost":1,"tag":"hacked"}}]}`)

	res := PlanResult{Intents: map[uint64]*AgentIntent{}}
	demuxProperties([]*RuntimeAgent{ra}, out, &res)

	if in := res.Intents[1]; in != nil && len(in.Properties) > 0 {
		t.Fatalf("fully-dropped values map must not produce properties, got %+v", in.Properties)
	}
}

func TestArchetypeProperties_DropsUndeclaredUIDs(t *testing.T) {
	// The archetype demux (callArchetypePurpose → properties branch) must filter
	// the directive BEFORE the per-member fan-out, so an undeclared UID never
	// reaches any member's intent.
	inv := &fakeInvoker{
		simpleQueue: []fakeReply{{
			text: `{"agents":[{"archetype_uid":"wolf","values":{"energy":42,"ghost":7}}]}`,
			usage: common.TokenUsage{
				LLMTarget:   common.LLMTarget{ProviderUID: "openai", ModelUID: "gpt-4o-mini"},
				TotalTokens: 3, Calls: 1,
			},
		}},
	}
	strat := models.LLMStrategy{
		Type:             models.LLMStrategyFull,
		UseForProperties: true,
		BatchSize:        1,
	}
	sc, groups := archetypeTestState(inv, []archSpec{{uid: "wolf", strat: strat, count: 3}})
	group := groups[0]

	res, err := (archetypeFullStrategy{}).Plan(sc, group)
	if err != nil {
		t.Fatalf("Plan error: %v", err)
	}
	for _, ra := range group.Members {
		in := res.Intents[ra.A.ID]
		if in == nil || len(in.Properties) == 0 {
			t.Fatalf("agent %d missing properties intent: %+v", ra.A.ID, in)
		}
		if _, ok := in.Properties["energy"]; !ok {
			t.Fatalf("agent %d lost the declared property: %+v", ra.A.ID, in.Properties)
		}
		if _, ok := in.Properties["ghost"]; ok {
			t.Fatalf("agent %d received an undeclared UID: %+v", ra.A.ID, in.Properties)
		}
	}
}

func TestArchetypeProperties_AllValuesDropped_ArchetypeOmitted(t *testing.T) {
	// A values map that filters to nothing must omit the archetype entirely
	// (rules cover it), never fan an empty directive out.
	inv := &fakeInvoker{
		simpleQueue: []fakeReply{{
			text: `{"agents":[{"archetype_uid":"wolf","values":{"ghost":7}}]}`,
			usage: common.TokenUsage{
				LLMTarget:   common.LLMTarget{ProviderUID: "openai", ModelUID: "gpt-4o-mini"},
				TotalTokens: 3, Calls: 1,
			},
		}},
	}
	strat := models.LLMStrategy{
		Type:             models.LLMStrategyFull,
		UseForProperties: true,
		BatchSize:        1,
	}
	sc, groups := archetypeTestState(inv, []archSpec{{uid: "wolf", strat: strat, count: 2}})
	group := groups[0]

	res, err := (archetypeFullStrategy{}).Plan(sc, group)
	if err != nil {
		t.Fatalf("Plan error: %v", err)
	}
	for id, in := range res.Intents {
		if in != nil && len(in.Properties) > 0 {
			t.Fatalf("agent %d received properties from a fully-dropped directive: %+v", id, in.Properties)
		}
	}
}
