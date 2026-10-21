package simulations

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"

	"intelli.sim.core/data/common"
	"intelli.sim.core/data/models"
	datasims "intelli.sim.core/data/simulations"
	"intelli.sim.core/modules/llm"
)

// fakeInvoker is a programmable llmInvoker for runtime tests. It records the
// requests it received and replays canned completions/errors in order.
// All mutable fields are protected by mu so the fake is safe for concurrent use
// (e.g. TestRunLLMPhase_ParallelDeterminism calls it from multiple goroutines).
type fakeInvoker struct {
	mu sync.Mutex

	// simpleResults / reasoningResults are consumed in order; when exhausted the
	// last one repeats. err (when set for an index) is returned instead.
	simpleQueue    []fakeReply
	reasoningQueue []fakeReply

	// reasoning advertises the ModelDescriptors reasoning flag.
	reasoning bool
	// descriptorsOK is the third return of ModelDescriptors.
	descriptorsOK bool

	// validateErr, when set, is returned by ValidateTarget.
	validateErr error

	// counters
	simpleCalls    int
	reasoningCalls int
	lastSimple     llm.SimpleRequest
	lastReasoning  llm.ReasoningRequest
}

type fakeReply struct {
	text  string
	usage common.TokenUsage
	err   error
}

func (f *fakeInvoker) Simple(ctx context.Context, req llm.SimpleRequest) (*llm.Completion, error) {
	f.mu.Lock()
	f.lastSimple = req
	idx := f.simpleCalls
	f.simpleCalls++
	rep := pick(f.simpleQueue, idx)
	f.mu.Unlock()
	if rep.err != nil {
		return nil, rep.err
	}
	return &llm.Completion{Text: rep.text, Usage: rep.usage}, nil
}

func (f *fakeInvoker) Reasoning(ctx context.Context, req llm.ReasoningRequest) (*llm.ReasoningCompletion, error) {
	f.mu.Lock()
	f.lastReasoning = req
	idx := f.reasoningCalls
	f.reasoningCalls++
	rep := pick(f.reasoningQueue, idx)
	f.mu.Unlock()
	if rep.err != nil {
		return nil, rep.err
	}
	return &llm.ReasoningCompletion{Analysis: rep.text, Usage: rep.usage}, nil
}

func (f *fakeInvoker) ModelDescriptors(providerUID, modelUID string) ([]llm.ConfigParamDescriptor, bool, bool) {
	f.mu.Lock()
	reasoning := f.reasoning
	descriptorsOK := f.descriptorsOK
	f.mu.Unlock()
	return nil, reasoning, descriptorsOK
}

func (f *fakeInvoker) ValidateTarget(target common.LLMTarget, purpose common.ModelCapability) error {
	f.mu.Lock()
	err := f.validateErr
	f.mu.Unlock()
	return err
}

// pick returns the entry at idx, or the last entry when idx is past the end, or
// a zero reply for an empty queue.
func pick(q []fakeReply, idx int) fakeReply {
	if len(q) == 0 {
		return fakeReply{}
	}
	if idx >= len(q) {
		return q[len(q)-1]
	}
	return q[idx]
}

// newTestRuntime builds an llmRuntime over a fake invoker and a fresh usage
// accumulator.
func newTestRuntime(inv llmInvoker) *llmRuntime {
	return &llmRuntime{
		inv:    inv,
		cfg:    RunConfig{},
		usage:  &datasims.SimulationTokenUsage{},
		runIdx: 0,
		logger: zap.NewNop(),
	}
}

// --- resolveTarget ------------------------------------------------------------

func TestResolveTarget_AgentOverridesEffortOnly(t *testing.T) {
	model := &models.Model{
		SimulationConfig: models.SimulationConfig{
			LLMSimulationExecutionConfig: models.LLMSimulationExecutionConfig{
				LLMConfig: models.LLMConfig{
					Target: common.LLMTarget{ProviderUID: "openai", ModelUID: "gpt-4o-mini", Effort: common.LLMEffortLow},
				},
			},
		},
	}
	sim := &datasims.Simulation{}
	agent := &models.Agent{
		LLMConfig: &models.LLMAgentConfig{
			// Agent overrides ONLY effort; provider/model are inherited from the model.
			LLMConfig: models.LLMConfig{Target: common.LLMTarget{Effort: common.LLMEffortHigh}},
		},
	}
	got, ok := resolveTarget(model, sim, agent)
	if !ok {
		t.Fatal("expected a resolvable target")
	}
	if got.ProviderUID != "openai" || got.ModelUID != "gpt-4o-mini" {
		t.Fatalf("provider/model not inherited: %+v", got)
	}
	if got.Effort != common.LLMEffortHigh {
		t.Fatalf("effort not overridden: got %q want high", got.Effort)
	}
}

func TestResolveTarget_SimOverridesModel(t *testing.T) {
	model := &models.Model{
		SimulationConfig: models.SimulationConfig{
			LLMSimulationExecutionConfig: models.LLMSimulationExecutionConfig{
				LLMConfig: models.LLMConfig{
					Target: common.LLMTarget{ProviderUID: "openai", ModelUID: "gpt-4o-mini"},
				},
			},
		},
	}
	sim := &datasims.Simulation{
		LLMExecutionTargetOverride: &common.LLMTarget{ProviderUID: "anthropic", ModelUID: "claude"},
	}
	got, ok := resolveTarget(model, sim, nil)
	if !ok {
		t.Fatal("expected a resolvable target")
	}
	if got.ProviderUID != "anthropic" || got.ModelUID != "claude" {
		t.Fatalf("sim override not applied: %+v", got)
	}
}

func TestResolveTarget_NoProviderDegrades(t *testing.T) {
	model := &models.Model{}
	sim := &datasims.Simulation{}
	agent := &models.Agent{LLMConfig: &models.LLMAgentConfig{LLMConfig: models.LLMConfig{Target: common.LLMTarget{Effort: common.LLMEffortHigh}}}}
	_, ok := resolveTarget(model, sim, agent)
	if ok {
		t.Fatal("expected ok=false when no level yields a provider/model")
	}
}

// --- callJSON -----------------------------------------------------------------

func TestCallJSON_HappyPathFoldsUsage(t *testing.T) {
	inv := &fakeInvoker{
		simpleQueue: []fakeReply{{
			text:  `{"fire":true}`,
			usage: common.TokenUsage{LLMTarget: common.LLMTarget{ProviderUID: "openai", ModelUID: "gpt-4o-mini"}, TotalTokens: 12, Calls: 1},
		}},
	}
	r := newTestRuntime(inv)
	target := common.LLMTarget{ProviderUID: "openai", ModelUID: "gpt-4o-mini"}
	var out interactionResp
	ok := r.callJSON(context.Background(), target, common.ModelCapabilitySimulationExecution,
		datasims.LLMUsageSimulationInteraction, "wolf", "sys", "user", nil, &out)
	if !ok {
		t.Fatal("expected ok=true on valid JSON")
	}
	if !out.Fire {
		t.Fatalf("out not populated: %+v", out)
	}
	if r.usage.Total.TotalTokens != 12 {
		t.Fatalf("usage not folded: total=%d want 12", r.usage.Total.TotalTokens)
	}
	if len(r.usage.ByRun) != 1 || r.usage.ByRun[0].TotalTokens != 12 {
		t.Fatalf("ByRun not grown: %+v", r.usage.ByRun)
	}
	// The explicit usagePurpose (interaction) — distinct from the validated
	// capability (execution) — must drive the ByPurpose attribution.
	if pu := byPurposeTotal(r.usage, datasims.LLMUsageSimulationInteraction); pu != 12 {
		t.Fatalf("ByPurpose[interaction]=%d want 12 (capability execution must not override usage purpose)", pu)
	}
	if r.inv.(*fakeInvoker).lastSimple.ResponseFormat != llm.ResponseFormatJSON {
		t.Fatal("ResponseFormat should be core-controlled JSON")
	}
}

func TestCallJSON_RetryThenRulesOnly(t *testing.T) {
	callErr := errors.New("provider down")
	inv := &fakeInvoker{
		simpleQueue: []fakeReply{{err: callErr}, {err: callErr}, {err: callErr}},
	}
	r := newTestRuntime(inv)
	target := common.LLMTarget{ProviderUID: "openai", ModelUID: "gpt-4o-mini"}
	fb := &models.FallbackPolicy{Steps: []models.FallbackStep{
		{Type: models.FallbackRetry, MaxRetries: 2, RetryDelayMs: 0},
		{Type: models.FallbackRulesOnly},
	}}
	var out interactionResp
	ok := r.callJSON(context.Background(), target, common.ModelCapabilitySimulationDecisions,
		datasims.LLMUsageSimulationDecision, "wolf", "sys", "user", fb, &out)
	if ok {
		t.Fatal("expected ok=false after retries exhaust into rules_only")
	}
	// 1 initial + 2 retries = 3 attempts.
	if inv.simpleCalls != 3 {
		t.Fatalf("expected 3 attempts (1 + MaxRetries 2), got %d", inv.simpleCalls)
	}
}

func TestCallJSON_MalformedJSONReturnsNotOk(t *testing.T) {
	inv := &fakeInvoker{
		simpleQueue: []fakeReply{{text: "this is not json", usage: common.TokenUsage{TotalTokens: 3, Calls: 1}}},
	}
	r := newTestRuntime(inv)
	target := common.LLMTarget{ProviderUID: "openai", ModelUID: "gpt-4o-mini"}
	var out interactionResp
	ok := r.callJSON(context.Background(), target, common.ModelCapabilitySimulationExecution,
		datasims.LLMUsageSimulationProperties, "", "sys", "user", nil, &out)
	if ok {
		t.Fatal("expected ok=false on malformed JSON (no panic)")
	}
}

func TestCallJSON_PicksReasoningWhenAdvertised(t *testing.T) {
	inv := &fakeInvoker{
		reasoning:     true,
		descriptorsOK: true,
		reasoningQueue: []fakeReply{{
			text:  `{"fire":true}`,
			usage: common.TokenUsage{TotalTokens: 7, Calls: 1},
		}},
	}
	r := newTestRuntime(inv)
	target := common.LLMTarget{ProviderUID: "openai", ModelUID: "o3"}
	var out interactionResp
	ok := r.callJSON(context.Background(), target, common.ModelCapabilitySimulationDecisions,
		datasims.LLMUsageSimulationMovement, "wolf", "sys", "user", nil, &out)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if inv.reasoningCalls != 1 {
		t.Fatalf("expected Reasoning() to be used once, got %d (simple=%d)", inv.reasoningCalls, inv.simpleCalls)
	}
	if inv.simpleCalls != 0 {
		t.Fatalf("Simple() must not be called when reasoning advertised, got %d", inv.simpleCalls)
	}
}

func TestCallJSON_NoProviderReturnsNotOk(t *testing.T) {
	r := newTestRuntime(&fakeInvoker{})
	var out interactionResp
	ok := r.callJSON(context.Background(), common.LLMTarget{}, common.ModelCapabilitySimulationExecution,
		datasims.LLMUsageSimulationProperties, "", "sys", "user", nil, &out)
	if ok {
		t.Fatal("expected ok=false when target has no provider")
	}
}

// --- MediateInteraction -------------------------------------------------------

func TestMediateInteraction_LLMDecidesFire(t *testing.T) {
	inv := &fakeInvoker{
		simpleQueue: []fakeReply{{text: `{"fire":false}`, usage: common.TokenUsage{TotalTokens: 4, Calls: 1}}},
	}
	r := newTestRuntime(inv)
	model := &models.Model{SimulationConfig: models.SimulationConfig{LLMSimulationExecutionConfig: models.LLMSimulationExecutionConfig{LLMConfig: models.LLMConfig{Target: common.LLMTarget{ProviderUID: "openai", ModelUID: "gpt-4o-mini"}}}}}
	rs := &RuntimeState{Sim: &datasims.Simulation{}, Model: model}
	in := models.Interaction{ID: "hunt", Type: models.InteractionTypeAgentAgent, UseLLM: true}
	initiator := &RuntimeAgent{A: &datasims.SimulationAgent{ID: 1, AgentUID: "wolf"}}
	target := &RuntimeAgent{A: &datasims.SimulationAgent{ID: 2, AgentUID: "sheep"}}
	fire, _ := r.MediateInteraction(context.Background(), rs, in, initiator, target)
	if fire {
		t.Fatal("expected fire=false from the LLM mediation response")
	}
}

func TestMediateInteraction_FailSoftFires(t *testing.T) {
	inv := &fakeInvoker{
		simpleQueue: []fakeReply{{err: errors.New("provider down")}},
	}
	r := newTestRuntime(inv)
	model := &models.Model{SimulationConfig: models.SimulationConfig{LLMSimulationExecutionConfig: models.LLMSimulationExecutionConfig{LLMConfig: models.LLMConfig{Target: common.LLMTarget{ProviderUID: "openai", ModelUID: "gpt-4o-mini"}}}}}
	rs := &RuntimeState{Sim: &datasims.Simulation{}, Model: model}
	in := models.Interaction{ID: "hunt", Type: models.InteractionTypeAgentAgent, UseLLM: true}
	initiator := &RuntimeAgent{A: &datasims.SimulationAgent{ID: 1, AgentUID: "wolf"}}
	target := &RuntimeAgent{A: &datasims.SimulationAgent{ID: 2, AgentUID: "sheep"}}
	fire, _ := r.MediateInteraction(context.Background(), rs, in, initiator, target)
	if !fire {
		t.Fatal("expected fail-soft fire=true when the LLM call errors")
	}
}

// --- Task 7: buildContext "Prior LLM context" block ----------------------------

func TestBuildContext_PriorLLMContextBlock_AppearsWhenPresent(t *testing.T) {
	// buildContext must include a "Prior LLM context for agent <id>:" block when
	// the agent has AgentMemoryLLM or AgentMemoryRule entries.
	model := &models.Model{}
	rs := &RuntimeState{Sim: &datasims.Simulation{}, Model: model}
	ra := &RuntimeAgent{
		A: &datasims.SimulationAgent{
			ID:       42,
			AgentUID: "wolf",
			Memory: []datasims.AgentMemory{
				{Step: 1, Type: datasims.AgentMemoryLLM, Summary: "chose: flee"},
				{Step: 2, Type: datasims.AgentMemoryRule, Summary: "rule fired: hunt"},
			},
		},
		Archetype: &models.Agent{UID: "wolf"},
	}
	group := &AgentGroup{ArchetypeUID: "wolf", Archetype: ra.Archetype}
	members := []*RuntimeAgent{ra}

	ctx := buildContext(rs, group, members)
	if ctx == "" {
		t.Fatal("expected non-empty buildContext output when agent has llm/rule memory")
	}
	if !ctxContains(ctx, "Prior LLM context for agent 42") {
		t.Fatalf("expected 'Prior LLM context for agent 42' in context, got:\n%s", ctx)
	}
	if !ctxContains(ctx, "chose: flee") {
		t.Fatalf("expected llm_response summary in context, got:\n%s", ctx)
	}
	if !ctxContains(ctx, "rule fired: hunt") {
		t.Fatalf("expected rule summary in context, got:\n%s", ctx)
	}
}

func TestBuildContext_PriorLLMContextBlock_AbsentWhenNoLLMRuleMemory(t *testing.T) {
	// buildContext must NOT include the prior LLM context block when the agent
	// has only non-LLM, non-rule memory entries (e.g. movement/interaction).
	model := &models.Model{}
	rs := &RuntimeState{Sim: &datasims.Simulation{}, Model: model}
	ra := &RuntimeAgent{
		A: &datasims.SimulationAgent{
			ID:       7,
			AgentUID: "sheep",
			Memory: []datasims.AgentMemory{
				{Step: 1, Type: datasims.AgentMemoryMovement, Summary: "moved"},
				{Step: 2, Type: datasims.AgentMemoryInteraction, Summary: "met_agent"},
			},
		},
		Archetype: &models.Agent{UID: "sheep"},
	}
	group := &AgentGroup{ArchetypeUID: "sheep", Archetype: ra.Archetype}
	members := []*RuntimeAgent{ra}

	ctx := buildContext(rs, group, members)
	if ctxContains(ctx, "Prior LLM context") {
		t.Fatalf("expected no 'Prior LLM context' block when no llm/rule memory; got:\n%s", ctx)
	}
}

func TestBuildContext_PriorLLMContextBlock_EmptyAgentNoBlock(t *testing.T) {
	// buildContext with an agent with no memory at all must produce no prior LLM
	// context block (and not panic).
	model := &models.Model{}
	rs := &RuntimeState{Sim: &datasims.Simulation{}, Model: model}
	ra := &RuntimeAgent{
		A:         &datasims.SimulationAgent{ID: 99, AgentUID: "wolf"},
		Archetype: &models.Agent{UID: "wolf"},
	}
	group := &AgentGroup{ArchetypeUID: "wolf", Archetype: ra.Archetype}
	members := []*RuntimeAgent{ra}

	ctx := buildContext(rs, group, members)
	if ctxContains(ctx, "Prior LLM context") {
		t.Fatalf("expected no 'Prior LLM context' block for agent with no memory; got:\n%s", ctx)
	}
}

func TestSummarizeMemoryByType_TrimsTrailingNewline(t *testing.T) {
	// Symmetry fix (T6-M1): SummarizeMemoryByType must trim the trailing newline
	// so it matches SummarizeMemory's behaviour.
	ra := &RuntimeAgent{A: &datasims.SimulationAgent{Memory: []datasims.AgentMemory{
		{Step: 1, Type: datasims.AgentMemoryRule, Summary: "rule_fired"},
	}}}
	s := SummarizeMemoryByType(ra, []datasims.AgentMemoryType{datasims.AgentMemoryRule}, 10)
	if len(s) > 0 && s[len(s)-1] == '\n' {
		t.Fatalf("SummarizeMemoryByType must not end with newline (symmetry with SummarizeMemory); got: %q", s)
	}
}

// ctxContains is a substring helper scoped to the llm_runtime_test file; distinct
// from the validation.go containsStr (which works on []string) and the
// spatial_test.go contains (which works on []uint64).
func ctxContains(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(s) < len(substr) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// --- Task 9: buildContext perception block ------------------------------------

// makeSimLLMConfig returns a *models.LLMSimulationAgentConfig with the given
// PerceptionRadius so test helpers stay compact.
func makeSimLLMConfig(pr float64) *models.LLMSimulationAgentConfig {
	return &models.LLMSimulationAgentConfig{PerceptionRadius: pr}
}

// TestBuildContext_PerceptionBlock verifies the "Nearby:" perception block:
//   - spatial model + agent with position + perceptionRadius>0 + neighbours within
//     radius → block appears, sorted by neighbour ID, lists archetype UID + dx/dy + state.
//   - non-spatial model → no block.
//   - radius==0 (no per-agent or model config) → no block.
func TestBuildContext_PerceptionBlock(t *testing.T) {
	// Shared archetypes.
	wolf := &models.Agent{
		UID: "wolf",
		LLMConfig: &models.LLMAgentConfig{
			SimulationConfig: makeSimLLMConfig(15.0),
		},
	}
	sheep := &models.Agent{UID: "sheep"}
	rabbit := &models.Agent{UID: "rabbit"}

	// Build a spatial model.
	spatialModel := &models.Model{
		IsSpatial: true,
		Agents:    []models.Agent{*wolf, *sheep, *rabbit},
	}

	// Acting agent: wolf at (10, 10).
	actorPos := common.Point{X: 10, Y: 10}
	actor := &RuntimeAgent{
		A: &datasims.SimulationAgent{
			ID:       1,
			AgentUID: "wolf",
			Position: &actorPos,
			State:    "hunting",
		},
		Archetype: wolf,
	}

	// Neighbour 1: sheep at (12, 10) — within radius 15.
	n1Pos := common.Point{X: 12, Y: 10}
	neigh1 := &RuntimeAgent{
		A: &datasims.SimulationAgent{
			ID:       3,
			AgentUID: "sheep",
			Position: &n1Pos,
			State:    "grazing",
		},
		Archetype: sheep,
	}

	// Neighbour 2: rabbit at (10, 18) — within radius 15.
	n2Pos := common.Point{X: 10, Y: 18}
	neigh2 := &RuntimeAgent{
		A: &datasims.SimulationAgent{
			ID:       2,
			AgentUID: "rabbit",
			Position: &n2Pos,
			State:    "idle",
		},
		Archetype: rabbit,
	}

	// Build RuntimeState with all three agents, no spatial index (brute-force
	// neighborsWithin path), bounded world 100×100.
	env := &datasims.SimulationEnvironment{Width: 100, Height: 100, Torus: false}
	rs := &RuntimeState{
		Sim:    &datasims.Simulation{},
		Model:  spatialModel,
		Env:    env,
		Agents: []*RuntimeAgent{actor, neigh1, neigh2},
		ByID:   map[uint64]*RuntimeAgent{1: actor, 3: neigh1, 2: neigh2},
	}

	group := &AgentGroup{ArchetypeUID: "wolf", Archetype: wolf}
	members := []*RuntimeAgent{actor}

	t.Run("SpatialWithNeighbours", func(t *testing.T) {
		ctx := buildContext(rs, group, members)
		if !ctxContains(ctx, "Nearby:") {
			t.Fatalf("expected 'Nearby:' block in context; got:\n%s", ctx)
		}
		// Both neighbours must appear.
		if !ctxContains(ctx, "rabbit") {
			t.Fatalf("expected neighbour 'rabbit' in Nearby block; got:\n%s", ctx)
		}
		if !ctxContains(ctx, "sheep") {
			t.Fatalf("expected neighbour 'sheep' in Nearby block; got:\n%s", ctx)
		}
		// Sorted by ID: rabbit (ID=2) before sheep (ID=3).
		rabbitIdx := indexOfSubstr(ctx, "rabbit")
		sheepIdx := indexOfSubstr(ctx, "sheep")
		if rabbitIdx < 0 || sheepIdx < 0 {
			t.Fatalf("neighbours missing from context: rabbit=%d sheep=%d\n%s", rabbitIdx, sheepIdx, ctx)
		}
		if rabbitIdx > sheepIdx {
			t.Fatalf("neighbours must be sorted by ID (rabbit ID=2 before sheep ID=3); got:\n%s", ctx)
		}
		// State must appear.
		if !ctxContains(ctx, "idle") {
			t.Fatalf("expected rabbit state 'idle' in Nearby block; got:\n%s", ctx)
		}
		if !ctxContains(ctx, "grazing") {
			t.Fatalf("expected sheep state 'grazing' in Nearby block; got:\n%s", ctx)
		}
	})

	t.Run("NonSpatialModel_NoBlock", func(t *testing.T) {
		nonSpatialModel := &models.Model{
			IsSpatial: false,
			Agents:    []models.Agent{*wolf, *sheep, *rabbit},
		}
		rsNS := &RuntimeState{
			Sim:    &datasims.Simulation{},
			Model:  nonSpatialModel,
			Env:    env,
			Agents: []*RuntimeAgent{actor, neigh1, neigh2},
			ByID:   map[uint64]*RuntimeAgent{1: actor, 3: neigh1, 2: neigh2},
		}
		ctx := buildContext(rsNS, group, members)
		if ctxContains(ctx, "Nearby:") {
			t.Fatalf("expected no 'Nearby:' block for non-spatial model; got:\n%s", ctx)
		}
	})

	t.Run("RadiusZero_NoBlock", func(t *testing.T) {
		// Wolf archetype has no SimulationConfig → perceptionRadius defaults to 0.
		wolfNoRadius := &models.Agent{
			UID:       "wolf",
			LLMConfig: &models.LLMAgentConfig{SimulationConfig: makeSimLLMConfig(0)},
		}
		actorNoRadius := &RuntimeAgent{
			A:         actor.A,
			Archetype: wolfNoRadius,
		}
		groupNR := &AgentGroup{ArchetypeUID: "wolf", Archetype: wolfNoRadius}
		rsNR := &RuntimeState{
			Sim:    &datasims.Simulation{},
			Model:  spatialModel,
			Env:    env,
			Agents: []*RuntimeAgent{actorNoRadius, neigh1, neigh2},
			ByID:   map[uint64]*RuntimeAgent{1: actorNoRadius, 3: neigh1, 2: neigh2},
		}
		ctx := buildContext(rsNR, groupNR, []*RuntimeAgent{actorNoRadius})
		if ctxContains(ctx, "Nearby:") {
			t.Fatalf("expected no 'Nearby:' block when perceptionRadius==0; got:\n%s", ctx)
		}
	})

	t.Run("AgentNoPosition_NoBlock", func(t *testing.T) {
		actorNoPos := &RuntimeAgent{
			A: &datasims.SimulationAgent{
				ID:       1,
				AgentUID: "wolf",
				Position: nil, // no position
				State:    "hunting",
			},
			Archetype: wolf,
		}
		rsNoPos := &RuntimeState{
			Sim:    &datasims.Simulation{},
			Model:  spatialModel,
			Env:    env,
			Agents: []*RuntimeAgent{actorNoPos, neigh1, neigh2},
			ByID:   map[uint64]*RuntimeAgent{1: actorNoPos, 3: neigh1, 2: neigh2},
		}
		ctx := buildContext(rsNoPos, group, []*RuntimeAgent{actorNoPos})
		if ctxContains(ctx, "Nearby:") {
			t.Fatalf("expected no 'Nearby:' block when actor has no position; got:\n%s", ctx)
		}
	})
}

// TestPerceptionRadius verifies the three-level resolver:
//
//	agent.SimulationConfig.PerceptionRadius > 0 → use it.
//	agent PR==0 but model LLMSimulationExecutionConfig.PerceptionRadius > 0 → use model.
//	both 0 → 0.
func TestPerceptionRadius(t *testing.T) {
	t.Run("AgentOverridesModel", func(t *testing.T) {
		ra := &RuntimeAgent{
			A: &datasims.SimulationAgent{AgentUID: "wolf"},
			Archetype: &models.Agent{
				UID: "wolf",
				LLMConfig: &models.LLMAgentConfig{
					SimulationConfig: makeSimLLMConfig(20.0),
				},
			},
		}
		if got := perceptionRadius(ra); got != 20.0 {
			t.Fatalf("expected 20.0, got %v", got)
		}
	})

	t.Run("FallsBackToModel", func(t *testing.T) {
		// Agent has no SimulationConfig; the model default should be used via
		// rs.Model.SimulationConfig.LLMSimulationExecutionConfig. But perceptionRadius only takes
		// a *RuntimeAgent — the model fallback path is tested through buildContext.
		// Here we verify that an agent with nil SimulationConfig yields 0 when there
		// is no place to fall back to.
		ra := &RuntimeAgent{
			A: &datasims.SimulationAgent{AgentUID: "wolf"},
			Archetype: &models.Agent{
				UID:       "wolf",
				LLMConfig: &models.LLMAgentConfig{SimulationConfig: nil},
			},
		}
		if got := perceptionRadius(ra); got != 0 {
			t.Fatalf("expected 0 when no SimulationConfig, got %v", got)
		}
	})

	t.Run("NilLLMConfig_Zero", func(t *testing.T) {
		ra := &RuntimeAgent{
			A:         &datasims.SimulationAgent{AgentUID: "wolf"},
			Archetype: &models.Agent{UID: "wolf", LLMConfig: nil},
		}
		if got := perceptionRadius(ra); got != 0 {
			t.Fatalf("expected 0 when LLMConfig is nil, got %v", got)
		}
	})

	t.Run("NilArchetype_Zero", func(t *testing.T) {
		ra := &RuntimeAgent{
			A:         &datasims.SimulationAgent{AgentUID: "wolf"},
			Archetype: nil,
		}
		if got := perceptionRadius(ra); got != 0 {
			t.Fatalf("expected 0 when Archetype is nil, got %v", got)
		}
	})
}

// TestBuildContext_PerceptionBlock_ModelFallback verifies the model-level
// LLMSimulationExecutionConfig.PerceptionRadius fallback when the agent archetype
// has no SimulationConfig.
func TestBuildContext_PerceptionBlock_ModelFallback(t *testing.T) {
	defaultPR := 15.0
	modelExec := models.LLMSimulationExecutionConfig{}
	modelExec.PerceptionRadius = defaultPR

	wolf := &models.Agent{
		UID:       "wolf",
		LLMConfig: &models.LLMAgentConfig{SimulationConfig: nil}, // no per-agent PR
	}
	sheep := &models.Agent{UID: "sheep"}

	spatialModel := &models.Model{
		IsSpatial: true,
		Agents:    []models.Agent{*wolf, *sheep},
		SimulationConfig: models.SimulationConfig{
			LLMSimulationExecutionConfig: modelExec,
		},
	}

	actorPos := common.Point{X: 10, Y: 10}
	actor := &RuntimeAgent{
		A: &datasims.SimulationAgent{
			ID:       1,
			AgentUID: "wolf",
			Position: &actorPos,
			State:    "hunting",
		},
		Archetype: wolf,
	}
	n1Pos := common.Point{X: 12, Y: 10}
	neigh1 := &RuntimeAgent{
		A: &datasims.SimulationAgent{
			ID:       2,
			AgentUID: "sheep",
			Position: &n1Pos,
			State:    "grazing",
		},
		Archetype: sheep,
	}

	env := &datasims.SimulationEnvironment{Width: 100, Height: 100, Torus: false}
	rs := &RuntimeState{
		Sim:    &datasims.Simulation{},
		Model:  spatialModel,
		Env:    env,
		Agents: []*RuntimeAgent{actor, neigh1},
		ByID:   map[uint64]*RuntimeAgent{1: actor, 2: neigh1},
	}

	group := &AgentGroup{ArchetypeUID: "wolf", Archetype: wolf}
	ctx := buildContext(rs, group, []*RuntimeAgent{actor})

	if !ctxContains(ctx, "Nearby:") {
		t.Fatalf("expected 'Nearby:' block via model fallback PerceptionRadius; got:\n%s", ctx)
	}
	if !ctxContains(ctx, "sheep") {
		t.Fatalf("expected neighbour 'sheep' in Nearby block; got:\n%s", ctx)
	}
}

// indexOfSubstr returns the index of the first occurrence of substr in s, or -1.
func indexOfSubstr(s, substr string) int {
	if len(substr) == 0 {
		return 0
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// --- Task 11: resolveCallTimeout + resolveRunMaxRetries -----------------------

// TestResolveCallTimeout_Precedence verifies the four-level timeout resolution:
//
//	target.TimeoutMs (most specific) → simCfg.RetryTimeoutMs → genTimeout → defaultRunCallTimeout.
func TestResolveCallTimeout_Precedence(t *testing.T) {
	msPtr := func(ms uint64) *uint64 { return &ms }

	genTimeout := 120 * time.Second
	simCfg := &models.LLMSimulationExecutionConfig{RetryTimeoutMs: 60_000}

	t.Run("TargetTimeoutMs_Wins", func(t *testing.T) {
		target := common.LLMTarget{TimeoutMs: msPtr(5_000)}
		got := resolveCallTimeout(target, simCfg, genTimeout)
		if got != 5*time.Second {
			t.Fatalf("want 5s (target), got %v", got)
		}
	})

	t.Run("SimCfgRetryTimeoutMs_WhenNoTarget", func(t *testing.T) {
		target := common.LLMTarget{} // no TimeoutMs
		got := resolveCallTimeout(target, simCfg, genTimeout)
		if got != 60*time.Second {
			t.Fatalf("want 60s (simCfg.RetryTimeoutMs), got %v", got)
		}
	})

	t.Run("GenTimeout_WhenNoSimCfg", func(t *testing.T) {
		target := common.LLMTarget{}
		got := resolveCallTimeout(target, nil, genTimeout)
		if got != genTimeout {
			t.Fatalf("want %v (genTimeout), got %v", genTimeout, got)
		}
	})

	t.Run("GenTimeout_WhenSimCfgRetryTimeoutIsZero", func(t *testing.T) {
		target := common.LLMTarget{}
		zeroCfg := &models.LLMSimulationExecutionConfig{RetryTimeoutMs: 0}
		got := resolveCallTimeout(target, zeroCfg, genTimeout)
		if got != genTimeout {
			t.Fatalf("want %v (genTimeout fallback when RetryTimeoutMs==0), got %v", genTimeout, got)
		}
	})

	t.Run("DefaultRunCallTimeout_LastResort", func(t *testing.T) {
		target := common.LLMTarget{}
		got := resolveCallTimeout(target, nil, 0)
		if got != defaultRunCallTimeout {
			t.Fatalf("want defaultRunCallTimeout=%v, got %v", defaultRunCallTimeout, got)
		}
	})
}

// TestResolveRunMaxRetries_Precedence verifies the retry budget resolution:
//
//	policy FallbackRetry step value → simCfg.MaxRetries (if policy has no retry step) → 0.
func TestResolveRunMaxRetries_Precedence(t *testing.T) {
	simCfg := &models.LLMSimulationExecutionConfig{MaxRetries: 5}

	t.Run("PolicyRetry_Wins", func(t *testing.T) {
		// Policy has a FallbackRetry step with MaxRetries=3.
		fb := &models.FallbackPolicy{Steps: []models.FallbackStep{
			{Type: models.FallbackRetry, MaxRetries: 3, RetryDelayMs: 0},
			{Type: models.FallbackRulesOnly},
		}}
		policyRetries, _, _ := resolveFallback(fb)
		got := resolveRunMaxRetries(simCfg, policyRetries, hasFallbackRetryStep(fb))
		if got != 3 {
			t.Fatalf("want 3 (policy), got %d", got)
		}
	})

	t.Run("SimCfgMaxRetries_WhenNoPolicyRetryStep", func(t *testing.T) {
		// Policy has no FallbackRetry step.
		fb := &models.FallbackPolicy{Steps: []models.FallbackStep{
			{Type: models.FallbackRulesOnly},
		}}
		policyRetries, _, _ := resolveFallback(fb)
		got := resolveRunMaxRetries(simCfg, policyRetries, hasFallbackRetryStep(fb))
		if got != 5 {
			t.Fatalf("want 5 (simCfg.MaxRetries), got %d", got)
		}
	})

	t.Run("Zero_WhenNilPolicyAndNilSimCfg", func(t *testing.T) {
		// nil fallback, nil simCfg → 0.
		policyRetries, _, _ := resolveFallback(nil)
		got := resolveRunMaxRetries(nil, policyRetries, hasFallbackRetryStep(nil))
		if got != 0 {
			t.Fatalf("want 0, got %d", got)
		}
	})

	t.Run("Zero_WhenNilSimCfgAndNoPolicyRetryStep", func(t *testing.T) {
		fb := &models.FallbackPolicy{Steps: []models.FallbackStep{
			{Type: models.FallbackRulesOnly},
		}}
		policyRetries, _, _ := resolveFallback(fb)
		got := resolveRunMaxRetries(nil, policyRetries, hasFallbackRetryStep(fb))
		if got != 0 {
			t.Fatalf("want 0, got %d", got)
		}
	})
}

// TestCallJSON_HonorsResolvedTimeout verifies that callJSON applies the
// run-level RetryTimeoutMs from simCfg when target has no TimeoutMs.
// We check that a cancelled context (simulating a timeout) propagates ok=false.
func TestCallJSON_HonorsResolvedTimeout(t *testing.T) {
	// Build a runtime with simCfg carrying a small RetryTimeoutMs.
	simCfg := &models.LLMSimulationExecutionConfig{RetryTimeoutMs: 1} // 1ms — effectively immediate
	inv := &fakeInvoker{
		// Simple returns an error to simulate the provider not responding in time.
		simpleQueue: []fakeReply{{err: errors.New("timeout")}},
	}
	r := &llmRuntime{
		inv:        inv,
		cfg:        RunConfig{},
		simCfg:     simCfg,
		genTimeout: 300 * time.Second,
		usage:      &datasims.SimulationTokenUsage{},
		logger:     zap.NewNop(),
	}
	target := common.LLMTarget{ProviderUID: "openai", ModelUID: "gpt-4o-mini"}
	var out interactionResp
	ok := r.callJSON(context.Background(), target, common.ModelCapabilitySimulationExecution,
		datasims.LLMUsageSimulationInteraction, "wolf", "sys", "user", nil, &out)
	if ok {
		t.Fatal("expected ok=false when the provider call fails")
	}
}

// TestCallJSON_HonorsRunLevelMaxRetries verifies that when the fallback policy
// has no FallbackRetry step, the run-level MaxRetries from simCfg is applied.
func TestCallJSON_HonorsRunLevelMaxRetries(t *testing.T) {
	callErr := errors.New("provider down")
	inv := &fakeInvoker{
		// Return error for all 3 calls (1 initial + 2 from simCfg.MaxRetries).
		simpleQueue: []fakeReply{{err: callErr}, {err: callErr}, {err: callErr}},
	}
	simCfg := &models.LLMSimulationExecutionConfig{MaxRetries: 2}
	r := &llmRuntime{
		inv:        inv,
		cfg:        RunConfig{},
		simCfg:     simCfg,
		genTimeout: 300 * time.Second,
		usage:      &datasims.SimulationTokenUsage{},
		logger:     zap.NewNop(),
	}
	target := common.LLMTarget{ProviderUID: "openai", ModelUID: "gpt-4o-mini"}
	// Fallback policy with NO FallbackRetry step → run-level MaxRetries should apply.
	fb := &models.FallbackPolicy{Steps: []models.FallbackStep{
		{Type: models.FallbackRulesOnly},
	}}
	var out interactionResp
	ok := r.callJSON(context.Background(), target, common.ModelCapabilitySimulationDecisions,
		datasims.LLMUsageSimulationDecision, "wolf", "sys", "user", fb, &out)
	if ok {
		t.Fatal("expected ok=false after retries exhaust")
	}
	// 1 initial + 2 retries from simCfg.MaxRetries = 3 total.
	if inv.simpleCalls != 3 {
		t.Fatalf("expected 3 attempts (1 + simCfg.MaxRetries 2), got %d", inv.simpleCalls)
	}
}

// --- generation-time context must not leak into per-step prompts --------------

// TestBuildContext_NoGenerationContextLeak asserts that buildContext, when called
// for any per-step purpose (movement / properties / decision / interaction), does
// NOT include the initialization placement prompt context. That string is the
// generation-time-only population/placement hint (used by buildCountPrompt /
// buildPlacementPrompt) and must not bleed into runtime LLM calls; only the
// model-level and archetype-level PromptContext remain.
func TestBuildContext_NoGenerationContextLeak(t *testing.T) {
	const placementMarker = "PLACEMENT_CTX_MARKER"

	arch := &models.Agent{
		UID: "wolf",
		Initialization: models.AgentInitialization{
			Placement: &models.Placement{Type: models.PlacementLLM, PromptContext: placementMarker},
		},
		LLMConfig: &models.LLMAgentConfig{
			LLMConfig: models.LLMConfig{PromptContext: "AGENT_CTX_MARKER"}, // must still appear
		},
	}
	model := &models.Model{
		SimulationConfig: models.SimulationConfig{
			LLMSimulationExecutionConfig: models.LLMSimulationExecutionConfig{
				LLMConfig: models.LLMConfig{
					PromptContext: "MODEL_CTX_MARKER", // must still appear
				},
			},
		},
	}
	rs := &RuntimeState{Sim: &datasims.Simulation{}, Model: model}
	group := &AgentGroup{ArchetypeUID: "wolf", Archetype: arch}
	ra := &RuntimeAgent{
		A:         &datasims.SimulationAgent{ID: 1, AgentUID: "wolf"},
		Archetype: arch,
	}
	members := []*RuntimeAgent{ra}

	perStepPurposes := []purpose{
		purposeMovement,
		purposeProperties,
		purposeDecision,
		purposeInteraction,
	}

	for _, p := range perStepPurposes {
		ctx := buildContext(rs, group, members)

		if ctxContains(ctx, placementMarker) {
			t.Errorf("purpose %v: placement (generation) prompt context leaked into per-step buildContext output:\n%s", p, ctx)
		}
		// The model and archetype PromptContext must still be present.
		if !ctxContains(ctx, "MODEL_CTX_MARKER") {
			t.Errorf("purpose %v: model PromptContext missing from buildContext output:\n%s", p, ctx)
		}
		if !ctxContains(ctx, "AGENT_CTX_MARKER") {
			t.Errorf("purpose %v: archetype PromptContext missing from buildContext output:\n%s", p, ctx)
		}
	}
}
