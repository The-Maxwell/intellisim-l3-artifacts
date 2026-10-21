package models

import (
	"fmt"

	"intelli.sim.core/data/common"
	datamodels "intelli.sim.core/data/models"
)

// ValidateShape enforces field-shape rules. A non-empty result MUST block PUT (422).
// Checks: enum membership, ScalarSpec exactly-one-of, UID uniqueness, reserved
// EnvironmentUID, geometry-vs-placement consistency, and Geometry.Position => pop==1.
func ValidateShape(m datamodels.Model) []ValidationIssue {
	var issues []ValidationIssue
	add := func(path, code, msg string) {
		issues = append(issues, ValidationIssue{Path: path, Code: code, Message: msg, Severity: "error"})
	}

	// UID uniqueness across agents + shapes + events; reserved "environment".
	seen := map[string]string{}
	checkUID := func(path, uid string) {
		if uid == "" {
			add(path, "uid_required", "uid is required")
			return
		}
		if uid == datamodels.EnvironmentUID {
			add(path, "uid_reserved", `"environment" is reserved`)
		}
		if prev, ok := seen[uid]; ok {
			add(path, "uid_duplicate", fmt.Sprintf("uid %q already used at %s", uid, prev))
		}
		seen[uid] = path
	}

	for i, a := range m.Agents {
		p := fmt.Sprintf("agents[%d]", i)
		checkUID(p+".uid", a.UID)
		if !validAgentType(a.Type) {
			add(p+".type", "enum", "invalid agent type")
		}
		if a.MoveType != "" && !validMoveType(a.MoveType) {
			add(p+".move_type", "enum", "invalid move_type")
		}
		for j, pr := range a.Properties {
			validateProperty(&issues, fmt.Sprintf("%s.properties[%d]", p, j), pr)
		}
		if a.Initialization.Placement != nil && a.Initialization.Placement.Type != "" && !validPlacementType(a.Initialization.Placement.Type) {
			add(p+".initialization.placement.type", "enum", "invalid placement type")
		}
		validateInitialization(&issues, p+".initialization", a.Initialization)
		validateFootprint(&issues, p+".footprint", a.Footprint)
		validateDisplay(&issues, p+".display", a.Display)
	}

	if m.Environment != nil {
		if m.Environment.CollisionMode != "" && !validCollisionMode(m.Environment.CollisionMode) {
			add("environment.collision_mode", "enum", "invalid collision_mode (block)")
		}
		for j, pr := range m.Environment.GlobalProperties {
			validateProperty(&issues, fmt.Sprintf("environment.global_properties[%d]", j), pr)
		}
		for i, sh := range m.Environment.Shapes {
			p := fmt.Sprintf("environment.shapes[%d]", i)
			checkUID(p+".uid", sh.UID)
			if !validShapeType(sh.Type) {
				add(p+".type", "enum", "invalid shape type")
			}
			for j, pr := range sh.Properties {
				validateProperty(&issues, fmt.Sprintf("%s.properties[%d]", p, j), pr)
			}
			validateInitialization(&issues, p+".initialization", sh.Initialization)
			validateShapeGeometry(&issues, p, sh)
			validateDisplay(&issues, p+".display", sh.Display)
		}
	}

	for i, r := range m.Rules {
		p := fmt.Sprintf("rules[%d]", i)
		if r.ID == "" {
			add(p+".id", "id_required", "rule id is required")
		}
		for j, c := range r.Conditions {
			validateCondition(&issues, fmt.Sprintf("%s.conditions[%d]", p, j), c)
		}
		for j, ac := range r.Actions {
			validateAction(&issues, fmt.Sprintf("%s.actions[%d]", p, j), ac)
		}
	}

	for i, in := range m.Interactions {
		p := fmt.Sprintf("interactions[%d]", i)
		if in.ID == "" {
			add(p+".id", "id_required", "interaction id is required")
		}
		if in.Type != "" && !validInteractionType(in.Type) {
			add(p+".type", "enum", "invalid interaction type")
		}
		for j, c := range in.Conditions {
			validateCondition(&issues, fmt.Sprintf("%s.conditions[%d]", p, j), c)
		}
		for j, ac := range in.InitiatorActions {
			validateAction(&issues, fmt.Sprintf("%s.initiator_actions[%d]", p, j), ac)
		}
		for j, ac := range in.TargetActions {
			validateAction(&issues, fmt.Sprintf("%s.target_actions[%d]", p, j), ac)
		}
	}

	for i, e := range m.Events {
		p := fmt.Sprintf("events[%d]", i)
		checkUID(p+".uid", e.UID)
		if e.Scope != "" && !validEventScope(e.Scope) {
			add(p+".scope", "enum", "invalid event scope")
		}
		for j, pr := range e.Producers {
			if !validEventProducer(pr) {
				add(fmt.Sprintf("%s.producers[%d]", p, j), "enum", "invalid event producer")
			}
		}
		for j, pr := range e.Payload {
			validateProperty(&issues, fmt.Sprintf("%s.payload[%d]", p, j), pr)
		}
	}

	if len(m.Widgets) > 0 {
		validateWidgets(&issues, m)
	}

	return issues
}

// validateWidgets enforces the structural per-type source matrix (delegated to
// common.ValidateWidget), widget-id uniqueness, and referential integrity of every
// widget source against the model's agents / properties / global properties.
func validateWidgets(issues *[]ValidationIssue, m datamodels.Model) {
	add := func(path, code, msg string) {
		*issues = append(*issues, ValidationIssue{Path: path, Code: code, Message: msg, Severity: "error"})
	}

	// agent uid -> set of its property uids; global property uid set.
	agentProps := make(map[string]map[string]bool, len(m.Agents))
	for _, a := range m.Agents {
		set := make(map[string]bool, len(a.Properties))
		for _, pr := range a.Properties {
			set[pr.UID] = true
		}
		agentProps[a.UID] = set
	}
	globalProps := map[string]bool{}
	if m.Environment != nil {
		for _, pr := range m.Environment.GlobalProperties {
			globalProps[pr.UID] = true
		}
	}

	seenIDs := map[string]string{}
	for i, w := range m.Widgets {
		p := fmt.Sprintf("widgets[%d]", i)

		// Structural shape (per-type source matrix).
		for _, wi := range common.ValidateWidget(i, w) {
			add(wi.Path, "widget", wi.Reason)
		}

		// Duplicate widget ids.
		if w.ID != "" {
			if prev, ok := seenIDs[w.ID]; ok {
				add(p+".id", "id_duplicate", fmt.Sprintf("widget id %q already used at %s", w.ID, prev))
			} else {
				seenIDs[w.ID] = p
			}
		}

		// Referential integrity of sources.
		if w.Source != nil {
			validateWidgetSourceRefs(issues, p+".source", *w.Source, agentProps, globalProps)
		}
		for j, s := range w.Series {
			validateWidgetSourceRefs(issues, fmt.Sprintf("%s.series[%d].source", p, j), s.Source, agentProps, globalProps)
		}

		// agent_table columns must be property uids of the table's archetype.
		if w.Type == common.WidgetAgentTable && w.Source != nil && len(w.Source.AgentUIDs) == 1 {
			if set, ok := agentProps[w.Source.AgentUIDs[0]]; ok {
				for j, col := range w.Columns {
					if !set[col] {
						add(fmt.Sprintf("%s.columns[%d]", p, j), "unresolved_ref",
							fmt.Sprintf("column %q is not a property uid of agent %q", col, w.Source.AgentUIDs[0]))
					}
				}
			}
		}
	}
}

// validateWidgetSourceRefs checks one widget source against the known agent uids
// and property sets. path already points at the source (e.g. "widgets[1].source").
func validateWidgetSourceRefs(issues *[]ValidationIssue, path string, src common.WidgetSource, agentProps map[string]map[string]bool, globalProps map[string]bool) {
	add := func(suffix, code, msg string) {
		*issues = append(*issues, ValidationIssue{Path: path + suffix, Code: code, Message: msg, Severity: "error"})
	}

	for j, uid := range src.AgentUIDs {
		if _, ok := agentProps[uid]; !ok {
			add(fmt.Sprintf(".agent_uids[%d]", j), "unresolved_ref", fmt.Sprintf("unknown agent uid %q", uid))
		}
	}

	switch src.Type {
	case common.WidgetSourceAgentProperty:
		if src.PropertyUID != "" && !widgetPropertyResolvable(src.PropertyUID, src.AgentUIDs, agentProps) {
			add(".property_uid", "unresolved_ref",
				fmt.Sprintf("property %q does not exist on the referenced agents", src.PropertyUID))
		}
	case common.WidgetSourceGlobalProperty:
		if src.PropertyUID != "" && !globalProps[src.PropertyUID] {
			add(".property_uid", "unresolved_ref",
				fmt.Sprintf("global property %q does not exist in environment.global_properties", src.PropertyUID))
		}
	// sim_metric intentionally has no referential checks — it reads built-in
	// engine run counters, not model-defined agents/properties.
	}
}

// widgetPropertyResolvable reports whether propUID exists on at least one of the
// referenced agents (or on ANY agent when agentUIDs is empty). When every
// referenced uid is unknown the function returns true — the unknown uids are
// already flagged and a property issue would only cascade noise.
func widgetPropertyResolvable(propUID string, agentUIDs []string, agentProps map[string]map[string]bool) bool {
	if len(agentUIDs) == 0 {
		for _, set := range agentProps {
			if set[propUID] {
				return true
			}
		}
		return false
	}
	anyKnown := false
	for _, uid := range agentUIDs {
		set, ok := agentProps[uid]
		if !ok {
			continue
		}
		anyKnown = true
		if set[propUID] {
			return true
		}
	}
	return !anyKnown
}

// ValidateCompleteness produces advisory errors/warnings; never blocks save.
func ValidateCompleteness(m datamodels.Model) ValidationResult {
	var res ValidationResult
	addErr := func(path, code, msg string) {
		res.Errors = append(res.Errors, ValidationIssue{Path: path, Code: code, Message: msg, Severity: "error"})
	}
	addWarn := func(path, code, msg string) {
		res.Warnings = append(res.Warnings, ValidationIssue{Path: path, Code: code, Message: msg, Severity: "warning"})
	}

	if m.Name == "" {
		addWarn("name", "missing_name", "model has no name")
	}
	if len(m.Agents) == 0 {
		addErr("agents", "no_agents", "model has no agents")
	}
	if m.IsSpatial {
		if m.Environment == nil || m.Environment.Width == 0 || m.Environment.Height == 0 {
			addErr("environment", "env_dims", "spatial model needs environment width & height > 0")
		}
	}

	uids := agentUIDSet(m)
	for i, a := range m.Agents {
		if a.Initialization.InitialState != "" && !contains(a.States, a.Initialization.InitialState) {
			addErr(fmt.Sprintf("agents[%d].initialization.initial_state", i), "bad_initial_state", "initial_state not in states")
		}
	}
	for i, r := range m.Rules {
		p := fmt.Sprintf("rules[%d]", i)
		envScoped := ruleTargetsEnvironment(r)
		// A DISABLED rule never executes, so its issues cannot silently no-op a
		// run — they degrade to warnings and must not block readiness (U7).
		// Re-enabling the rule re-runs validation and flags them again.
		addIssue := addErr
		if !r.Enabled {
			addIssue = func(path, code, msg string) {
				addWarn(path, code, msg+" (rule is disabled — advisory only)")
			}
		}
		for _, t := range r.AppliesTo {
			if t == datamodels.EnvironmentUID {
				continue // reserved environment singleton is always a valid target
			}
			if !uids[t] {
				addIssue(p+".applies_to", "unresolved_ref", "applies_to references unknown agent uid "+t)
			}
		}
		// Readiness blockers matching the engine's silent-no-op truth table
		// (modules/simulations/rules.go): a condition/action shaped so the matching
		// handler always treats it as a no-op is flagged here rather than left to
		// silently never fire. These block SimulationReady but never save (unlike
		// ValidateShape).
		for j, c := range r.Conditions {
			validateConditionReadiness(addIssue, fmt.Sprintf("%s.conditions[%d]", p, j), c, envScoped)
		}
		for j, a := range r.Actions {
			validateActionReadiness(addIssue, fmt.Sprintf("%s.actions[%d]", p, j), a)
		}
	}

	// Advisory: set/modify actions targeting the environment must reference a
	// DECLARED global property — the run engine warns and no-ops otherwise.
	globals := map[string]bool{}
	if m.Environment != nil {
		for _, p := range m.Environment.GlobalProperties {
			globals[p.UID] = true
		}
	}
	for i, r := range m.Rules {
		for j, a := range r.Actions {
			if (a.Type == datamodels.ActionTypeSetProperty || a.Type == datamodels.ActionTypeModifyProperty) &&
				a.TargetUID == datamodels.EnvironmentUID && a.PropertyUID != "" && !globals[a.PropertyUID] {
				addWarn(fmt.Sprintf("rules[%d].actions[%d].property_uid", i, j), "unresolved_ref",
					fmt.Sprintf("environment-targeted action references undeclared global property %q (the engine will skip it)", a.PropertyUID))
			}
		}
	}

	res.SimulationReady = len(res.Errors) == 0
	return res
}

// ruleTargetsEnvironment reports whether the rule's AppliesTo includes the
// reserved EnvironmentUID (an environment-scoped rule, evaluated at runtime by
// modules/simulations' runEnvironmentRules/evalEnvCondition rather than the
// per-agent path).
func ruleTargetsEnvironment(r datamodels.Rule) bool {
	for _, t := range r.AppliesTo {
		if t == datamodels.EnvironmentUID {
			return true
		}
	}
	return false
}

// envConditionSupported reports whether the engine's evalEnvCondition
// (modules/simulations/rules.go) actually evaluates condition type t for an
// environment-scoped rule. The environment is a positionless global observer;
// every other condition type always evaluates false at runtime.
func envConditionSupported(t datamodels.ConditionType) bool {
	switch t {
	case datamodels.ConditionTypeGlobalProperty, datamodels.ConditionTypeStep,
		datamodels.ConditionTypeRandom, datamodels.ConditionTypeComposite, datamodels.ConditionTypeEvent:
		return true
	}
	return false
}

// validateConditionReadiness recurses into c (and, for composite, its
// SubConditions), reporting a missing_field readiness error for every field the
// matching engine condition handler in modules/simulations/rules.go treats as
// "always false" when absent — these block SimulationReady but never save
// (ValidateShape is the save gate). When envScoped is true (the owning rule
// applies to the reserved environment singleton), a condition type
// evalEnvCondition does not support is additionally flagged
// env_condition_unsupported.
func validateConditionReadiness(addErr func(path, code, msg string), path string, c datamodels.Condition, envScoped bool) {
	if envScoped && !envConditionSupported(c.Type) {
		addErr(path, "env_condition_unsupported",
			fmt.Sprintf("condition type %q is not supported for environment-scoped rules (evalEnvCondition always evaluates it false)", c.Type))
	}
	switch c.Type {
	case datamodels.ConditionTypeProximity, datamodels.ConditionTypeCount:
		if c.Radius == nil {
			addErr(path+".radius", "missing_field", "radius is required (the engine treats a missing radius as always false)")
		}
	case datamodels.ConditionTypeComposite:
		if len(c.SubConditions) == 0 {
			addErr(path+".sub_conditions", "missing_field", "sub_conditions is required for a composite condition (an empty composite always evaluates false)")
		}
	case datamodels.ConditionTypeEvent:
		if c.TargetUID == "" && c.PropertyUID == "" {
			addErr(path+".target_uid", "missing_field", "target_uid (or property_uid) is required to identify the event")
		}
	case datamodels.ConditionTypeCustom:
		if s, ok := c.Parameters["property"].(string); !ok || s == "" {
			addErr(path+".parameters.property", "missing_field", `parameters["property"] is required for a custom condition`)
		}
	case datamodels.ConditionTypeGlobalProperty, datamodels.ConditionTypeProperty:
		if c.PropertyUID == "" {
			addErr(path+".property_uid", "missing_field", "property_uid is required")
		}
	case datamodels.ConditionTypePosition:
		// evalPosition supports exactly two forms; anything else is a silent
		// always-false no-op (U8c).
		inShape := (c.Operator == datamodels.ConditionOpIn || c.Operator == datamodels.ConditionOpNotIn) &&
			(c.TargetUID != "" || len(c.TargetUIDs) > 0)
		inRange := c.Operator == datamodels.ConditionOpInRange && c.MinValue != nil && c.MaxValue != nil
		if !inShape && !inRange {
			addErr(path, "unsupported_form",
				`position condition must be either operator "in"/"not_in" with a shape target_uid, or "in_range" with min_value+max_value (any other form always evaluates false)`)
		}
	}
	for j, sub := range c.SubConditions {
		validateConditionReadiness(addErr, fmt.Sprintf("%s.sub_conditions[%d]", path, j), sub, envScoped)
	}
}

// validateActionReadiness reports a missing_field readiness error for every
// field the matching engine action handler in modules/simulations/rules.go
// treats as a no-op when absent.
func validateActionReadiness(addErr func(path, code, msg string), path string, a datamodels.Action) {
	switch a.Type {
	case datamodels.ActionTypeSpawnAgent:
		if a.SpawnAgentUID == "" {
			addErr(path+".spawn_agent_uid", "missing_field", "spawn_agent_uid is required")
		}
	case datamodels.ActionTypeTransform:
		if a.SpawnAgentUID == "" && a.TargetUID == "" {
			addErr(path+".spawn_agent_uid", "missing_field", "spawn_agent_uid or target_uid is required")
		}
	case datamodels.ActionTypeEmitEvent:
		if a.EventUID == "" {
			addErr(path+".event_uid", "missing_field", "event_uid is required")
		}
	case datamodels.ActionTypeTransition:
		if a.NewState == "" {
			addErr(path+".new_state", "missing_field", "new_state is required")
		}
	case datamodels.ActionTypeSetProperty:
		if a.PropertyUID == "" {
			addErr(path+".property_uid", "missing_field", "property_uid is required")
		}
		if a.Value == nil {
			addErr(path+".value", "missing_field", "value is required")
		}
	case datamodels.ActionTypeModifyProperty:
		if a.PropertyUID == "" {
			addErr(path+".property_uid", "missing_field", "property_uid is required")
		}
	case datamodels.ActionTypeInteract:
		if a.TargetUID == "" {
			addErr(path+".target_uid", "missing_field", "target_uid (the interaction id) is required")
		}
	case datamodels.ActionTypeCustom:
		if _, ok := a.Parameters["set_properties"].(map[string]interface{}); !ok {
			addErr(path+".parameters.set_properties", "missing_field", `parameters["set_properties"] is required for a custom action`)
		}
	}
}

// ===================== enum helpers =====================

func validAgentType(t datamodels.AgentType) bool {
	switch t {
	case datamodels.AgentTypePlain, datamodels.AgentTypeArchetype:
		return true
	}
	return false
}

func validMoveType(t datamodels.AgentMoveType) bool {
	switch t {
	case datamodels.AgentMoveTypeStatic, datamodels.AgentMoveTypeMovable:
		return true
	}
	return false
}

func validPropertyType(t datamodels.AgentPropertyType) bool {
	switch t {
	case datamodels.PropertyTypeString, datamodels.PropertyTypeNumber, datamodels.PropertyTypeEnum,
		datamodels.PropertyTypeBoolean, datamodels.PropertyTypeColor:
		return true
	}
	return false
}

func validShapeType(t datamodels.ShapeType) bool {
	switch t {
	case datamodels.ShapeTypeRectangle, datamodels.ShapeTypeCircle, datamodels.ShapeTypeLine, datamodels.ShapeTypePolygon:
		return true
	}
	return false
}

func validPlacementType(t datamodels.PlacementType) bool {
	switch t {
	case datamodels.PlacementRandom, datamodels.PlacementUniform, datamodels.PlacementClustered,
		datamodels.PlacementAtShape, datamodels.PlacementManual, datamodels.PlacementLLM:
		return true
	}
	return false
}

func validConditionType(t datamodels.ConditionType) bool {
	switch t {
	case datamodels.ConditionTypeGlobalProperty, datamodels.ConditionTypeProperty, datamodels.ConditionTypePosition,
		datamodels.ConditionTypeProximity, datamodels.ConditionTypeStep, datamodels.ConditionTypeComposite,
		datamodels.ConditionTypeCount, datamodels.ConditionTypeBarrier, datamodels.ConditionTypeState,
		datamodels.ConditionTypeDistance, datamodels.ConditionTypeDecision, datamodels.ConditionTypeRandom,
		datamodels.ConditionTypeMemory, datamodels.ConditionTypeEvent, datamodels.ConditionTypeCustom:
		return true
	}
	return false
}

func validConditionOperator(o datamodels.ConditionOperator) bool {
	switch o {
	case datamodels.ConditionOpEquals, datamodels.ConditionOpNotEquals, datamodels.ConditionOpGreaterThan,
		datamodels.ConditionOpLessThan, datamodels.ConditionOpGreaterOrEq, datamodels.ConditionOpLessOrEq,
		datamodels.ConditionOpContains, datamodels.ConditionOpInRange, datamodels.ConditionOpIn, datamodels.ConditionOpNotIn:
		return true
	}
	return false
}

func validActionType(t datamodels.ActionType) bool {
	switch t {
	case datamodels.ActionTypeSetProperty, datamodels.ActionTypeModifyProperty, datamodels.ActionTypeMove,
		datamodels.ActionTypeMoveToward, datamodels.ActionTypeMoveAway, datamodels.ActionTypeInteract,
		datamodels.ActionTypeEmitEvent, datamodels.ActionTypeTransition, datamodels.ActionTypeCallLLM,
		datamodels.ActionTypeSpawnAgent, datamodels.ActionTypeRemoveAgent, datamodels.ActionTypeTransform,
		datamodels.ActionTypeTeleport, datamodels.ActionTypeCustom:
		return true
	}
	return false
}

func validActionModifier(mod datamodels.PropertyModifier) bool {
	switch mod {
	case datamodels.PropertyModifierSet, datamodels.PropertyModifierAdd, datamodels.PropertyModifierSubtract,
		datamodels.PropertyModifierMultiply, datamodels.PropertyModifierDivide, datamodels.PropertyModifierIncrement,
		datamodels.PropertyModifierDecrement, datamodels.PropertyModifierToggle:
		return true
	}
	return false
}

func validInteractionType(t datamodels.InteractionType) bool {
	switch t {
	case datamodels.InteractionTypeAgentAgent, datamodels.InteractionTypeAgentShape, datamodels.InteractionTypeAgentGlobal:
		return true
	}
	return false
}

func validEventScope(s datamodels.EventScope) bool {
	switch s {
	case datamodels.EventScopeGlobal, datamodels.EventScopeLocal, datamodels.EventScopeTargeted:
		return true
	}
	return false
}

func validEventProducer(p datamodels.EventProducer) bool {
	switch p {
	case datamodels.EventProducerAgent, datamodels.EventProducerEnvironment, datamodels.EventProducerLLM:
		return true
	}
	return false
}

func validGenerationMode(m datamodels.PropertyGenerationMode) bool {
	switch m {
	case datamodels.GenModeDefault, datamodels.GenModeRandom, datamodels.GenModeLLM:
		return true
	}
	return false
}

func validDistribution(d datamodels.DistributionType) bool {
	switch d {
	case datamodels.DistUniform, datamodels.DistNormal, datamodels.DistChoice:
		return true
	}
	return false
}

func validCountMode(c datamodels.PopulationCountMode) bool {
	switch c {
	case datamodels.CountFixed, datamodels.CountRandomRange, datamodels.CountLLM:
		return true
	}
	return false
}

func validFootprintKind(k datamodels.AgentFootprintKind) bool {
	switch k {
	case datamodels.FootprintPoint, datamodels.FootprintCircle, datamodels.FootprintRectangle, datamodels.FootprintPolygon:
		return true
	}
	return false
}

func validDisplayKind(k datamodels.DisplayKind) bool {
	switch k {
	case datamodels.DisplayAuto, datamodels.DisplayImage, datamodels.DisplayFigure:
		return true
	}
	return false
}

func validCollisionMode(m datamodels.CollisionMode) bool {
	switch m {
	case datamodels.CollisionNone, datamodels.CollisionBlock:
		return true
	}
	return false
}

func validDisplayMime(mime string) bool {
	switch mime {
	case "", "image/png", "image/jpeg", "image/gif", "image/webp":
		return true
	}
	return false
}

// ===================== sub-validators =====================

func validateProperty(issues *[]ValidationIssue, path string, p datamodels.AgentProperty) {
	add := func(code, msg string) {
		*issues = append(*issues, ValidationIssue{Path: path, Code: code, Message: msg, Severity: "error"})
	}
	if p.Type != "" && !validPropertyType(p.Type) {
		add("enum", "invalid property type")
	}
	if p.Type == datamodels.PropertyTypeColor {
		// A color property's default and allowed values must be hex strings (#RGB /
		// #RRGGBB / #RRGGBBAA). An empty/absent default is allowed (the sampler then
		// generates a random hex at t=0).
		if s, ok := p.DefaultValue.(string); ok && s != "" && !datamodels.IsHexColor(s) {
			add("invalid_color", "color default_value must be a hex colour like #RRGGBB")
		} else if p.DefaultValue != nil {
			if _, ok := p.DefaultValue.(string); !ok {
				add("invalid_color", "color default_value must be a hex colour string")
			}
		}
		for i, av := range p.AllowedValues {
			if s, ok := av.(string); !ok || (s != "" && !datamodels.IsHexColor(s)) {
				add("invalid_color", fmt.Sprintf("allowed_values[%d] must be a hex colour like #RRGGBB", i))
			}
		}
	}
	if p.MinValue != nil && p.MaxValue != nil && *p.MinValue > *p.MaxValue {
		add("min_max", "min_value must be <= max_value")
	}
	if p.Generation != nil {
		validateGeneration(issues, path+".generation", p)
	}
}

// validateGeneration enforces the PropertyGeneration shape rules for a property
// that carries an explicit generation spec. path points at the spec itself
// (e.g. "agents[2].properties[0].generation").
func validateGeneration(issues *[]ValidationIssue, path string, p datamodels.AgentProperty) {
	g := p.Generation
	add := func(suffix, code, msg string) {
		*issues = append(*issues, ValidationIssue{Path: path + suffix, Code: code, Message: msg, Severity: "error"})
	}

	if !validGenerationMode(g.Mode) {
		add(".mode", "enum", "invalid generation mode (default|random|llm)")
	}

	switch g.Mode {
	case datamodels.GenModeRandom:
		if !validDistribution(g.Distribution) {
			add(".distribution", "enum", `generation mode "random" requires a distribution (uniform|normal|choice)`)
			break
		}
		switch g.Distribution {
		case datamodels.DistUniform:
			if p.Type != datamodels.PropertyTypeNumber {
				add(".distribution", "generation_type", "uniform distribution requires a number property")
			}
		case datamodels.DistNormal:
			if p.Type != datamodels.PropertyTypeNumber {
				add(".distribution", "generation_type", "normal distribution requires a number property")
			}
			if g.Mean == nil {
				add(".mean", "required", "normal distribution requires mean")
			}
			if g.StdDev == nil {
				add(".std_dev", "required", "normal distribution requires std_dev")
			} else if *g.StdDev <= 0 {
				add(".std_dev", "positive", "std_dev must be > 0")
			}
		case datamodels.DistChoice:
			if len(g.Choices) == 0 {
				add(".choices", "required", "choice distribution requires at least one choice")
			}
			for k, c := range g.Choices {
				cp := fmt.Sprintf(".choices[%d]", k)
				if c.Weight <= 0 {
					add(cp+".weight", "positive", "choice weight must be > 0")
				}
				if !choiceValueCompatible(c.Value, p) {
					add(cp+".value", "type_mismatch",
						fmt.Sprintf("choice value is not compatible with property type %q", p.Type))
				}
			}
		}
	case datamodels.GenModeDefault, datamodels.GenModeLLM:
		modeName := string(g.Mode)
		if g.Distribution != "" {
			add(".distribution", "generation_field_forbidden", "distribution must be absent for mode "+modeName)
		}
		if g.Mean != nil {
			add(".mean", "generation_field_forbidden", "mean must be absent for mode "+modeName)
		}
		if g.StdDev != nil {
			add(".std_dev", "generation_field_forbidden", "std_dev must be absent for mode "+modeName)
		}
		if len(g.Choices) > 0 {
			add(".choices", "generation_field_forbidden", "choices must be absent for mode "+modeName)
		}
	}

	if g.Min != nil && g.Max != nil && *g.Min >= *g.Max {
		add(".min", "min_max", "generation min must be < max")
	}
}

// choiceValueCompatible reports whether a choice value fits the property's type:
// number -> float64-coercible, boolean -> bool, string/color -> string, enum ->
// member of AllowedValues (when AllowedValues is non-empty).
func choiceValueCompatible(v interface{}, p datamodels.AgentProperty) bool {
	switch p.Type {
	case datamodels.PropertyTypeNumber:
		_, ok := coerceFloat(v)
		return ok
	case datamodels.PropertyTypeBoolean:
		_, ok := v.(bool)
		return ok
	case datamodels.PropertyTypeString:
		_, ok := v.(string)
		return ok
	case datamodels.PropertyTypeColor:
		s, ok := v.(string)
		return ok && datamodels.IsHexColor(s)
	case datamodels.PropertyTypeEnum:
		if len(p.AllowedValues) == 0 {
			return true
		}
		for _, av := range p.AllowedValues {
			if av == v {
				return true
			}
			// Numeric tolerance: 2 vs 2.0 across JSON/BSON decodings.
			if fa, ok := coerceFloat(av); ok {
				if fv, ok2 := coerceFloat(v); ok2 && fa == fv {
					return true
				}
			}
		}
		return false
	}
	// Unknown property type is flagged separately by the enum check.
	return true
}

// coerceFloat converts the common dynamic numeric representations (JSON float64,
// BSON int32/int64, Go ints) to float64.
func coerceFloat(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int32:
		return float64(n), true
	case int64:
		return float64(n), true
	case uint:
		return float64(n), true
	case uint32:
		return float64(n), true
	case uint64:
		return float64(n), true
	}
	return 0, false
}

// validateInitialization enforces the CountMode rules of an AgentInitialization.
// path points at the initialization (e.g. "agents[1].initialization").
func validateInitialization(issues *[]ValidationIssue, path string, init datamodels.AgentInitialization) {
	add := func(suffix, code, msg string) {
		*issues = append(*issues, ValidationIssue{Path: path + suffix, Code: code, Message: msg, Severity: "error"})
	}

	// Empty CountMode is treated as fixed.
	if init.CountMode != "" && !validCountMode(init.CountMode) {
		add(".count_mode", "enum", "invalid count_mode (fixed|random_range|llm)")
		return
	}

	switch init.CountMode {
	case datamodels.CountRandomRange:
		if init.MinPopulation == nil || init.MaxPopulation == nil {
			add(".count_mode", "count_range_required", `count_mode "random_range" requires min_population and max_population`)
		} else if *init.MinPopulation > *init.MaxPopulation {
			add(".min_population", "min_max", "min_population must be <= max_population")
		}
	case datamodels.CountLLM:
		if init.MinPopulation != nil && init.MaxPopulation != nil && *init.MinPopulation > *init.MaxPopulation {
			add(".min_population", "min_max", "min_population must be <= max_population")
		}
	}

	// Placement clustered-parameter sanity (typed, deterministic).
	if pl := init.Placement; pl != nil && pl.Type == datamodels.PlacementClustered {
		if pl.Clusters != nil && *pl.Clusters == 0 {
			add(".placement.clusters", "placement_param", "clusters must be >= 1 when set")
		}
		if pl.ClusterRadius != nil && *pl.ClusterRadius == 0 {
			add(".placement.cluster_radius", "placement_param", "cluster_radius must be > 0 when set")
		}
	}
}

func validateCondition(issues *[]ValidationIssue, path string, c datamodels.Condition) {
	add := func(code, msg string) {
		*issues = append(*issues, ValidationIssue{Path: path, Code: code, Message: msg, Severity: "error"})
	}
	if c.Type != "" && !validConditionType(c.Type) {
		add("enum", "invalid condition type")
	}
	if c.Operator != "" && !validConditionOperator(c.Operator) {
		add("enum", "invalid condition operator")
	}
	if c.LogicOperator != "" {
		switch c.LogicOperator {
		case datamodels.LogicOpAND, datamodels.LogicOpOR, datamodels.LogicOpNOT:
		default:
			add("enum", "invalid logic operator")
		}
	}
	if c.MinValue != nil && c.MaxValue != nil && *c.MinValue > *c.MaxValue {
		add("min_max", "min_value must be <= max_value")
	}
	for i, sub := range c.SubConditions {
		validateCondition(issues, fmt.Sprintf("%s.sub_conditions[%d]", path, i), sub)
	}
}

func validateAction(issues *[]ValidationIssue, path string, a datamodels.Action) {
	add := func(code, msg string) {
		*issues = append(*issues, ValidationIssue{Path: path, Code: code, Message: msg, Severity: "error"})
	}
	if a.Type != "" && !validActionType(a.Type) {
		add("enum", "invalid action type")
	}
	if a.Modifier != "" && !validActionModifier(a.Modifier) {
		add("enum", "invalid action modifier")
	}
	if a.SpawnPosition != nil && a.SpawnPosition.Type != "" {
		switch a.SpawnPosition.Type {
		case datamodels.SpawnPositionSame, datamodels.SpawnPositionRandom, datamodels.SpawnPositionOffset,
			datamodels.SpawnPositionNearby, datamodels.SpawnPositionAtShape:
		default:
			add("enum", "invalid spawn position type")
		}
	}
}

// validateShapeGeometry enforces, when Placement.Type != "llm": a type-correct Geometry,
// ScalarSpec exactly-one-of, and Geometry.Position => Initialization.PopulationCount == 1.
func validateShapeGeometry(issues *[]ValidationIssue, path string, sh datamodels.Shape) {
	add := func(suffix, code, msg string) {
		*issues = append(*issues, ValidationIssue{Path: path + suffix, Code: code, Message: msg, Severity: "error"})
	}

	isLLMPlacement := sh.Initialization.Placement != nil && sh.Initialization.Placement.Type == datamodels.PlacementLLM

	g := sh.Geometry
	if g == nil {
		if !isLLMPlacement {
			add(".geometry", "geometry_required", "geometry is required for non-llm placement")
		}
		return
	}

	// ScalarSpec exactly-one-of on each present spec.
	checkSpec := func(suffix string, s *datamodels.ScalarSpec) {
		if s == nil {
			return
		}
		hasValue := s.Value != nil
		hasRange := s.Min != nil && s.Max != nil
		if hasValue == hasRange {
			add(suffix, "scalar_spec", "scalar spec must set exactly one of value or (min,max)")
		}
		if hasRange && *s.Min > *s.Max {
			add(suffix, "scalar_spec_range", "scalar spec min must be <= max")
		}
	}
	checkSpec(".geometry.width", g.Width)
	checkSpec(".geometry.height", g.Height)
	checkSpec(".geometry.radius", g.Radius)

	if !isLLMPlacement {
		switch sh.Type {
		case datamodels.ShapeTypeRectangle:
			if g.Width == nil || g.Height == nil {
				add(".geometry", "geometry_type", "rectangle geometry requires width and height")
			}
		case datamodels.ShapeTypeCircle:
			if g.Radius == nil {
				add(".geometry", "geometry_type", "circle geometry requires radius")
			}
		case datamodels.ShapeTypePolygon:
			if len(g.Points) < 3 {
				add(".geometry", "geometry_type", "polygon geometry requires at least 3 points")
			}
		case datamodels.ShapeTypeLine:
			if len(g.Points) != 2 {
				add(".geometry", "geometry_type", "line geometry requires exactly 2 points")
			}
		}
	}

	// A fixed Position implies a single instance.
	if g.Position != nil && sh.Initialization.PopulationCount != 1 {
		add(".geometry.position", "geometry_position_population", "a shape with a fixed geometry.position must have population_count == 1")
	}
}

// validateFootprint enforces the agent simulation-body shape rules. path points at the
// footprint (e.g. "agents[1].footprint"). A nil footprint (point) is always valid.
func validateFootprint(issues *[]ValidationIssue, path string, f *datamodels.AgentFootprint) {
	if f == nil {
		return
	}
	add := func(code, msg string) {
		*issues = append(*issues, ValidationIssue{Path: path, Code: code, Message: msg, Severity: "error"})
	}
	if f.Kind != "" && !validFootprintKind(f.Kind) {
		add("enum", "invalid footprint kind (point|circle|rectangle|polygon)")
		return
	}
	switch f.Kind {
	case datamodels.FootprintCircle:
		if f.Radius == nil || *f.Radius <= 0 {
			add("footprint_dim", "circle footprint requires radius > 0")
		}
	case datamodels.FootprintRectangle:
		if f.Width == nil || f.Height == nil || *f.Width == 0 || *f.Height == 0 {
			add("footprint_dim", "rectangle footprint requires width and height > 0")
		}
	case datamodels.FootprintPolygon:
		if len(f.Points) < 3 {
			add("footprint_dim", "polygon footprint requires at least 3 points")
		}
	}
}

// validateDisplay enforces the display shape rules for agents and shapes. path points at
// the display (e.g. "agents[1].display"). A nil display (auto) is always valid. The image
// byte-size cap is enforced separately at save time (after compression).
func validateDisplay(issues *[]ValidationIssue, path string, d *datamodels.Display) {
	if d == nil {
		return
	}
	add := func(suffix, code, msg string) {
		*issues = append(*issues, ValidationIssue{Path: path + suffix, Code: code, Message: msg, Severity: "error"})
	}
	if d.Kind != "" && !validDisplayKind(d.Kind) {
		add("", "enum", "invalid display kind (auto|image|figure)")
		return
	}
	switch d.Kind {
	case datamodels.DisplayImage:
		if d.Image == nil || d.Image.Data == "" {
			add(".image", "required", `display kind "image" requires image data`)
		} else if !validDisplayMime(d.Image.MimeType) {
			add(".image.mime_type", "enum", "unsupported image mime type (png|jpeg|gif|webp)")
		}
	case datamodels.DisplayFigure:
		if len(d.Figure) < 3 {
			add(".figure", "figure_points", `display kind "figure" requires at least 3 points`)
		}
	}
}

// ===================== misc helpers =====================

func agentUIDSet(m datamodels.Model) map[string]bool {
	set := make(map[string]bool, len(m.Agents))
	for _, a := range m.Agents {
		if a.UID != "" {
			set[a.UID] = true
		}
	}
	return set
}

func contains(ss []string, target string) bool {
	for _, s := range ss {
		if s == target {
			return true
		}
	}
	return false
}
