package simulations

import (
	"context"
	"errors"
	"maps"
	"reflect"
	"testing"
	"time"

	"intelli.sim.core/data/common"
	datamodels "intelli.sim.core/data/models"
	datasims "intelli.sim.core/data/simulations"
	"intelli.sim.core/modules/llm"
)

// --- fake llmInvoker -----------------------------------------------------------

type fakeLLM struct {
	simple          func(req llm.SimpleRequest) (*llm.Completion, error)
	reason          func(req llm.ReasoningRequest) (*llm.ReasoningCompletion, error)
	reasoningModels map[string]bool // providerUID|modelUID -> reasoning?
	calls           int
}

func descKey(providerUID, modelUID string) string { return providerUID + "|" + modelUID }

func usageFor(target common.LLMTarget) common.TokenUsage {
	return common.TokenUsage{
		LLMTarget:        target,
		PromptTokens:     10,
		CompletionTokens: 5,
		TotalTokens:      15,
		Calls:            1,
		LatencyMs:        1,
	}
}

func (f *fakeLLM) Simple(ctx context.Context, req llm.SimpleRequest) (*llm.Completion, error) {
	f.calls++
	if f.simple != nil {
		c, err := f.simple(req)
		if err != nil {
			return nil, err
		}
		if c.Usage.Calls == 0 {
			c.Usage = usageFor(req.Target)
		}
		return c, nil
	}
	return &llm.Completion{Text: "{}", FinishReason: "stop", Usage: usageFor(req.Target)}, nil
}

func (f *fakeLLM) Reasoning(ctx context.Context, req llm.ReasoningRequest) (*llm.ReasoningCompletion, error) {
	f.calls++
	if f.reason != nil {
		c, err := f.reason(req)
		if err != nil {
			return nil, err
		}
		if c.Usage.Calls == 0 {
			c.Usage = usageFor(req.Target)
		}
		return c, nil
	}
	return &llm.ReasoningCompletion{Analysis: "{}", FinishReason: "stop", Usage: usageFor(req.Target)}, nil
}

func (f *fakeLLM) ModelDescriptors(providerUID, modelUID string) ([]llm.ConfigParamDescriptor, bool, bool) {
	return nil, f.reasoningModels[descKey(providerUID, modelUID)], true
}
func (f *fakeLLM) MaxImages(string, string) (uint32, bool) { return 0, false }

func (f *fakeLLM) ValidateTarget(target common.LLMTarget, purpose common.ModelCapability) error {
	return nil
}

// --- helpers -------------------------------------------------------------------

func testCfg() ModuleConfig {
	return ModuleConfig{GenerationTimeout: 5 * time.Second, LLMBatchSize: 40}
}

func target(provider, model string) common.LLMTarget {
	return common.LLMTarget{ProviderUID: provider, ModelUID: model}
}

// newSim builds a Simulation with the given snapshot and appends one running
// generation run with the given Seq (matching what the service does before
// calling Run).
func newSim(snap *datamodels.Model, seed uint64, seq uint64) *datasims.Simulation {
	return &datasims.Simulation{
		ModelSnapshot:  snap,
		Seed:           seed,
		GenerationRuns: []datasims.GenerationRun{{Seq: seq, Status: datasims.RunRunning}},
	}
}

// fullScope is a whole-state regeneration scope.
func fullScope() datasims.GenerationScope { return datasims.GenerationScope{Kind: datasims.ScopeFull} }

// pureRandomModel builds a spatial model with 2 fixed-count archetypes (random
// placement, no llm), one fixed-position circle shape, and one non-llm global
// property. No LLM targets anywhere.
func pureRandomModel() *datamodels.Model {
	return &datamodels.Model{
		Name:        "PredatorPrey",
		Description: "test model",
		IsSpatial:   true,
		Agents: []datamodels.Agent{
			{
				UID:    "prey",
				Name:   "Prey",
				States: []string{"alive"},
				Properties: []datamodels.AgentProperty{
					numberProp("energy", f64ptr(0), f64ptr(100)),
				},
				Initialization: datamodels.AgentInitialization{
					PopulationCount: 3,
					InitialState:    "alive",
					CountMode:       datamodels.CountFixed,
					Placement:       &datamodels.Placement{Type: datamodels.PlacementRandom},
				},
			},
			{
				UID:    "predator",
				Name:   "Predator",
				States: []string{"hunting"},
				Properties: []datamodels.AgentProperty{
					numberProp("speed", f64ptr(1), f64ptr(10)),
				},
				Initialization: datamodels.AgentInitialization{
					PopulationCount: 2,
					InitialState:    "hunting",
					CountMode:       datamodels.CountFixed,
					Placement:       &datamodels.Placement{Type: datamodels.PlacementRandom},
				},
			},
		},
		Environment: &datamodels.Environment{
			Width:  100,
			Height: 100,
			GlobalProperties: []datamodels.AgentProperty{
				numberProp("temperature", f64ptr(0), f64ptr(40)),
			},
			Shapes: []datamodels.Shape{
				{
					UID:     "lake",
					Name:    "Lake",
					Type:    datamodels.ShapeTypeCircle,
					Visible: true,
					Initialization: datamodels.AgentInitialization{
						PopulationCount: 1,
						Placement:       &datamodels.Placement{Type: datamodels.PlacementRandom},
					},
					Geometry: &datamodels.ShapeGeometry{
						Position: &common.Point{X: 50, Y: 50},
						Radius:   &datamodels.ScalarSpec{Value: u64ptr(10)},
					},
				},
			},
		},
	}
}

// --- Test 1: pure-random, no llm targets ---------------------------------------

func TestGenerator_PureRandom_NoLLM(t *testing.T) {
	f := &fakeLLM{}
	g := NewGenerator(f, testCfg())

	sim := newSim(pureRandomModel(), 1234, 1)
	warns, requests, err := g.Run(context.Background(), sim, fullScope(), nil)
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if f.calls != 0 {
		t.Fatalf("expected 0 LLM calls, got %d", f.calls)
	}
	if requests != 0 {
		t.Fatalf("expected 0 requests, got %d", requests)
	}
	// The pure-random path (number props with min/max, no LLM) must produce zero
	// structural warnings. If this fails, update pureRandomModel or this assertion.
	if len(warns) != 0 {
		t.Errorf("expected 0 warnings on pure-random path, got %d: %+v", len(warns), warns)
	}

	// Counts.
	prey, pred := 0, 0
	for _, a := range sim.Agents {
		switch a.AgentUID {
		case "prey":
			prey++
		case "predator":
			pred++
		}
		if a.Position == nil {
			t.Fatalf("spatial agent %d has nil position", a.ID)
		}
		if a.Position.X >= 100 || a.Position.Y >= 100 {
			t.Fatalf("agent position out of bounds: %+v", a.Position)
		}
	}
	if prey != 3 || pred != 2 {
		t.Fatalf("expected 3 prey + 2 predator, got %d + %d", prey, pred)
	}

	// Environment.
	if sim.Environment == nil {
		t.Fatalf("environment is nil")
	}
	if _, ok := sim.Environment.GlobalProperties["temperature"]; !ok {
		t.Fatalf("temperature global property not set")
	}
	if len(sim.Environment.Shapes) != 1 {
		t.Fatalf("expected 1 shape, got %d", len(sim.Environment.Shapes))
	}
	if sim.Environment.Shapes[0].Position != (common.Point{X: 50, Y: 50}) {
		t.Fatalf("shape position: %+v", sim.Environment.Shapes[0].Position)
	}
	if sim.Environment.Shapes[0].Radius == nil || *sim.Environment.Shapes[0].Radius != 10 {
		t.Fatalf("shape radius: %v", sim.Environment.Shapes[0].Radius)
	}

	// Determinism: same seed → identical result.
	sim2 := newSim(pureRandomModel(), 1234, 1)
	if _, _, err := g.Run(context.Background(), sim2, fullScope(), nil); err != nil {
		t.Fatalf("Run #2 error: %v", err)
	}
	if !reflect.DeepEqual(sim.Agents, sim2.Agents) {
		t.Fatalf("agents not deterministic:\n%+v\nvs\n%+v", sim.Agents, sim2.Agents)
	}
	if !reflect.DeepEqual(sim.Environment, sim2.Environment) {
		t.Fatalf("environment not deterministic")
	}
}

// TestGenerator_RNG_DependsOnlyOnSeed pins the ISS-03 fix: the generation RNG
// must derive from sim.Seed ALONE, never from len(sim.GenerationRuns) — two full
// runs sharing a seed must produce byte-identical output even when one of them
// carries an artificially longer/differently-numbered GenerationRuns slice (as
// if several regenerations had already happened on that sim).
func TestGenerator_RNG_DependsOnlyOnSeed(t *testing.T) {
	f := &fakeLLM{}
	g := NewGenerator(f, testCfg())

	sim1 := newSim(pureRandomModel(), 1234, 1)
	if _, _, err := g.Run(context.Background(), sim1, fullScope(), nil); err != nil {
		t.Fatalf("run #1 error: %v", err)
	}

	sim2 := newSim(pureRandomModel(), 1234, 1)
	sim2.GenerationRuns = []datasims.GenerationRun{
		{Seq: 1, Status: datasims.RunCompleted},
		{Seq: 2, Status: datasims.RunCompleted},
		{Seq: 3, Status: datasims.RunRunning},
	}
	if _, _, err := g.Run(context.Background(), sim2, fullScope(), nil); err != nil {
		t.Fatalf("run #2 error: %v", err)
	}

	if !reflect.DeepEqual(sim1.Agents, sim2.Agents) {
		t.Fatalf("agents diverged despite identical seed (GenerationRuns length must not affect the RNG):\n%+v\nvs\n%+v", sim1.Agents, sim2.Agents)
	}
	if !reflect.DeepEqual(sim1.Environment, sim2.Environment) {
		t.Fatalf("environment diverged despite identical seed")
	}
}

// --- Manual placement leaves instances unplaced --------------------------------

func manualPlacementModel() *datamodels.Model {
	return &datamodels.Model{
		Name:      "ManualVsRandom",
		IsSpatial: true,
		Agents: []datamodels.Agent{
			{
				UID:    "hand",
				Name:   "Hand",
				States: []string{"idle"},
				Initialization: datamodels.AgentInitialization{
					PopulationCount: 3,
					InitialState:    "idle",
					CountMode:       datamodels.CountFixed,
					Placement:       &datamodels.Placement{Type: datamodels.PlacementManual},
				},
			},
			{
				UID:    "auto",
				Name:   "Auto",
				States: []string{"idle"},
				Initialization: datamodels.AgentInitialization{
					PopulationCount: 2,
					InitialState:    "idle",
					CountMode:       datamodels.CountFixed,
					Placement:       &datamodels.Placement{Type: datamodels.PlacementRandom},
				},
			},
		},
		Environment: &datamodels.Environment{Width: 100, Height: 100},
	}
}

// TestGenerator_ManualPlacement_LeavesUnplaced verifies that manual-placement
// agents are emitted without a position (the user places them per-simulation
// before the run), while other placement types are resolved as usual.
func TestGenerator_ManualPlacement_LeavesUnplaced(t *testing.T) {
	g := NewGenerator(&fakeLLM{}, testCfg())
	sim := newSim(manualPlacementModel(), 1234, 1)
	if _, _, err := g.Run(context.Background(), sim, fullScope(), nil); err != nil {
		t.Fatalf("Run error: %v", err)
	}

	hand, auto := 0, 0
	for _, a := range sim.Agents {
		switch a.AgentUID {
		case "hand":
			hand++
			if a.Position != nil {
				t.Errorf("manual-placement agent %d should be unplaced, got %+v", a.ID, a.Position)
			}
		case "auto":
			auto++
			if a.Position == nil {
				t.Errorf("random-placement agent %d should be positioned, got nil", a.ID)
			}
		}
	}
	if hand != 3 || auto != 2 {
		t.Fatalf("expected 3 hand + 2 auto, got %d + %d", hand, auto)
	}

	// The generated (unplaced) simulation must not be start-ready.
	res := ValidateInstance(*sim)
	if res.StartReady {
		t.Errorf("simulation with unplaced manual agents must not be start-ready")
	}
}

// --- Test 2: LLM property batch happy path -------------------------------------

func llmPropModel() *datamodels.Model {
	return &datamodels.Model{
		Name:      "EnergyModel",
		IsSpatial: false,
		Agents: []datamodels.Agent{
			{
				UID:    "worker",
				Name:   "Worker",
				States: []string{"idle"},
				Properties: []datamodels.AgentProperty{
					{
						UID:        "energy",
						Type:       datamodels.PropertyTypeNumber,
						MinValue:   f64ptr(0),
						MaxValue:   f64ptr(100),
						Generation: &datamodels.PropertyGeneration{Mode: datamodels.GenModeLLM},
					},
				},
				Initialization: datamodels.AgentInitialization{
					PopulationCount: 3,
					InitialState:    "idle",
					CountMode:       datamodels.CountFixed,
				},
			},
		},
	}
}

func TestGenerator_LLMPropertyBatch_HappyPath(t *testing.T) {
	f := &fakeLLM{
		simple: func(req llm.SimpleRequest) (*llm.Completion, error) {
			return &llm.Completion{Text: `{"instances":[{"instance":0,"values":{"energy":42}},{"instance":1,"values":{"energy":42}},{"instance":2,"values":{"energy":42}}]}`}, nil
		},
	}
	g := NewGenerator(f, testCfg())

	sim := newSim(llmPropModel(), 7, 1)
	tgt := target("openai", "gpt-test")
	sim.LLMGenerationTargetOverride = &tgt

	warns, requests, err := g.Run(context.Background(), sim, fullScope(), nil)
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if requests == 0 {
		t.Fatalf("expected >0 requests")
	}
	_ = warns

	if len(sim.Agents) != 3 {
		t.Fatalf("expected 3 agents, got %d", len(sim.Agents))
	}
	for _, a := range sim.Agents {
		if got := asFloat(t, a.Properties["energy"]); got != 42 {
			t.Fatalf("energy: got %v want 42", got)
		}
	}

	// Token folding.
	if sim.TokenUsage == nil {
		t.Fatalf("token usage nil")
	}
	if sim.TokenUsage.Total.Calls == 0 {
		t.Fatalf("total calls 0")
	}
	if !hasPurpose(sim.TokenUsage.ByPurpose, datasims.LLMUsageSimulationGeneration) {
		t.Fatalf("no simulation_generation purpose")
	}
	if len(sim.TokenUsage.ByGeneration) < 1 || sim.TokenUsage.ByGeneration[0].Calls == 0 {
		t.Fatalf("ByGeneration[0] calls 0: %+v", sim.TokenUsage.ByGeneration)
	}
	if len(sim.TokenUsage.ByModel) == 0 {
		t.Fatalf("ByModel empty")
	}
}

func hasPurpose(ps []datasims.PurposeTokenUsage, p datasims.LLMUsagePurpose) bool {
	for _, e := range ps {
		if e.Purpose == p {
			return true
		}
	}
	return false
}

// --- Test 3: LLM batch malformed JSON ------------------------------------------

func TestGenerator_LLMPropertyBatch_MalformedJSON(t *testing.T) {
	f := &fakeLLM{
		simple: func(req llm.SimpleRequest) (*llm.Completion, error) {
			return &llm.Completion{Text: "not json"}, nil
		},
	}
	g := NewGenerator(f, testCfg())

	sim := newSim(llmPropModel(), 9, 1)
	tgt := target("openai", "gpt-test")
	sim.LLMGenerationTargetOverride = &tgt

	warns, _, err := g.Run(context.Background(), sim, fullScope(), nil)
	if err != nil {
		t.Fatalf("Run error (should be nil): %v", err)
	}
	if len(warns) == 0 {
		t.Fatalf("expected warnings for malformed JSON")
	}
	if len(sim.Agents) != 3 {
		t.Fatalf("expected 3 agents, got %d", len(sim.Agents))
	}
	for _, a := range sim.Agents {
		v := asFloat(t, a.Properties["energy"])
		if v < 0 || v > 100 {
			t.Fatalf("fallback energy out of bounds: %v", v)
		}
	}
}

// --- Test 4: count clamped, reasoning path -------------------------------------

func llmCountModel() *datamodels.Model {
	return &datamodels.Model{
		Name:      "CountModel",
		IsSpatial: false,
		Agents: []datamodels.Agent{
			{
				UID:    "bug",
				Name:   "Bug",
				States: []string{"alive"},
				Initialization: datamodels.AgentInitialization{
					PopulationCount: 3,
					MinPopulation:   u64ptr(2),
					MaxPopulation:   u64ptr(5),
					InitialState:    "alive",
					CountMode:       datamodels.CountLLM,
				},
			},
		},
	}
}

func TestGenerator_CountClamped_ReasoningPath(t *testing.T) {
	reasonCalled := false
	f := &fakeLLM{
		reasoningModels: map[string]bool{descKey("openai", "o-test"): true},
		reason: func(req llm.ReasoningRequest) (*llm.ReasoningCompletion, error) {
			reasonCalled = true
			return &llm.ReasoningCompletion{Analysis: `{"count":99}`}, nil
		},
		simple: func(req llm.SimpleRequest) (*llm.Completion, error) {
			t.Fatalf("Simple should not be called for reasoning model count")
			return nil, errors.New("unreachable")
		},
	}
	g := NewGenerator(f, testCfg())

	sim := newSim(llmCountModel(), 5, 1)
	tgt := target("openai", "o-test")
	sim.LLMGenerationTargetOverride = &tgt

	warns, _, err := g.Run(context.Background(), sim, fullScope(), nil)
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if !reasonCalled {
		t.Fatalf("expected reasoning path to be used")
	}
	if len(sim.Agents) != 5 {
		t.Fatalf("expected count clamped to 5, got %d", len(sim.Agents))
	}
	if len(warns) == 0 {
		t.Fatalf("expected a clamp warning")
	}
}

// --- Test 5: placement directives sampled --------------------------------------

func llmPlacementModel() *datamodels.Model {
	return &datamodels.Model{
		Name:      "PlacementModel",
		IsSpatial: true,
		Agents: []datamodels.Agent{
			{
				UID:    "fish",
				Name:   "Fish",
				States: []string{"swim"},
				Initialization: datamodels.AgentInitialization{
					PopulationCount: 10,
					InitialState:    "swim",
					CountMode:       datamodels.CountFixed,
					Placement:       &datamodels.Placement{Type: datamodels.PlacementLLM},
				},
			},
		},
		Environment: &datamodels.Environment{Width: 100, Height: 100},
	}
}

func TestGenerator_PlacementDirectives_Sampled(t *testing.T) {
	reasonCalled := false
	f := &fakeLLM{
		reasoningModels: map[string]bool{descKey("openai", "o-test"): true},
		reason: func(req llm.ReasoningRequest) (*llm.ReasoningCompletion, error) {
			reasonCalled = true
			// Single cluster directive covering all, tiny radius near centre.
			return &llm.ReasoningCompletion{Analysis: `{"directives":[{"kind":"cluster","fraction":1.0,"center":{"x":50,"y":50},"radius":2}]}`}, nil
		},
	}
	g := NewGenerator(f, testCfg())

	sim := newSim(llmPlacementModel(), 11, 1)
	tgt := target("openai", "o-test")
	sim.LLMGenerationTargetOverride = &tgt

	_, _, err := g.Run(context.Background(), sim, fullScope(), nil)
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if !reasonCalled {
		t.Fatalf("expected reasoning path for placement")
	}
	if len(sim.Agents) != 10 {
		t.Fatalf("expected 10 agents, got %d", len(sim.Agents))
	}
	// Clustered near centre (radius 2): allow generous margin.
	for _, a := range sim.Agents {
		if a.Position == nil {
			t.Fatalf("nil position")
		}
		dx := int(a.Position.X) - 50
		dy := int(a.Position.Y) - 50
		if dx < -20 || dx > 20 || dy < -20 || dy > 20 {
			t.Fatalf("agent not clustered near centre: %+v", a.Position)
		}
	}
}

// --- Test 6: partial scope placement-only --------------------------------------

func TestGenerator_PartialScope_PlacementOnly(t *testing.T) {
	f := &fakeLLM{}
	g := NewGenerator(f, testCfg())

	snap := pureRandomModel()
	sim := newSim(snap, 99, 1)
	if _, _, err := g.Run(context.Background(), sim, fullScope(), nil); err != nil {
		t.Fatalf("full run error: %v", err)
	}

	// Capture prey instance properties + ids + positions.
	type snapshot struct {
		props map[string]interface{}
		id    uint64
		pos   common.Point
	}
	preyBefore := map[uint64]snapshot{}
	var predBefore []snapshot
	for _, a := range sim.Agents {
		if a.AgentUID == "prey" {
			preyBefore[a.ID] = snapshot{props: cloneMap(a.Properties), id: a.ID, pos: *a.Position}
		} else {
			predBefore = append(predBefore, snapshot{props: cloneMap(a.Properties), id: a.ID, pos: *a.Position})
		}
	}

	// Append a second run with a placement-only scope on prey.
	sim.GenerationRuns = append(sim.GenerationRuns, datasims.GenerationRun{Seq: 2, Status: datasims.RunRunning})
	scope := datasims.GenerationScope{
		Kind:      datasims.ScopeAgents,
		AgentUIDs: []string{"prey"},
		Parts:     []datasims.GenerationPart{datasims.PartPlacement},
	}
	if _, _, err := g.Run(context.Background(), sim, scope, nil); err != nil {
		t.Fatalf("placement-only run error: %v", err)
	}

	preyCount := 0
	positionsChanged := false
	for _, a := range sim.Agents {
		if a.AgentUID == "prey" {
			preyCount++
			before, ok := preyBefore[a.ID]
			if !ok {
				t.Fatalf("prey id %d not present before (count/ids changed)", a.ID)
			}
			if !reflect.DeepEqual(before.props, a.Properties) {
				t.Fatalf("prey %d properties changed: %v vs %v", a.ID, before.props, a.Properties)
			}
			if before.pos != *a.Position {
				positionsChanged = true
			}
		}
	}
	if preyCount != 3 {
		t.Fatalf("prey count changed: %d", preyCount)
	}
	if !positionsChanged {
		t.Fatalf("expected at least one prey position to change")
	}

	// Predators untouched.
	predNow := []snapshot{}
	for _, a := range sim.Agents {
		if a.AgentUID == "predator" {
			predNow = append(predNow, snapshot{props: cloneMap(a.Properties), id: a.ID, pos: *a.Position})
		}
	}
	if !reflect.DeepEqual(predBefore, predNow) {
		t.Fatalf("predators changed:\n%+v\nvs\n%+v", predBefore, predNow)
	}
}

func cloneMap(m map[string]interface{}) map[string]interface{} {
	return maps.Clone(m)
}

// --- Test 7: token folding dimensions ------------------------------------------

func TestGenerator_TokenFolding_Dimensions(t *testing.T) {
	f := &fakeLLM{
		simple: func(req llm.SimpleRequest) (*llm.Completion, error) {
			return &llm.Completion{Text: `{"instances":[{"instance":0,"values":{"energy":10}},{"instance":1,"values":{"energy":10}},{"instance":2,"values":{"energy":10}}]}`}, nil
		},
	}
	g := NewGenerator(f, testCfg())

	sim := newSim(llmPropModel(), 3, 1)
	tgt := target("openai", "gpt-test")
	sim.LLMGenerationTargetOverride = &tgt

	if _, _, err := g.Run(context.Background(), sim, fullScope(), nil); err != nil {
		t.Fatalf("Run error: %v", err)
	}

	tu := sim.TokenUsage
	if tu == nil {
		t.Fatalf("nil token usage")
	}
	if tu.Total.Calls == 0 {
		t.Fatalf("Total empty")
	}
	if len(tu.ByModel) == 0 {
		t.Fatalf("ByModel empty")
	}
	foundAgent := false
	for _, a := range tu.ByAgent {
		if a.AgentUID == "worker" && a.Calls > 0 {
			foundAgent = true
		}
	}
	if !foundAgent {
		t.Fatalf("ByAgent missing worker: %+v", tu.ByAgent)
	}
	if !hasPurpose(tu.ByPurpose, datasims.LLMUsageSimulationGeneration) {
		t.Fatalf("ByPurpose missing simulation_generation")
	}
	// runIdx == Seq-1 == 0.
	if len(tu.ByGeneration) != 1 {
		t.Fatalf("expected ByGeneration len 1, got %d", len(tu.ByGeneration))
	}
	if tu.ByGeneration[0].Calls == 0 {
		t.Fatalf("ByGeneration[0] empty")
	}
}

// --- nil snapshot guard --------------------------------------------------------

func TestGenerator_NilSnapshot(t *testing.T) {
	g := NewGenerator(&fakeLLM{}, testCfg())
	_, _, err := g.Run(context.Background(), &datasims.Simulation{}, fullScope(), nil)
	if err == nil {
		t.Fatalf("expected error for nil snapshot")
	}
	_, _, err = g.Run(context.Background(), nil, fullScope(), nil)
	if err == nil {
		t.Fatalf("expected error for nil sim")
	}
}
