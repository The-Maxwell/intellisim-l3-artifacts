package simulations

import (
	"context"
	"testing"

	"go.uber.org/zap"

	"intelli.sim.core/data/common"
	"intelli.sim.core/data/models"
	datasims "intelli.sim.core/data/simulations"
)

// byPurposeTotal returns the accumulated TotalTokens attributed to purpose in
// the usage's ByPurpose dimension (0 when absent). Shared test helper.
func byPurposeTotal(u *datasims.SimulationTokenUsage, purpose datasims.LLMUsagePurpose) uint64 {
	if u == nil {
		return 0
	}
	for i := range u.ByPurpose {
		if u.ByPurpose[i].Purpose == purpose {
			return u.ByPurpose[i].TotalTokens
		}
	}
	return 0
}

// individualTestSetup builds an n-member individual-scope group plus the runtime
// state and step context wired over inv. The archetype declares a numeric
// property "energy" and is movable so full-control actions validate. strat is
// the per-group resolved strategy (Type/flags/BatchSize/DecisionFrequency).
func individualTestSetup(inv llmInvoker, strat models.LLMStrategy, n int) (*StepContext, *AgentGroup) {
	arch := &models.Agent{
		UID:      "wolf",
		Type:     models.AgentTypePlain,
		MoveType: models.AgentMoveTypeMovable,
		Properties: []models.AgentProperty{
			{UID: "energy", Type: models.PropertyTypeNumber},
		},
		States: []string{"idle", "hunting"},
	}
	model := &models.Model{
		SimulationConfig: models.SimulationConfig{
			LLMSimulationExecutionConfig: models.LLMSimulationExecutionConfig{
				LLMConfig: models.LLMConfig{Target: common.LLMTarget{ProviderUID: "openai", ModelUID: "gpt-4o-mini"}},
			},
		},
		Agents: []models.Agent{*arch},
		Rules: []models.Rule{
			{
				ID:        "r1",
				Enabled:   true,
				AppliesTo: []string{"wolf"},
				Conditions: []models.Condition{
					{Type: models.ConditionTypeDecision, Value: "search_food"},
					{Type: models.ConditionTypeDecision, Value: "flee"},
				},
			},
		},
	}
	sim := &datasims.Simulation{}
	rs := &RuntimeState{Sim: sim, Model: model}

	members := make([]*RuntimeAgent, 0, n)
	rs.ByID = map[uint64]*RuntimeAgent{}
	for i := 0; i < n; i++ {
		id := uint64(i + 1)
		ra := &RuntimeAgent{
			A: &datasims.SimulationAgent{
				ID:         id,
				AgentUID:   "wolf",
				State:      "idle",
				Position:   &common.Point{X: uint64(10 * (i + 1)), Y: 10},
				Properties: map[string]any{"energy": float64(50)},
			},
			Archetype: arch,
		}
		members = append(members, ra)
		rs.Agents = append(rs.Agents, ra)
		rs.ByID[id] = ra
	}

	group := &AgentGroup{
		ArchetypeUID: "wolf",
		Archetype:    arch,
		Members:      members,
		Strategy:     strat,
		Target:       common.LLMTarget{ProviderUID: "openai", ModelUID: "gpt-4o-mini"},
	}

	llmRT := &llmRuntime{
		inv:    inv,
		cfg:    RunConfig{},
		usage:  &datasims.SimulationTokenUsage{},
		runIdx: 0,
		logger: zap.NewNop(),
	}
	sc := &StepContext{RS: rs, LLM: llmRT, Cfg: RunConfig{}, Ctx: context.Background()}
	return sc, group
}

func TestIndividualDecision_PerAgentLabels(t *testing.T) {
	// 3 agents, decision strategy, UseForDecisions; fake returns a distinct label
	// per instance index => each agent's intent carries its own label.
	inv := &fakeInvoker{
		simpleQueue: []fakeReply{{
			text: `{"agents":[
				{"instance":0,"label":"search_food"},
				{"instance":1,"label":"flee"},
				{"instance":2,"label":"search_food"}
			]}`,
			usage: common.TokenUsage{
				LLMTarget:   common.LLMTarget{ProviderUID: "openai", ModelUID: "gpt-4o-mini"},
				TotalTokens: 9, Calls: 1,
			},
		}},
	}
	strat := models.LLMStrategy{
		Type:            models.LLMStrategyDecision,
		UseForDecisions: true,
		BatchSize:       10, // one call covering all 3
	}
	sc, group := individualTestSetup(inv, strat, 3)

	res, err := (individualDecisionStrategy{}).Plan(sc, group)
	if err != nil {
		t.Fatalf("Plan error: %v", err)
	}
	want := map[uint64]string{1: "search_food", 2: "flee", 3: "search_food"}
	for id, label := range want {
		in := res.Intents[id]
		if in == nil || len(in.Decisions) == 0 {
			t.Fatalf("agent %d has no decision intent: %+v", id, in)
		}
		if in.Decisions[0].Label != label {
			t.Fatalf("agent %d label=%q want %q", id, in.Decisions[0].Label, label)
		}
	}
	// SET ra.LastDecision when a decision is produced (last_decision fallback).
	for _, ra := range group.Members {
		if ra.LastDecision == nil {
			t.Fatalf("agent %d LastDecision not set", ra.A.ID)
		}
	}
	// Usage attributed to the decision sub-purpose.
	if got := byPurposeTotal(sc.LLM.usage, datasims.LLMUsageSimulationDecision); got != 9 {
		t.Fatalf("ByPurpose[decision]=%d want 9", got)
	}
}

func TestIndividualFull_BatchesAgentsUpToBatchSize(t *testing.T) {
	// BatchSize=2, 3 agents => exactly 2 calls (2 + 1). Fake replies (the demux
	// never needs the second call's content for the count assertion).
	inv := &fakeInvoker{
		simpleQueue: []fakeReply{
			{text: `{"agents":[{"instance":0,"actions":[]},{"instance":1,"actions":[]}]}`,
				usage: common.TokenUsage{TotalTokens: 3, Calls: 1}},
			{text: `{"agents":[{"instance":0,"actions":[]}]}`,
				usage: common.TokenUsage{TotalTokens: 2, Calls: 1}},
		},
	}
	strat := models.LLMStrategy{
		Type:            models.LLMStrategyFull,
		UseForDecisions: true, // a single enabled purpose
		BatchSize:       2,
	}
	sc, group := individualTestSetup(inv, strat, 3)

	if _, err := (individualFullStrategy{}).Plan(sc, group); err != nil {
		t.Fatalf("Plan error: %v", err)
	}
	if inv.simpleCalls != 2 {
		t.Fatalf("expected 2 LLM calls (BatchSize 2 over 3 agents), got %d", inv.simpleCalls)
	}
}

func TestIndividual_NoEnabledPurpose_NoCalls(t *testing.T) {
	// All UseFor* false => zero LLM calls and empty intents for both individual
	// strategies.
	stratFull := models.LLMStrategy{Type: models.LLMStrategyFull, BatchSize: 4}
	stratDec := models.LLMStrategy{Type: models.LLMStrategyDecision, BatchSize: 4}

	invF := &fakeInvoker{}
	scF, groupF := individualTestSetup(invF, stratFull, 3)
	resF, err := (individualFullStrategy{}).Plan(scF, groupF)
	if err != nil {
		t.Fatalf("full Plan error: %v", err)
	}
	if invF.simpleCalls != 0 || invF.reasoningCalls != 0 {
		t.Fatalf("full: expected 0 LLM calls, got simple=%d reasoning=%d", invF.simpleCalls, invF.reasoningCalls)
	}
	if len(resF.Intents) != 0 {
		t.Fatalf("full: expected empty intents, got %d", len(resF.Intents))
	}

	invD := &fakeInvoker{}
	scD, groupD := individualTestSetup(invD, stratDec, 3)
	resD, err := (individualDecisionStrategy{}).Plan(scD, groupD)
	if err != nil {
		t.Fatalf("decision Plan error: %v", err)
	}
	if invD.simpleCalls != 0 || invD.reasoningCalls != 0 {
		t.Fatalf("decision: expected 0 LLM calls, got simple=%d reasoning=%d", invD.simpleCalls, invD.reasoningCalls)
	}
	if len(resD.Intents) != 0 {
		t.Fatalf("decision: expected empty intents, got %d", len(resD.Intents))
	}
}

// --- Task 12: archetype strategies + hybrid ------------------------------------

// archetypeTestState builds a runtime state from a set of archetype descriptors.
// Each entry of specs declares an archetype UID, its resolved LLMStrategy and the
// number of members; members are placed at distinct positions so per-agent
// variation (VectorToward) diverges. A lone "prey" archetype at a fixed point is
// added as a movement target so "toward" directives resolve. Returns the state,
// the step context (over inv) and the per-archetype groups (in specs order).
func archetypeTestState(inv llmInvoker, specs []archSpec) (*StepContext, []*AgentGroup) {
	model := &models.Model{
		SimulationConfig: models.SimulationConfig{
			LLMSimulationExecutionConfig: models.LLMSimulationExecutionConfig{
				LLMConfig: models.LLMConfig{Target: common.LLMTarget{ProviderUID: "openai", ModelUID: "gpt-4o-mini"}},
			},
		},
	}
	// A stationary prey at (200,200) so "toward"/nearest target resolution works.
	prey := &models.Agent{UID: "prey", Type: models.AgentTypeArchetype, MoveType: models.AgentMoveTypeStatic}
	model.Agents = append(model.Agents, *prey)

	sim := &datasims.Simulation{Seed: 7}
	rs := &RuntimeState{Sim: sim, Model: model, ByID: map[uint64]*RuntimeAgent{}}

	preyRA := &RuntimeAgent{
		A:         &datasims.SimulationAgent{ID: 1, AgentUID: "prey", State: "idle", Position: &common.Point{X: 200, Y: 200}},
		Archetype: prey,
	}
	rs.Agents = append(rs.Agents, preyRA)
	rs.ByID[1] = preyRA

	nextID := uint64(2)
	groups := make([]*AgentGroup, 0, len(specs))
	for _, sp := range specs {
		arch := &models.Agent{
			UID:      sp.uid,
			Type:     models.AgentTypeArchetype,
			MoveType: models.AgentMoveTypeMovable,
			Properties: []models.AgentProperty{
				{UID: "energy", Type: models.PropertyTypeNumber},
			},
			States: []string{"idle", "hunting"},
		}
		model.Agents = append(model.Agents, *arch)
		members := make([]*RuntimeAgent, 0, sp.count)
		for i := 0; i < sp.count; i++ {
			id := nextID
			nextID++
			ra := &RuntimeAgent{
				A: &datasims.SimulationAgent{
					ID:         id,
					AgentUID:   sp.uid,
					State:      "idle",
					Position:   &common.Point{X: uint64(10 + 17*int(id)), Y: uint64(10 + 23*int(id))},
					Properties: map[string]any{"energy": float64(50)},
				},
				Archetype: arch,
			}
			members = append(members, ra)
			rs.Agents = append(rs.Agents, ra)
			rs.ByID[id] = ra
		}
		groups = append(groups, &AgentGroup{
			ArchetypeUID: sp.uid,
			Archetype:    arch,
			Members:      members,
			Strategy:     sp.strat,
			Target:       common.LLMTarget{ProviderUID: "openai", ModelUID: "gpt-4o-mini"},
		})
	}

	llmRT := &llmRuntime{
		inv:    inv,
		cfg:    RunConfig{},
		usage:  &datasims.SimulationTokenUsage{},
		runIdx: 0,
		logger: zap.NewNop(),
	}
	sc := &StepContext{RS: rs, LLM: llmRT, Cfg: RunConfig{}, Ctx: context.Background()}
	return sc, groups
}

type archSpec struct {
	uid   string
	strat models.LLMStrategy
	count int
}

func TestArchetypeDecision_OneCallPerArchetype_AgentsVary(t *testing.T) {
	// 4 agents in ONE archetype, decision strategy, BatchSize<=1 => EXACTLY 1 LLM
	// call (the archetype is the unit, not the agent). The strategy also drives
	// movement so the single archetype directive fans out per-agent with
	// local-context resolution + seeded jitter, yielding NON-IDENTICAL velocities.
	inv := &fakeInvoker{
		simpleQueue: []fakeReply{
			// First call (decision purpose): one label for the archetype.
			{text: `{"agents":[{"archetype_uid":"wolf","label":"search_food"}]}`,
				usage: common.TokenUsage{
					LLMTarget:   common.LLMTarget{ProviderUID: "openai", ModelUID: "gpt-4o-mini"},
					TotalTokens: 5, Calls: 1,
				}},
			// Second call (movement purpose): one directive for the archetype.
			{text: `{"agents":[{"archetype_uid":"wolf","intent":"toward","target_ref":"prey","speed":0.8}]}`,
				usage: common.TokenUsage{
					LLMTarget:   common.LLMTarget{ProviderUID: "openai", ModelUID: "gpt-4o-mini"},
					TotalTokens: 6, Calls: 1,
				}},
		},
	}
	strat := models.LLMStrategy{
		Type:            models.LLMStrategyDecision,
		UseForDecisions: true,
		UseForMovement:  true,
		BatchSize:       1, // archetype unit; 1 archetype => 1 call per purpose
	}
	sc, groups := archetypeTestState(inv, []archSpec{{uid: "wolf", strat: strat, count: 4}})
	group := groups[0]

	res, err := (archetypeDecisionStrategy{}).Plan(sc, group)
	if err != nil {
		t.Fatalf("Plan error: %v", err)
	}
	// EXACTLY one call per enabled purpose (decision + movement) = 2 calls; a 5th
	// agent would NOT add calls (archetype scope).
	if inv.simpleCalls != 2 {
		t.Fatalf("expected 2 LLM calls (1 per purpose, archetype scope), got %d", inv.simpleCalls)
	}

	// Every member got the shared decision label.
	for _, ra := range group.Members {
		in := res.Intents[ra.A.ID]
		if in == nil || len(in.Decisions) == 0 || in.Decisions[0].Label != "search_food" {
			t.Fatalf("agent %d missing shared decision label: %+v", ra.A.ID, in)
		}
		if ra.LastDecision == nil || ra.LastDecision.Label != "search_food" {
			t.Fatalf("agent %d LastDecision not set to shared label", ra.A.ID)
		}
	}

	// The per-agent resolved velocities must DIVERGE (different positions + jitter).
	seen := map[Vector]bool{}
	distinct := 0
	for _, ra := range group.Members {
		if seen[ra.Velocity] {
			continue
		}
		seen[ra.Velocity] = true
		distinct++
	}
	if distinct < 2 {
		t.Fatalf("archetype agents must not all share one velocity; distinct=%d velocities=%+v", distinct, velocitiesOf(group))
	}
}

func TestArchetypeDecision_DistributionMode_OneCall_AgentsVaryLabels(t *testing.T) {
	// One LLM call returns a weighted label distribution for the archetype; the engine
	// samples one label PER MEMBER, so members diverge and each LastDecision matches its
	// own sampled label (not a single shared one).
	inv := &fakeInvoker{
		simpleQueue: []fakeReply{
			{text: `{"agents":[{"archetype_uid":"wolf","choices":[{"label":"flee","weight":1},{"label":"hold","weight":1}]}]}`,
				usage: common.TokenUsage{
					LLMTarget:   common.LLMTarget{ProviderUID: "openai", ModelUID: "gpt-4o-mini"},
					TotalTokens: 5, Calls: 1,
				}},
		},
	}
	strat := models.LLMStrategy{
		Type:            models.LLMStrategyDecision,
		UseForDecisions: true,
		BatchSize:       1,
		ArchetypeMode:   models.ArchetypeModeDistribution,
	}
	sc, groups := archetypeTestState(inv, []archSpec{{uid: "wolf", strat: strat, count: 12}})
	group := groups[0]

	res, err := (archetypeDecisionStrategy{}).Plan(sc, group)
	if err != nil {
		t.Fatalf("Plan error: %v", err)
	}
	if inv.simpleCalls != 1 {
		t.Fatalf("expected exactly 1 LLM call (archetype scope), got %d", inv.simpleCalls)
	}

	labels := map[string]int{}
	for _, ra := range group.Members {
		in := res.Intents[ra.A.ID]
		if in == nil || len(in.Decisions) != 1 {
			t.Fatalf("agent %d missing sampled decision: %+v", ra.A.ID, in)
		}
		labels[in.Decisions[0].Label]++
		if ra.LastDecision == nil || ra.LastDecision.Label != in.Decisions[0].Label {
			t.Fatalf("agent %d LastDecision %v must match its sampled label %q", ra.A.ID, ra.LastDecision, in.Decisions[0].Label)
		}
	}
	if labels["flee"] == 0 || labels["hold"] == 0 {
		t.Fatalf("distribution must diversify labels across members: %v", labels)
	}
}

func TestArchetypeFull_DistributionMode_VariantsSampled(t *testing.T) {
	// One LLM call returns weighted action-set variants; each member samples ONE whole
	// variant. Only the decision purpose is enabled to keep this to a single call.
	inv := &fakeInvoker{
		simpleQueue: []fakeReply{
			{text: `{"agents":[{"archetype_uid":"wolf","variants":[` +
				`{"weight":1,"actions":[{"type":"transition","new_state":"hunting"}]},` +
				`{"weight":1,"actions":[{"type":"set_property","property_uid":"energy","value":10}]}]}]}`,
				usage: common.TokenUsage{
					LLMTarget:   common.LLMTarget{ProviderUID: "openai", ModelUID: "gpt-4o-mini"},
					TotalTokens: 7, Calls: 1,
				}},
		},
	}
	strat := models.LLMStrategy{
		Type:            models.LLMStrategyFull,
		UseForDecisions: true, // only the decision purpose → one call
		BatchSize:       1,
		ArchetypeMode:   models.ArchetypeModeDistribution,
	}
	sc, groups := archetypeTestState(inv, []archSpec{{uid: "wolf", strat: strat, count: 12}})
	group := groups[0]

	res, err := (archetypeFullStrategy{}).Plan(sc, group)
	if err != nil {
		t.Fatalf("Plan error: %v", err)
	}
	if inv.simpleCalls != 1 {
		t.Fatalf("expected exactly 1 LLM call, got %d", inv.simpleCalls)
	}

	transition, setprop := 0, 0
	for _, ra := range group.Members {
		in := res.Intents[ra.A.ID]
		if in == nil || len(in.Actions) != 1 {
			t.Fatalf("agent %d must carry exactly ONE variant's actions (no mixing): %+v", ra.A.ID, in)
		}
		switch in.Actions[0].Type {
		case models.ActionTypeTransition:
			transition++
		case models.ActionTypeSetProperty:
			setprop++
		}
	}
	if transition == 0 || setprop == 0 {
		t.Fatalf("both variants must be sampled across members: transition=%d set_property=%d", transition, setprop)
	}
}

func TestArchetypeFull_BatchesArchetypes(t *testing.T) {
	// 2 archetypes, full strategy, BatchSize=2 => ONE call covering both, demuxed
	// by archetype_uid into each archetype's directive.
	inv := &fakeInvoker{
		simpleQueue: []fakeReply{
			{text: `{"agents":[
				{"archetype_uid":"wolf","actions":[{"type":"transition","new_state":"hunting"}]},
				{"archetype_uid":"sheep","actions":[{"type":"transition","new_state":"hunting"}]}
			]}`,
				usage: common.TokenUsage{
					LLMTarget:   common.LLMTarget{ProviderUID: "openai", ModelUID: "gpt-4o-mini"},
					TotalTokens: 8, Calls: 1,
				}},
		},
	}
	strat := models.LLMStrategy{
		Type:            models.LLMStrategyFull,
		UseForDecisions: true, // a single enabled purpose drives the action directive
		BatchSize:       2,    // batch BOTH archetypes into one call
	}
	sc, groups := archetypeTestState(inv, []archSpec{
		{uid: "wolf", strat: strat, count: 2},
		{uid: "sheep", strat: strat, count: 3},
	})

	res := planArchetypeBatch(sc, groups, archetypeFullStrategy{})
	if inv.simpleCalls != 1 {
		t.Fatalf("expected 1 batched LLM call covering 2 archetypes, got %d", inv.simpleCalls)
	}
	// Each archetype's members received the demuxed transition action.
	for _, g := range groups {
		for _, ra := range g.Members {
			in := res[g.ArchetypeUID].Intents[ra.A.ID]
			if in == nil || len(in.Actions) == 0 || in.Actions[0].NewState != "hunting" {
				t.Fatalf("archetype %s agent %d missing demuxed action: %+v", g.ArchetypeUID, ra.A.ID, in)
			}
		}
	}
}

func TestHybrid_RoutesPerArchetype(t *testing.T) {
	// Archetype A (wolf) = full×archetype (LLM-driven); archetype B (sheep) = none
	// (rules only). Only A makes LLM calls; B yields empty intents.
	inv := &fakeInvoker{
		simpleQueue: []fakeReply{
			{text: `{"agents":[{"archetype_uid":"wolf","actions":[{"type":"transition","new_state":"hunting"}]}]}`,
				usage: common.TokenUsage{
					LLMTarget:   common.LLMTarget{ProviderUID: "openai", ModelUID: "gpt-4o-mini"},
					TotalTokens: 4, Calls: 1,
				}},
		},
	}
	fullArch := models.LLMStrategy{Type: models.LLMStrategyFull, UseForDecisions: true, BatchSize: 1}
	noneArch := models.LLMStrategy{Type: models.LLMStrategyNone}

	sc, groups := archetypeTestState(inv, []archSpec{
		{uid: "wolf", strat: fullArch, count: 2},
		{uid: "sheep", strat: noneArch, count: 2},
	})

	// Mark each archetype's per-archetype strategy on the model so the hybrid
	// strategy resolves it (LLMConfig.Strategy overrides the model default).
	for _, g := range groups {
		g.Archetype.LLMConfig = &models.LLMAgentConfig{Strategy: models.LLMAgentStrategy{LLMStrategy: g.Strategy}}
	}
	// Sync the runtime model.Agents LLMConfig too (resolution reads the archetype).
	for i := range sc.RS.Model.Agents {
		for _, g := range groups {
			if sc.RS.Model.Agents[i].UID == g.ArchetypeUID {
				sc.RS.Model.Agents[i].LLMConfig = g.Archetype.LLMConfig
			}
		}
	}

	hybrid := hybridStrategy{}
	for _, g := range groups {
		res, err := hybrid.Plan(sc, g)
		if err != nil {
			t.Fatalf("hybrid Plan(%s) error: %v", g.ArchetypeUID, err)
		}
		switch g.ArchetypeUID {
		case "wolf":
			for _, ra := range g.Members {
				in := res.Intents[ra.A.ID]
				if in == nil || len(in.Actions) == 0 || in.Actions[0].NewState != "hunting" {
					t.Fatalf("wolf agent %d missing LLM-driven action: %+v", ra.A.ID, in)
				}
			}
		case "sheep":
			if len(res.Intents) != 0 {
				t.Fatalf("sheep (none) must yield no intents, got %d", len(res.Intents))
			}
		}
	}
	if inv.simpleCalls != 1 {
		t.Fatalf("expected exactly 1 LLM call (only the full archetype), got %d", inv.simpleCalls)
	}
}

// velocitiesOf collects the per-agent velocities of a group (test diagnostics).
func velocitiesOf(g *AgentGroup) []Vector {
	out := make([]Vector, 0, len(g.Members))
	for _, ra := range g.Members {
		out = append(out, ra.Velocity)
	}
	return out
}

// --- Task 7: LLMNote set at demux points ----------------------------------------

func TestIndividualDecision_LLMNoteSet(t *testing.T) {
	// When the individual decision strategy demuxes a label, each agent's intent
	// must have LLMNote set to "chose: <label>" (compact, no raw LLM text).
	inv := &fakeInvoker{
		simpleQueue: []fakeReply{{
			text: `{"agents":[
				{"instance":0,"label":"flee"},
				{"instance":1,"label":"search_food"}
			]}`,
			usage: common.TokenUsage{TotalTokens: 5, Calls: 1},
		}},
	}
	strat := models.LLMStrategy{
		Type:            models.LLMStrategyDecision,
		UseForDecisions: true,
		BatchSize:       10,
	}
	sc, group := individualTestSetup(inv, strat, 2)

	res, err := (individualDecisionStrategy{}).Plan(sc, group)
	if err != nil {
		t.Fatalf("Plan error: %v", err)
	}
	wantNotes := map[uint64]string{1: "chose: flee", 2: "chose: search_food"}
	for id, want := range wantNotes {
		in := res.Intents[id]
		if in == nil {
			t.Fatalf("agent %d has no intent", id)
		}
		if in.LLMNote != want {
			t.Fatalf("agent %d LLMNote=%q want %q", id, in.LLMNote, want)
		}
	}
}

func TestIndividualFull_LLMNoteSet(t *testing.T) {
	// When the individual full strategy demuxes actions, each agent's intent must
	// have LLMNote set to "actions: <type,type,...>" (de-duplicated, comma-separated).
	inv := &fakeInvoker{
		simpleQueue: []fakeReply{{
			text: `{"agents":[
				{"instance":0,"actions":[{"type":"transition","new_state":"hunting"},{"type":"transition","new_state":"idle"}]},
				{"instance":1,"actions":[{"type":"transition","new_state":"hunting"}]}
			]}`,
			usage: common.TokenUsage{TotalTokens: 8, Calls: 1},
		}},
	}
	strat := models.LLMStrategy{
		Type:            models.LLMStrategyFull,
		UseForDecisions: true, // decision purpose for full-control actions
		BatchSize:       10,
	}
	sc, group := individualTestSetup(inv, strat, 2)

	res, err := (individualFullStrategy{}).Plan(sc, group)
	if err != nil {
		t.Fatalf("Plan error: %v", err)
	}
	// Agent 1: two "transition" actions => de-duplicated to one type => "actions: transition"
	in1 := res.Intents[1]
	if in1 == nil || in1.LLMNote == "" {
		t.Fatalf("agent 1 LLMNote empty; intent=%+v", in1)
	}
	if in1.LLMNote != "actions: transition" {
		t.Fatalf("agent 1 LLMNote=%q want %q", in1.LLMNote, "actions: transition")
	}
	// Agent 2: one "transition" action.
	in2 := res.Intents[2]
	if in2 == nil || in2.LLMNote == "" {
		t.Fatalf("agent 2 LLMNote empty; intent=%+v", in2)
	}
	if in2.LLMNote != "actions: transition" {
		t.Fatalf("agent 2 LLMNote=%q want %q", in2.LLMNote, "actions: transition")
	}
}

func TestArchetypeDirective_LLMNoteDecision(t *testing.T) {
	// ApplyDirectiveWithVariation must set LLMNote="chose: <label>" when the
	// directive contains a decision.
	inv := &fakeInvoker{
		simpleQueue: []fakeReply{{
			text:  `{"agents":[{"archetype_uid":"wolf","label":"search_food"}]}`,
			usage: common.TokenUsage{TotalTokens: 4, Calls: 1},
		}},
	}
	strat := models.LLMStrategy{
		Type:            models.LLMStrategyDecision,
		UseForDecisions: true,
		BatchSize:       1,
	}
	sc, groups := archetypeTestState(inv, []archSpec{{uid: "wolf", strat: strat, count: 2}})
	group := groups[0]

	res, err := (archetypeDecisionStrategy{}).Plan(sc, group)
	if err != nil {
		t.Fatalf("Plan error: %v", err)
	}
	for _, ra := range group.Members {
		in := res.Intents[ra.A.ID]
		if in == nil {
			t.Fatalf("agent %d missing intent", ra.A.ID)
		}
		if in.LLMNote != "chose: search_food" {
			t.Fatalf("agent %d LLMNote=%q want %q", ra.A.ID, in.LLMNote, "chose: search_food")
		}
	}
}

func TestArchetypeDirective_LLMNoteActions(t *testing.T) {
	// ApplyDirectiveWithVariation must set LLMNote="actions: <type,...>" when the
	// directive contains actions (full archetype strategy path).
	inv := &fakeInvoker{
		simpleQueue: []fakeReply{{
			text:  `{"agents":[{"archetype_uid":"wolf","actions":[{"type":"transition","new_state":"hunting"}]}]}`,
			usage: common.TokenUsage{TotalTokens: 5, Calls: 1},
		}},
	}
	strat := models.LLMStrategy{
		Type:            models.LLMStrategyFull,
		UseForDecisions: true,
		BatchSize:       1,
	}
	sc, groups := archetypeTestState(inv, []archSpec{{uid: "wolf", strat: strat, count: 2}})
	group := groups[0]

	res, err := (archetypeFullStrategy{}).Plan(sc, group)
	if err != nil {
		t.Fatalf("Plan error: %v", err)
	}
	for _, ra := range group.Members {
		in := res.Intents[ra.A.ID]
		if in == nil {
			t.Fatalf("agent %d missing intent", ra.A.ID)
		}
		if in.LLMNote != "actions: transition" {
			t.Fatalf("agent %d LLMNote=%q want %q", ra.A.ID, in.LLMNote, "actions: transition")
		}
	}
}
