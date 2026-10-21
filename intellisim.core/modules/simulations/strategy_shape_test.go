package simulations

import (
	"context"
	"sort"
	"testing"

	"intelli.sim.core/data/common"
	"intelli.sim.core/data/models"
	datasims "intelli.sim.core/data/simulations"
)

// --- test helpers ---------------------------------------------------------------

// makeShapeRS builds a RuntimeState containing the given model shapes
// (Environment.Shapes) and runtime shapes (Env.Shapes). Each shapeSpec declares
// the UID, whether the model shape has an LLMShapeConfig, and the strategy type.
type shapeSpec struct {
	uid       string
	strategy  models.LLMStrategyType // "" / "none" → skip; anything else → active
	props     map[string]interface{} // initial runtime properties
	immutable []string               // property UIDs marked Immutable on the model shape
}

func makeShapeRS(specs []shapeSpec) *RuntimeState {
	modelShapes := make([]models.Shape, 0, len(specs))
	runtimeShapes := make([]datasims.SimulationShape, 0, len(specs))

	for i, sp := range specs {
		ms := models.Shape{
			UID:  sp.uid,
			Name: sp.uid,
			Type: models.ShapeTypeRectangle,
		}
		if sp.strategy != "" && sp.strategy != models.LLMStrategyNone {
			ms.LLMConfig = &models.LLMShapeConfig{
				LLMConfig: models.LLMConfig{Target: common.LLMTarget{ProviderUID: "openai", ModelUID: "gpt-4o-mini"}},
				Strategy: models.LLMShapeStrategy{
					LLMStrategy: models.LLMStrategy{Type: sp.strategy},
				},
			}
		}
		// Add immutable properties to the model shape.
		immutableSet := map[string]bool{}
		for _, puid := range sp.immutable {
			ms.Properties = append(ms.Properties, models.AgentProperty{
				UID:       puid,
				Immutable: true,
			})
			immutableSet[puid] = true
		}
		// Declare every runtime property as a mutable model property (unless
		// already declared immutable above): applyShapeUpdates drops undeclared
		// UIDs, so the fixture must declare what the tests expect to be settable.
		propKeys := make([]string, 0, len(sp.props))
		for k := range sp.props {
			propKeys = append(propKeys, k)
		}
		sort.Strings(propKeys)
		for _, k := range propKeys {
			if immutableSet[k] {
				continue
			}
			ms.Properties = append(ms.Properties, models.AgentProperty{UID: k})
		}
		modelShapes = append(modelShapes, ms)

		props := make(map[string]interface{})
		for k, v := range sp.props {
			props[k] = v
		}
		rs := datasims.SimulationShape{
			ID:         uint64(i + 1),
			ShapeUID:   sp.uid,
			Name:       sp.uid,
			Type:       models.ShapeTypeRectangle,
			Properties: props,
		}
		runtimeShapes = append(runtimeShapes, rs)
	}

	model := &models.Model{
		LLMConfig: models.LLMConfig{
			Target: common.LLMTarget{ProviderUID: "openai", ModelUID: "gpt-4o-mini"},
		},
		Environment: &models.Environment{
			UID:    models.EnvironmentUID,
			Shapes: modelShapes,
		},
	}
	sim := &datasims.Simulation{
		ModelSnapshot: model,
	}
	env := &datasims.SimulationEnvironment{
		Shapes: runtimeShapes,
	}
	return &RuntimeState{
		Sim:   sim,
		Model: model,
		Env:   env,
	}
}

// --- groupShapesByUID ------------------------------------------------------------

func TestGroupShapesByUID_ActiveShapeGrouped(t *testing.T) {
	rs := makeShapeRS([]shapeSpec{
		{uid: "lake", strategy: models.LLMStrategyFull, props: map[string]interface{}{"level": 10.0}},
	})

	groups := groupShapesByUID(rs)
	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}
	if groups[0].ShapeUID != "lake" {
		t.Fatalf("expected ShapeUID=lake, got %q", groups[0].ShapeUID)
	}
	if len(groups[0].Members) != 1 {
		t.Fatalf("expected 1 member, got %d", len(groups[0].Members))
	}
}

func TestGroupShapesByUID_NoneStrategySkipped(t *testing.T) {
	rs := makeShapeRS([]shapeSpec{
		{uid: "wall", strategy: models.LLMStrategyNone, props: nil},
	})

	groups := groupShapesByUID(rs)
	if len(groups) != 0 {
		t.Fatalf("expected 0 groups (none strategy), got %d", len(groups))
	}
}

func TestGroupShapesByUID_NoLLMConfigSkipped(t *testing.T) {
	// strategy "" → no LLMConfig on the model shape → must be skipped.
	rs := makeShapeRS([]shapeSpec{
		{uid: "rock", strategy: "", props: nil},
	})

	groups := groupShapesByUID(rs)
	if len(groups) != 0 {
		t.Fatalf("expected 0 groups (no LLMConfig), got %d", len(groups))
	}
}

func TestGroupShapesByUID_MultipleInstancesSameUID(t *testing.T) {
	// Two runtime shapes with the same ShapeUID → they should be in one group.
	model := &models.Model{
		LLMConfig: models.LLMConfig{
			Target: common.LLMTarget{ProviderUID: "openai", ModelUID: "gpt-4o-mini"},
		},
		Environment: &models.Environment{
			UID: models.EnvironmentUID,
			Shapes: []models.Shape{
				{
					UID:  "lake",
					Type: models.ShapeTypeCircle,
					LLMConfig: &models.LLMShapeConfig{
						LLMConfig: models.LLMConfig{Target: common.LLMTarget{ProviderUID: "openai", ModelUID: "gpt-4o-mini"}},
						Strategy: models.LLMShapeStrategy{
							LLMStrategy: models.LLMStrategy{Type: models.LLMStrategyFull},
						},
					},
				},
			},
		},
	}
	env := &datasims.SimulationEnvironment{
		Shapes: []datasims.SimulationShape{
			{ID: 1, ShapeUID: "lake", Properties: map[string]interface{}{"level": 1.0}},
			{ID: 2, ShapeUID: "lake", Properties: map[string]interface{}{"level": 2.0}},
		},
	}
	rs := &RuntimeState{
		Sim:   &datasims.Simulation{ModelSnapshot: model},
		Model: model,
		Env:   env,
	}

	groups := groupShapesByUID(rs)
	if len(groups) != 1 {
		t.Fatalf("expected 1 group for two instances of same UID, got %d", len(groups))
	}
	if len(groups[0].Members) != 2 {
		t.Fatalf("expected 2 members in group, got %d", len(groups[0].Members))
	}
}

func TestGroupShapesByUID_DeterministicOrder(t *testing.T) {
	// Groups must be returned in first-seen order (slice iteration order).
	rs := makeShapeRS([]shapeSpec{
		{uid: "alpha", strategy: models.LLMStrategyFull},
		{uid: "beta", strategy: models.LLMStrategyDecision},
		{uid: "gamma", strategy: models.LLMStrategyFull},
	})

	for attempt := 0; attempt < 5; attempt++ {
		groups := groupShapesByUID(rs)
		if len(groups) != 3 {
			t.Fatalf("attempt %d: expected 3 groups, got %d", attempt, len(groups))
		}
		if groups[0].ShapeUID != "alpha" || groups[1].ShapeUID != "beta" || groups[2].ShapeUID != "gamma" {
			t.Fatalf("attempt %d: group order not deterministic: got %v %v %v",
				attempt, groups[0].ShapeUID, groups[1].ShapeUID, groups[2].ShapeUID)
		}
	}
}

func TestGroupShapesByUID_TargetResolved_ShapeOverridesModel(t *testing.T) {
	// Shape LLMConfig.Target (ProviderUID="anthropic") overrides the model default.
	model := &models.Model{
		LLMConfig: models.LLMConfig{
			Target: common.LLMTarget{ProviderUID: "openai", ModelUID: "gpt-4o"},
		},
		Environment: &models.Environment{
			UID: models.EnvironmentUID,
			Shapes: []models.Shape{
				{
					UID:  "river",
					Type: models.ShapeTypeRectangle,
					LLMConfig: &models.LLMShapeConfig{
						LLMConfig: models.LLMConfig{Target: common.LLMTarget{ProviderUID: "anthropic", ModelUID: "claude-3"}},
						Strategy: models.LLMShapeStrategy{
							LLMStrategy: models.LLMStrategy{Type: models.LLMStrategyFull},
						},
					},
				},
			},
		},
	}
	env := &datasims.SimulationEnvironment{
		Shapes: []datasims.SimulationShape{
			{ID: 1, ShapeUID: "river", Properties: map[string]interface{}{}},
		},
	}
	rs := &RuntimeState{
		Sim:   &datasims.Simulation{ModelSnapshot: model},
		Model: model,
		Env:   env,
	}

	groups := groupShapesByUID(rs)
	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}
	if groups[0].Target.ProviderUID != "anthropic" {
		t.Fatalf("expected shape target ProviderUID=anthropic, got %q", groups[0].Target.ProviderUID)
	}
}

func TestGroupShapesByUID_TargetFallsBackToModel(t *testing.T) {
	// Shape LLMConfig.Target empty → model-level EXECUTION target used (the shape LLM
	// phase is part of Level-2 execution, so it reads the execution config).
	model := &models.Model{
		SimulationConfig: models.SimulationConfig{
			LLMSimulationExecutionConfig: models.LLMSimulationExecutionConfig{
				LLMConfig: models.LLMConfig{
					Target: common.LLMTarget{ProviderUID: "openai", ModelUID: "gpt-4o-mini"},
				},
			},
		},
		Environment: &models.Environment{
			UID: models.EnvironmentUID,
			Shapes: []models.Shape{
				{
					UID:  "forest",
					Type: models.ShapeTypePolygon,
					LLMConfig: &models.LLMShapeConfig{
						LLMConfig: models.LLMConfig{Target: common.LLMTarget{}}, // empty → inherit model default
						Strategy: models.LLMShapeStrategy{
							LLMStrategy: models.LLMStrategy{Type: models.LLMStrategyDecision},
						},
					},
				},
			},
		},
	}
	env := &datasims.SimulationEnvironment{
		Shapes: []datasims.SimulationShape{
			{ID: 1, ShapeUID: "forest", Properties: map[string]interface{}{}},
		},
	}
	rs := &RuntimeState{
		Sim:   &datasims.Simulation{ModelSnapshot: model},
		Model: model,
		Env:   env,
	}

	groups := groupShapesByUID(rs)
	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}
	if groups[0].Target.ProviderUID != "openai" {
		t.Fatalf("expected model fallback target ProviderUID=openai, got %q", groups[0].Target.ProviderUID)
	}
}

// --- planShapeUpdates -----------------------------------------------------------

func TestPlanShapeUpdates_HappyPath(t *testing.T) {
	inv := &fakeInvoker{
		simpleQueue: []fakeReply{{
			text:  `{"properties":{"level":5}}`,
			usage: common.TokenUsage{LLMTarget: common.LLMTarget{ProviderUID: "openai", ModelUID: "gpt-4o-mini"}, TotalTokens: 10, Calls: 1},
		}},
	}
	r := newTestRuntime(inv)

	rs := makeShapeRS([]shapeSpec{
		{uid: "lake", strategy: models.LLMStrategyFull, props: map[string]interface{}{"level": 1.0}},
	})
	groups := groupShapesByUID(rs)
	if len(groups) != 1 {
		t.Fatalf("expected 1 group")
	}

	updates := planShapeUpdates(context.Background(), rs, r, groups[0])
	if updates == nil {
		t.Fatal("expected non-nil updates from happy-path LLM response")
	}
	if v, ok := updates["level"]; !ok || v != float64(5) {
		t.Fatalf("expected updates[level]=5, got %v (ok=%v)", v, ok)
	}
}

func TestPlanShapeUpdates_LLMError_ReturnsNil(t *testing.T) {
	// Provider error → fail-soft → nil.
	inv := &fakeInvoker{
		simpleQueue: []fakeReply{{text: "NOT JSON"}},
	}
	r := newTestRuntime(inv)

	rs := makeShapeRS([]shapeSpec{
		{uid: "lake", strategy: models.LLMStrategyFull, props: map[string]interface{}{"level": 1.0}},
	})
	groups := groupShapesByUID(rs)

	updates := planShapeUpdates(context.Background(), rs, r, groups[0])
	if updates != nil {
		t.Fatalf("expected nil on LLM/parse error, got %v", updates)
	}
}

func TestPlanShapeUpdates_TokensAttributedToShapePurpose(t *testing.T) {
	inv := &fakeInvoker{
		simpleQueue: []fakeReply{{
			text:  `{"properties":{"level":7}}`,
			usage: common.TokenUsage{LLMTarget: common.LLMTarget{ProviderUID: "openai", ModelUID: "gpt-4o-mini"}, TotalTokens: 20, Calls: 1},
		}},
	}
	r := newTestRuntime(inv)

	rs := makeShapeRS([]shapeSpec{
		{uid: "lake", strategy: models.LLMStrategyFull, props: map[string]interface{}{"level": 1.0}},
	})
	groups := groupShapesByUID(rs)
	planShapeUpdates(context.Background(), rs, r, groups[0])

	// Token attribution must be to LLMUsagePurposeSimulationShape.
	if got := byPurposeTotal(r.usage, datasims.LLMUsagePurposeSimulationShape); got != 20 {
		t.Fatalf("ByPurpose[simulation_shape]=%d want 20", got)
	}
	// Total must also reflect the spend.
	if r.usage.Total.TotalTokens != 20 {
		t.Fatalf("Total.TotalTokens=%d want 20", r.usage.Total.TotalTokens)
	}
}

func TestPlanShapeUpdates_AgentUIDIsShapeUID(t *testing.T) {
	// The attribution key (agentUID) for ByAgent must be the ShapeUID so that token
	// spend is identifiable per shape definition — consistent with the agent path.
	inv := &fakeInvoker{
		simpleQueue: []fakeReply{{
			text:  `{"properties":{"level":3}}`,
			usage: common.TokenUsage{LLMTarget: common.LLMTarget{ProviderUID: "openai", ModelUID: "gpt-4o-mini"}, TotalTokens: 8, Calls: 1},
		}},
	}
	r := newTestRuntime(inv)

	rs := makeShapeRS([]shapeSpec{
		{uid: "river", strategy: models.LLMStrategyDecision, props: map[string]interface{}{"level": 2.0}},
	})
	groups := groupShapesByUID(rs)
	planShapeUpdates(context.Background(), rs, r, groups[0])

	// ByAgent must have exactly one entry keyed on "river".
	found := false
	for _, a := range r.usage.ByAgent {
		if a.AgentUID == "river" {
			found = true
			if a.TotalTokens != 8 {
				t.Fatalf("ByAgent[river].TotalTokens=%d want 8", a.TotalTokens)
			}
		}
	}
	if !found {
		t.Fatal("expected ByAgent entry keyed on shape UID 'river'")
	}
}

// --- applyShapeUpdates ----------------------------------------------------------

func TestApplyShapeUpdates_WritesProperties(t *testing.T) {
	rs := makeShapeRS([]shapeSpec{
		{uid: "lake", strategy: models.LLMStrategyFull, props: map[string]interface{}{"level": 1.0}},
	})
	groups := groupShapesByUID(rs)

	updates := map[string]interface{}{"level": float64(5)}
	applyShapeUpdates(rs, groups[0], updates)

	if v := rs.Env.Shapes[0].Properties["level"]; v != float64(5) {
		t.Fatalf("expected Properties[level]=5, got %v", v)
	}
}

func TestApplyShapeUpdates_NilUpdates_Noop(t *testing.T) {
	rs := makeShapeRS([]shapeSpec{
		{uid: "lake", strategy: models.LLMStrategyFull, props: map[string]interface{}{"level": 42.0}},
	})
	groups := groupShapesByUID(rs)

	applyShapeUpdates(rs, groups[0], nil)

	if v := rs.Env.Shapes[0].Properties["level"]; v != float64(42) {
		t.Fatalf("nil updates must leave properties unchanged, got %v", v)
	}
}

func TestApplyShapeUpdates_ImmutablePropertySkipped(t *testing.T) {
	// Model shape declares "level" Immutable → LLM update must be silently skipped.
	rs := makeShapeRS([]shapeSpec{
		{
			uid:       "lake",
			strategy:  models.LLMStrategyFull,
			props:     map[string]interface{}{"level": 10.0},
			immutable: []string{"level"},
		},
	})
	groups := groupShapesByUID(rs)

	updates := map[string]interface{}{"level": float64(99)}
	applyShapeUpdates(rs, groups[0], updates)

	if v := rs.Env.Shapes[0].Properties["level"]; v != float64(10) {
		t.Fatalf("immutable property must not be overwritten, got %v", v)
	}
}

func TestApplyShapeUpdates_NonImmutablePropertyWritten(t *testing.T) {
	// "label" is NOT immutable; "locked" IS immutable. LLM may update "label" only.
	model := &models.Model{
		LLMConfig: models.LLMConfig{
			Target: common.LLMTarget{ProviderUID: "openai", ModelUID: "gpt-4o-mini"},
		},
		Environment: &models.Environment{
			UID: models.EnvironmentUID,
			Shapes: []models.Shape{
				{
					UID:  "zone",
					Type: models.ShapeTypeRectangle,
					Properties: []models.AgentProperty{
						{UID: "label", Immutable: false},
						{UID: "locked", Immutable: true},
					},
					LLMConfig: &models.LLMShapeConfig{
						LLMConfig: models.LLMConfig{Target: common.LLMTarget{ProviderUID: "openai", ModelUID: "gpt-4o-mini"}},
						Strategy: models.LLMShapeStrategy{
							LLMStrategy: models.LLMStrategy{Type: models.LLMStrategyFull},
						},
					},
				},
			},
		},
	}
	env := &datasims.SimulationEnvironment{
		Shapes: []datasims.SimulationShape{
			{ID: 1, ShapeUID: "zone", Properties: map[string]interface{}{"label": "old", "locked": "fixed"}},
		},
	}
	rs := &RuntimeState{
		Sim:   &datasims.Simulation{ModelSnapshot: model},
		Model: model,
		Env:   env,
	}

	groups := groupShapesByUID(rs)
	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}

	updates := map[string]interface{}{"label": "new", "locked": "hacked"}
	applyShapeUpdates(rs, groups[0], updates)

	if v := rs.Env.Shapes[0].Properties["label"]; v != "new" {
		t.Fatalf("non-immutable property 'label' should be updated, got %v", v)
	}
	if v := rs.Env.Shapes[0].Properties["locked"]; v != "fixed" {
		t.Fatalf("immutable property 'locked' must not be overwritten, got %v", v)
	}
}

func TestApplyShapeUpdates_UndeclaredPropertySkipped(t *testing.T) {
	// The shape prompt promises "unknown or immutable UIDs are dropped": a UID not
	// declared on the model shape must never be written to the runtime shape.
	model := &models.Model{
		Environment: &models.Environment{
			UID: models.EnvironmentUID,
			Shapes: []models.Shape{
				{
					UID:  "lake",
					Type: models.ShapeTypeCircle,
					Properties: []models.AgentProperty{
						{UID: "level"}, // the only declared (mutable) property
					},
					LLMConfig: &models.LLMShapeConfig{
						LLMConfig: models.LLMConfig{Target: common.LLMTarget{ProviderUID: "openai", ModelUID: "gpt-4o-mini"}},
						Strategy: models.LLMShapeStrategy{
							LLMStrategy: models.LLMStrategy{Type: models.LLMStrategyFull},
						},
					},
				},
			},
		},
	}
	env := &datasims.SimulationEnvironment{
		Shapes: []datasims.SimulationShape{
			{ID: 1, ShapeUID: "lake", Properties: map[string]interface{}{"level": 1.0}},
		},
	}
	rs := &RuntimeState{
		Sim:   &datasims.Simulation{ModelSnapshot: model},
		Model: model,
		Env:   env,
	}

	groups := groupShapesByUID(rs)
	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}

	updates := map[string]interface{}{"level": float64(5), "ghost": float64(99)}
	applyShapeUpdates(rs, groups[0], updates)

	if v := rs.Env.Shapes[0].Properties["level"]; v != float64(5) {
		t.Fatalf("declared property 'level' should be updated, got %v", v)
	}
	if _, ok := rs.Env.Shapes[0].Properties["ghost"]; ok {
		t.Fatalf("undeclared property 'ghost' must not be written: %+v", rs.Env.Shapes[0].Properties)
	}
}

func TestApplyShapeUpdates_MultipleMembers(t *testing.T) {
	// Two instances of the same ShapeUID → both must receive the update.
	model := &models.Model{
		LLMConfig: models.LLMConfig{
			Target: common.LLMTarget{ProviderUID: "openai", ModelUID: "gpt-4o-mini"},
		},
		Environment: &models.Environment{
			UID: models.EnvironmentUID,
			Shapes: []models.Shape{
				{
					UID:  "lake",
					Type: models.ShapeTypeCircle,
					Properties: []models.AgentProperty{
						{UID: "level"}, // declared so applyShapeUpdates accepts the write
					},
					LLMConfig: &models.LLMShapeConfig{
						LLMConfig: models.LLMConfig{Target: common.LLMTarget{ProviderUID: "openai", ModelUID: "gpt-4o-mini"}},
						Strategy: models.LLMShapeStrategy{
							LLMStrategy: models.LLMStrategy{Type: models.LLMStrategyFull},
						},
					},
				},
			},
		},
	}
	env := &datasims.SimulationEnvironment{
		Shapes: []datasims.SimulationShape{
			{ID: 1, ShapeUID: "lake", Properties: map[string]interface{}{"level": 1.0}},
			{ID: 2, ShapeUID: "lake", Properties: map[string]interface{}{"level": 2.0}},
		},
	}
	rs := &RuntimeState{
		Sim:   &datasims.Simulation{ModelSnapshot: model},
		Model: model,
		Env:   env,
	}

	groups := groupShapesByUID(rs)
	updates := map[string]interface{}{"level": float64(9)}
	applyShapeUpdates(rs, groups[0], updates)

	for i, sh := range rs.Env.Shapes {
		if v := sh.Properties["level"]; v != float64(9) {
			t.Fatalf("shape[%d] Properties[level]=%v want 9", i, v)
		}
	}
}

// --- runShapePhase ---------------------------------------------------------------

func TestRunShapePhase_NilRuntime_Noop(t *testing.T) {
	// A nil llmRT must silently return without touching shapes.
	rs := makeShapeRS([]shapeSpec{
		{uid: "lake", strategy: models.LLMStrategyFull, props: map[string]interface{}{"level": 7.0}},
	})

	runShapePhase(context.Background(), rs, nil) // must not panic

	if v := rs.Env.Shapes[0].Properties["level"]; v != float64(7) {
		t.Fatalf("nil llmRT must be a no-op, but level changed to %v", v)
	}
}

func TestRunShapePhase_ActiveShape_PropertiesUpdated(t *testing.T) {
	// Full end-to-end: LLM returns {"properties":{"level":5}}, shape gets level=5.
	inv := &fakeInvoker{
		simpleQueue: []fakeReply{{
			text:  `{"properties":{"level":5}}`,
			usage: common.TokenUsage{LLMTarget: common.LLMTarget{ProviderUID: "openai", ModelUID: "gpt-4o-mini"}, TotalTokens: 10, Calls: 1},
		}},
	}
	r := newTestRuntime(inv)

	rs := makeShapeRS([]shapeSpec{
		{uid: "lake", strategy: models.LLMStrategyFull, props: map[string]interface{}{"level": 1.0}},
	})

	runShapePhase(context.Background(), rs, r)

	if v := rs.Env.Shapes[0].Properties["level"]; v != float64(5) {
		t.Fatalf("expected level=5 after runShapePhase, got %v", v)
	}
}

func TestRunShapePhase_NoneStrategy_Untouched(t *testing.T) {
	// A shape with a none-strategy must be skipped entirely (no LLM call, no update).
	inv := &fakeInvoker{
		simpleQueue: []fakeReply{{text: `{"properties":{"level":99}}`}},
	}
	r := newTestRuntime(inv)

	rs := makeShapeRS([]shapeSpec{
		{uid: "wall", strategy: models.LLMStrategyNone, props: map[string]interface{}{"level": 3.0}},
	})

	runShapePhase(context.Background(), rs, r)

	if inv.simpleCalls != 0 {
		t.Fatalf("expected 0 LLM calls for none-strategy shape, got %d", inv.simpleCalls)
	}
	if v := rs.Env.Shapes[0].Properties["level"]; v != float64(3) {
		t.Fatalf("none-strategy shape level must be untouched, got %v", v)
	}
}

func TestRunShapePhase_DeterministicGroupOrder(t *testing.T) {
	// Two active shapes; LLM is called once per group in first-seen order.
	// We verify the call order is stable across repeated invocations by checking the
	// final property values (each call returns a different level keyed on the shape).
	inv := &fakeInvoker{
		simpleQueue: []fakeReply{
			{text: `{"properties":{"level":10}}`, usage: common.TokenUsage{TotalTokens: 1, Calls: 1}},
			{text: `{"properties":{"level":20}}`, usage: common.TokenUsage{TotalTokens: 1, Calls: 1}},
		},
	}
	r := newTestRuntime(inv)

	rs := makeShapeRS([]shapeSpec{
		{uid: "alpha", strategy: models.LLMStrategyFull, props: map[string]interface{}{"level": 0.0}},
		{uid: "beta", strategy: models.LLMStrategyFull, props: map[string]interface{}{"level": 0.0}},
	})

	runShapePhase(context.Background(), rs, r)

	// alpha (first group) gets the first reply (10); beta (second) gets the second (20).
	if v := rs.Env.Shapes[0].Properties["level"]; v != float64(10) {
		t.Fatalf("alpha level=%v want 10", v)
	}
	if v := rs.Env.Shapes[1].Properties["level"]; v != float64(20) {
		t.Fatalf("beta level=%v want 20", v)
	}
}

func TestRunShapePhase_LLMError_FailSoft_OtherShapesUnaffected(t *testing.T) {
	// First group call fails (malformed JSON); second group succeeds. The run must
	// not abort; second shape updates normally.
	inv := &fakeInvoker{
		simpleQueue: []fakeReply{
			{text: "NOT JSON", usage: common.TokenUsage{TotalTokens: 1, Calls: 1}},
			{text: `{"properties":{"level":7}}`, usage: common.TokenUsage{TotalTokens: 2, Calls: 1}},
		},
	}
	r := newTestRuntime(inv)

	rs := makeShapeRS([]shapeSpec{
		{uid: "alpha", strategy: models.LLMStrategyFull, props: map[string]interface{}{"level": 1.0}},
		{uid: "beta", strategy: models.LLMStrategyFull, props: map[string]interface{}{"level": 2.0}},
	})

	runShapePhase(context.Background(), rs, r) // must not panic or abort

	// alpha: LLM failed → no-update → still 1.
	if v := rs.Env.Shapes[0].Properties["level"]; v != float64(1) {
		t.Fatalf("alpha level should be unchanged (fail-soft), got %v", v)
	}
	// beta: second call succeeded → 7.
	if v := rs.Env.Shapes[1].Properties["level"]; v != float64(7) {
		t.Fatalf("beta level should be 7, got %v", v)
	}
}

func TestRunShapePhase_TokensAttributedToSimulationShape(t *testing.T) {
	// Token accounting: purpose must be simulation_shape and ByAgent keyed on ShapeUID.
	inv := &fakeInvoker{
		simpleQueue: []fakeReply{{
			text:  `{"properties":{"level":5}}`,
			usage: common.TokenUsage{LLMTarget: common.LLMTarget{ProviderUID: "openai", ModelUID: "gpt-4o-mini"}, TotalTokens: 15, Calls: 1},
		}},
	}
	r := newTestRuntime(inv)

	rs := makeShapeRS([]shapeSpec{
		{uid: "lake", strategy: models.LLMStrategyFull, props: map[string]interface{}{"level": 0.0}},
	})

	runShapePhase(context.Background(), rs, r)

	if got := byPurposeTotal(r.usage, datasims.LLMUsagePurposeSimulationShape); got != 15 {
		t.Fatalf("ByPurpose[simulation_shape]=%d want 15", got)
	}
}

func TestRunShapePhase_NilEnv_Noop(t *testing.T) {
	// rs.Env == nil → no shapes to iterate → must not panic.
	r := newTestRuntime(&fakeInvoker{})
	model := &models.Model{
		Environment: &models.Environment{
			UID:    models.EnvironmentUID,
			Shapes: []models.Shape{},
		},
	}
	rs := &RuntimeState{
		Sim:   &datasims.Simulation{ModelSnapshot: model},
		Model: model,
		Env:   nil, // no runtime environment
	}
	runShapePhase(context.Background(), rs, r) // must not panic
}
