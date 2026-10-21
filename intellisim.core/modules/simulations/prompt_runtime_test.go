package simulations

import (
	"strings"
	"testing"

	"intelli.sim.core/data/common"
	"intelli.sim.core/data/models"
	datasims "intelli.sim.core/data/simulations"
)

// runtimePromptFixture builds a minimal spatial runtime state with one wolf
// agent, one shape, one global, and recognizable context markers.
func runtimePromptFixture() (*RuntimeState, *AgentGroup) {
	arch := &models.Agent{
		UID:  "wolf",
		Name: "Wolf",
		Properties: []models.AgentProperty{
			{UID: "energy", Name: "Energy", Type: models.PropertyTypeNumber, MinValue: f64ptr(0), MaxValue: f64ptr(100), LLMPromptContext: "ENERGY_CTX_MARKER"},
			{UID: "tag", Name: "Tag", Type: models.PropertyTypeString, Immutable: true},
		},
	}
	m := &models.Model{
		Name:      "M",
		IsSpatial: true,
		Agents:    []models.Agent{*arch},
		SimulationConfig: models.SimulationConfig{
			LLMSimulationExecutionConfig: models.LLMSimulationExecutionConfig{
				LLMConfig: models.LLMConfig{PromptContext: "EXEC_CTX_MARKER"},
			},
		},
	}
	ra := &RuntimeAgent{
		A: &datasims.SimulationAgent{
			ID: 1, AgentUID: "wolf", State: "alive",
			Position:   &common.Point{X: 3, Y: 4},
			Properties: map[string]any{"energy": 50.0},
		},
		Archetype: arch,
	}
	group := &AgentGroup{
		ArchetypeUID: "wolf",
		Archetype:    arch,
		Members:      []*RuntimeAgent{ra},
		Strategy:     models.LLMStrategy{Type: models.LLMStrategyFull, UseForMovement: true, UseForProperties: true, UseForDecisions: true},
	}
	rs := &RuntimeState{
		Model: m,
		Sim:   &datasims.Simulation{Steps: 20},
		Step:  3,
		Env: &datasims.SimulationEnvironment{
			Width: 50, Height: 40, Torus: true,
			GlobalProperties: map[string]interface{}{"temperature": 21.5},
			Shapes: []datasims.SimulationShape{
				{ShapeUID: "lake", Name: "Lake", Type: "circle", Position: common.Point{X: 10, Y: 12}},
			},
		},
		Agents: []*RuntimeAgent{ra},
		ByID:   map[uint64]*RuntimeAgent{1: ra},
	}
	return rs, group
}

const runHeaderMarker = "RUNNING agent-based simulation"

func TestRuntimePrompts_UseRunHeader(t *testing.T) {
	rs, group := runtimePromptFixture()

	sys, _ := buildIndividualPrompt(rs, group, purposeMovement, group.Members)
	if !strings.Contains(sys, runHeaderMarker) || !strings.Contains(sys, strictJSONHeader) {
		t.Errorf("individual system prompt missing run header / strict-JSON:\n%s", sys)
	}
	if strings.Contains(sys, "You initialize") {
		t.Errorf("individual system prompt still uses the generation header")
	}

	sys, _ = buildArchetypePrompt(rs, []*AgentGroup{group}, purposeMovement, archKindFull, models.ArchetypeModeIdentical)
	if !strings.Contains(sys, runHeaderMarker) || !strings.Contains(sys, strictJSONHeader) {
		t.Errorf("archetype system prompt missing run header / strict-JSON:\n%s", sys)
	}

	in := models.Interaction{ID: "hunt", Name: "Hunt", UseLLM: true}
	sys, user := buildInteractionPrompt(rs, in, group.Members[0], group.Members[0])
	if !strings.Contains(sys, runHeaderMarker) {
		t.Errorf("interaction system prompt missing run header:\n%s", sys)
	}
	// B4: the builder must NOT append the JSON reminder — callJSON owns it.
	if strings.Contains(user, strings.TrimSpace(userJSONReminder)) {
		t.Errorf("interaction user message carries a builder-side JSON reminder (double reminder):\n%s", user)
	}
}

func TestBuildSharedRuntimeBlocks(t *testing.T) {
	rs, group := runtimePromptFixture()

	blocks := buildSharedRuntimeBlocks(rs)
	for _, want := range []string{
		"Step 3 of 20",
		"World: 50×40",
		"toroidal",
		"lake",             // shape uid listed
		"temperature=21.5", // global current value
	} {
		if !strings.Contains(blocks, want) {
			t.Errorf("shared blocks missing %q:\n%s", want, blocks)
		}
	}

	// Wired once into each runtime user message.
	_, user := buildIndividualPrompt(rs, group, purposeMovement, group.Members)
	if !strings.Contains(user, "Step 3 of 20") || !strings.Contains(user, "World: 50×40") {
		t.Errorf("individual user message missing shared blocks:\n%s", user)
	}
	_, user = buildArchetypePrompt(rs, []*AgentGroup{group}, purposeMovement, archKindFull, models.ArchetypeModeIdentical)
	if got := strings.Count(user, "World: 50×40"); got != 1 {
		t.Errorf("archetype user message must contain the world block exactly once, got %d:\n%s", got, user)
	}

	// Nil-safe + non-spatial: no world block, still a step line.
	rs.Model.IsSpatial = false
	blocks = buildSharedRuntimeBlocks(rs)
	if strings.Contains(blocks, "World:") {
		t.Errorf("non-spatial model must not get a world block:\n%s", blocks)
	}
	if buildSharedRuntimeBlocks(nil) != "" {
		t.Errorf("nil rs must yield empty blocks")
	}
}

func TestShapePrompt_ContextAndSchema(t *testing.T) {
	rs, _ := runtimePromptFixture()
	rs.Model.LLMConfig.PromptContext = "MODELGEN_CTX_MARKER" // must NOT appear (B3)

	ms := &models.Shape{
		UID: "lake", Name: "Lake", Description: "A cold lake.",
		Properties: []models.AgentProperty{
			{UID: "level", Name: "Level", Type: models.PropertyTypeNumber, MinValue: f64ptr(0), MaxValue: f64ptr(10)},
			{UID: "kind", Name: "Kind", Type: models.PropertyTypeString, Immutable: true},
		},
	}
	g := &shapeGroup{ShapeUID: "lake", ModelShape: ms, Members: []*datasims.SimulationShape{&rs.Env.Shapes[0]}}

	sys, user := buildShapePrompt(rs, g)
	_ = sys
	for _, want := range []string{
		"Step 3 of 20",    // shared blocks (Task 3)
		"Lake",            // shape name
		"A cold lake.",    // shape description
		"level (number)",  // settable property schema line
		"EXEC_CTX_MARKER", // execution-config guidance (B3 fix)
	} {
		if !strings.Contains(user, want) {
			t.Errorf("shape user message missing %q:\n%s", want, user)
		}
	}
	if strings.Contains(user, "MODELGEN_CTX_MARKER") {
		t.Errorf("model-generation context leaked into shape prompt (B3):\n%s", user)
	}
	if strings.Contains(user, "kind (string)") {
		t.Errorf("immutable shape property must be excluded from the settable schema:\n%s", user)
	}
}

func TestPropertySchemaBlock_InPropertyPrompts(t *testing.T) {
	rs, group := runtimePromptFixture()

	// properties purpose → schema block present, immutable excluded.
	_, user := buildIndividualPrompt(rs, group, purposeProperties, group.Members)
	if !strings.Contains(user, "energy (number) [0..100]") {
		t.Errorf("individual properties prompt missing settable schema line:\n%s", user)
	}
	if !strings.Contains(user, "ENERGY_CTX_MARKER") {
		t.Errorf("per-property llm_prompt_context missing:\n%s", user)
	}
	if strings.Contains(user, "tag (string)") {
		t.Errorf("immutable property leaked into settable schema:\n%s", user)
	}

	// full-control decision purpose (actions may set_property) → block present.
	_, user = buildIndividualPrompt(rs, group, purposeDecision, group.Members)
	if !strings.Contains(user, "energy (number) [0..100]") {
		t.Errorf("full-control decision prompt missing settable schema:\n%s", user)
	}

	// movement purpose → no schema block.
	_, user = buildIndividualPrompt(rs, group, purposeMovement, group.Members)
	if strings.Contains(user, "Properties (settable") {
		t.Errorf("movement prompt must not carry the settable schema:\n%s", user)
	}

	// archetype builder, properties purpose.
	_, user = buildArchetypePrompt(rs, []*AgentGroup{group}, purposeProperties, archKindFull, models.ArchetypeModeIdentical)
	if !strings.Contains(user, "energy (number) [0..100]") {
		t.Errorf("archetype properties prompt missing settable schema:\n%s", user)
	}
}

func TestDecisionSpec_HintsAndEmits(t *testing.T) {
	rs, group := runtimePromptFixture()
	group.Strategy = models.LLMStrategy{Type: models.LLMStrategyDecision, UseForDecisions: true}
	rs.Model.Rules = []models.Rule{{
		ID: "r1", Enabled: true, AppliesTo: []string{"wolf"},
		Conditions: []models.Condition{{Type: models.ConditionTypeDecision, Value: "hunt"}},
		Actions:    []models.Action{{Type: models.ActionTypeMoveToward, TargetUID: "sheep"}},
	}}
	rs.Model.Events = []models.EventDefinition{{UID: "howl", Producers: []models.EventProducer{models.EventProducerLLM}}}

	sys, _ := buildIndividualPrompt(rs, group, purposeDecision, group.Members)
	if !strings.Contains(sys, "hunt (triggers: move_toward)") {
		t.Errorf("individual decision spec missing hinted label:\n%s", sys)
	}
	if !strings.Contains(sys, "emit_uid") || !strings.Contains(sys, "howl") {
		t.Errorf("individual decision spec missing emit advertisement:\n%s", sys)
	}

	sys, _ = buildArchetypePrompt(rs, []*AgentGroup{group}, purposeDecision, archKindDecision, models.ArchetypeModeDistribution)
	if !strings.Contains(sys, "hunt (triggers: move_toward)") || !strings.Contains(sys, "howl") {
		t.Errorf("archetype decision spec missing hints/emits:\n%s", sys)
	}
}

func TestFullSpecs_AdvertiseEmittableEvents(t *testing.T) {
	rs, group := runtimePromptFixture()
	rs.Model.Events = []models.EventDefinition{
		{UID: "howl", Producers: []models.EventProducer{models.EventProducerLLM}, LLMPromptContext: " wolf howl to rally the pack "},
		{UID: "rain", Producers: []models.EventProducer{models.EventProducerEnvironment}}, // not LLM-emittable
	}
	const wantLine = "Emittable events: howl (wolf howl to rally the pack).\n"

	// Individual full-control decision spec (fixture strategy is full + UseForDecisions).
	sys, _ := buildIndividualPrompt(rs, group, purposeDecision, group.Members)
	if !strings.Contains(sys, wantLine) {
		t.Errorf("individual full spec missing emittable-events line:\n%s", sys)
	}
	if strings.Contains(sys, "rain") {
		t.Errorf("non-LLM event leaked into the advertisement:\n%s", sys)
	}

	// Archetype full-control specs, both modes.
	batch := []*AgentGroup{group}
	for _, mode := range []models.ArchetypeMode{models.ArchetypeModeIdentical, models.ArchetypeModeDistribution} {
		spec := archetypeSystemSpec(purposeDecision, archKindFull, mode, rs, batch)
		if !strings.Contains(spec, wantLine) {
			t.Errorf("archetype full spec (mode=%s) missing emittable-events line:\n%s", mode, spec)
		}
	}

	// No emittable events → no line anywhere.
	rs.Model.Events = nil
	sys, _ = buildIndividualPrompt(rs, group, purposeDecision, group.Members)
	if strings.Contains(sys, "Emittable events") {
		t.Errorf("individual full spec must omit the line without emittable events:\n%s", sys)
	}
	spec := archetypeSystemSpec(purposeDecision, archKindFull, models.ArchetypeModeIdentical, rs, batch)
	if strings.Contains(spec, "Emittable events") {
		t.Errorf("archetype full spec must omit the line without emittable events:\n%s", spec)
	}
}

func TestDecisionSpecs_EmitAdvertisementCarriesContext(t *testing.T) {
	rs, group := runtimePromptFixture()
	group.Strategy = models.LLMStrategy{Type: models.LLMStrategyDecision, UseForDecisions: true}
	rs.Model.Events = []models.EventDefinition{
		{UID: "howl", Producers: []models.EventProducer{models.EventProducerLLM}, LLMPromptContext: "wolf howl to rally the pack"},
	}
	const wantLabel = "howl (wolf howl to rally the pack)"

	sys, _ := buildIndividualPrompt(rs, group, purposeDecision, group.Members)
	if !strings.Contains(sys, wantLabel) {
		t.Errorf("individual decision spec missing enriched emit label:\n%s", sys)
	}

	batch := []*AgentGroup{group}
	for _, mode := range []models.ArchetypeMode{models.ArchetypeModeIdentical, models.ArchetypeModeDistribution} {
		spec := archetypeSystemSpec(purposeDecision, archKindDecision, mode, rs, batch)
		if !strings.Contains(spec, wantLabel) {
			t.Errorf("archetype decision spec (mode=%s) missing enriched emit label:\n%s", mode, spec)
		}
	}
}

func TestArchetypeDistDecisionSpec_EmitPerChoiceWording(t *testing.T) {
	// The distribution parser reads emit_uid/emit_data PER CHOICE ENTRY
	// (archetypeDecisionChoice); the advertisement must say so.
	rs, group := runtimePromptFixture()
	group.Strategy = models.LLMStrategy{Type: models.LLMStrategyDecision, UseForDecisions: true}
	rs.Model.Events = []models.EventDefinition{
		{UID: "howl", Producers: []models.EventProducer{models.EventProducerLLM}},
	}
	batch := []*AgentGroup{group}

	dist := archetypeSystemSpec(purposeDecision, archKindDecision, models.ArchetypeModeDistribution, rs, batch)
	if !strings.Contains(dist, "choice entry") {
		t.Errorf("distribution decision spec must say the emit fields go on each choice entry:\n%s", dist)
	}
	ident := archetypeSystemSpec(purposeDecision, archKindDecision, models.ArchetypeModeIdentical, rs, batch)
	if strings.Contains(ident, "choice entry") {
		t.Errorf("identical-mode decision spec must not mention choice entries:\n%s", ident)
	}
}

func TestRuntimeSpecs_ExamplesParse(t *testing.T) {
	rs, group := runtimePromptFixture()

	// Individual specs.
	cases := []struct {
		p     purpose
		strat models.LLMStrategy
		into  any
	}{
		{purposeProperties, group.Strategy, &propsResp{}},
		{purposeMovement, group.Strategy, &movementResp{}},
		{purposeDecision, models.LLMStrategy{Type: models.LLMStrategyDecision}, &decisionResp{}},
		{purposeDecision, models.LLMStrategy{Type: models.LLMStrategyFull}, &fullDecisionResp{}},
	}
	for _, c := range cases {
		spec := purposeSystemSpec(c.p, c.strat, nil, nil)
		assertParses(t, extractJSONExample(t, spec), c.into)
	}
	// Movement semantics present.
	mv := purposeSystemSpec(purposeMovement, group.Strategy, nil, nil)
	if !strings.Contains(mv, "NEAREST") || !strings.Contains(mv, "x,y") {
		t.Errorf("movement spec missing target_ref semantics:\n%s", mv)
	}

	// Archetype specs (identical + distribution, both kinds).
	batch := []*AgentGroup{group}
	type archCase struct {
		p    purpose
		kind archetypeKind
		mode models.ArchetypeMode
		into any
	}
	archCases := []archCase{
		{purposeProperties, archKindFull, models.ArchetypeModeIdentical, &propsResp{}},
		{purposeMovement, archKindFull, models.ArchetypeModeIdentical, &movementResp{}},
		{purposeDecision, archKindDecision, models.ArchetypeModeIdentical, &decisionResp{}},
		{purposeDecision, archKindDecision, models.ArchetypeModeDistribution, &decisionDistResp{}},
		{purposeDecision, archKindFull, models.ArchetypeModeIdentical, &fullArchetypeResp{}},
		{purposeDecision, archKindFull, models.ArchetypeModeDistribution, &fullArchetypeDistResp{}},
	}
	for _, c := range archCases {
		spec := archetypeSystemSpec(c.p, c.kind, c.mode, rs, batch)
		assertParses(t, extractJSONExample(t, spec), c.into)
	}
}

// modifierVocabulary is the exact validModifier vocabulary (decision.go); the
// full-control specs must enumerate it so the LLM never invents a numeric modifier.
const modifierVocabulary = "set|add|subtract|multiply|divide|increment|decrement|toggle"

func TestFullControlSpecs_EnumerateModifiers(t *testing.T) {
	rs, group := runtimePromptFixture()

	spec := purposeSystemSpec(purposeDecision, models.LLMStrategy{Type: models.LLMStrategyFull}, nil, nil)
	if !strings.Contains(spec, modifierVocabulary) {
		t.Errorf("individual full-control spec missing modifier vocabulary:\n%s", spec)
	}

	batch := []*AgentGroup{group}
	for _, mode := range []models.ArchetypeMode{models.ArchetypeModeIdentical, models.ArchetypeModeDistribution} {
		spec := archetypeSystemSpec(purposeDecision, archKindFull, mode, rs, batch)
		if !strings.Contains(spec, modifierVocabulary) {
			t.Errorf("archetype full-control spec (mode=%s) missing modifier vocabulary:\n%s", mode, spec)
		}
	}
}

func TestWriteAgentLine_JSONProperties(t *testing.T) {
	rs, group := runtimePromptFixture()
	group.Members[0].A.Properties = map[string]any{"energy": 50.0, "mood": "calm"}
	_, user := buildIndividualPrompt(rs, group, purposeMovement, group.Members)
	if !strings.Contains(user, `properties={"energy":50,"mood":"calm"}`) {
		t.Errorf("agent line must render properties as canonical JSON:\n%s", user)
	}
	if strings.Contains(user, "map[") {
		t.Errorf("Go map formatting leaked into the prompt:\n%s", user)
	}
}

func TestArchetypeAggregateBlock(t *testing.T) {
	rs, group := runtimePromptFixture()
	// Second member to make the stats non-trivial.
	ra2 := &RuntimeAgent{
		A: &datasims.SimulationAgent{
			ID: 2, AgentUID: "wolf", State: "hungry",
			Position:   &common.Point{X: 7, Y: 8},
			Properties: map[string]any{"energy": 30.0},
		},
		Archetype: group.Archetype,
	}
	group.Members = append(group.Members, ra2)

	block := archetypeAggregateBlock(rs, group)
	for _, want := range []string{
		"2 live",
		"alive=1", "hungry=1",
		"energy mean=40 min=30 max=50",
		"centroid=(5,6)",
	} {
		if !strings.Contains(block, want) {
			t.Errorf("aggregate block missing %q:\n%s", want, block)
		}
	}

	// Wired into the archetype user message; per-member memory dumps KEPT.
	_, user := buildArchetypePrompt(rs, []*AgentGroup{group}, purposeMovement, archKindFull, models.ArchetypeModeIdentical)
	if !strings.Contains(user, "Population stats:") {
		t.Errorf("archetype prompt missing aggregate block:\n%s", user)
	}
}

func TestShapePrompt_UsesRunHeader(t *testing.T) {
	rs, _ := runtimePromptFixture()
	ms := &models.Shape{UID: "lake", Name: "Lake", Type: models.ShapeTypeCircle}
	g := &shapeGroup{ShapeUID: "lake", ModelShape: ms, Members: []*datasims.SimulationShape{&rs.Env.Shapes[0]}}
	sys, _ := buildShapePrompt(rs, g)
	if !strings.Contains(sys, runHeaderMarker) || !strings.Contains(sys, strictJSONHeader) {
		t.Errorf("shape system prompt missing run header / strict-JSON:\n%s", sys)
	}
}

func TestInteractionPrompt_Enriched(t *testing.T) {
	rs, group := runtimePromptFixture()
	tgt := &RuntimeAgent{
		A: &datasims.SimulationAgent{
			ID: 2, AgentUID: "sheep", State: "grazing",
			Position:   &common.Point{X: 6, Y: 8},
			Properties: map[string]any{"health": 80.0},
		},
	}
	in := models.Interaction{
		ID: "hunt", Name: "Hunt", Description: "Wolf hunts sheep.",
		InitiatorActions: []models.Action{{Type: models.ActionTypeModifyProperty, PropertyUID: "energy", Modifier: models.PropertyModifierAdd, Value: 10.0}},
		TargetActions:    []models.Action{{Type: models.ActionTypeRemoveAgent}},
	}
	_, user := buildInteractionPrompt(rs, in, group.Members[0], tgt)
	for _, want := range []string{
		"pos=(3,4)",                  // initiator position
		"pos=(6,8)",                  // target position
		`{"energy":50}`,              // initiator properties (canonical JSON)
		`{"health":80}`,              // target properties
		"Distance:",                  // body distance line
		"initiator gets [modify_property]",
		"target gets [remove_agent]",
	} {
		if !strings.Contains(user, want) {
			t.Errorf("interaction user message missing %q:\n%s", want, user)
		}
	}
}
