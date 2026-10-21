package models

import (
	"encoding/json"
	"strings"

	"intelli.sim.core/data/common"
	datamodels "intelli.sim.core/data/models"
)

// systemPromptHeader frames the model-generation role (Level-1 of the
// meta-system) and the STRICT output contract. It is scope-independent.
const systemPromptHeader = `You are the model-generation engine (Level 1) of an LLM-driven meta-system for ` +
	`agent-based modeling (ABM). Your job is to translate the researcher's natural-language ` +
	`description into a structured ABM model: agents (and archetypes) with typed properties, ` +
	`behavior rules made of conditions and actions, agent-agent / agent-shape / agent-global ` +
	`interactions, model-level events, dashboard widgets, and (for spatial models) an ` +
	`environment with shapes.

OUTPUT CONTRACT (STRICT):
- Return ONLY a single JSON object matching the schema below. No prose, no markdown, no code ` +
	"fences" + ` — nothing outside the JSON.
- The top-level object is the envelope: {"display_message": "...", "model": { ... }}.
- "display_message" is a short, friendly summary of what you changed, shown to the user. ` +
	`Write it in the researcher's language (the language of their latest message).
- "model" contains only the structural model. Never emit ids, timestamps, ownership, ` +
	`llm_config, targets, strategies, locked, or immutable — those are user-owned and ignored.

REFERENTIAL INTEGRITY:
- Every entity has a stable identifier: agents/shapes/events use "uid", rules/interactions use ` +
	`"id". Reuse the EXACT existing identifier when editing an entity; invent a new, descriptive, ` +
	`lowercase snake_case identifier for a new one.
- "environment" is a reserved uid; never use it for an agent, shape, or property.
- References (rules.applies_to, condition/action target_uid(s), interaction initiator/target uids, ` +
	`event producer/consumer uids, action event_uid / spawn_agent_uid) MUST resolve to a uid/id that ` +
	`exists in the model.
- Editing is incremental at BOTH levels. Entities: return only the entities you add or change; ` +
	`entities you omit are kept as-is — do NOT re-emit unchanged entities, and never attempt to ` +
	`delete by omission. Fields: within an entity you are changing, emit ONLY the fields whose ` +
	`values change (always include its "uid"/"id" so it can be matched); omitted fields keep their ` +
	`current values. To CLEAR a list (e.g. remove all conditions) emit it as an empty array []; ` +
	`omitting the list keeps it unchanged.
`

// envelopeContract restates the exact wrapper shape plus a compact few-shot so the
// model anchors on the envelope rather than emitting a bare model.
const envelopeContract = `
## ENVELOPE (return EXACTLY this shape)
{"display_message": "<short summary>", "model": { <only the in-scope sections> }}

## EXAMPLE
{"display_message":"Added a prey agent that reproduces.","model":{"agents":[{"uid":"prey","name":"Prey","type":"agent","move_type":"movable","initialization":{"population_count":40,"initial_state":"alive"},"states":["alive"]}]}}
`

// buildSystemPrompt assembles the model-generation system prompt for the given
// scope: role + framing + STRICT JSON rules, a scope-trimmed schema description,
// the current in-scope model slice (so the LLM edits against real values), a list
// of FROZEN (locked) entities, the scope instruction, and the envelope contract.
func buildSystemPrompt(existing datamodels.Model, scope EditScope) string {
	var b strings.Builder
	b.WriteString(systemPromptHeader)
	b.WriteString(schemaDocFor(scope))

	if scope.Section == "" || scope.Section == "rules" || scope.Section == "interactions" {
		b.WriteString(schemaDocSemantics)
	}

	if hasExisting(existing) {
		if slice := currentSliceJSON(existing, scope); slice != "" {
			b.WriteString("\n## CURRENT MODEL (baseline to edit)\n")
			b.WriteString(slice)
			b.WriteString("\n")
		}
	}

	if locked := lockedSummary(existing, scope); locked != "" {
		b.WriteString("\n## FROZEN ENTITIES — do NOT modify, rename, or remove these:\n")
		b.WriteString(locked)
	}

	b.WriteString(researcherGuidance(existing.LLMConfig.PromptContext))
	b.WriteString(scopeInstruction(scope))
	b.WriteString(envelopeContract)
	return b.String()
}

// buildVisionSystemPrompt frames an image->environment extraction. It asks for ONLY
// the environment section (shapes with grid coordinates + a color property).
func buildVisionSystemPrompt(env datamodels.Environment, promptContext string) string {
	var b strings.Builder
	b.WriteString(`You are the environment-extraction engine (Level 1, vision) of an LLM-driven ` +
		`agent-based-modeling meta-system. Analyse the supplied reference image(s) of a spatial ` +
		`layout and produce ONLY the model environment.

When MULTIPLE images are supplied, each is preceded by a text label (e.g. ` +
		`"Reference image 2 — obstacles: ..."). Treat the images as complementary views/layers of ONE ` +
		`environment (e.g. a base map plus an obstacle or zone overlay) and SYNTHESIZE a single ` +
		`coherent layout from all of them — do not emit a separate environment per image.

OUTPUT CONTRACT (STRICT):
- Return ONLY the envelope JSON {"display_message":"...","model":{"environment":{ ... }}}. No prose ` +
		"outside the JSON.\n" +
		`- Describe the layout as environment.shapes: rectangle/circle/line/polygon templates.
- Coordinates are integer GRID cells with origin (0,0) at the TOP-LEFT, x increasing right, y ` +
		`increasing down. Use geometry.position for the fixed anchor and a population_count of 1 for ` +
		`fixed shapes.
- Encode each shape's colour as a property with uid "color", type "color", and a hex value like ` +
		`"#3366CC".
- Give every shape a descriptive lowercase snake_case uid.
`)
	b.WriteString(schemaDocEnvironment)
	if env.Width > 0 || env.Height > 0 {
		b.WriteString("\n## TARGET GRID\n")
		b.WriteString(jsonBlock(map[string]uint64{"width": env.Width, "height": env.Height}))
		b.WriteString("\n")
	}
	b.WriteString(researcherGuidance(promptContext))
	b.WriteString(envelopeContract)
	return b.String()
}

// researcherGuidance renders the user-authored model-generation context
// (Model.LLMConfig.PromptContext) as a follow-it section, or "" when the
// context is empty/whitespace.
func researcherGuidance(promptContext string) string {
	ctx := strings.TrimSpace(promptContext)
	if ctx == "" {
		return ""
	}
	return "\n## RESEARCHER GUIDANCE (user-authored, follow it)\n" + ctx + "\n"
}

// ===================== schema docs =====================

// schemaDocFor returns a concise, human-readable schema description derived from
// the LLM* contract, trimmed to the active scope. Kept in lockstep with
// llmcontract.go.
func schemaDocFor(scope EditScope) string {
	var b strings.Builder
	b.WriteString("\n## SCHEMA (model object — emit ONLY the sections listed here)\n")

	include := func(section string) bool {
		return scope.Section == "" || scope.Section == section
	}
	if scope.Section == "" {
		b.WriteString(schemaDocModelScalars)
	}
	if include("agents") {
		b.WriteString(schemaDocAgents)
	}
	if include("environment") {
		b.WriteString(schemaDocEnvironment)
	}
	if include("rules") {
		b.WriteString(schemaDocRules)
	}
	if include("interactions") {
		b.WriteString(schemaDocInteractions)
	}
	if include("events") {
		b.WriteString(schemaDocEvents)
	}
	if include("widgets") {
		b.WriteString(schemaDocWidgets)
	}
	return b.String()
}

const schemaDocModelScalars = `model: { description?, is_spatial? (bool), termination?, agents[], environment?, rules[], interactions[], events[], widgets[] }
  termination is the RUN-STOP condition: the simulation ends as soon as it holds. It is a condition
  object restricted to types population | step | global_property | composite (same shape as rule
  conditions). When the user asks for the simulation to stop on some state, author it here — e.g.
  "stop when everyone has left": {"type":"population","target_uid":"<agent uid>","operator":"eq","value":0}.
  NEVER express stopping as a rule — rules cannot stop the run.
`

const schemaDocAgents = `agents[]: { uid, name, description?, type ("agent"|"archetype"), move_type ("static"|"movable")?,
  properties[]: { uid, name, type ("string"|"number"|"enum"|"boolean"|"color"), description?, min_value?, max_value?,
    default_value?, allowed_values?, llm_prompt_context?,
    generation?: { mode ("default"|"random"|"llm"), distribution ("uniform"|"normal"|"choice")? (mode "random" only),
      mean? + std_dev? (>0) (normal; number properties only), min?, max? (init-range override; min < max),
      choices?: [{value, weight (>0)}] (choice; values must fit the property type), prompt_context? (llm-mode hint) } },
  states[]?, initialization: { population_count (>=1), min_population?, max_population?, initial_state? (one of states),
    count_mode? ("fixed"|"random_range"|"llm"; random_range draws from [min_population..max_population]),
    placement?: { type ("random"|"uniform"|"clustered"|"at_shape"|"manual"|"llm"), shape_uid?, prompt_context?, parameters? } } }
`

const schemaDocEnvironment = `environment: { width, height, torus (bool), collision_mode? ("none"|"block"), llm_prompt_context?,
  global_properties[]: (same shape as agent properties),
  shapes[]: { uid, name, description?, type ("rectangle"|"circle"|"line"|"polygon"), layer, visible (bool),
    properties[]: (same as agent properties; encode colour as uid "color", type "color", hex value),
    initialization: (same as agent initialization),
    geometry?: { position?: {x,y}, width?: {value|min,max}, height?: {...}, radius?: {...}, points?: [{x,y}],
      thickness?, fill?, stroke?, stroke_width? } } }
  geometry width/height/radius are OBJECTS — {"value":n} fixed or {"min":a,"max":b} ranged — never a bare number.
  ALL spatial dimensions and coordinates (environment width/height, shape geometry, positions, points) are
  unitless INTEGER grid cells — whole numbers only, never fractional or real-world units. When the user gives
  real-world sizes (meters), pick a grid granularity yourself (e.g. 1 cell = 0.5 m) so the smallest feature
  (a door, a wall thickness) is at least 1 cell, and express EVERY dimension in those cells consistently.
  collision_mode "block" makes agents and shapes SOLID: agents cannot pass through each other or through any
  FILLED rectangle/circle/polygon shape (lines are open). Use it when the user says things cannot pass through
  walls or each other. With "block", never cover walkable floor area with a filled shape (no room-sized
  background rectangles — agents could not move); author walls as thin filled segments around the walkable space.
  A shape with "fill": false is PASSABLE (visual outline only) — author doors, exits and sensor zones as
  unfilled shapes placed in a gap between wall segments; agents walk through them, and a rule condition
  {"type":"position","operator":"in","target_uid":"<shape uid>"} is true while an agent is inside one
  (use it e.g. to remove agents that reach an exit).
`

const schemaDocRules = `rules[]: { id, name, description?, priority (int), applies_to[] (agent uids),
  conditions[]: { type (one of: global_property, property, position, proximity, step, composite, count, barrier, state,
    distance, decision, random, memory, event, custom), property_uid?, operator ("eq"|"neq"|"gt"|"lt"|"gte"|"lte"|
    "contains"|"in_range"|"in"|"not_in")?, value?, min_value?, max_value?, radius?, target_uid?, target_uids?,
    logic_operator ("AND"|"OR"|"NOT")?, sub_conditions[]?, parameters?, description? },
  actions[]: { type (one of: set_property, modify_property, move, move_toward, move_away, interact, emit_event,
    transition, call_llm, spawn_agent, remove_agent, transform, teleport, custom), property_uid?, value?,
    modifier ("set"|"add"|"subtract"|"multiply"|"divide"|"increment"|"decrement"|"toggle")?, target_uid?, target_uids?,
    speed?, direction?, event_uid?, event_data?, new_state?, spawn_agent_uid?, spawn_count?, spawn_properties?,
    spawn_position?: { type ("same"|"random"|"offset"|"nearby"|"at_shape"), offset_x?, offset_y?, radius?, shape_uid? },
    spawn_inherit_from?, parameters?, description?, llm_prompt_context? },
  execute_once (bool), cooldown_steps (int), enabled (bool) }
`

const schemaDocInteractions = `interactions[]: { id, name, description?, type ("agent_agent"|"agent_shape"|"agent_global"),
  initiator_uids[], target_uids[], radius?, conditions[] (as in rules), initiator_actions[] (as in rules),
  target_actions[] (as in rules), bidirectional (bool), use_llm (bool), enabled (bool), llm_prompt_context? }
  initiator_actions apply to the INITIATOR, target_actions to the TARGET (e.g. when a predator consumes prey, remove_agent belongs in target_actions).
`

const schemaDocEvents = `events[]: { uid, name, description?, scope ("global"|"local"|"targeted"), radius?,
  payload[]: (same as agent properties), producers[] ("agent"|"environment"|"llm"), producer_uids?, consumer_uids?,
  ttl_steps, llm_prompt_context? }
`

const schemaDocWidgets = `widgets[]: { id, type ("monitor"|"line_chart"|"bar_chart"|"pie_chart"|"histogram"|"agent_table"|"text_log"), title,
  source?: { type ("agent_property"|"global_property"|"population"|"state_distribution"|"sim_metric"), property_uid?, agent_uids?,
    aggregation ("avg"|"sum"|"min"|"max"|"count"|"median")?,
    metric ("total_agents"|"agents_spawned"|"agents_removed"|"interactions"|"events"|"rule_executions"|"movement_distance")? },
  series?: [{ label, color?, source }], columns? (property uids), buckets? (>=2),
  log_kinds? ("events"|"state_transitions"|"llm_decisions"), position: { x, y, w, h } (dashboard grid cells) }
Widget/source matrix: monitor = one source (agent_property|global_property|population|sim_metric). line_chart = series only
(each series source may be agent_property|global_property|population|sim_metric).
bar_chart = series OR one source (population|state_distribution). pie_chart = one source (population|state_distribution).
histogram = one agent_property source with property_uid and NO aggregation. agent_table = agent_property source with
exactly 1 agent_uid (no property_uid/aggregation) and columns listing that agent's property uids. text_log = log_kinds only.
agent_property sources require property_uid + aggregation; global_property requires property_uid (no agent_uids/aggregation);
population takes an optional agent_uids filter; state_distribution requires exactly 1 agent_uid.
sim_metric requires metric and takes NO property_uid/agent_uids/aggregation; metrics are cumulative built-in engine run
counters (movement_distance = total distance moved by all agents). sim_metric is allowed ONLY in monitor and line_chart series.
`

// schemaDocSemantics clarifies the non-obvious condition/action types so
// generated rules use them correctly. Included whenever rules or interactions
// are in scope. Keep terse — one line per family. Verified against the engine
// (modules/simulations/rules.go): count/barrier/transform corrected to match
// the actual evaluation/application code.
const schemaDocSemantics = `## SEMANTICS (non-obvious types)
Conditions: proximity = another agent (or a named target_uid shape) within radius; count = number of nearby agents within radius compared to value; barrier = the agent overlaps a shape (any, or a named target_uid) or touches a bounded (non-torus) world edge; memory = a recent agent-memory entry matches; decision = the agent's last LLM decision label equals value; composite = AND/OR/NOT over sub_conditions; random = fires with probability value (0..1).
Actions: interact = force-fire the interaction whose id is target_uid; call_llm = request an on-demand LLM decision next step (agent needs a decision-capable strategy); transform = turn the agent into archetype spawn_agent_uid (or target_uid): a replacement agent of the target archetype is created at this agent's position (spawn_properties/spawn_inherit_from carry values over) and the ORIGINAL agent is removed the same step; teleport = jump to a position; emit_event requires the event's producers to include "agent"; set_property/modify_property may target an environment global via target_uid "environment".
`

// ===================== current slice + locks =====================

// currentSliceJSON marshals the contract projection of the in-scope slice of the
// current model so the LLM edits against the real current values. Returns "" if
// the projection is empty.
func currentSliceJSON(existing datamodels.Model, scope EditScope) string {
	m := projectToContract(existing, scope)
	if m == nil || isEmptyLLMModel(m) {
		return ""
	}
	return jsonBlock(m)
}

// projectToContract builds an LLMModel containing only the in-scope sections of
// the current model (further narrowed to scope.UID when set), so the prompt shows
// the LLM exactly the slice it is allowed to edit.
func projectToContract(existing datamodels.Model, scope EditScope) *LLMModel {
	out := &LLMModel{}
	whole := scope.Section == ""

	include := func(section string) bool { return whole || scope.Section == section }
	matchUID := func(uid string) bool { return scope.UID == "" || scope.UID == uid }

	if whole {
		out.Description = existing.Description
		out.IsSpatial = llmBPtr(existing.IsSpatial)
		if existing.SimulationConfig.Termination != nil {
			t := conditionsToLLM([]datamodels.Condition{*existing.SimulationConfig.Termination})[0]
			out.Termination = &t
		}
	}
	if include("agents") {
		for _, a := range existing.Agents {
			if !matchUID(a.UID) {
				continue
			}
			out.Agents = append(out.Agents, agentToLLM(a))
		}
	}
	if include("environment") && existing.Environment != nil {
		out.Environment = environmentToLLM(*existing.Environment, scope.UID)
	}
	if include("rules") {
		for _, r := range existing.Rules {
			if !matchUID(r.ID) {
				continue
			}
			out.Rules = append(out.Rules, ruleToLLM(r))
		}
	}
	if include("interactions") {
		for _, in := range existing.Interactions {
			if !matchUID(in.ID) {
				continue
			}
			out.Interactions = append(out.Interactions, interactionToLLM(in))
		}
	}
	if include("events") {
		for _, e := range existing.Events {
			if !matchUID(e.UID) {
				continue
			}
			out.Events = append(out.Events, eventToLLM(e))
		}
	}
	if include("widgets") {
		for _, w := range existing.Widgets {
			if !matchUID(w.ID) {
				continue
			}
			out.Widgets = append(out.Widgets, widgetToLLM(w))
		}
	}
	return out
}

// lockedSummary lists the FROZEN (locked) entities relevant to the scope, as a
// compact bullet list of "<type> <id> (\"name\")". Returns "" when none apply.
func lockedSummary(existing datamodels.Model, scope EditScope) string {
	whole := scope.Section == ""
	include := func(section string) bool { return whole || scope.Section == section }
	var lines []string

	if include("agents") {
		for _, a := range existing.Agents {
			if a.Locked {
				lines = append(lines, lockedLine("agent", a.UID, a.Name))
			}
		}
	}
	if include("environment") && existing.Environment != nil {
		for _, s := range existing.Environment.Shapes {
			if s.Locked {
				lines = append(lines, lockedLine("shape", s.UID, s.Name))
			}
		}
	}
	if include("rules") {
		for _, r := range existing.Rules {
			if r.Locked {
				lines = append(lines, lockedLine("rule", r.ID, r.Name))
			}
		}
	}
	if include("interactions") {
		for _, in := range existing.Interactions {
			if in.Locked {
				lines = append(lines, lockedLine("interaction", in.ID, in.Name))
			}
		}
	}
	if include("events") {
		for _, e := range existing.Events {
			if e.Locked {
				lines = append(lines, lockedLine("event", e.UID, e.Name))
			}
		}
	}
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "\n") + "\n"
}

func lockedLine(kind, id, name string) string {
	if name != "" {
		return "- " + kind + " " + id + " (\"" + name + "\")"
	}
	return "- " + kind + " " + id
}

// scopeInstruction tells the model what subset of the envelope to populate.
func scopeInstruction(scope EditScope) string {
	switch {
	case scope.Section == "":
		return "\n## SCOPE\nYou may edit any section of the model. Return only the entities you add or change.\n"
	case scope.UID != "":
		return "\n## SCOPE\nEdit ONLY the " + scope.Section + " entity \"" + scope.UID +
			"\". Return only the \"" + scope.Section + "\" array containing just that one entity; emit no other section.\n"
	default:
		return "\n## SCOPE\nEdit ONLY the \"" + scope.Section + "\" section. Return only the \"" +
			scope.Section + "\" array; emit no other section.\n"
	}
}

// ===================== domain -> contract projection helpers =====================

func agentToLLM(a datamodels.Agent) LLMAgent {
	return LLMAgent{
		UID:            a.UID,
		Name:           a.Name,
		Description:    a.Description,
		Type:           string(a.Type),
		MoveType:       string(a.MoveType),
		Properties:     propertiesToLLM(a.Properties),
		States:         a.States,
		Initialization: initializationToLLM(a.Initialization),
	}
}

func propertiesToLLM(in []datamodels.AgentProperty) []LLMAgentProperty {
	if in == nil {
		return nil
	}
	out := make([]LLMAgentProperty, 0, len(in))
	for _, p := range in {
		out = append(out, LLMAgentProperty{
			UID:              p.UID,
			Name:             p.Name,
			Type:             string(p.Type),
			Description:      p.Description,
			MinValue:         p.MinValue,
			MaxValue:         p.MaxValue,
			DefaultValue:     p.DefaultValue,
			AllowedValues:    p.AllowedValues,
			LLMPromptContext: p.LLMPromptContext,
			Generation:       generationToLLM(p.Generation),
		})
	}
	return out
}

func generationToLLM(g *datamodels.PropertyGeneration) *LLMPropertyGeneration {
	if g == nil {
		return nil
	}
	out := &LLMPropertyGeneration{
		Mode:          string(g.Mode),
		Distribution:  string(g.Distribution),
		Mean:          g.Mean,
		StdDev:        g.StdDev,
		Min:           g.Min,
		Max:           g.Max,
		PromptContext: g.PromptContext,
	}
	for _, c := range g.Choices {
		out.Choices = append(out.Choices, LLMChoiceWeight{Value: c.Value, Weight: c.Weight})
	}
	return out
}

func initializationToLLM(in datamodels.AgentInitialization) LLMAgentInitialization {
	out := LLMAgentInitialization{
		PopulationCount: llmU64Ptr(in.PopulationCount),
		MinPopulation:   llmU64From(in.MinPopulation),
		MaxPopulation:   llmU64From(in.MaxPopulation),
		InitialState:    in.InitialState,
		CountMode:       string(in.CountMode),
	}
	if in.Placement != nil {
		out.Placement = &LLMPlacement{
			Type:          string(in.Placement.Type),
			ShapeUID:      in.Placement.ShapeUID,
			PromptContext: in.Placement.PromptContext,
			Clusters:      llmU64From(in.Placement.Clusters),
			ClusterRadius: llmU64From(in.Placement.ClusterRadius),
		}
	}
	return out
}

func environmentToLLM(env datamodels.Environment, shapeUID string) *LLMEnvironment {
	out := &LLMEnvironment{
		UID:              env.UID,
		Width:            llmU64Ptr(env.Width),
		Height:           llmU64Ptr(env.Height),
		Torus:            llmBPtr(env.Torus),
		CollisionMode:    string(env.CollisionMode),
		GlobalProperties: propertiesToLLM(env.GlobalProperties),
		LLMPromptContext: env.LLMPromptContext,
	}
	for _, s := range env.Shapes {
		if shapeUID != "" && s.UID != shapeUID {
			continue
		}
		out.Shapes = append(out.Shapes, shapeToLLM(s))
	}
	return out
}

func shapeToLLM(s datamodels.Shape) LLMShape {
	out := LLMShape{
		UID:            s.UID,
		Name:           s.Name,
		Description:    s.Description,
		Type:           string(s.Type),
		Layer:          llmUPtr(s.Layer),
		Visible:        llmBPtr(s.Visible),
		Properties:     propertiesToLLM(s.Properties),
		Initialization: initializationToLLM(s.Initialization),
	}
	if s.Geometry != nil {
		g := &LLMShapeGeometry{
			Width:       scalarSpecToLLM(s.Geometry.Width),
			Height:      scalarSpecToLLM(s.Geometry.Height),
			Radius:      scalarSpecToLLM(s.Geometry.Radius),
			Thickness:   llmUFrom(s.Geometry.Thickness),
			Fill:        llmBFrom(s.Geometry.Fill),
			Stroke:      llmBFrom(s.Geometry.Stroke),
			StrokeWidth: llmUFrom(s.Geometry.StrokeWidth),
		}
		if s.Geometry.Position != nil {
			g.Position = &LLMPoint{X: llmUint64(s.Geometry.Position.X), Y: llmUint64(s.Geometry.Position.Y)}
		}
		for _, p := range s.Geometry.Points {
			g.Points = append(g.Points, LLMPoint{X: llmUint64(p.X), Y: llmUint64(p.Y)})
		}
		out.Geometry = g
	}
	return out
}

func scalarSpecToLLM(in *datamodels.ScalarSpec) *LLMScalarSpec {
	if in == nil {
		return nil
	}
	return &LLMScalarSpec{Value: llmU64From(in.Value), Min: llmU64From(in.Min), Max: llmU64From(in.Max)}
}

func ruleToLLM(r datamodels.Rule) LLMRule {
	return LLMRule{
		ID:            r.ID,
		Name:          r.Name,
		Description:   r.Description,
		Priority:      llmIPtr(r.Priority),
		AppliesTo:     r.AppliesTo,
		Conditions:    conditionsToLLM(r.Conditions),
		Actions:       actionsToLLM(r.Actions),
		ExecuteOnce:   llmBPtr(r.ExecuteOnce),
		CooldownSteps: llmIPtr(r.CooldownSteps),
		Enabled:       llmBPtr(r.Enabled),
	}
}

func conditionsToLLM(in []datamodels.Condition) []LLMCondition {
	if in == nil {
		return nil
	}
	out := make([]LLMCondition, 0, len(in))
	for _, c := range in {
		out = append(out, LLMCondition{
			Type:          string(c.Type),
			PropertyUID:   c.PropertyUID,
			Operator:      string(c.Operator),
			Value:         c.Value,
			MinValue:      c.MinValue,
			MaxValue:      c.MaxValue,
			Radius:        c.Radius,
			TargetUID:     c.TargetUID,
			TargetUIDs:    c.TargetUIDs,
			LogicOperator: string(c.LogicOperator),
			SubConditions: conditionsToLLM(c.SubConditions),
			Parameters:    c.Parameters,
			Description:   c.Description,
		})
	}
	return out
}

func actionsToLLM(in []datamodels.Action) []LLMAction {
	if in == nil {
		return nil
	}
	out := make([]LLMAction, 0, len(in))
	for _, a := range in {
		la := LLMAction{
			Type:             string(a.Type),
			PropertyUID:      a.PropertyUID,
			Value:            a.Value,
			Modifier:         string(a.Modifier),
			TargetUID:        a.TargetUID,
			TargetUIDs:       a.TargetUIDs,
			Speed:            a.Speed,
			Direction:        a.Direction,
			EventUID:         a.EventUID,
			EventData:        a.EventData,
			NewState:         a.NewState,
			SpawnAgentUID:    a.SpawnAgentUID,
			SpawnCount:       llmIPtr(a.SpawnCount),
			SpawnProperties:  a.SpawnProperties,
			SpawnInheritFrom: a.SpawnInheritFrom,
			Parameters:       a.Parameters,
			Description:      a.Description,
			LLMPromptContext: a.LLMPromptContext,
		}
		if a.SpawnPosition != nil {
			la.SpawnPosition = &LLMSpawnPosition{
				Type:     string(a.SpawnPosition.Type),
				OffsetX:  a.SpawnPosition.OffsetX,
				OffsetY:  a.SpawnPosition.OffsetY,
				Radius:   a.SpawnPosition.Radius,
				ShapeUID: a.SpawnPosition.ShapeUID,
			}
		}
		out = append(out, la)
	}
	return out
}

func interactionToLLM(in datamodels.Interaction) LLMInteraction {
	return LLMInteraction{
		ID:               in.ID,
		Name:             in.Name,
		Description:      in.Description,
		Type:             string(in.Type),
		InitiatorUIDs:    in.InitiatorUIDs,
		TargetUIDs:       in.TargetUIDs,
		Radius:           in.Radius,
		Conditions:       conditionsToLLM(in.Conditions),
		InitiatorActions: actionsToLLM(in.InitiatorActions),
		TargetActions:    actionsToLLM(in.TargetActions),
		Bidirectional:    llmBPtr(in.Bidirectional),
		UseLLM:           llmBPtr(in.UseLLM),
		Enabled:          llmBPtr(in.Enabled),
		LLMPromptContext: in.LLMPromptContext,
	}
}

func eventToLLM(e datamodels.EventDefinition) LLMEvent {
	out := LLMEvent{
		UID:              e.UID,
		Name:             e.Name,
		Description:      e.Description,
		Scope:            string(e.Scope),
		Radius:           e.Radius,
		Payload:          propertiesToLLM(e.Payload),
		ProducerUIDs:     e.ProducerUIDs,
		ConsumerUIDs:     e.ConsumerUIDs,
		TTLSteps:         llmUPtr(e.TTLSteps),
		LLMPromptContext: e.LLMPromptContext,
	}
	for _, p := range e.Producers {
		out.Producers = append(out.Producers, string(p))
	}
	return out
}

func widgetToLLM(w common.WidgetConfig) LLMWidget {
	out := LLMWidget{
		ID:       w.ID,
		Type:     string(w.Type),
		Title:    w.Title,
		Columns:  w.Columns,
		Buckets:  llmUFrom(w.Buckets),
		Position: &LLMWidgetPosition{X: llmUint(w.Position.X), Y: llmUint(w.Position.Y), W: llmUint(w.Position.W), H: llmUint(w.Position.H)},
	}
	if w.Source != nil {
		s := widgetSourceToLLM(*w.Source)
		out.Source = &s
	}
	for _, s := range w.Series {
		out.Series = append(out.Series, LLMWidgetSeries{Label: s.Label, Color: s.Color, Source: widgetSourceToLLM(s.Source)})
	}
	for _, k := range w.LogKinds {
		out.LogKinds = append(out.LogKinds, string(k))
	}
	return out
}

func widgetSourceToLLM(s common.WidgetSource) LLMWidgetSource {
	return LLMWidgetSource{
		Type:        string(s.Type),
		PropertyUID: s.PropertyUID,
		AgentUIDs:   s.AgentUIDs,
		Aggregation: string(s.Aggregation),
		Metric:      string(s.Metric),
	}
}

// ===================== misc =====================

// hasExisting reports whether the model carries any structural content worth
// showing the LLM as a baseline.
func hasExisting(m datamodels.Model) bool {
	return m.Description != "" || len(m.Agents) > 0 || m.Environment != nil ||
		len(m.Rules) > 0 || len(m.Interactions) > 0 || len(m.Events) > 0 || len(m.Widgets) > 0
}

func isEmptyLLMModel(m *LLMModel) bool {
	return m.Description == "" && m.IsSpatial == nil && m.Termination == nil && len(m.Agents) == 0 &&
		m.Environment == nil && len(m.Rules) == 0 && len(m.Interactions) == 0 && len(m.Events) == 0 &&
		len(m.Widgets) == 0
}

func jsonBlock(v interface{}) string {
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(b)
}

func boolPtr(b bool) *bool       { return &b }
