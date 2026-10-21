package models

import (
	"intelli.sim.core/data/models"
	mongocommon "intelli.sim.core/data/mongodb/common"
)

// ---- top level ----

func ModelToDocument(m models.Model) (ModelDocument, error) {
	id, err := mongocommon.ParseOptionalID(m.ID)
	if err != nil {
		return ModelDocument{}, err
	}
	chatID, err := mongocommon.ParseOptionalID(m.ChatID)
	if err != nil {
		return ModelDocument{}, err
	}
	return ModelDocument{
		ID:               id,
		Name:             m.Name,
		Description:      m.Description,
		ChatID:           chatID,
		LLMConfig:        llmConfigToDocument(m.LLMConfig),
		SimulationConfig: simulationConfigToDocument(m.SimulationConfig),
		IsSpatial:        m.IsSpatial,
		SpatialCellSize:  m.SpatialCellSize,
		Agents:           mongocommon.MapSlice(m.Agents, agentToDocument),
		Environment:      environmentPtrToDocument(m.Environment),
		Rules:            mongocommon.MapSlice(m.Rules, ruleToDocument),
		Interactions:     mongocommon.MapSlice(m.Interactions, interactionToDocument),
		Events:           mongocommon.MapSlice(m.Events, eventToDocument),
		Images:           mongocommon.MapSlice(m.Images, imageToDocument),
		Widgets:          mongocommon.WidgetConfigsToDocuments(m.Widgets),
		CreatedAt:        m.CreatedAt,
		UpdatedAt:        m.UpdatedAt,
		CreatedBy:        m.CreatedBy,
		UpdatedBy:        m.UpdatedBy,
	}, nil
}

func ModelFromDocument(d ModelDocument) models.Model {
	return models.Model{
		ID:               mongocommon.FormatOptionalID(d.ID),
		Name:             d.Name,
		Description:      d.Description,
		ChatID:           mongocommon.FormatOptionalID(d.ChatID),
		LLMConfig:        llmConfigFromDocument(d.LLMConfig),
		SimulationConfig: simulationConfigFromDocument(d.SimulationConfig),
		IsSpatial:        d.IsSpatial,
		SpatialCellSize:  d.SpatialCellSize,
		Agents:           mongocommon.MapSlice(d.Agents, agentFromDocument),
		Environment:      environmentPtrFromDocument(d.Environment),
		Rules:            mongocommon.MapSlice(d.Rules, ruleFromDocument),
		Interactions:     mongocommon.MapSlice(d.Interactions, interactionFromDocument),
		Events:           mongocommon.MapSlice(d.Events, eventFromDocument),
		Images:           mongocommon.MapSlice(d.Images, imageFromDocument),
		Widgets:          mongocommon.WidgetConfigsFromDocuments(d.Widgets),
		CreatedAt:        d.CreatedAt,
		UpdatedAt:        d.UpdatedAt,
		CreatedBy:        d.CreatedBy,
		UpdatedBy:        d.UpdatedBy,
	}
}

// ---- images ----

func imageToDocument(i models.ModelImage) ModelImageDocument {
	return ModelImageDocument{UID: i.UID, Name: i.Name, MimeType: i.MimeType, Data: i.Data, Description: i.Description}
}

func imageFromDocument(d ModelImageDocument) models.ModelImage {
	return models.ModelImage{UID: d.UID, Name: d.Name, MimeType: d.MimeType, Data: d.Data, Description: d.Description}
}

// ---- agents ----

func agentToDocument(a models.Agent) AgentDocument {
	return AgentDocument{
		UID: a.UID, Name: a.Name, Description: a.Description,
		Type: a.Type, MoveType: a.MoveType,
		Properties:     mongocommon.MapSlice(a.Properties, propertyToDocument),
		States:         append([]string(nil), a.States...),
		Initialization: initializationToDocument(a.Initialization),
		LLMConfig:      llmAgentConfigPtrToDocument(a.LLMConfig),
		Footprint:      footprintPtrToDocument(a.Footprint),
		Display:        displayPtrToDocument(a.Display),
		Collidable:     a.Collidable,
		Locked:         a.Locked,
		CreatedAt:      a.CreatedAt, UpdatedAt: a.UpdatedAt,
	}
}

func agentFromDocument(d AgentDocument) models.Agent {
	return models.Agent{
		UID: d.UID, Name: d.Name, Description: d.Description,
		Type: d.Type, MoveType: d.MoveType,
		Properties:     mongocommon.MapSlice(d.Properties, propertyFromDocument),
		States:         append([]string(nil), d.States...),
		Initialization: initializationFromDocument(d.Initialization),
		LLMConfig:      llmAgentConfigPtrFromDocument(d.LLMConfig),
		Footprint:      footprintPtrFromDocument(d.Footprint),
		Display:        displayPtrFromDocument(d.Display),
		Collidable:     d.Collidable,
		Locked:         d.Locked,
		CreatedAt:      d.CreatedAt, UpdatedAt: d.UpdatedAt,
	}
}

func propertyToDocument(p models.AgentProperty) AgentPropertyDocument {
	return AgentPropertyDocument{
		UID: p.UID, Name: p.Name, Type: p.Type, Description: p.Description,
		MinValue: p.MinValue, MaxValue: p.MaxValue,
		DefaultValue:     p.DefaultValue,
		AllowedValues:    append([]interface{}(nil), p.AllowedValues...),
		Immutable:        p.Immutable,
		LLMPromptContext: p.LLMPromptContext,
		Generation:       propertyGenerationPtrToDocument(p.Generation),
	}
}

func propertyFromDocument(d AgentPropertyDocument) models.AgentProperty {
	return models.AgentProperty{
		UID: d.UID, Name: d.Name, Type: d.Type, Description: d.Description,
		MinValue: d.MinValue, MaxValue: d.MaxValue,
		DefaultValue:     mongocommon.NormalizeValue(d.DefaultValue),
		AllowedValues:    mongocommon.NormalizeSlice(d.AllowedValues),
		Immutable:        d.Immutable,
		LLMPromptContext: d.LLMPromptContext,
		Generation:       propertyGenerationPtrFromDocument(d.Generation),
	}
}

func initializationToDocument(i models.AgentInitialization) AgentInitializationDocument {
	return AgentInitializationDocument{
		PopulationCount: i.PopulationCount, MinPopulation: i.MinPopulation, MaxPopulation: i.MaxPopulation,
		InitialState: i.InitialState, Placement: placementPtrToDocument(i.Placement),
		CountMode: i.CountMode,
	}
}

func initializationFromDocument(d AgentInitializationDocument) models.AgentInitialization {
	return models.AgentInitialization{
		PopulationCount: d.PopulationCount, MinPopulation: d.MinPopulation, MaxPopulation: d.MaxPopulation,
		InitialState: d.InitialState, Placement: placementPtrFromDocument(d.Placement),
		CountMode: d.CountMode,
	}
}

func placementPtrToDocument(p *models.Placement) *PlacementDocument {
	if p == nil {
		return nil
	}
	return &PlacementDocument{
		Type: p.Type, ShapeUID: p.ShapeUID, PromptContext: p.PromptContext,
		Clusters: p.Clusters, ClusterRadius: p.ClusterRadius,
	}
}

func placementPtrFromDocument(d *PlacementDocument) *models.Placement {
	if d == nil {
		return nil
	}
	return &models.Placement{
		Type: d.Type, ShapeUID: d.ShapeUID, PromptContext: d.PromptContext,
		Clusters: d.Clusters, ClusterRadius: d.ClusterRadius,
	}
}

// ---- environment / shapes ----

func environmentPtrToDocument(e *models.Environment) *EnvironmentDocument {
	if e == nil {
		return nil
	}
	return &EnvironmentDocument{
		UID: e.UID, Width: e.Width, Height: e.Height, Torus: e.Torus,
		Shapes:           mongocommon.MapSlice(e.Shapes, shapeToDocument),
		GlobalProperties: mongocommon.MapSlice(e.GlobalProperties, propertyToDocument),
		CollisionMode:    e.CollisionMode,
		CollisionConfig:  CollisionConfigPtrToDocument(e.CollisionConfig),
		LLMPromptContext: e.LLMPromptContext,
	}
}

// CollisionConfigPtrToDocument / CollisionConfigPtrFromDocument expose the U5
// collision-config mapping for other document packages (simulations env doc).
func CollisionConfigPtrToDocument(c *models.CollisionConfig) *CollisionConfigDocument {
	if c == nil {
		return nil
	}
	return &CollisionConfigDocument{MaxIterations: c.MaxIterations, StuckPolicy: c.StuckPolicy}
}

// CollisionConfigPtrFromDocument is the inverse of CollisionConfigPtrToDocument.
func CollisionConfigPtrFromDocument(d *CollisionConfigDocument) *models.CollisionConfig {
	if d == nil {
		return nil
	}
	return &models.CollisionConfig{MaxIterations: d.MaxIterations, StuckPolicy: d.StuckPolicy}
}

func environmentPtrFromDocument(d *EnvironmentDocument) *models.Environment {
	if d == nil {
		return nil
	}
	return &models.Environment{
		UID: d.UID, Width: d.Width, Height: d.Height, Torus: d.Torus,
		Shapes:           mongocommon.MapSlice(d.Shapes, shapeFromDocument),
		GlobalProperties: mongocommon.MapSlice(d.GlobalProperties, propertyFromDocument),
		CollisionMode:    d.CollisionMode,
		CollisionConfig:  CollisionConfigPtrFromDocument(d.CollisionConfig),
		LLMPromptContext: d.LLMPromptContext,
	}
}

func shapeToDocument(s models.Shape) ShapeDocument {
	return ShapeDocument{
		UID: s.UID, Name: s.Name, Description: s.Description, Type: s.Type,
		Layer: s.Layer, Visible: s.Visible,
		Properties:     mongocommon.MapSlice(s.Properties, propertyToDocument),
		Initialization: initializationToDocument(s.Initialization),
		Geometry:       geometryPtrToDocument(s.Geometry),
		Display:        displayPtrToDocument(s.Display),
		LLMConfig:      llmShapeConfigPtrToDocument(s.LLMConfig),
		Locked:         s.Locked,
	}
}

func shapeFromDocument(d ShapeDocument) models.Shape {
	return models.Shape{
		UID: d.UID, Name: d.Name, Description: d.Description, Type: d.Type,
		Layer: d.Layer, Visible: d.Visible,
		Properties:     mongocommon.MapSlice(d.Properties, propertyFromDocument),
		Initialization: initializationFromDocument(d.Initialization),
		Geometry:       geometryPtrFromDocument(d.Geometry),
		Display:        displayPtrFromDocument(d.Display),
		LLMConfig:      llmShapeConfigPtrFromDocument(d.LLMConfig),
		Locked:         d.Locked,
	}
}

func scalarSpecPtrToDocument(s *models.ScalarSpec) *ScalarSpecDocument {
	if s == nil {
		return nil
	}
	return &ScalarSpecDocument{Value: s.Value, Min: s.Min, Max: s.Max}
}

func scalarSpecPtrFromDocument(d *ScalarSpecDocument) *models.ScalarSpec {
	if d == nil {
		return nil
	}
	return &models.ScalarSpec{Value: d.Value, Min: d.Min, Max: d.Max}
}

func geometryPtrToDocument(g *models.ShapeGeometry) *ShapeGeometryDocument {
	if g == nil {
		return nil
	}
	return &ShapeGeometryDocument{
		Position:  mongocommon.PointPtrToDocument(g.Position),
		Width:     scalarSpecPtrToDocument(g.Width),
		Height:    scalarSpecPtrToDocument(g.Height),
		Radius:    scalarSpecPtrToDocument(g.Radius),
		Points:    mongocommon.MapSlice(g.Points, mongocommon.PointToDocument),
		Thickness: g.Thickness, Fill: g.Fill, Stroke: g.Stroke, StrokeWidth: g.StrokeWidth,
	}
}

func geometryPtrFromDocument(d *ShapeGeometryDocument) *models.ShapeGeometry {
	if d == nil {
		return nil
	}
	return &models.ShapeGeometry{
		Position:  mongocommon.PointPtrFromDocument(d.Position),
		Width:     scalarSpecPtrFromDocument(d.Width),
		Height:    scalarSpecPtrFromDocument(d.Height),
		Radius:    scalarSpecPtrFromDocument(d.Radius),
		Points:    mongocommon.MapSlice(d.Points, mongocommon.PointFromDocument),
		Thickness: d.Thickness, Fill: d.Fill, Stroke: d.Stroke, StrokeWidth: d.StrokeWidth,
	}
}

// ---- agent footprint / display (shared spatial) ----

func footprintPtrToDocument(f *models.AgentFootprint) *AgentFootprintDocument {
	if f == nil {
		return nil
	}
	return &AgentFootprintDocument{
		Kind:   f.Kind,
		Radius: f.Radius,
		Width:  f.Width,
		Height: f.Height,
		Points: mongocommon.MapSlice(f.Points, mongocommon.PointToDocument),
	}
}

func footprintPtrFromDocument(d *AgentFootprintDocument) *models.AgentFootprint {
	if d == nil {
		return nil
	}
	return &models.AgentFootprint{
		Kind:   d.Kind,
		Radius: d.Radius,
		Width:  d.Width,
		Height: d.Height,
		Points: mongocommon.MapSlice(d.Points, mongocommon.PointFromDocument),
	}
}

func displayPtrToDocument(d *models.Display) *DisplayDocument {
	if d == nil {
		return nil
	}
	return &DisplayDocument{
		Kind:   d.Kind,
		Image:  displayImagePtrToDocument(d.Image),
		Figure: mongocommon.MapSlice(d.Figure, mongocommon.PointToDocument),
		Scale:  d.Scale,
	}
}

func displayPtrFromDocument(d *DisplayDocument) *models.Display {
	if d == nil {
		return nil
	}
	return &models.Display{
		Kind:   d.Kind,
		Image:  displayImagePtrFromDocument(d.Image),
		Figure: mongocommon.MapSlice(d.Figure, mongocommon.PointFromDocument),
		Scale:  d.Scale,
	}
}

func displayImagePtrToDocument(i *models.DisplayImageData) *DisplayImageDocument {
	if i == nil {
		return nil
	}
	return &DisplayImageDocument{Data: i.Data, MimeType: i.MimeType, Width: i.Width, Height: i.Height}
}

func displayImagePtrFromDocument(d *DisplayImageDocument) *models.DisplayImageData {
	if d == nil {
		return nil
	}
	return &models.DisplayImageData{Data: d.Data, MimeType: d.MimeType, Width: d.Width, Height: d.Height}
}

// ---- rules / conditions / actions / interactions ----

func ruleToDocument(r models.Rule) RuleDocument {
	return RuleDocument{
		ID: r.ID, Name: r.Name, Description: r.Description, Priority: r.Priority,
		AppliesTo:   append([]string(nil), r.AppliesTo...),
		Conditions:  mongocommon.MapSlice(r.Conditions, conditionToDocument),
		Actions:     mongocommon.MapSlice(r.Actions, actionToDocument),
		ExecuteOnce: r.ExecuteOnce, CooldownSteps: r.CooldownSteps, Enabled: r.Enabled,
		Locked: r.Locked,
	}
}

func ruleFromDocument(d RuleDocument) models.Rule {
	return models.Rule{
		ID: d.ID, Name: d.Name, Description: d.Description, Priority: d.Priority,
		AppliesTo:   append([]string(nil), d.AppliesTo...),
		Conditions:  mongocommon.MapSlice(d.Conditions, conditionFromDocument),
		Actions:     mongocommon.MapSlice(d.Actions, actionFromDocument),
		ExecuteOnce: d.ExecuteOnce, CooldownSteps: d.CooldownSteps, Enabled: d.Enabled,
		Locked: d.Locked,
	}
}

// ConditionToDocument / ConditionFromDocument expose the condition mapping for
// other document packages (the simulations documents persist the U2 termination
// condition with the same encoding rules used for rule conditions).
func ConditionToDocument(c models.Condition) ConditionDocument { return conditionToDocument(c) }

// ConditionFromDocument is the inverse of ConditionToDocument.
func ConditionFromDocument(d ConditionDocument) models.Condition { return conditionFromDocument(d) }

func conditionToDocument(c models.Condition) ConditionDocument {
	return ConditionDocument{
		Type: c.Type, PropertyUID: c.PropertyUID, Operator: c.Operator,
		Value: c.Value, MinValue: c.MinValue, MaxValue: c.MaxValue, Radius: c.Radius,
		TargetUID: c.TargetUID, TargetUIDs: append([]string(nil), c.TargetUIDs...),
		LogicOperator: c.LogicOperator,
		SubConditions: mongocommon.MapSlice(c.SubConditions, conditionToDocument),
		Parameters:    mongocommon.CopyMap(c.Parameters),
		Description:   c.Description,
	}
}

func conditionFromDocument(d ConditionDocument) models.Condition {
	return models.Condition{
		Type: d.Type, PropertyUID: d.PropertyUID, Operator: d.Operator,
		Value: mongocommon.NormalizeValue(d.Value), MinValue: d.MinValue, MaxValue: d.MaxValue, Radius: d.Radius,
		TargetUID: d.TargetUID, TargetUIDs: append([]string(nil), d.TargetUIDs...),
		LogicOperator: d.LogicOperator,
		SubConditions: mongocommon.MapSlice(d.SubConditions, conditionFromDocument),
		Parameters:    mongocommon.NormalizeMap(d.Parameters),
		Description:   d.Description,
	}
}

func actionToDocument(a models.Action) ActionDocument {
	return ActionDocument{
		Type: a.Type, PropertyUID: a.PropertyUID, Value: a.Value, Modifier: a.Modifier,
		TargetUID: a.TargetUID, TargetUIDs: append([]string(nil), a.TargetUIDs...),
		Speed: a.Speed, Direction: a.Direction,
		EventUID: a.EventUID, EventData: a.EventData, NewState: a.NewState,
		SpawnAgentUID: a.SpawnAgentUID, SpawnCount: a.SpawnCount,
		SpawnProperties:  mongocommon.CopyMap(a.SpawnProperties),
		SpawnPosition:    spawnPositionPtrToDocument(a.SpawnPosition),
		SpawnInheritFrom: append([]string(nil), a.SpawnInheritFrom...),
		Parameters:       mongocommon.CopyMap(a.Parameters),
		Description:      a.Description,
		LLMPromptContext: a.LLMPromptContext,
	}
}

func actionFromDocument(d ActionDocument) models.Action {
	return models.Action{
		Type: d.Type, PropertyUID: d.PropertyUID, Value: mongocommon.NormalizeValue(d.Value), Modifier: d.Modifier,
		TargetUID: d.TargetUID, TargetUIDs: append([]string(nil), d.TargetUIDs...),
		Speed: d.Speed, Direction: d.Direction,
		EventUID: d.EventUID, EventData: mongocommon.NormalizeValue(d.EventData), NewState: d.NewState,
		SpawnAgentUID: d.SpawnAgentUID, SpawnCount: d.SpawnCount,
		SpawnProperties:  mongocommon.NormalizeMap(d.SpawnProperties),
		SpawnPosition:    spawnPositionPtrFromDocument(d.SpawnPosition),
		SpawnInheritFrom: append([]string(nil), d.SpawnInheritFrom...),
		Parameters:       mongocommon.NormalizeMap(d.Parameters),
		Description:      d.Description,
		LLMPromptContext: d.LLMPromptContext,
	}
}

func spawnPositionPtrToDocument(s *models.SpawnPosition) *SpawnPositionDocument {
	if s == nil {
		return nil
	}
	return &SpawnPositionDocument{Type: s.Type, OffsetX: s.OffsetX, OffsetY: s.OffsetY, Radius: s.Radius, ShapeUID: s.ShapeUID}
}

func spawnPositionPtrFromDocument(d *SpawnPositionDocument) *models.SpawnPosition {
	if d == nil {
		return nil
	}
	return &models.SpawnPosition{Type: d.Type, OffsetX: d.OffsetX, OffsetY: d.OffsetY, Radius: d.Radius, ShapeUID: d.ShapeUID}
}

func interactionToDocument(i models.Interaction) InteractionDocument {
	return InteractionDocument{
		ID: i.ID, Name: i.Name, Description: i.Description, Type: i.Type,
		InitiatorUIDs:    append([]string(nil), i.InitiatorUIDs...),
		TargetUIDs:       append([]string(nil), i.TargetUIDs...),
		Radius:           i.Radius,
		Conditions:       mongocommon.MapSlice(i.Conditions, conditionToDocument),
		InitiatorActions: mongocommon.MapSlice(i.InitiatorActions, actionToDocument),
		TargetActions:    mongocommon.MapSlice(i.TargetActions, actionToDocument),
		Bidirectional:    i.Bidirectional, UseLLM: i.UseLLM, Enabled: i.Enabled,
		Locked:           i.Locked,
		LLMPromptContext: i.LLMPromptContext,
	}
}

func interactionFromDocument(d InteractionDocument) models.Interaction {
	return models.Interaction{
		ID: d.ID, Name: d.Name, Description: d.Description, Type: d.Type,
		InitiatorUIDs:    append([]string(nil), d.InitiatorUIDs...),
		TargetUIDs:       append([]string(nil), d.TargetUIDs...),
		Radius:           d.Radius,
		Conditions:       mongocommon.MapSlice(d.Conditions, conditionFromDocument),
		InitiatorActions: mongocommon.MapSlice(d.InitiatorActions, actionFromDocument),
		TargetActions:    mongocommon.MapSlice(d.TargetActions, actionFromDocument),
		Bidirectional:    d.Bidirectional, UseLLM: d.UseLLM, Enabled: d.Enabled,
		Locked:           d.Locked,
		LLMPromptContext: d.LLMPromptContext,
	}
}

// ---- events ----

func eventToDocument(e models.EventDefinition) EventDefinitionDocument {
	return EventDefinitionDocument{
		UID: e.UID, Name: e.Name, Description: e.Description,
		Scope: e.Scope, Radius: e.Radius,
		Payload:          mongocommon.MapSlice(e.Payload, propertyToDocument),
		Producers:        append([]models.EventProducer(nil), e.Producers...),
		ProducerUIDs:     append([]string(nil), e.ProducerUIDs...),
		ConsumerUIDs:     append([]string(nil), e.ConsumerUIDs...),
		TTLSteps:         e.TTLSteps,
		LLMPromptContext: e.LLMPromptContext,
		LLMTarget:        mongocommon.LLMTargetPtrToDocument(e.LLMTarget),
		Locked:           e.Locked,
	}
}

func eventFromDocument(d EventDefinitionDocument) models.EventDefinition {
	return models.EventDefinition{
		UID: d.UID, Name: d.Name, Description: d.Description,
		Scope: d.Scope, Radius: d.Radius,
		Payload:          mongocommon.MapSlice(d.Payload, propertyFromDocument),
		Producers:        append([]models.EventProducer(nil), d.Producers...),
		ProducerUIDs:     append([]string(nil), d.ProducerUIDs...),
		ConsumerUIDs:     append([]string(nil), d.ConsumerUIDs...),
		TTLSteps:         d.TTLSteps,
		LLMPromptContext: d.LLMPromptContext,
		LLMTarget:        mongocommon.LLMTargetPtrFromDocument(d.LLMTarget),
		Locked:           d.Locked,
	}
}

// ---- LLM config / strategy / fallback ----

func llmConfigToDocument(c models.LLMConfig) LLMConfigDocument {
	return LLMConfigDocument{
		Target:        mongocommon.LLMTargetToDocument(c.Target),
		PromptContext: c.PromptContext,
	}
}

func llmConfigFromDocument(d LLMConfigDocument) models.LLMConfig {
	return models.LLMConfig{
		Target:        mongocommon.LLMTargetFromDocument(d.Target),
		PromptContext: d.PromptContext,
	}
}

func simulationConfigToDocument(c models.SimulationConfig) SimulationConfigDocument {
	doc := SimulationConfigDocument{
		UpdateMode:                    c.UpdateMode,
		LLMSimulationGenerationConfig: llmConfigToDocument(c.LLMSimulationGenerationConfig),
		LLMSimulationExecutionConfig:  llmSimulationExecutionConfigToDocument(c.LLMSimulationExecutionConfig),
	}
	if c.Termination != nil {
		t := conditionToDocument(*c.Termination)
		doc.Termination = &t
	}
	return doc
}

func simulationConfigFromDocument(d SimulationConfigDocument) models.SimulationConfig {
	cfg := models.SimulationConfig{
		UpdateMode:                    d.UpdateMode,
		LLMSimulationGenerationConfig: llmConfigFromDocument(d.LLMSimulationGenerationConfig),
		LLMSimulationExecutionConfig:  llmSimulationExecutionConfigFromDocument(d.LLMSimulationExecutionConfig),
	}
	if d.Termination != nil {
		t := conditionFromDocument(*d.Termination)
		cfg.Termination = &t
	}
	return cfg
}

func llmAgentConfigPtrToDocument(c *models.LLMAgentConfig) *LLMAgentConfigDocument {
	if c == nil {
		return nil
	}
	return &LLMAgentConfigDocument{
		LLMConfigDocument: llmConfigToDocument(c.LLMConfig),
		Strategy:          llmAgentStrategyToDocument(c.Strategy),
		SimulationConfig:  llmSimulationAgentConfigPtrToDocument(c.SimulationConfig),
	}
}

func llmAgentConfigPtrFromDocument(d *LLMAgentConfigDocument) *models.LLMAgentConfig {
	if d == nil {
		return nil
	}
	return &models.LLMAgentConfig{
		LLMConfig:        llmConfigFromDocument(d.LLMConfigDocument),
		Strategy:         llmAgentStrategyFromDocument(d.Strategy),
		SimulationConfig: llmSimulationAgentConfigPtrFromDocument(d.SimulationConfig),
	}
}

func llmShapeConfigPtrToDocument(c *models.LLMShapeConfig) *LLMShapeConfigDocument {
	if c == nil {
		return nil
	}
	return &LLMShapeConfigDocument{
		LLMConfigDocument: llmConfigToDocument(c.LLMConfig),
		Strategy:          llmShapeStrategyToDocument(c.Strategy),
	}
}

func llmShapeConfigPtrFromDocument(d *LLMShapeConfigDocument) *models.LLMShapeConfig {
	if d == nil {
		return nil
	}
	return &models.LLMShapeConfig{
		LLMConfig: llmConfigFromDocument(d.LLMConfigDocument),
		Strategy:  llmShapeStrategyFromDocument(d.Strategy),
	}
}

func llmStrategyToDocument(s models.LLMStrategy) LLMStrategyDocument {
	return LLMStrategyDocument{
		Type: s.Type, BatchSize: s.BatchSize,
		UseForMovement: s.UseForMovement, UseForInteraction: s.UseForInteraction,
		UseForProperties: s.UseForProperties, UseForDecisions: s.UseForDecisions,
		DecisionFrequency: s.DecisionFrequency,
		ArchetypeMode:     s.ArchetypeMode, VariationStrength: s.VariationStrength,
	}
}

func llmStrategyFromDocument(d LLMStrategyDocument) models.LLMStrategy {
	return models.LLMStrategy{
		Type: d.Type, BatchSize: d.BatchSize,
		UseForMovement: d.UseForMovement, UseForInteraction: d.UseForInteraction,
		UseForProperties: d.UseForProperties, UseForDecisions: d.UseForDecisions,
		DecisionFrequency: d.DecisionFrequency,
		ArchetypeMode:     d.ArchetypeMode, VariationStrength: d.VariationStrength,
	}
}

func llmAgentStrategyToDocument(s models.LLMAgentStrategy) LLMAgentStrategyDocument {
	return LLMAgentStrategyDocument{
		LLMStrategyDocument: llmStrategyToDocument(s.LLMStrategy),
	}
}

func llmAgentStrategyFromDocument(d LLMAgentStrategyDocument) models.LLMAgentStrategy {
	return models.LLMAgentStrategy{
		LLMStrategy: llmStrategyFromDocument(d.LLMStrategyDocument),
	}
}

func llmShapeStrategyToDocument(s models.LLMShapeStrategy) LLMShapeStrategyDocument {
	return LLMShapeStrategyDocument{
		LLMStrategyDocument: llmStrategyToDocument(s.LLMStrategy),
	}
}

func llmShapeStrategyFromDocument(d LLMShapeStrategyDocument) models.LLMShapeStrategy {
	return models.LLMShapeStrategy{
		LLMStrategy: llmStrategyFromDocument(d.LLMStrategyDocument),
	}
}

func llmSimulationAgentConfigPtrToDocument(c *models.LLMSimulationAgentConfig) *LLMSimulationAgentConfigDocument {
	if c == nil {
		return nil
	}
	d := llmSimulationAgentConfigToDocument(*c)
	return &d
}

func llmSimulationAgentConfigPtrFromDocument(d *LLMSimulationAgentConfigDocument) *models.LLMSimulationAgentConfig {
	if d == nil {
		return nil
	}
	c := llmSimulationAgentConfigFromDocument(*d)
	return &c
}

func llmSimulationAgentConfigToDocument(c models.LLMSimulationAgentConfig) LLMSimulationAgentConfigDocument {
	return LLMSimulationAgentConfigDocument{
		PerceptionRadius: c.PerceptionRadius, InteractionRadius: c.InteractionRadius,
		MemorySize: c.MemorySize, FallbackPolicy: fallbackPolicyPtrToDocument(c.FallbackPolicy),
	}
}

func llmSimulationAgentConfigFromDocument(d LLMSimulationAgentConfigDocument) models.LLMSimulationAgentConfig {
	return models.LLMSimulationAgentConfig{
		PerceptionRadius: d.PerceptionRadius, InteractionRadius: d.InteractionRadius,
		MemorySize: d.MemorySize, FallbackPolicy: fallbackPolicyPtrFromDocument(d.FallbackPolicy),
	}
}

func llmSimulationExecutionConfigToDocument(c models.LLMSimulationExecutionConfig) LLMSimulationExecutionConfigDocument {
	return LLMSimulationExecutionConfigDocument{
		LLMConfigDocument:                llmConfigToDocument(c.LLMConfig),
		LLMSimulationAgentConfigDocument: llmSimulationAgentConfigToDocument(c.LLMSimulationAgentConfig),
		ParallelWorkers:                  c.ParallelWorkers,
		RetryTimeoutMs:                   c.RetryTimeoutMs,
		MaxRetries:                       c.MaxRetries,
		StreamBatchSize:                  c.StreamBatchSize,
		DefaultStrategy:                  llmStrategyToDocument(c.DefaultStrategy),
	}
}

func llmSimulationExecutionConfigFromDocument(d LLMSimulationExecutionConfigDocument) models.LLMSimulationExecutionConfig {
	return models.LLMSimulationExecutionConfig{
		LLMConfig:                llmConfigFromDocument(d.LLMConfigDocument),
		LLMSimulationAgentConfig: llmSimulationAgentConfigFromDocument(d.LLMSimulationAgentConfigDocument),
		ParallelWorkers:          d.ParallelWorkers,
		RetryTimeoutMs:           d.RetryTimeoutMs,
		MaxRetries:               d.MaxRetries,
		StreamBatchSize:          d.StreamBatchSize,
		DefaultStrategy:          llmStrategyFromDocument(d.DefaultStrategy),
	}
}

func fallbackPolicyPtrToDocument(p *models.FallbackPolicy) *FallbackPolicyDocument {
	if p == nil {
		return nil
	}
	return &FallbackPolicyDocument{Steps: mongocommon.MapSlice(p.Steps, fallbackStepToDocument)}
}

func fallbackPolicyPtrFromDocument(d *FallbackPolicyDocument) *models.FallbackPolicy {
	if d == nil {
		return nil
	}
	return &models.FallbackPolicy{Steps: mongocommon.MapSlice(d.Steps, fallbackStepFromDocument)}
}

func fallbackStepToDocument(s models.FallbackStep) FallbackStepDocument {
	return FallbackStepDocument{Type: s.Type, MaxRetries: s.MaxRetries, RetryDelayMs: s.RetryDelayMs}
}

func fallbackStepFromDocument(d FallbackStepDocument) models.FallbackStep {
	return models.FallbackStep{Type: d.Type, MaxRetries: d.MaxRetries, RetryDelayMs: d.RetryDelayMs}
}
