package models

import (
	datachats "intelli.sim.core/data/chats"
	"intelli.sim.core/data/common"
	datamodels "intelli.sim.core/data/models"
)

// ModelMapper converts between domain entities and API DTOs. Pure — the service
// supplies live values (validation, chat availability).
type ModelMapper struct{}

func NewModelMapper() *ModelMapper { return &ModelMapper{} }

// ===================== Model (domain -> DTO) =====================

func (m *ModelMapper) ToInputDTO(model datamodels.Model) ModelInputDTO {
	dto := ModelInputDTO{
		Name:             model.Name,
		Description:      model.Description,
		IsSpatial:        model.IsSpatial,
		SpatialCellSize:  model.SpatialCellSize,
		LLMConfig:        llmConfigToDTO(model.LLMConfig),
		SimulationConfig: simulationConfigToDTO(model.SimulationConfig),
	}
	if model.Agents != nil {
		dto.Agents = make([]AgentDTO, 0, len(model.Agents))
		for _, a := range model.Agents {
			dto.Agents = append(dto.Agents, agentToDTO(a))
		}
	}
	if model.Environment != nil {
		env := environmentToDTO(*model.Environment)
		dto.Environment = &env
	}
	if model.Rules != nil {
		dto.Rules = make([]RuleDTO, 0, len(model.Rules))
		for _, r := range model.Rules {
			dto.Rules = append(dto.Rules, ruleToDTO(r))
		}
	}
	if model.Interactions != nil {
		dto.Interactions = make([]InteractionDTO, 0, len(model.Interactions))
		for _, i := range model.Interactions {
			dto.Interactions = append(dto.Interactions, interactionToDTO(i))
		}
	}
	if model.Events != nil {
		dto.Events = make([]EventDefDTO, 0, len(model.Events))
		for _, e := range model.Events {
			dto.Events = append(dto.Events, eventToDTO(e))
		}
	}
	if model.Images != nil {
		dto.Images = make([]ModelImageDTO, 0, len(model.Images))
		for _, img := range model.Images {
			dto.Images = append(dto.Images, imageToDTO(img))
		}
	}
	if model.Widgets != nil {
		dto.Widgets = make([]WidgetConfigDTO, 0, len(model.Widgets))
		for _, w := range model.Widgets {
			dto.Widgets = append(dto.Widgets, widgetToDTO(w))
		}
	}
	return dto
}

// ===================== Model (DTO -> domain) =====================

func (m *ModelMapper) FromInputDTO(in ModelInputDTO) datamodels.Model {
	model := datamodels.Model{
		Name:             in.Name,
		Description:      in.Description,
		IsSpatial:        in.IsSpatial,
		SpatialCellSize:  in.SpatialCellSize,
		LLMConfig:        llmConfigFromDTO(in.LLMConfig),
		SimulationConfig: simulationConfigFromDTO(in.SimulationConfig),
	}
	if in.Agents != nil {
		model.Agents = make([]datamodels.Agent, 0, len(in.Agents))
		for _, a := range in.Agents {
			model.Agents = append(model.Agents, agentFromDTO(a))
		}
	}
	if in.Environment != nil {
		env := environmentFromDTO(*in.Environment)
		model.Environment = &env
	}
	if in.Rules != nil {
		model.Rules = make([]datamodels.Rule, 0, len(in.Rules))
		for _, r := range in.Rules {
			model.Rules = append(model.Rules, ruleFromDTO(r))
		}
	}
	if in.Interactions != nil {
		model.Interactions = make([]datamodels.Interaction, 0, len(in.Interactions))
		for _, i := range in.Interactions {
			model.Interactions = append(model.Interactions, interactionFromDTO(i))
		}
	}
	if in.Events != nil {
		model.Events = make([]datamodels.EventDefinition, 0, len(in.Events))
		for _, e := range in.Events {
			model.Events = append(model.Events, eventFromDTO(e))
		}
	}
	if in.Images != nil {
		model.Images = make([]datamodels.ModelImage, 0, len(in.Images))
		for _, img := range in.Images {
			model.Images = append(model.Images, imageFromDTO(img))
		}
	}
	if in.Widgets != nil {
		model.Widgets = make([]common.WidgetConfig, 0, len(in.Widgets))
		for _, w := range in.Widgets {
			model.Widgets = append(model.Widgets, widgetFromDTO(w))
		}
	}
	return model
}

// ===================== top-level responses =====================

func (m *ModelMapper) ToModelResponse(model datamodels.Model, v ValidationResult) ModelResponse {
	return ModelResponse{
		ID:         model.ID,
		ChatID:     model.ChatID,
		Model:      m.ToInputDTO(model),
		Validation: v,
		CreatedAt:  model.CreatedAt,
		UpdatedAt:  model.UpdatedAt,
		CreatedBy:  model.CreatedBy,
		UpdatedBy:  model.UpdatedBy,
	}
}

func (m *ModelMapper) ToSummary(model datamodels.Model, hasChat, simReady bool) ModelSummaryResponse {
	return ModelSummaryResponse{
		ID:              model.ID,
		Name:            model.Name,
		Description:     model.Description,
		IsSpatial:       model.IsSpatial,
		HasChat:         hasChat,
		AgentCount:      len(model.Agents),
		SimulationReady: simReady,
		UpdatedAt:       model.UpdatedAt,
		CreatedBy:       model.CreatedBy,
	}
}

// ===================== chat messages =====================

func (m *ModelMapper) ToMessageResponse(msg datachats.ChatMessage) ModelChatMessageResponse {
	resp := ModelChatMessageResponse{
		ID:        msg.ID,
		ChatID:    msg.ChatID,
		Role:      string(msg.Role),
		Content:   msg.TextContent,
		Raw:       msg.RawContent,
		Usage:     msg.TokenUsage,
		CreatedAt: msg.CreatedAt,
		CreatedBy: msg.CreatedBy,
	}
	if msg.LLMTarget != nil {
		dto := execDTOFromTarget(*msg.LLMTarget)
		resp.LLMTarget = &dto
	}
	if msg.Error != nil {
		resp.Error = &MessageErrorDTO{Code: msg.Error.Code, Message: msg.Error.Message}
	}
	return resp
}

func (m *ModelMapper) ToMessagesResponse(msgs []datachats.ChatMessage) []ModelChatMessageResponse {
	out := make([]ModelChatMessageResponse, 0, len(msgs))
	for _, msg := range msgs {
		out = append(out, m.ToMessageResponse(msg))
	}
	return out
}

// ===================== agents =====================

func agentToDTO(a datamodels.Agent) AgentDTO {
	dto := AgentDTO{
		UID:            a.UID,
		Name:           a.Name,
		Description:    a.Description,
		Type:           string(a.Type),
		MoveType:       string(a.MoveType),
		States:         a.States,
		Initialization: agentInitToDTO(a.Initialization),
		Locked:         a.Locked,
		CreatedAt:      a.CreatedAt,
		UpdatedAt:      a.UpdatedAt,
	}
	dto.Properties = propertiesToDTO(a.Properties)
	if a.LLMConfig != nil {
		c := llmAgentConfigToDTO(*a.LLMConfig)
		dto.LLMConfig = &c
	}
	dto.Footprint = footprintToDTO(a.Footprint)
	dto.Display = displayToDTO(a.Display)
	dto.Collidable = a.Collidable
	return dto
}

func agentFromDTO(a AgentDTO) datamodels.Agent {
	out := datamodels.Agent{
		UID:            a.UID,
		Name:           a.Name,
		Description:    a.Description,
		Type:           datamodels.AgentType(a.Type),
		MoveType:       datamodels.AgentMoveType(a.MoveType),
		States:         a.States,
		Initialization: agentInitFromDTO(a.Initialization),
		Locked:         a.Locked,
		CreatedAt:      a.CreatedAt,
		UpdatedAt:      a.UpdatedAt,
	}
	out.Properties = propertiesFromDTO(a.Properties)
	if a.LLMConfig != nil {
		c := llmAgentConfigFromDTO(*a.LLMConfig)
		out.LLMConfig = &c
	}
	out.Footprint = footprintFromDTO(a.Footprint)
	out.Display = displayFromDTO(a.Display)
	out.Collidable = a.Collidable
	return out
}

func propertiesToDTO(props []datamodels.AgentProperty) []AgentPropertyDTO {
	if props == nil {
		return nil
	}
	out := make([]AgentPropertyDTO, 0, len(props))
	for _, p := range props {
		out = append(out, AgentPropertyDTO{
			UID:              p.UID,
			Name:             p.Name,
			Type:             string(p.Type),
			Description:      p.Description,
			MinValue:         p.MinValue,
			MaxValue:         p.MaxValue,
			DefaultValue:     p.DefaultValue,
			AllowedValues:    p.AllowedValues,
			Immutable:        p.Immutable,
			LLMPromptContext: p.LLMPromptContext,
			Generation:       generationToDTO(p.Generation),
		})
	}
	return out
}

func propertiesFromDTO(props []AgentPropertyDTO) []datamodels.AgentProperty {
	if props == nil {
		return nil
	}
	out := make([]datamodels.AgentProperty, 0, len(props))
	for _, p := range props {
		out = append(out, datamodels.AgentProperty{
			UID:              p.UID,
			Name:             p.Name,
			Type:             datamodels.AgentPropertyType(p.Type),
			Description:      p.Description,
			MinValue:         p.MinValue,
			MaxValue:         p.MaxValue,
			DefaultValue:     p.DefaultValue,
			AllowedValues:    p.AllowedValues,
			Immutable:        p.Immutable,
			LLMPromptContext: p.LLMPromptContext,
			Generation:       generationFromDTO(p.Generation),
		})
	}
	return out
}

func generationToDTO(g *datamodels.PropertyGeneration) *PropertyGenerationDTO {
	if g == nil {
		return nil
	}
	dto := &PropertyGenerationDTO{
		Mode:          string(g.Mode),
		Distribution:  string(g.Distribution),
		Mean:          g.Mean,
		StdDev:        g.StdDev,
		Min:           g.Min,
		Max:           g.Max,
		PromptContext: g.PromptContext,
	}
	if g.Choices != nil {
		dto.Choices = make([]ChoiceWeightDTO, 0, len(g.Choices))
		for _, c := range g.Choices {
			dto.Choices = append(dto.Choices, ChoiceWeightDTO{Value: c.Value, Weight: c.Weight})
		}
	}
	return dto
}

func generationFromDTO(g *PropertyGenerationDTO) *datamodels.PropertyGeneration {
	if g == nil {
		return nil
	}
	out := &datamodels.PropertyGeneration{
		Mode:          datamodels.PropertyGenerationMode(g.Mode),
		Distribution:  datamodels.DistributionType(g.Distribution),
		Mean:          g.Mean,
		StdDev:        g.StdDev,
		Min:           g.Min,
		Max:           g.Max,
		PromptContext: g.PromptContext,
	}
	if g.Choices != nil {
		out.Choices = make([]datamodels.ChoiceWeight, 0, len(g.Choices))
		for _, c := range g.Choices {
			out.Choices = append(out.Choices, datamodels.ChoiceWeight{Value: c.Value, Weight: c.Weight})
		}
	}
	return out
}

func agentInitToDTO(init datamodels.AgentInitialization) AgentInitializationDTO {
	dto := AgentInitializationDTO{
		PopulationCount: init.PopulationCount,
		MinPopulation:   init.MinPopulation,
		MaxPopulation:   init.MaxPopulation,
		InitialState:    init.InitialState,
		CountMode:       string(init.CountMode),
	}
	if init.Placement != nil {
		p := placementToDTO(*init.Placement)
		dto.Placement = &p
	}
	return dto
}

func agentInitFromDTO(init AgentInitializationDTO) datamodels.AgentInitialization {
	out := datamodels.AgentInitialization{
		PopulationCount: init.PopulationCount,
		MinPopulation:   init.MinPopulation,
		MaxPopulation:   init.MaxPopulation,
		InitialState:    init.InitialState,
		CountMode:       datamodels.PopulationCountMode(init.CountMode),
	}
	if init.Placement != nil {
		p := placementFromDTO(*init.Placement)
		out.Placement = &p
	}
	return out
}

func placementToDTO(p datamodels.Placement) PlacementDTO {
	return PlacementDTO{
		Type:          string(p.Type),
		ShapeUID:      p.ShapeUID,
		PromptContext: p.PromptContext,
		Clusters:      p.Clusters,
		ClusterRadius: p.ClusterRadius,
	}
}

func placementFromDTO(p PlacementDTO) datamodels.Placement {
	return datamodels.Placement{
		Type:          datamodels.PlacementType(p.Type),
		ShapeUID:      p.ShapeUID,
		PromptContext: p.PromptContext,
		Clusters:      p.Clusters,
		ClusterRadius: p.ClusterRadius,
	}
}

// ===================== environment + shapes =====================

func environmentToDTO(env datamodels.Environment) EnvironmentDTO {
	dto := EnvironmentDTO{
		UID:              env.UID,
		Width:            env.Width,
		Height:           env.Height,
		Torus:            env.Torus,
		GlobalProperties: propertiesToDTO(env.GlobalProperties),
		CollisionMode:    string(env.CollisionMode),
		LLMPromptContext: env.LLMPromptContext,
	}
	if env.CollisionConfig != nil {
		dto.CollisionConfig = &CollisionConfigDTO{
			MaxIterations: env.CollisionConfig.MaxIterations,
			StuckPolicy:   string(env.CollisionConfig.StuckPolicy),
		}
	}
	if env.Shapes != nil {
		dto.Shapes = make([]ShapeDTO, 0, len(env.Shapes))
		for _, s := range env.Shapes {
			dto.Shapes = append(dto.Shapes, shapeToDTO(s))
		}
	}
	return dto
}

func environmentFromDTO(env EnvironmentDTO) datamodels.Environment {
	out := datamodels.Environment{
		UID:              env.UID,
		Width:            env.Width,
		Height:           env.Height,
		Torus:            env.Torus,
		GlobalProperties: propertiesFromDTO(env.GlobalProperties),
		CollisionMode:    datamodels.CollisionMode(env.CollisionMode),
		LLMPromptContext: env.LLMPromptContext,
	}
	if env.CollisionConfig != nil {
		out.CollisionConfig = &datamodels.CollisionConfig{
			MaxIterations: env.CollisionConfig.MaxIterations,
			StuckPolicy:   datamodels.CollisionStuckPolicy(env.CollisionConfig.StuckPolicy),
		}
	}
	if env.Shapes != nil {
		out.Shapes = make([]datamodels.Shape, 0, len(env.Shapes))
		for _, s := range env.Shapes {
			out.Shapes = append(out.Shapes, shapeFromDTO(s))
		}
	}
	return out
}

func shapeToDTO(s datamodels.Shape) ShapeDTO {
	dto := ShapeDTO{
		UID:            s.UID,
		Name:           s.Name,
		Description:    s.Description,
		Type:           string(s.Type),
		Layer:          s.Layer,
		Visible:        s.Visible,
		Properties:     propertiesToDTO(s.Properties),
		Initialization: agentInitToDTO(s.Initialization),
		Locked:         s.Locked,
	}
	if s.Geometry != nil {
		g := shapeGeometryToDTO(*s.Geometry)
		dto.Geometry = &g
	}
	dto.Display = displayToDTO(s.Display)
	if s.LLMConfig != nil {
		c := llmShapeConfigToDTO(*s.LLMConfig)
		dto.LLMConfig = &c
	}
	return dto
}

func shapeFromDTO(s ShapeDTO) datamodels.Shape {
	out := datamodels.Shape{
		UID:            s.UID,
		Name:           s.Name,
		Description:    s.Description,
		Type:           datamodels.ShapeType(s.Type),
		Layer:          s.Layer,
		Visible:        s.Visible,
		Properties:     propertiesFromDTO(s.Properties),
		Initialization: agentInitFromDTO(s.Initialization),
		Locked:         s.Locked,
	}
	if s.Geometry != nil {
		g := shapeGeometryFromDTO(*s.Geometry)
		out.Geometry = &g
	}
	out.Display = displayFromDTO(s.Display)
	if s.LLMConfig != nil {
		c := llmShapeConfigFromDTO(*s.LLMConfig)
		out.LLMConfig = &c
	}
	return out
}

func shapeGeometryToDTO(g datamodels.ShapeGeometry) ShapeGeometryDTO {
	dto := ShapeGeometryDTO{
		Width:       scalarSpecToDTO(g.Width),
		Height:      scalarSpecToDTO(g.Height),
		Radius:      scalarSpecToDTO(g.Radius),
		Thickness:   g.Thickness,
		Fill:        g.Fill,
		Stroke:      g.Stroke,
		StrokeWidth: g.StrokeWidth,
	}
	if g.Position != nil {
		p := pointToDTO(*g.Position)
		dto.Position = &p
	}
	if g.Points != nil {
		dto.Points = make([]PointDTO, 0, len(g.Points))
		for _, pt := range g.Points {
			dto.Points = append(dto.Points, pointToDTO(pt))
		}
	}
	return dto
}

func shapeGeometryFromDTO(g ShapeGeometryDTO) datamodels.ShapeGeometry {
	out := datamodels.ShapeGeometry{
		Width:       scalarSpecFromDTO(g.Width),
		Height:      scalarSpecFromDTO(g.Height),
		Radius:      scalarSpecFromDTO(g.Radius),
		Thickness:   g.Thickness,
		Fill:        g.Fill,
		Stroke:      g.Stroke,
		StrokeWidth: g.StrokeWidth,
	}
	if g.Position != nil {
		p := pointFromDTO(*g.Position)
		out.Position = &p
	}
	if g.Points != nil {
		out.Points = make([]common.Point, 0, len(g.Points))
		for _, pt := range g.Points {
			out.Points = append(out.Points, pointFromDTO(pt))
		}
	}
	return out
}

func scalarSpecToDTO(s *datamodels.ScalarSpec) *ScalarSpecDTO {
	if s == nil {
		return nil
	}
	return &ScalarSpecDTO{Value: s.Value, Min: s.Min, Max: s.Max}
}

func scalarSpecFromDTO(s *ScalarSpecDTO) *datamodels.ScalarSpec {
	if s == nil {
		return nil
	}
	return &datamodels.ScalarSpec{Value: s.Value, Min: s.Min, Max: s.Max}
}

func pointToDTO(p common.Point) PointDTO   { return PointDTO{X: p.X, Y: p.Y} }
func pointFromDTO(p PointDTO) common.Point { return common.Point{X: p.X, Y: p.Y} }

func pointsToDTO(pts []common.Point) []PointDTO {
	if pts == nil {
		return nil
	}
	out := make([]PointDTO, 0, len(pts))
	for _, p := range pts {
		out = append(out, pointToDTO(p))
	}
	return out
}

func pointsFromDTO(pts []PointDTO) []common.Point {
	if pts == nil {
		return nil
	}
	out := make([]common.Point, 0, len(pts))
	for _, p := range pts {
		out = append(out, pointFromDTO(p))
	}
	return out
}

// ---- agent footprint / display (shared spatial) ----

func footprintToDTO(f *datamodels.AgentFootprint) *AgentFootprintDTO {
	if f == nil {
		return nil
	}
	return &AgentFootprintDTO{
		Kind:   string(f.Kind),
		Radius: f.Radius,
		Width:  f.Width,
		Height: f.Height,
		Points: pointsToDTO(f.Points),
	}
}

func footprintFromDTO(f *AgentFootprintDTO) *datamodels.AgentFootprint {
	if f == nil {
		return nil
	}
	return &datamodels.AgentFootprint{
		Kind:   datamodels.AgentFootprintKind(f.Kind),
		Radius: f.Radius,
		Width:  f.Width,
		Height: f.Height,
		Points: pointsFromDTO(f.Points),
	}
}

func displayToDTO(d *datamodels.Display) *DisplayDTO {
	if d == nil {
		return nil
	}
	dto := &DisplayDTO{
		Kind:   string(d.Kind),
		Figure: pointsToDTO(d.Figure),
		Scale:  d.Scale,
	}
	if d.Image != nil {
		dto.Image = &DisplayImageDTO{Data: d.Image.Data, MimeType: d.Image.MimeType, Width: d.Image.Width, Height: d.Image.Height}
	}
	return dto
}

func displayFromDTO(d *DisplayDTO) *datamodels.Display {
	if d == nil {
		return nil
	}
	out := &datamodels.Display{
		Kind:   datamodels.DisplayKind(d.Kind),
		Figure: pointsFromDTO(d.Figure),
		Scale:  d.Scale,
	}
	if d.Image != nil {
		out.Image = &datamodels.DisplayImageData{Data: d.Image.Data, MimeType: d.Image.MimeType, Width: d.Image.Width, Height: d.Image.Height}
	}
	return out
}

// ===================== rules =====================

func ruleToDTO(r datamodels.Rule) RuleDTO {
	return RuleDTO{
		ID:            r.ID,
		Name:          r.Name,
		Description:   r.Description,
		Priority:      r.Priority,
		AppliesTo:     r.AppliesTo,
		Conditions:    conditionsToDTO(r.Conditions),
		Actions:       actionsToDTO(r.Actions),
		ExecuteOnce:   r.ExecuteOnce,
		CooldownSteps: r.CooldownSteps,
		Enabled:       r.Enabled,
		Locked:        r.Locked,
	}
}

func ruleFromDTO(r RuleDTO) datamodels.Rule {
	return datamodels.Rule{
		ID:            r.ID,
		Name:          r.Name,
		Description:   r.Description,
		Priority:      r.Priority,
		AppliesTo:     r.AppliesTo,
		Conditions:    conditionsFromDTO(r.Conditions),
		Actions:       actionsFromDTO(r.Actions),
		ExecuteOnce:   r.ExecuteOnce,
		CooldownSteps: r.CooldownSteps,
		Enabled:       r.Enabled,
		Locked:        r.Locked,
	}
}

func conditionsToDTO(conds []datamodels.Condition) []ConditionDTO {
	if conds == nil {
		return nil
	}
	out := make([]ConditionDTO, 0, len(conds))
	for _, c := range conds {
		out = append(out, conditionToDTO(c))
	}
	return out
}

func conditionsFromDTO(conds []ConditionDTO) []datamodels.Condition {
	if conds == nil {
		return nil
	}
	out := make([]datamodels.Condition, 0, len(conds))
	for _, c := range conds {
		out = append(out, conditionFromDTO(c))
	}
	return out
}

func conditionToDTO(c datamodels.Condition) ConditionDTO {
	return ConditionDTO{
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
		SubConditions: conditionsToDTO(c.SubConditions),
		Parameters:    c.Parameters,
		Description:   c.Description,
	}
}

func conditionFromDTO(c ConditionDTO) datamodels.Condition {
	return datamodels.Condition{
		Type:          datamodels.ConditionType(c.Type),
		PropertyUID:   c.PropertyUID,
		Operator:      datamodels.ConditionOperator(c.Operator),
		Value:         c.Value,
		MinValue:      c.MinValue,
		MaxValue:      c.MaxValue,
		Radius:        c.Radius,
		TargetUID:     c.TargetUID,
		TargetUIDs:    c.TargetUIDs,
		LogicOperator: datamodels.LogicOperator(c.LogicOperator),
		SubConditions: conditionsFromDTO(c.SubConditions),
		Parameters:    c.Parameters,
		Description:   c.Description,
	}
}

func actionsToDTO(actions []datamodels.Action) []ActionDTO {
	if actions == nil {
		return nil
	}
	out := make([]ActionDTO, 0, len(actions))
	for _, a := range actions {
		out = append(out, actionToDTO(a))
	}
	return out
}

func actionsFromDTO(actions []ActionDTO) []datamodels.Action {
	if actions == nil {
		return nil
	}
	out := make([]datamodels.Action, 0, len(actions))
	for _, a := range actions {
		out = append(out, actionFromDTO(a))
	}
	return out
}

func actionToDTO(a datamodels.Action) ActionDTO {
	dto := ActionDTO{
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
		SpawnCount:       a.SpawnCount,
		SpawnProperties:  a.SpawnProperties,
		SpawnInheritFrom: a.SpawnInheritFrom,
		Parameters:       a.Parameters,
		Description:      a.Description,
		LLMPromptContext: a.LLMPromptContext,
	}
	if a.SpawnPosition != nil {
		sp := spawnPositionToDTO(*a.SpawnPosition)
		dto.SpawnPosition = &sp
	}
	return dto
}

func actionFromDTO(a ActionDTO) datamodels.Action {
	out := datamodels.Action{
		Type:             datamodels.ActionType(a.Type),
		PropertyUID:      a.PropertyUID,
		Value:            a.Value,
		Modifier:         datamodels.PropertyModifier(a.Modifier),
		TargetUID:        a.TargetUID,
		TargetUIDs:       a.TargetUIDs,
		Speed:            a.Speed,
		Direction:        a.Direction,
		EventUID:         a.EventUID,
		EventData:        a.EventData,
		NewState:         a.NewState,
		SpawnAgentUID:    a.SpawnAgentUID,
		SpawnCount:       a.SpawnCount,
		SpawnProperties:  a.SpawnProperties,
		SpawnInheritFrom: a.SpawnInheritFrom,
		Parameters:       a.Parameters,
		Description:      a.Description,
		LLMPromptContext: a.LLMPromptContext,
	}
	if a.SpawnPosition != nil {
		sp := spawnPositionFromDTO(*a.SpawnPosition)
		out.SpawnPosition = &sp
	}
	return out
}

func spawnPositionToDTO(s datamodels.SpawnPosition) SpawnPositionDTO {
	return SpawnPositionDTO{
		Type:     string(s.Type),
		OffsetX:  s.OffsetX,
		OffsetY:  s.OffsetY,
		Radius:   s.Radius,
		ShapeUID: s.ShapeUID,
	}
}

func spawnPositionFromDTO(s SpawnPositionDTO) datamodels.SpawnPosition {
	return datamodels.SpawnPosition{
		Type:     datamodels.SpawnPositionType(s.Type),
		OffsetX:  s.OffsetX,
		OffsetY:  s.OffsetY,
		Radius:   s.Radius,
		ShapeUID: s.ShapeUID,
	}
}

// ===================== interactions =====================

func interactionToDTO(i datamodels.Interaction) InteractionDTO {
	return InteractionDTO{
		ID:               i.ID,
		Name:             i.Name,
		Description:      i.Description,
		Type:             string(i.Type),
		InitiatorUIDs:    i.InitiatorUIDs,
		TargetUIDs:       i.TargetUIDs,
		Radius:           i.Radius,
		Conditions:       conditionsToDTO(i.Conditions),
		InitiatorActions: actionsToDTO(i.InitiatorActions),
		TargetActions:    actionsToDTO(i.TargetActions),
		Bidirectional:    i.Bidirectional,
		UseLLM:           i.UseLLM,
		Enabled:          i.Enabled,
		Locked:           i.Locked,
		LLMPromptContext: i.LLMPromptContext,
	}
}

func interactionFromDTO(i InteractionDTO) datamodels.Interaction {
	return datamodels.Interaction{
		ID:               i.ID,
		Name:             i.Name,
		Description:      i.Description,
		Type:             datamodels.InteractionType(i.Type),
		InitiatorUIDs:    i.InitiatorUIDs,
		TargetUIDs:       i.TargetUIDs,
		Radius:           i.Radius,
		Conditions:       conditionsFromDTO(i.Conditions),
		InitiatorActions: actionsFromDTO(i.InitiatorActions),
		TargetActions:    actionsFromDTO(i.TargetActions),
		Bidirectional:    i.Bidirectional,
		UseLLM:           i.UseLLM,
		Enabled:          i.Enabled,
		Locked:           i.Locked,
		LLMPromptContext: i.LLMPromptContext,
	}
}

// ===================== events =====================

func eventToDTO(e datamodels.EventDefinition) EventDefDTO {
	dto := EventDefDTO{
		UID:              e.UID,
		Name:             e.Name,
		Description:      e.Description,
		Scope:            string(e.Scope),
		Radius:           e.Radius,
		Payload:          propertiesToDTO(e.Payload),
		ProducerUIDs:     e.ProducerUIDs,
		ConsumerUIDs:     e.ConsumerUIDs,
		TTLSteps:         e.TTLSteps,
		LLMPromptContext: e.LLMPromptContext,
		Locked:           e.Locked,
	}
	if e.Producers != nil {
		dto.Producers = make([]string, 0, len(e.Producers))
		for _, p := range e.Producers {
			dto.Producers = append(dto.Producers, string(p))
		}
	}
	if e.LLMTarget != nil {
		t := execDTOFromTarget(*e.LLMTarget)
		dto.LLMTarget = &t
	}
	return dto
}

func eventFromDTO(e EventDefDTO) datamodels.EventDefinition {
	out := datamodels.EventDefinition{
		UID:              e.UID,
		Name:             e.Name,
		Description:      e.Description,
		Scope:            datamodels.EventScope(e.Scope),
		Radius:           e.Radius,
		Payload:          propertiesFromDTO(e.Payload),
		ProducerUIDs:     e.ProducerUIDs,
		ConsumerUIDs:     e.ConsumerUIDs,
		TTLSteps:         e.TTLSteps,
		LLMPromptContext: e.LLMPromptContext,
		Locked:           e.Locked,
	}
	if e.Producers != nil {
		out.Producers = make([]datamodels.EventProducer, 0, len(e.Producers))
		for _, p := range e.Producers {
			out.Producers = append(out.Producers, datamodels.EventProducer(p))
		}
	}
	if e.LLMTarget != nil {
		t := targetFromExecDTO(*e.LLMTarget)
		out.LLMTarget = &t
	}
	return out
}

// ===================== images =====================

func imageToDTO(img datamodels.ModelImage) ModelImageDTO {
	return ModelImageDTO{
		UID:         img.UID,
		Name:        img.Name,
		MimeType:    img.MimeType,
		Data:        img.Data,
		Description: img.Description,
	}
}

func imageFromDTO(img ModelImageDTO) datamodels.ModelImage {
	return datamodels.ModelImage{
		UID:         img.UID,
		Name:        img.Name,
		MimeType:    img.MimeType,
		Data:        img.Data,
		Description: img.Description,
	}
}

// ===================== widgets =====================

func widgetToDTO(w common.WidgetConfig) WidgetConfigDTO {
	dto := WidgetConfigDTO{
		ID:       w.ID,
		Type:     string(w.Type),
		Title:    w.Title,
		Columns:  w.Columns,
		Buckets:  w.Buckets,
		Position: WidgetPositionDTO{X: w.Position.X, Y: w.Position.Y, W: w.Position.W, H: w.Position.H},
	}
	if w.Source != nil {
		s := widgetSourceToDTO(*w.Source)
		dto.Source = &s
	}
	if w.Series != nil {
		dto.Series = make([]WidgetSeriesDTO, 0, len(w.Series))
		for _, s := range w.Series {
			dto.Series = append(dto.Series, WidgetSeriesDTO{Label: s.Label, Color: s.Color, Source: widgetSourceToDTO(s.Source)})
		}
	}
	if w.LogKinds != nil {
		dto.LogKinds = make([]string, 0, len(w.LogKinds))
		for _, k := range w.LogKinds {
			dto.LogKinds = append(dto.LogKinds, string(k))
		}
	}
	return dto
}

func widgetFromDTO(w WidgetConfigDTO) common.WidgetConfig {
	out := common.WidgetConfig{
		ID:       w.ID,
		Type:     common.WidgetType(w.Type),
		Title:    w.Title,
		Columns:  w.Columns,
		Buckets:  w.Buckets,
		Position: common.WidgetPosition{X: w.Position.X, Y: w.Position.Y, W: w.Position.W, H: w.Position.H},
	}
	if w.Source != nil {
		s := widgetSourceFromDTO(*w.Source)
		out.Source = &s
	}
	if w.Series != nil {
		out.Series = make([]common.WidgetSeries, 0, len(w.Series))
		for _, s := range w.Series {
			out.Series = append(out.Series, common.WidgetSeries{Label: s.Label, Color: s.Color, Source: widgetSourceFromDTO(s.Source)})
		}
	}
	if w.LogKinds != nil {
		out.LogKinds = make([]common.WidgetLogKind, 0, len(w.LogKinds))
		for _, k := range w.LogKinds {
			out.LogKinds = append(out.LogKinds, common.WidgetLogKind(k))
		}
	}
	return out
}

func widgetSourceToDTO(s common.WidgetSource) WidgetSourceDTO {
	return WidgetSourceDTO{
		Type:        string(s.Type),
		PropertyUID: s.PropertyUID,
		AgentUIDs:   s.AgentUIDs,
		Aggregation: string(s.Aggregation),
		Metric:      string(s.Metric),
	}
}

func widgetSourceFromDTO(s WidgetSourceDTO) common.WidgetSource {
	return common.WidgetSource{
		Type:        common.WidgetSourceType(s.Type),
		PropertyUID: s.PropertyUID,
		AgentUIDs:   s.AgentUIDs,
		Aggregation: common.WidgetAggregation(s.Aggregation),
		Metric:      common.WidgetMetric(s.Metric),
	}
}

// ===================== llm config / strategy / fallback =====================

func llmConfigToDTO(c datamodels.LLMConfig) LLMConfigDTO {
	return LLMConfigDTO{
		Target:        execDTOFromTarget(c.Target),
		PromptContext: c.PromptContext,
	}
}

func llmConfigFromDTO(c LLMConfigDTO) datamodels.LLMConfig {
	return datamodels.LLMConfig{
		Target:        targetFromExecDTO(c.Target),
		PromptContext: c.PromptContext,
	}
}

func simulationConfigToDTO(c datamodels.SimulationConfig) SimulationConfigDTO {
	dto := SimulationConfigDTO{
		UpdateMode:                    string(c.UpdateMode),
		LLMSimulationGenerationConfig: llmConfigToDTO(c.LLMSimulationGenerationConfig),
		LLMSimulationExecutionConfig:  llmSimulationExecutionConfigToDTO(c.LLMSimulationExecutionConfig),
	}
	if c.Termination != nil {
		t := conditionToDTO(*c.Termination)
		dto.Termination = &t
	}
	return dto
}

func simulationConfigFromDTO(c SimulationConfigDTO) datamodels.SimulationConfig {
	cfg := datamodels.SimulationConfig{
		UpdateMode:                    datamodels.ModelUpdateMode(c.UpdateMode),
		LLMSimulationGenerationConfig: llmConfigFromDTO(c.LLMSimulationGenerationConfig),
		LLMSimulationExecutionConfig:  llmSimulationExecutionConfigFromDTO(c.LLMSimulationExecutionConfig),
	}
	if c.Termination != nil {
		t := conditionFromDTO(*c.Termination)
		cfg.Termination = &t
	}
	return cfg
}

func llmAgentConfigToDTO(c datamodels.LLMAgentConfig) LLMAgentConfigDTO {
	dto := LLMAgentConfigDTO{
		Target:        execDTOFromTarget(c.Target),
		PromptContext: c.PromptContext,
		Strategy:      llmAgentStrategyToDTO(c.Strategy),
	}
	if c.SimulationConfig != nil {
		sc := llmSimulationAgentConfigToDTO(*c.SimulationConfig)
		dto.SimulationConfig = &sc
	}
	return dto
}

func llmAgentConfigFromDTO(c LLMAgentConfigDTO) datamodels.LLMAgentConfig {
	out := datamodels.LLMAgentConfig{
		LLMConfig: datamodels.LLMConfig{
			Target:        targetFromExecDTO(c.Target),
			PromptContext: c.PromptContext,
		},
		Strategy: llmAgentStrategyFromDTO(c.Strategy),
	}
	if c.SimulationConfig != nil {
		sc := llmSimulationAgentConfigFromDTO(*c.SimulationConfig)
		out.SimulationConfig = &sc
	}
	return out
}

func llmShapeConfigToDTO(c datamodels.LLMShapeConfig) LLMShapeConfigDTO {
	return LLMShapeConfigDTO{
		Target:        execDTOFromTarget(c.Target),
		PromptContext: c.PromptContext,
		Strategy:      llmShapeStrategyToDTO(c.Strategy),
	}
}

func llmShapeConfigFromDTO(c LLMShapeConfigDTO) datamodels.LLMShapeConfig {
	return datamodels.LLMShapeConfig{
		LLMConfig: datamodels.LLMConfig{
			Target:        targetFromExecDTO(c.Target),
			PromptContext: c.PromptContext,
		},
		Strategy: llmShapeStrategyFromDTO(c.Strategy),
	}
}

func llmStrategyToDTO(s datamodels.LLMStrategy) LLMStrategyDTO {
	return LLMStrategyDTO{
		Type:              string(s.Type),
		BatchSize:         s.BatchSize,
		UseForMovement:    s.UseForMovement,
		UseForInteraction: s.UseForInteraction,
		UseForProperties:  s.UseForProperties,
		UseForDecisions:   s.UseForDecisions,
		DecisionFrequency: s.DecisionFrequency,
		ArchetypeMode:     string(s.ArchetypeMode),
		VariationStrength: s.VariationStrength,
	}
}

func llmStrategyFromDTO(s LLMStrategyDTO) datamodels.LLMStrategy {
	return datamodels.LLMStrategy{
		Type:              datamodels.LLMStrategyType(s.Type),
		BatchSize:         s.BatchSize,
		UseForMovement:    s.UseForMovement,
		UseForInteraction: s.UseForInteraction,
		UseForProperties:  s.UseForProperties,
		UseForDecisions:   s.UseForDecisions,
		DecisionFrequency: s.DecisionFrequency,
		ArchetypeMode:     datamodels.ArchetypeMode(s.ArchetypeMode),
		VariationStrength: s.VariationStrength,
	}
}

func llmAgentStrategyToDTO(s datamodels.LLMAgentStrategy) LLMAgentStrategyDTO {
	return LLMAgentStrategyDTO{
		LLMStrategyDTO: llmStrategyToDTO(s.LLMStrategy),
	}
}

func llmAgentStrategyFromDTO(s LLMAgentStrategyDTO) datamodels.LLMAgentStrategy {
	return datamodels.LLMAgentStrategy{
		LLMStrategy: llmStrategyFromDTO(s.LLMStrategyDTO),
	}
}

func llmShapeStrategyToDTO(s datamodels.LLMShapeStrategy) LLMShapeStrategyDTO {
	return LLMShapeStrategyDTO{
		LLMStrategyDTO: llmStrategyToDTO(s.LLMStrategy),
	}
}

func llmShapeStrategyFromDTO(s LLMShapeStrategyDTO) datamodels.LLMShapeStrategy {
	return datamodels.LLMShapeStrategy{
		LLMStrategy: llmStrategyFromDTO(s.LLMStrategyDTO),
	}
}

func llmSimulationAgentConfigToDTO(c datamodels.LLMSimulationAgentConfig) LLMSimulationAgentConfigDTO {
	dto := LLMSimulationAgentConfigDTO{
		PerceptionRadius:  c.PerceptionRadius,
		InteractionRadius: c.InteractionRadius,
		MemorySize:        c.MemorySize,
	}
	if c.FallbackPolicy != nil {
		fp := fallbackPolicyToDTO(*c.FallbackPolicy)
		dto.FallbackPolicy = &fp
	}
	return dto
}

func llmSimulationAgentConfigFromDTO(c LLMSimulationAgentConfigDTO) datamodels.LLMSimulationAgentConfig {
	out := datamodels.LLMSimulationAgentConfig{
		PerceptionRadius:  c.PerceptionRadius,
		InteractionRadius: c.InteractionRadius,
		MemorySize:        c.MemorySize,
	}
	if c.FallbackPolicy != nil {
		fp := fallbackPolicyFromDTO(*c.FallbackPolicy)
		out.FallbackPolicy = &fp
	}
	return out
}

func llmSimulationExecutionConfigToDTO(c datamodels.LLMSimulationExecutionConfig) LLMSimulationExecutionConfigDTO {
	return LLMSimulationExecutionConfigDTO{
		Target:                      execDTOFromTarget(c.Target),
		PromptContext:               c.PromptContext,
		LLMSimulationAgentConfigDTO: llmSimulationAgentConfigToDTO(c.LLMSimulationAgentConfig),
		ParallelWorkers:             c.ParallelWorkers,
		RetryTimeoutMs:              c.RetryTimeoutMs,
		MaxRetries:                  c.MaxRetries,
		StreamBatchSize:             c.StreamBatchSize,
		DefaultStrategy:             llmStrategyToDTO(c.DefaultStrategy),
	}
}

func llmSimulationExecutionConfigFromDTO(c LLMSimulationExecutionConfigDTO) datamodels.LLMSimulationExecutionConfig {
	return datamodels.LLMSimulationExecutionConfig{
		LLMConfig: datamodels.LLMConfig{
			Target:        targetFromExecDTO(c.Target),
			PromptContext: c.PromptContext,
		},
		LLMSimulationAgentConfig: llmSimulationAgentConfigFromDTO(c.LLMSimulationAgentConfigDTO),
		ParallelWorkers:          c.ParallelWorkers,
		RetryTimeoutMs:           c.RetryTimeoutMs,
		MaxRetries:               c.MaxRetries,
		StreamBatchSize:          c.StreamBatchSize,
		DefaultStrategy:          llmStrategyFromDTO(c.DefaultStrategy),
	}
}

func fallbackPolicyToDTO(p datamodels.FallbackPolicy) FallbackPolicyDTO {
	dto := FallbackPolicyDTO{}
	if p.Steps != nil {
		dto.Steps = make([]FallbackStepDTO, 0, len(p.Steps))
		for _, s := range p.Steps {
			dto.Steps = append(dto.Steps, FallbackStepDTO{
				Type:         string(s.Type),
				MaxRetries:   s.MaxRetries,
				RetryDelayMs: s.RetryDelayMs,
			})
		}
	}
	return dto
}

func fallbackPolicyFromDTO(p FallbackPolicyDTO) datamodels.FallbackPolicy {
	out := datamodels.FallbackPolicy{}
	if p.Steps != nil {
		out.Steps = make([]datamodels.FallbackStep, 0, len(p.Steps))
		for _, s := range p.Steps {
			out.Steps = append(out.Steps, datamodels.FallbackStep{
				Type:         datamodels.FallbackPolicyType(s.Type),
				MaxRetries:   s.MaxRetries,
				RetryDelayMs: s.RetryDelayMs,
			})
		}
	}
	return out
}
