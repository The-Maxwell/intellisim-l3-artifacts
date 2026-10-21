package models

import (
	"strings"
	"time"

	"intelli.sim.core/data/common"
	datamodels "intelli.sim.core/data/models"
)

// llmMerge applies an LLM-generated envelope onto an existing model, producing a
// new model and a set of fail-soft warnings. It is the heart of the LLM
// partial-edit feature and is governed by four hard rules:
//
//   - SCOPE: an empty scope touches all sections; scope.Section limits the merge
//     to a single section; scope.Section+scope.UID limits it to one entity inside
//     that section. Out-of-scope sections are copied through untouched.
//   - UPSERT: incoming entities are matched to existing ones by key (UID for
//     agents/shapes/events, ID for rules/interactions). A known key updates in
//     place; an unknown key appends a new entity. Existing entities absent from
//     the incoming slice are RETAINED (a subset edit must never delete others).
//   - LOCKS: an existing entity with Locked==true is frozen — any incoming change
//     to it is skipped and a warning is emitted.
//   - USER-ONLY PRESERVATION: the LLM can never overwrite user-only fields
//     (LLMConfig, Locked, CreatedAt, Immutable, LLMTarget) nor model-level
//     identity (ids, ownership, timestamps, ChatID, LLMConfig). On update these
//     are carried over from the previous entity verbatim.
//
// FAIL-SOFT: every field is validated; an invalid/missing field keeps the
// previous value (on update) or a schema default (on new) and emits a warning. A
// malformed entity is skipped or has its valid fields kept — the function never
// panics on nil/odd input.
//
// FIELD-LEVEL RETENTION (UPDATE): the contract DTOs distinguish an absent field
// from an explicit zero — scalar fields where zero is meaningful (bools,
// numerics) are pointer-typed and strings/slices follow the same intent. On
// update an absent field (nil pointer, empty string, nil slice) keeps the
// previous value; an explicit value — including the zero value or an explicitly
// emitted empty array (non-nil, length 0) that CLEARS a list — is applied. On
// create, an absent field takes a schema default (Enabled defaults to true for a
// new rule/interaction; other absent bools default false).
//
// The input model and its slices are never mutated: out is a value copy and each
// section is replaced wholesale with a freshly built slice.
func llmMerge(existing datamodels.Model, env LLMGenerationEnvelope, scope EditScope) (datamodels.Model, []MergeWarning) {
	out := existing // value copy; nested slices are replaced wholesale below, never mutated in place.
	var warnings []MergeWarning

	if env.Model == nil {
		warnings = append(warnings, MergeWarning{Path: "model", Reason: "response contained no model; existing model kept"})
		return out, warnings
	}

	now := time.Now().UTC()

	wholeModel := scope.Section == ""
	inScope := func(section string) bool {
		return wholeModel || scope.Section == section
	}

	// Description: whole-model scope only.
	if wholeModel && env.Model.Description != "" {
		out.Description = env.Model.Description
	}

	// IsSpatial: only when explicitly provided AND environment/whole-model scope.
	if env.Model.IsSpatial != nil && (wholeModel || scope.Section == "environment") {
		out.IsSpatial = bool(*env.Model.IsSpatial)
	}

	// Termination (U7): whole-model scope only; restricted to the engine's
	// termination subset — an unsupported condition keeps the previous value.
	if wholeModel && env.Model.Termination != nil {
		if cond, ok := terminationFromLLM(*env.Model.Termination); ok {
			out.SimulationConfig.Termination = &cond
		} else {
			warnings = append(warnings, MergeWarning{Path: "termination",
				Reason: "unsupported termination condition (allowed types: population, step, global_property, composite); previous kept"})
		}
	}

	if inScope("agents") {
		out.Agents, warnings = mergeAgents(out.Agents, env.Model.Agents, scope.UID, now, warnings)
	}
	if inScope("environment") {
		out.Environment, warnings = mergeEnvironment(out.Environment, env.Model.Environment, scope.UID, warnings)
	}
	if inScope("rules") {
		out.Rules, warnings = mergeRules(out.Rules, env.Model.Rules, scope.UID, warnings)
	}
	if inScope("interactions") {
		out.Interactions, warnings = mergeInteractions(out.Interactions, env.Model.Interactions, scope.UID, warnings)
	}
	if inScope("events") {
		out.Events, warnings = mergeEvents(out.Events, env.Model.Events, scope.UID, warnings)
	}
	if inScope("widgets") {
		out.Widgets, warnings = mergeWidgets(out.Widgets, env.Model.Widgets, scope.UID, warnings)
	}

	return out, warnings
}

// ===================== agents =====================

func mergeAgents(existing []datamodels.Agent, incoming []LLMAgent, scopeUID string, now time.Time, warnings []MergeWarning) ([]datamodels.Agent, []MergeWarning) {
	out := cloneAgents(existing)
	index := make(map[string]int, len(out))
	for i, a := range out {
		index[a.UID] = i
	}

	for _, in := range incoming {
		if in.UID == "" {
			warnings = append(warnings, MergeWarning{Path: "agents", Reason: "incoming agent has empty uid; skipped"})
			continue
		}
		if scopeUID != "" && in.UID != scopeUID {
			warnings = append(warnings, MergeWarning{Path: "agents[" + in.UID + "]", Reason: "outside the requested entity scope; ignored"})
			continue
		}
		if i, ok := index[in.UID]; ok {
			if out[i].Locked {
				warnings = append(warnings, MergeWarning{Path: "agents[" + in.UID + "]", Reason: "agent " + in.UID + " is locked; ignored"})
				continue
			}
			out[i], warnings = mergeAgent(out[i], in, now, warnings)
		} else {
			a, w := newAgent(in, now)
			warnings = append(warnings, w...)
			out = append(out, a)
			index[in.UID] = len(out) - 1
		}
	}
	return out, warnings
}

// mergeAgent updates prev with the valid fields of in, preserving user-only
// fields. An absent field (empty string, nil slice) keeps prev's value.
func mergeAgent(prev datamodels.Agent, in LLMAgent, now time.Time, warnings []MergeWarning) (datamodels.Agent, []MergeWarning) {
	a := prev // keeps LLMConfig, Locked, CreatedAt (user-only) by default.
	path := "agents[" + in.UID + "]"

	if in.Name != "" {
		a.Name = in.Name
	}
	if in.Description != "" {
		a.Description = in.Description
	}
	if in.Type != "" {
		if validAgentType(datamodels.AgentType(in.Type)) {
			a.Type = datamodels.AgentType(in.Type)
		} else {
			warnings = append(warnings, MergeWarning{Path: path + ".type", Reason: "invalid agent type " + in.Type + "; previous kept"})
		}
	}
	if in.MoveType != "" {
		if validMoveType(datamodels.AgentMoveType(in.MoveType)) {
			a.MoveType = datamodels.AgentMoveType(in.MoveType)
		} else {
			warnings = append(warnings, MergeWarning{Path: path + ".move_type", Reason: "invalid move_type " + in.MoveType + "; previous kept"})
		}
	}
	if in.States != nil {
		a.States = append([]string(nil), in.States...)
	}
	a.Properties, warnings = mergeProperties(prev.Properties, in.Properties, path, warnings)
	a.Initialization, warnings = initializationFromLLM(in.Initialization, prev.Initialization, path, warnings)
	a.UpdatedAt = now
	return a, warnings
}

func newAgent(in LLMAgent, now time.Time) (datamodels.Agent, []MergeWarning) {
	var warnings []MergeWarning
	path := "agents[" + in.UID + "]"
	a := datamodels.Agent{
		UID:         in.UID,
		Name:        in.Name,
		Description: in.Description,
		States:      append([]string(nil), in.States...),
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if validAgentType(datamodels.AgentType(in.Type)) {
		a.Type = datamodels.AgentType(in.Type)
	} else {
		a.Type = datamodels.AgentTypePlain
		warnings = append(warnings, MergeWarning{Path: path + ".type", Reason: "missing/invalid agent type; defaulted to " + string(datamodels.AgentTypePlain)})
	}
	if in.MoveType != "" && validMoveType(datamodels.AgentMoveType(in.MoveType)) {
		a.MoveType = datamodels.AgentMoveType(in.MoveType)
	} else if in.MoveType != "" {
		warnings = append(warnings, MergeWarning{Path: path + ".move_type", Reason: "invalid move_type " + in.MoveType + "; left empty"})
	}
	a.Properties, warnings = mergeProperties(nil, in.Properties, path, warnings)
	a.Initialization, warnings = initializationFromLLM(in.Initialization, datamodels.AgentInitialization{}, path, warnings)
	return a, warnings
}

// mergeProperties upserts properties by UID, preserving each existing property's
// Immutable flag (user-only). Properties absent from the incoming slice are kept.
func mergeProperties(existing []datamodels.AgentProperty, incoming []LLMAgentProperty, parentPath string, warnings []MergeWarning) ([]datamodels.AgentProperty, []MergeWarning) {
	if incoming == nil {
		// No properties supplied for this entity → keep what we had.
		return append([]datamodels.AgentProperty(nil), existing...), warnings
	}
	out := append([]datamodels.AgentProperty(nil), existing...)
	index := make(map[string]int, len(out))
	for i, p := range out {
		if p.UID != "" {
			index[p.UID] = i
		}
	}
	for _, in := range incoming {
		path := parentPath + ".properties[" + in.UID + "]"
		if i, ok := index[in.UID]; ok && in.UID != "" {
			out[i], warnings = mergeProperty(out[i], in, path, warnings)
		} else {
			p, w := propertyFromLLM(in, datamodels.AgentProperty{}, path)
			warnings = append(warnings, w...)
			out = append(out, p)
			if in.UID != "" {
				index[in.UID] = len(out) - 1
			}
		}
	}
	return out, warnings
}

// mergeProperty updates an existing property in place; absent fields keep prev.
func mergeProperty(prev datamodels.AgentProperty, in LLMAgentProperty, path string, warnings []MergeWarning) (datamodels.AgentProperty, []MergeWarning) {
	p := prev // keeps Immutable (user-only) and any field the LLM omits.
	if in.Name != "" {
		p.Name = in.Name
	}
	if in.Description != "" {
		p.Description = in.Description
	}
	if in.MinValue != nil {
		p.MinValue = copyFloat(in.MinValue)
	}
	if in.MaxValue != nil {
		p.MaxValue = copyFloat(in.MaxValue)
	}
	if in.DefaultValue != nil {
		p.DefaultValue = in.DefaultValue
	}
	if in.AllowedValues != nil {
		p.AllowedValues = append([]interface{}(nil), in.AllowedValues...)
	}
	if in.LLMPromptContext != "" {
		p.LLMPromptContext = in.LLMPromptContext
	}
	if in.Type != "" {
		if validPropertyType(datamodels.AgentPropertyType(in.Type)) {
			p.Type = datamodels.AgentPropertyType(in.Type)
		} else {
			warnings = append(warnings, MergeWarning{Path: path + ".type", Reason: "invalid property type " + in.Type + "; previous kept"})
		}
	}
	if in.Generation != nil {
		p.Generation, warnings = generationFromLLM(in.Generation, prev.Generation, path+".generation", warnings)
	}
	return p, warnings
}

// propertyFromLLM builds a NEW property from in (absent fields take zero values),
// keeping prev.Immutable (user-only) and falling back to prev's type on an
// invalid enum.
func propertyFromLLM(in LLMAgentProperty, prev datamodels.AgentProperty, path string) (datamodels.AgentProperty, []MergeWarning) {
	var warnings []MergeWarning
	p := datamodels.AgentProperty{
		UID:              in.UID,
		Name:             in.Name,
		Description:      in.Description,
		MinValue:         copyFloat(in.MinValue),
		MaxValue:         copyFloat(in.MaxValue),
		DefaultValue:     in.DefaultValue,
		AllowedValues:    append([]interface{}(nil), in.AllowedValues...),
		Immutable:        prev.Immutable, // user-only, preserved.
		LLMPromptContext: in.LLMPromptContext,
	}
	if in.Type != "" {
		if validPropertyType(datamodels.AgentPropertyType(in.Type)) {
			p.Type = datamodels.AgentPropertyType(in.Type)
		} else {
			p.Type = prev.Type
			warnings = append(warnings, MergeWarning{Path: path + ".type", Reason: "invalid property type " + in.Type + "; previous kept"})
		}
	} else {
		p.Type = prev.Type
	}
	p.Generation, warnings = generationFromLLM(in.Generation, prev.Generation, path+".generation", warnings)
	return p, warnings
}

// generationFromLLM merges an incoming generation spec onto prev. An absent field
// keeps prev's value. An invalid (or, on a fresh spec, missing) mode/distribution
// enum keeps the previous value when prev exists, or DROPS the whole spec when it
// does not — always with a warning. A nil incoming spec returns prev unchanged.
func generationFromLLM(in *LLMPropertyGeneration, prev *datamodels.PropertyGeneration, path string, warnings []MergeWarning) (*datamodels.PropertyGeneration, []MergeWarning) {
	if in == nil {
		return prev, warnings
	}

	g := datamodels.PropertyGeneration{}
	if prev != nil {
		g = *prev
		g.Choices = append([]datamodels.ChoiceWeight(nil), prev.Choices...)
	}

	if in.Mode != "" {
		if validGenerationMode(datamodels.PropertyGenerationMode(in.Mode)) {
			g.Mode = datamodels.PropertyGenerationMode(in.Mode)
		} else {
			if prev == nil {
				warnings = append(warnings, MergeWarning{Path: path + ".mode", Reason: "invalid generation mode " + in.Mode + "; generation spec dropped"})
				return nil, warnings
			}
			warnings = append(warnings, MergeWarning{Path: path + ".mode", Reason: "invalid generation mode " + in.Mode + "; previous kept"})
		}
	} else if prev == nil {
		warnings = append(warnings, MergeWarning{Path: path + ".mode", Reason: "missing generation mode; generation spec dropped"})
		return nil, warnings
	}

	if in.Distribution != "" {
		if validDistribution(datamodels.DistributionType(in.Distribution)) {
			g.Distribution = datamodels.DistributionType(in.Distribution)
		} else {
			if prev == nil {
				warnings = append(warnings, MergeWarning{Path: path + ".distribution", Reason: "invalid distribution " + in.Distribution + "; generation spec dropped"})
				return nil, warnings
			}
			warnings = append(warnings, MergeWarning{Path: path + ".distribution", Reason: "invalid distribution " + in.Distribution + "; previous kept"})
		}
	}

	if in.Mean != nil {
		g.Mean = copyFloat(in.Mean)
	}
	if in.StdDev != nil {
		g.StdDev = copyFloat(in.StdDev)
	}
	if in.Min != nil {
		g.Min = copyFloat(in.Min)
	}
	if in.Max != nil {
		g.Max = copyFloat(in.Max)
	}
	if in.Choices != nil {
		choices := make([]datamodels.ChoiceWeight, 0, len(in.Choices))
		for _, c := range in.Choices {
			// interface{} scalar values are JSON-decoded and treated as immutable.
			choices = append(choices, datamodels.ChoiceWeight{Value: c.Value, Weight: c.Weight})
		}
		g.Choices = choices
	}
	if in.PromptContext != "" {
		g.PromptContext = in.PromptContext
	}
	return &g, warnings
}

// initializationFromLLM merges in onto prev: an absent pointer/string/placement
// keeps prev's value. An invalid count_mode keeps prev's value with a warning.
// Used by both the update path (prev = existing) and the create path (prev =
// zero). parentPath is the owning entity path (e.g. "agents[fox]").
func initializationFromLLM(in LLMAgentInitialization, prev datamodels.AgentInitialization, parentPath string, warnings []MergeWarning) (datamodels.AgentInitialization, []MergeWarning) {
	init := prev
	if in.PopulationCount != nil {
		init.PopulationCount = uint64(*in.PopulationCount)
	}
	if in.MinPopulation != nil {
		init.MinPopulation = in.MinPopulation.std()
	}
	if in.MaxPopulation != nil {
		init.MaxPopulation = in.MaxPopulation.std()
	}
	if in.InitialState != "" {
		init.InitialState = in.InitialState
	}
	if in.Placement != nil {
		base := datamodels.Placement{}
		if prev.Placement != nil {
			base = *prev.Placement
		}
		if in.Placement.Type != "" {
			base.Type = datamodels.PlacementType(in.Placement.Type)
		}
		if in.Placement.ShapeUID != "" {
			base.ShapeUID = in.Placement.ShapeUID
		}
		if in.Placement.PromptContext != "" {
			base.PromptContext = in.Placement.PromptContext
		}
		if in.Placement.Clusters != nil {
			base.Clusters = in.Placement.Clusters.std()
		}
		if in.Placement.ClusterRadius != nil {
			base.ClusterRadius = in.Placement.ClusterRadius.std()
		}
		init.Placement = &base
	}
	if in.CountMode != "" {
		if validCountMode(datamodels.PopulationCountMode(in.CountMode)) {
			init.CountMode = datamodels.PopulationCountMode(in.CountMode)
		} else {
			warnings = append(warnings, MergeWarning{Path: parentPath + ".initialization.count_mode", Reason: "invalid count_mode " + in.CountMode + "; previous kept"})
		}
	}
	return init, warnings
}

// ===================== environment + shapes =====================

func mergeEnvironment(existing *datamodels.Environment, incoming *LLMEnvironment, scopeUID string, warnings []MergeWarning) (*datamodels.Environment, []MergeWarning) {
	if incoming == nil {
		return existing, warnings
	}

	var base datamodels.Environment
	if existing != nil {
		base = *existing
		base.Shapes = append([]datamodels.Shape(nil), existing.Shapes...)
		base.GlobalProperties = append([]datamodels.AgentProperty(nil), existing.GlobalProperties...)
	}
	// UID is reserved/fixed; the LLM never sets it.
	base.UID = datamodels.EnvironmentUID

	// Scalar environment fields apply only when not narrowed to a single shape;
	// an absent (nil/empty) field keeps the previous value.
	if scopeUID == "" {
		if incoming.Width != nil {
			base.Width = uint64(*incoming.Width)
		}
		if incoming.Height != nil {
			base.Height = uint64(*incoming.Height)
		}
		if incoming.Torus != nil {
			base.Torus = bool(*incoming.Torus)
		}
		if incoming.CollisionMode != "" {
			mode := datamodels.CollisionMode(incoming.CollisionMode)
			if incoming.CollisionMode == "none" {
				mode = datamodels.CollisionNone // stored as the empty zero value
			}
			if validCollisionMode(mode) {
				base.CollisionMode = mode
			} else {
				warnings = append(warnings, MergeWarning{Path: "environment.collision_mode",
					Reason: "invalid collision_mode " + incoming.CollisionMode + " (allowed: none, block); previous kept"})
			}
		}
		if incoming.LLMPromptContext != "" {
			base.LLMPromptContext = incoming.LLMPromptContext
		}
		base.GlobalProperties, warnings = mergeProperties(base.GlobalProperties, incoming.GlobalProperties, "environment.global_properties", warnings)
	}

	base.Shapes, warnings = mergeShapes(base.Shapes, incoming.Shapes, scopeUID, warnings)
	return &base, warnings
}

func mergeShapes(existing []datamodels.Shape, incoming []LLMShape, scopeUID string, warnings []MergeWarning) ([]datamodels.Shape, []MergeWarning) {
	out := append([]datamodels.Shape(nil), existing...)
	index := make(map[string]int, len(out))
	for i, s := range out {
		index[s.UID] = i
	}
	for _, in := range incoming {
		if in.UID == "" {
			warnings = append(warnings, MergeWarning{Path: "environment.shapes", Reason: "incoming shape has empty uid; skipped"})
			continue
		}
		if scopeUID != "" && in.UID != scopeUID {
			warnings = append(warnings, MergeWarning{Path: "environment.shapes[" + in.UID + "]", Reason: "outside the requested entity scope; ignored"})
			continue
		}
		if i, ok := index[in.UID]; ok {
			if out[i].Locked {
				warnings = append(warnings, MergeWarning{Path: "environment.shapes[" + in.UID + "]", Reason: "shape " + in.UID + " is locked; ignored"})
				continue
			}
			out[i], warnings = mergeShape(out[i], in, warnings)
		} else {
			s, w := newShape(in)
			warnings = append(warnings, w...)
			out = append(out, s)
			index[in.UID] = len(out) - 1
		}
	}
	return out, warnings
}

// mergeShape updates an existing shape in place; absent fields keep prev.
func mergeShape(prev datamodels.Shape, in LLMShape, warnings []MergeWarning) (datamodels.Shape, []MergeWarning) {
	s := prev // keeps LLMConfig, Locked (user-only).
	path := "environment.shapes[" + in.UID + "]"
	if in.Name != "" {
		s.Name = in.Name
	}
	if in.Description != "" {
		s.Description = in.Description
	}
	if in.Type != "" {
		if validShapeType(datamodels.ShapeType(in.Type)) {
			s.Type = datamodels.ShapeType(in.Type)
		} else {
			warnings = append(warnings, MergeWarning{Path: path + ".type", Reason: "invalid shape type " + in.Type + "; previous kept"})
		}
	}
	if in.Layer != nil {
		s.Layer = uint(*in.Layer)
	}
	if in.Visible != nil {
		s.Visible = bool(*in.Visible)
	}
	s.Properties, warnings = mergeProperties(prev.Properties, in.Properties, path, warnings)
	s.Initialization, warnings = initializationFromLLM(in.Initialization, prev.Initialization, path, warnings)
	s.Geometry = geometryFromLLM(in.Geometry, prev.Geometry)
	return s, warnings
}

func newShape(in LLMShape) (datamodels.Shape, []MergeWarning) {
	var warnings []MergeWarning
	path := "environment.shapes[" + in.UID + "]"
	s := datamodels.Shape{
		UID:         in.UID,
		Name:        in.Name,
		Description: in.Description,
	}
	if in.Layer != nil {
		s.Layer = uint(*in.Layer)
	}
	if in.Visible != nil {
		s.Visible = bool(*in.Visible)
	}
	if validShapeType(datamodels.ShapeType(in.Type)) {
		s.Type = datamodels.ShapeType(in.Type)
	} else {
		s.Type = datamodels.ShapeTypeRectangle
		warnings = append(warnings, MergeWarning{Path: path + ".type", Reason: "missing/invalid shape type; defaulted to " + string(datamodels.ShapeTypeRectangle)})
	}
	s.Properties, warnings = mergeProperties(nil, in.Properties, path, warnings)
	s.Initialization, warnings = initializationFromLLM(in.Initialization, datamodels.AgentInitialization{}, path, warnings)
	s.Geometry = geometryFromLLM(in.Geometry, nil)
	return s, warnings
}

// geometryFromLLM merges in onto prev: an absent geometry (nil) keeps prev; an
// absent nested field keeps prev's nested value. Points follow the slice
// contract — nil keeps prev, a non-nil slice (incl. empty) replaces it.
func geometryFromLLM(in *LLMShapeGeometry, prev *datamodels.ShapeGeometry) *datamodels.ShapeGeometry {
	if in == nil {
		return prev
	}
	g := datamodels.ShapeGeometry{}
	if prev != nil {
		g = *prev
	}
	if in.Width != nil {
		g.Width = scalarSpecFromLLM(in.Width)
	}
	if in.Height != nil {
		g.Height = scalarSpecFromLLM(in.Height)
	}
	if in.Radius != nil {
		g.Radius = scalarSpecFromLLM(in.Radius)
	}
	if in.Thickness != nil {
		g.Thickness = in.Thickness.std()
	}
	if in.Fill != nil {
		g.Fill = in.Fill.std()
	}
	if in.Stroke != nil {
		g.Stroke = in.Stroke.std()
	}
	if in.StrokeWidth != nil {
		g.StrokeWidth = in.StrokeWidth.std()
	}
	if in.Position != nil {
		g.Position = &common.Point{X: uint64(in.Position.X), Y: uint64(in.Position.Y)}
	}
	if in.Points != nil {
		pts := make([]common.Point, 0, len(in.Points))
		for _, p := range in.Points {
			pts = append(pts, common.Point{X: uint64(p.X), Y: uint64(p.Y)})
		}
		g.Points = pts
	}
	return &g
}

func scalarSpecFromLLM(in *LLMScalarSpec) *datamodels.ScalarSpec {
	if in == nil {
		return nil
	}
	return &datamodels.ScalarSpec{
		Value: in.Value.std(),
		Min:   in.Min.std(),
		Max:   in.Max.std(),
	}
}

// ===================== rules =====================

func mergeRules(existing []datamodels.Rule, incoming []LLMRule, scopeUID string, warnings []MergeWarning) ([]datamodels.Rule, []MergeWarning) {
	out := append([]datamodels.Rule(nil), existing...)
	index := make(map[string]int, len(out))
	for i, r := range out {
		index[r.ID] = i
	}
	for _, in := range incoming {
		if in.ID == "" {
			warnings = append(warnings, MergeWarning{Path: "rules", Reason: "incoming rule has empty id; skipped"})
			continue
		}
		if scopeUID != "" && in.ID != scopeUID {
			warnings = append(warnings, MergeWarning{Path: "rules[" + in.ID + "]", Reason: "outside the requested entity scope; ignored"})
			continue
		}
		if i, ok := index[in.ID]; ok {
			if out[i].Locked {
				warnings = append(warnings, MergeWarning{Path: "rules[" + in.ID + "]", Reason: "rule " + in.ID + " is locked; ignored"})
				continue
			}
			out[i] = mergeRule(out[i], in) // preserves Locked + omitted fields.
		} else {
			out = append(out, newRule(in))
			index[in.ID] = len(out) - 1
		}
	}
	return out, warnings
}

// mergeRule updates prev with the fields in supplies; an absent field (empty
// string, nil pointer, nil slice) keeps prev. A non-nil (possibly empty) slice
// replaces the previous list. Locked is user-only and preserved.
func mergeRule(prev datamodels.Rule, in LLMRule) datamodels.Rule {
	r := prev
	r.ID = in.ID
	if in.Name != "" {
		r.Name = in.Name
	}
	if in.Description != "" {
		r.Description = in.Description
	}
	if in.Priority != nil {
		r.Priority = int(*in.Priority)
	}
	if in.AppliesTo != nil {
		r.AppliesTo = append([]string(nil), in.AppliesTo...)
	}
	if in.Conditions != nil {
		r.Conditions = conditionsFromLLM(in.Conditions)
	}
	if in.Actions != nil {
		r.Actions = actionsFromLLM(in.Actions)
	}
	if in.ExecuteOnce != nil {
		r.ExecuteOnce = bool(*in.ExecuteOnce)
	}
	if in.CooldownSteps != nil {
		r.CooldownSteps = int(*in.CooldownSteps)
	}
	if in.Enabled != nil {
		r.Enabled = bool(*in.Enabled)
	}
	return r
}

// newRule builds a NEW rule. Absent scalar fields take zero values, except
// Enabled which defaults to true — an LLM creating a rule intends it active.
func newRule(in LLMRule) datamodels.Rule {
	return datamodels.Rule{
		ID:            in.ID,
		Name:          in.Name,
		Description:   in.Description,
		Priority:      in.Priority.or(0),
		AppliesTo:     append([]string(nil), in.AppliesTo...),
		Conditions:    conditionsFromLLM(in.Conditions),
		Actions:       actionsFromLLM(in.Actions),
		ExecuteOnce:   in.ExecuteOnce.or(false),
		CooldownSteps: in.CooldownSteps.or(0),
		Enabled:       in.Enabled.or(true),
	}
}

// terminationFromLLM converts an LLM-authored termination condition and
// reports whether it stays within the engine's termination subset
// (population | step | global_property | composite over the same subset —
// mirrors modules/simulations' terminationConditionTypes, which models cannot
// import without a cycle).
func terminationFromLLM(in LLMCondition) (datamodels.Condition, bool) {
	cond := conditionFromLLM(in)
	if !terminationSubsetOK(cond) {
		return datamodels.Condition{}, false
	}
	return cond, true
}

func terminationSubsetOK(c datamodels.Condition) bool {
	switch c.Type {
	case datamodels.ConditionTypePopulation, datamodels.ConditionTypeStep, datamodels.ConditionTypeGlobalProperty:
		return true
	case datamodels.ConditionTypeComposite:
		if len(c.SubConditions) == 0 {
			return false
		}
		for _, sc := range c.SubConditions {
			if !terminationSubsetOK(sc) {
				return false
			}
		}
		return true
	}
	return false
}

func conditionsFromLLM(in []LLMCondition) []datamodels.Condition {
	if in == nil {
		return nil
	}
	out := make([]datamodels.Condition, 0, len(in))
	for _, c := range in {
		out = append(out, conditionFromLLM(c))
	}
	return out
}

func conditionFromLLM(c LLMCondition) datamodels.Condition {
	return datamodels.Condition{
		Type:        datamodels.ConditionType(c.Type),
		PropertyUID: c.PropertyUID,
		Operator:    datamodels.ConditionOperator(c.Operator),
		// interface{} scalar values are JSON-decoded and treated as immutable (not deep-copied).
		Value:         c.Value,
		MinValue:      copyFloat(c.MinValue),
		MaxValue:      copyFloat(c.MaxValue),
		Radius:        copyFloat(c.Radius),
		TargetUID:     c.TargetUID,
		TargetUIDs:    append([]string(nil), c.TargetUIDs...),
		LogicOperator: datamodels.LogicOperator(c.LogicOperator),
		SubConditions: conditionsFromLLM(c.SubConditions),
		Parameters:    cloneAnyMap(c.Parameters),
		Description:   c.Description,
	}
}

func actionsFromLLM(in []LLMAction) []datamodels.Action {
	if in == nil {
		return nil
	}
	out := make([]datamodels.Action, 0, len(in))
	for _, a := range in {
		out = append(out, actionFromLLM(a))
	}
	return out
}

func actionFromLLM(a LLMAction) datamodels.Action {
	out := datamodels.Action{
		Type:        datamodels.ActionType(a.Type),
		PropertyUID: a.PropertyUID,
		// interface{} scalar values are JSON-decoded and treated as immutable (not deep-copied).
		Value:            a.Value,
		Modifier:         datamodels.PropertyModifier(a.Modifier),
		TargetUID:        a.TargetUID,
		TargetUIDs:       append([]string(nil), a.TargetUIDs...),
		Speed:            copyFloat(a.Speed),
		Direction:        copyFloat(a.Direction),
		EventUID:         a.EventUID,
		EventData:        a.EventData,
		NewState:         a.NewState,
		SpawnAgentUID:    a.SpawnAgentUID,
		SpawnCount:       a.SpawnCount.or(0),
		SpawnProperties:  cloneAnyMap(a.SpawnProperties),
		SpawnInheritFrom: append([]string(nil), a.SpawnInheritFrom...),
		Parameters:       cloneAnyMap(a.Parameters),
		Description:      a.Description,
		LLMPromptContext: a.LLMPromptContext,
	}
	if a.SpawnPosition != nil {
		out.SpawnPosition = &datamodels.SpawnPosition{
			Type:     datamodels.SpawnPositionType(a.SpawnPosition.Type),
			OffsetX:  copyFloat(a.SpawnPosition.OffsetX),
			OffsetY:  copyFloat(a.SpawnPosition.OffsetY),
			Radius:   copyFloat(a.SpawnPosition.Radius),
			ShapeUID: a.SpawnPosition.ShapeUID,
		}
	}
	return out
}

// ===================== interactions =====================

func mergeInteractions(existing []datamodels.Interaction, incoming []LLMInteraction, scopeUID string, warnings []MergeWarning) ([]datamodels.Interaction, []MergeWarning) {
	out := append([]datamodels.Interaction(nil), existing...)
	index := make(map[string]int, len(out))
	for i, in := range out {
		index[in.ID] = i
	}
	for _, in := range incoming {
		if in.ID == "" {
			warnings = append(warnings, MergeWarning{Path: "interactions", Reason: "incoming interaction has empty id; skipped"})
			continue
		}
		if scopeUID != "" && in.ID != scopeUID {
			warnings = append(warnings, MergeWarning{Path: "interactions[" + in.ID + "]", Reason: "outside the requested entity scope; ignored"})
			continue
		}
		if i, ok := index[in.ID]; ok {
			if out[i].Locked {
				warnings = append(warnings, MergeWarning{Path: "interactions[" + in.ID + "]", Reason: "interaction " + in.ID + " is locked; ignored"})
				continue
			}
			out[i] = mergeInteraction(out[i], in) // preserves Locked + omitted fields.
		} else {
			out = append(out, newInteraction(in))
			index[in.ID] = len(out) - 1
		}
	}
	return out, warnings
}

// mergeInteraction updates prev with the fields in supplies; an absent field
// keeps prev. A non-nil (possibly empty) slice replaces the previous list.
// Type falls back to prev's value on an empty/invalid string. Locked is
// user-only and preserved.
func mergeInteraction(prev datamodels.Interaction, in LLMInteraction) datamodels.Interaction {
	it := prev
	it.ID = in.ID
	if in.Name != "" {
		it.Name = in.Name
	}
	if in.Description != "" {
		it.Description = in.Description
	}
	if in.Type != "" {
		it.Type = datamodels.InteractionType(in.Type)
	}
	if in.InitiatorUIDs != nil {
		it.InitiatorUIDs = append([]string(nil), in.InitiatorUIDs...)
	}
	if in.TargetUIDs != nil {
		it.TargetUIDs = append([]string(nil), in.TargetUIDs...)
	}
	if in.Radius != nil {
		it.Radius = copyFloat(in.Radius)
	}
	if in.Conditions != nil {
		it.Conditions = conditionsFromLLM(in.Conditions)
	}
	if in.InitiatorActions != nil {
		it.InitiatorActions = actionsFromLLM(in.InitiatorActions)
	}
	if in.TargetActions != nil {
		it.TargetActions = actionsFromLLM(in.TargetActions)
	}
	if in.Bidirectional != nil {
		it.Bidirectional = bool(*in.Bidirectional)
	}
	if in.UseLLM != nil {
		it.UseLLM = bool(*in.UseLLM)
	}
	if in.Enabled != nil {
		it.Enabled = bool(*in.Enabled)
	}
	if in.LLMPromptContext != "" {
		it.LLMPromptContext = in.LLMPromptContext
	}
	return it
}

// newInteraction builds a NEW interaction. Absent bools take false, except
// Enabled which defaults to true — an LLM creating an interaction intends it
// active.
func newInteraction(in LLMInteraction) datamodels.Interaction {
	return datamodels.Interaction{
		ID:               in.ID,
		Name:             in.Name,
		Description:      in.Description,
		Type:             datamodels.InteractionType(in.Type),
		InitiatorUIDs:    append([]string(nil), in.InitiatorUIDs...),
		TargetUIDs:       append([]string(nil), in.TargetUIDs...),
		Radius:           copyFloat(in.Radius),
		Conditions:       conditionsFromLLM(in.Conditions),
		InitiatorActions: actionsFromLLM(in.InitiatorActions),
		TargetActions:    actionsFromLLM(in.TargetActions),
		Bidirectional:    in.Bidirectional.or(false),
		UseLLM:           in.UseLLM.or(false),
		Enabled:          in.Enabled.or(true),
		LLMPromptContext: in.LLMPromptContext,
	}
}

// ===================== events =====================

func mergeEvents(existing []datamodels.EventDefinition, incoming []LLMEvent, scopeUID string, warnings []MergeWarning) ([]datamodels.EventDefinition, []MergeWarning) {
	out := append([]datamodels.EventDefinition(nil), existing...)
	index := make(map[string]int, len(out))
	for i, e := range out {
		index[e.UID] = i
	}
	for _, in := range incoming {
		if in.UID == "" {
			warnings = append(warnings, MergeWarning{Path: "events", Reason: "incoming event has empty uid; skipped"})
			continue
		}
		if scopeUID != "" && in.UID != scopeUID {
			warnings = append(warnings, MergeWarning{Path: "events[" + in.UID + "]", Reason: "outside the requested entity scope; ignored"})
			continue
		}
		if i, ok := index[in.UID]; ok {
			if out[i].Locked {
				warnings = append(warnings, MergeWarning{Path: "events[" + in.UID + "]", Reason: "event " + in.UID + " is locked; ignored"})
				continue
			}
			out[i], warnings = eventFromLLM(in, out[i], warnings)
		} else {
			e, w := eventFromLLM(in, datamodels.EventDefinition{}, warnings)
			warnings = w
			out = append(out, e)
			index[in.UID] = len(out) - 1
		}
	}
	return out, warnings
}

// eventFromLLM merges in onto prev (zero on create); an absent field keeps prev.
// A non-nil (possibly empty) slice replaces the previous list. LLMTarget and
// Locked are user-only and preserved.
func eventFromLLM(in LLMEvent, prev datamodels.EventDefinition, warnings []MergeWarning) (datamodels.EventDefinition, []MergeWarning) {
	path := "events[" + in.UID + "]"
	e := prev // keeps LLMTarget, Locked (user-only) and any field the LLM omits.
	e.UID = in.UID
	if in.Name != "" {
		e.Name = in.Name
	}
	if in.Description != "" {
		e.Description = in.Description
	}
	if in.Radius != nil {
		e.Radius = copyFloat(in.Radius)
	}
	if in.ProducerUIDs != nil {
		e.ProducerUIDs = append([]string(nil), in.ProducerUIDs...)
	}
	if in.ConsumerUIDs != nil {
		e.ConsumerUIDs = append([]string(nil), in.ConsumerUIDs...)
	}
	if in.TTLSteps != nil {
		e.TTLSteps = uint(*in.TTLSteps)
	}
	if in.LLMPromptContext != "" {
		e.LLMPromptContext = in.LLMPromptContext
	}
	if in.Scope != "" {
		if validEventScope(datamodels.EventScope(in.Scope)) {
			e.Scope = datamodels.EventScope(in.Scope)
		} else {
			warnings = append(warnings, MergeWarning{Path: path + ".scope", Reason: "invalid event scope " + in.Scope + "; previous kept"})
		}
	}
	if in.Producers != nil {
		producers := make([]datamodels.EventProducer, 0, len(in.Producers))
		for _, p := range in.Producers {
			if validEventProducer(datamodels.EventProducer(p)) {
				producers = append(producers, datamodels.EventProducer(p))
			} else {
				warnings = append(warnings, MergeWarning{Path: path + ".producers", Reason: "invalid event producer " + p + "; dropped"})
			}
		}
		e.Producers = producers
	}
	e.Payload, warnings = mergeProperties(prev.Payload, in.Payload, path+".payload", warnings)
	return e, warnings
}

// ===================== widgets =====================

// mergeWidgets upserts incoming widgets by ID into the model's widget list.
// Widgets are model-level templates with NO locked flag, so no lock handling
// applies. Each resulting widget is structurally validated via
// common.ValidateWidget: an invalid update keeps the previous widget, an invalid
// new widget is skipped — both with a warning. A nil incoming slice keeps the
// existing list (no delete-by-omission).
func mergeWidgets(existing []common.WidgetConfig, incoming []LLMWidget, scopeUID string, warnings []MergeWarning) ([]common.WidgetConfig, []MergeWarning) {
	out := append([]common.WidgetConfig(nil), existing...)
	index := make(map[string]int, len(out))
	for i, w := range out {
		index[w.ID] = i
	}
	for _, in := range incoming {
		if in.ID == "" {
			warnings = append(warnings, MergeWarning{Path: "widgets", Reason: "incoming widget has empty id; skipped"})
			continue
		}
		if scopeUID != "" && in.ID != scopeUID {
			warnings = append(warnings, MergeWarning{Path: "widgets[" + in.ID + "]", Reason: "outside the requested entity scope; ignored"})
			continue
		}
		path := "widgets[" + in.ID + "]"
		if i, ok := index[in.ID]; ok {
			candidate := widgetFromLLM(in, out[i])
			if issues := common.ValidateWidget(i, candidate); len(issues) > 0 {
				warnings = append(warnings, MergeWarning{Path: path, Reason: "structurally invalid (" + widgetIssueSummary(issues) + "); previous kept"})
				continue
			}
			out[i] = candidate
		} else {
			candidate := widgetFromLLM(in, common.WidgetConfig{})
			if issues := common.ValidateWidget(len(out), candidate); len(issues) > 0 {
				warnings = append(warnings, MergeWarning{Path: path, Reason: "structurally invalid (" + widgetIssueSummary(issues) + "); skipped"})
				continue
			}
			out = append(out, candidate)
			index[in.ID] = len(out) - 1
		}
	}
	return out, warnings
}

// widgetFromLLM merges in onto prev (zero on create); an absent field keeps
// prev's value, a non-nil (possibly empty) slice replaces the previous list.
func widgetFromLLM(in LLMWidget, prev common.WidgetConfig) common.WidgetConfig {
	w := prev
	w.ID = in.ID
	if in.Type != "" {
		w.Type = common.WidgetType(in.Type)
	}
	if in.Title != "" {
		w.Title = in.Title
	}
	if in.Source != nil {
		src := widgetSourceFromLLM(*in.Source)
		w.Source = &src
	}
	if in.Series != nil {
		series := make([]common.WidgetSeries, 0, len(in.Series))
		for _, s := range in.Series {
			series = append(series, common.WidgetSeries{Label: s.Label, Color: s.Color, Source: widgetSourceFromLLM(s.Source)})
		}
		w.Series = series
	}
	if in.Columns != nil {
		w.Columns = append([]string(nil), in.Columns...)
	}
	if in.Buckets != nil {
		w.Buckets = in.Buckets.std()
	}
	if in.LogKinds != nil {
		kinds := make([]common.WidgetLogKind, 0, len(in.LogKinds))
		for _, k := range in.LogKinds {
			kinds = append(kinds, common.WidgetLogKind(k))
		}
		w.LogKinds = kinds
	}
	if in.Position != nil {
		w.Position = common.WidgetPosition{X: uint(in.Position.X), Y: uint(in.Position.Y), W: uint(in.Position.W), H: uint(in.Position.H)}
	}
	return w
}

func widgetSourceFromLLM(in LLMWidgetSource) common.WidgetSource {
	return common.WidgetSource{
		Type:        common.WidgetSourceType(in.Type),
		PropertyUID: in.PropertyUID,
		AgentUIDs:   append([]string(nil), in.AgentUIDs...),
		Aggregation: common.WidgetAggregation(in.Aggregation),
		Metric:      common.WidgetMetric(in.Metric),
	}
}

// widgetIssueSummary flattens ValidateWidget issues into a single warning reason.
func widgetIssueSummary(issues []common.WidgetIssue) string {
	reasons := make([]string, 0, len(issues))
	for _, i := range issues {
		reasons = append(reasons, i.Reason)
	}
	return strings.Join(reasons, "; ")
}

// ===================== small helpers =====================

func cloneAgents(in []datamodels.Agent) []datamodels.Agent {
	out := make([]datamodels.Agent, len(in))
	for i, a := range in {
		a.Properties = append([]datamodels.AgentProperty(nil), a.Properties...)
		a.States = append([]string(nil), a.States...)
		// Initialization.Placement pointer is intentionally shallow here: the update
		// path (mergeAgent/newAgent) always overwrites Initialization wholesale via
		// initializationFromLLM, so retained entities never share the pointer with
		// a live input model.
		out[i] = a
	}
	return out
}

func copyFloat(p *float64) *float64 {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}

func pointsFromLLM(in []LLMPoint) []common.Point {
	if in == nil {
		return nil
	}
	out := make([]common.Point, 0, len(in))
	for _, p := range in {
		out = append(out, common.Point{X: uint64(p.X), Y: uint64(p.Y)})
	}
	return out
}

func pointsToLLM(in []common.Point) []LLMPoint {
	if in == nil {
		return nil
	}
	out := make([]LLMPoint, 0, len(in))
	for _, p := range in {
		out = append(out, LLMPoint{X: llmUint64(p.X), Y: llmUint64(p.Y)})
	}
	return out
}

func copyBool(p *bool) *bool {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}


// derefBool returns *p, or def when p is nil. Used on the new-entity path where an
// absent bool takes a schema default (e.g. Enabled defaults true for a new rule).
func derefBool(p *bool, def bool) bool {
	if p == nil {
		return def
	}
	return *p
}

// cloneAnyMap returns a shallow copy of m (nil stays nil). Values are JSON-decoded
// interface{} and treated as immutable; only the top-level map is cloned so the
// merged model never shares a backing map with the LLM envelope.
func cloneAnyMap(m map[string]interface{}) map[string]interface{} {
	if m == nil {
		return nil
	}
	out := make(map[string]interface{}, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
