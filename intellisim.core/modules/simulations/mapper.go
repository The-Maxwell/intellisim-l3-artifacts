package simulations

import (
	"intelli.sim.core/data/common"
	datamodels "intelli.sim.core/data/models"
	datasims "intelli.sim.core/data/simulations"
)

// ===================== LLMTarget =====================

func llmTargetToDTO(t common.LLMTarget) LLMTargetDTO {
	d := LLMTargetDTO{
		ProviderUID: t.ProviderUID,
		ModelUID:    t.ModelUID,
		Effort:      string(t.Effort),
		TimeoutMs:   t.TimeoutMs,
	}
	if len(t.Generation) > 0 {
		d.Generation = make(map[string]ConfigValueDTO, len(t.Generation))
		for k, v := range t.Generation {
			d.Generation[k] = configValueToDTO(v)
		}
	}
	return d
}

func llmTargetFromDTO(d *LLMTargetDTO) *common.LLMTarget {
	if d == nil {
		return nil
	}
	t := &common.LLMTarget{
		ProviderUID: d.ProviderUID,
		ModelUID:    d.ModelUID,
		Effort:      common.LLMEffort(d.Effort),
		TimeoutMs:   d.TimeoutMs,
	}
	if len(d.Generation) > 0 {
		t.Generation = make(common.GenerationParams, len(d.Generation))
		for k, v := range d.Generation {
			t.Generation[k] = configValueFromDTO(v)
		}
	}
	// empty provider_uid clears the override
	return normalizeOverride(t)
}

// normalizeOverride collapses an override with no provider to nil so callers
// never have to special-case the "clear" sentinel: empty provider_uid clears the
// override.
func normalizeOverride(t *common.LLMTarget) *common.LLMTarget {
	if t == nil || t.ProviderUID == "" {
		return nil
	}
	return t
}

func configValueToDTO(v common.ConfigValue) ConfigValueDTO {
	return ConfigValueDTO{Double: v.Double, Int: v.Int, Uint: v.Uint, Bool: v.Bool, String: v.String, List: v.List}
}

func configValueFromDTO(v ConfigValueDTO) common.ConfigValue {
	return common.ConfigValue{Double: v.Double, Int: v.Int, Uint: v.Uint, Bool: v.Bool, String: v.String, List: v.List}
}

// ===================== Point =====================

func pointToDTO(p common.Point) PointDTO   { return PointDTO{X: p.X, Y: p.Y} }
func pointFromDTO(p PointDTO) common.Point { return common.Point{X: p.X, Y: p.Y} }

// ===================== Agents =====================

func agentsToDTO(agents []datasims.SimulationAgent) []SimulationAgentDTO {
	if agents == nil {
		return nil
	}
	out := make([]SimulationAgentDTO, 0, len(agents))
	for _, a := range agents {
		dto := SimulationAgentDTO{
			ID:         a.ID,
			AgentUID:   a.AgentUID,
			Name:       a.Name,
			Properties: a.Properties,
			State:      a.State,
		}
		if a.Position != nil {
			p := pointToDTO(*a.Position)
			dto.Position = &p
		}
		if len(a.Memory) > 0 {
			dto.Memory = agentMemoryToDTO(a.Memory)
		}
		out = append(out, dto)
	}
	return out
}

// agentMemoryToDTO maps a slice of AgentMemory entries to their compact DTO
// form. Details and Timestamp are deliberately excluded to keep the payload
// small; only Step, Type, and Summary are surfaced.
func agentMemoryToDTO(mem []datasims.AgentMemory) []AgentMemoryDTO {
	out := make([]AgentMemoryDTO, 0, len(mem))
	for _, m := range mem {
		out = append(out, AgentMemoryDTO{
			Step:    m.Step,
			Type:    string(m.Type),
			Summary: m.Summary,
		})
	}
	return out
}

func agentsFromDTO(dtos []SimulationAgentDTO) []datasims.SimulationAgent {
	if dtos == nil {
		return nil
	}
	out := make([]datasims.SimulationAgent, 0, len(dtos))
	for _, d := range dtos {
		a := datasims.SimulationAgent{
			ID:         d.ID,
			AgentUID:   d.AgentUID,
			Name:       d.Name,
			Properties: d.Properties,
			State:      d.State,
		}
		if d.Position != nil {
			p := pointFromDTO(*d.Position)
			a.Position = &p
		}
		out = append(out, a)
	}
	return out
}

// ===================== Environment =====================

func environmentToDTO(env *datasims.SimulationEnvironment) *SimulationEnvironmentDTO {
	if env == nil {
		return nil
	}
	dto := &SimulationEnvironmentDTO{
		Width:            env.Width,
		Height:           env.Height,
		Torus:            env.Torus,
		CollisionMode:    string(env.CollisionMode),
		GlobalProperties: env.GlobalProperties,
	}
	if env.CollisionConfig != nil {
		dto.CollisionConfig = &CollisionConfigDTO{
			MaxIterations: env.CollisionConfig.MaxIterations,
			StuckPolicy:   string(env.CollisionConfig.StuckPolicy),
		}
	}
	if env.Shapes != nil {
		dto.Shapes = make([]SimulationShapeDTO, 0, len(env.Shapes))
		for _, s := range env.Shapes {
			dto.Shapes = append(dto.Shapes, shapeToDTO(s))
		}
	}
	return dto
}

func environmentFromDTO(d *SimulationEnvironmentDTO) *datasims.SimulationEnvironment {
	if d == nil {
		return nil
	}
	env := &datasims.SimulationEnvironment{
		Width:            d.Width,
		Height:           d.Height,
		Torus:            d.Torus,
		CollisionMode:    datamodels.CollisionMode(d.CollisionMode),
		GlobalProperties: d.GlobalProperties,
	}
	if d.CollisionConfig != nil {
		env.CollisionConfig = &datamodels.CollisionConfig{
			MaxIterations: d.CollisionConfig.MaxIterations,
			StuckPolicy:   datamodels.CollisionStuckPolicy(d.CollisionConfig.StuckPolicy),
		}
	}
	if d.Shapes != nil {
		env.Shapes = make([]datasims.SimulationShape, 0, len(d.Shapes))
		for _, s := range d.Shapes {
			env.Shapes = append(env.Shapes, shapeFromDTO(s))
		}
	}
	return env
}

func shapeToDTO(s datasims.SimulationShape) SimulationShapeDTO {
	dto := SimulationShapeDTO{
		ID:          s.ID,
		ShapeUID:    s.ShapeUID,
		Name:        s.Name,
		Type:        string(s.Type),
		Layer:       s.Layer,
		Visible:     s.Visible,
		Properties:  s.Properties,
		Position:    pointToDTO(s.Position),
		Width:       s.Width,
		Height:      s.Height,
		Radius:      s.Radius,
		Thickness:   s.Thickness,
		Fill:        s.Fill,
		Stroke:      s.Stroke,
		StrokeWidth: s.StrokeWidth,
	}
	if s.Points != nil {
		dto.Points = make([]PointDTO, 0, len(s.Points))
		for _, p := range s.Points {
			dto.Points = append(dto.Points, pointToDTO(p))
		}
	}
	return dto
}

func shapeFromDTO(d SimulationShapeDTO) datasims.SimulationShape {
	s := datasims.SimulationShape{
		ID:          d.ID,
		ShapeUID:    d.ShapeUID,
		Name:        d.Name,
		Type:        datamodels.ShapeType(d.Type),
		Layer:       d.Layer,
		Visible:     d.Visible,
		Properties:  d.Properties,
		Position:    pointFromDTO(d.Position),
		Width:       d.Width,
		Height:      d.Height,
		Radius:      d.Radius,
		Thickness:   d.Thickness,
		Fill:        d.Fill,
		Stroke:      d.Stroke,
		StrokeWidth: d.StrokeWidth,
	}
	if d.Points != nil {
		s.Points = make([]common.Point, 0, len(d.Points))
		for _, p := range d.Points {
			s.Points = append(s.Points, pointFromDTO(p))
		}
	}
	return s
}

// ===================== Widgets =====================

func widgetsToDTO(widgets []common.WidgetConfig) []WidgetConfigDTO {
	if widgets == nil {
		return nil
	}
	out := make([]WidgetConfigDTO, 0, len(widgets))
	for _, w := range widgets {
		out = append(out, widgetToDTO(w))
	}
	return out
}

func widgetsFromDTO(dtos []WidgetConfigDTO) []common.WidgetConfig {
	if dtos == nil {
		return nil
	}
	out := make([]common.WidgetConfig, 0, len(dtos))
	for _, d := range dtos {
		out = append(out, widgetFromDTO(d))
	}
	return out
}

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

func widgetFromDTO(d WidgetConfigDTO) common.WidgetConfig {
	out := common.WidgetConfig{
		ID:       d.ID,
		Type:     common.WidgetType(d.Type),
		Title:    d.Title,
		Columns:  d.Columns,
		Buckets:  d.Buckets,
		Position: common.WidgetPosition{X: d.Position.X, Y: d.Position.Y, W: d.Position.W, H: d.Position.H},
	}
	if d.Source != nil {
		s := widgetSourceFromDTO(*d.Source)
		out.Source = &s
	}
	if d.Series != nil {
		out.Series = make([]common.WidgetSeries, 0, len(d.Series))
		for _, s := range d.Series {
			out.Series = append(out.Series, common.WidgetSeries{Label: s.Label, Color: s.Color, Source: widgetSourceFromDTO(s.Source)})
		}
	}
	if d.LogKinds != nil {
		out.LogKinds = make([]common.WidgetLogKind, 0, len(d.LogKinds))
		for _, k := range d.LogKinds {
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

func widgetSourceFromDTO(d WidgetSourceDTO) common.WidgetSource {
	return common.WidgetSource{
		Type:        common.WidgetSourceType(d.Type),
		PropertyUID: d.PropertyUID,
		AgentUIDs:   d.AgentUIDs,
		Aggregation: common.WidgetAggregation(d.Aggregation),
		Metric:      common.WidgetMetric(d.Metric),
	}
}

// ===================== GenerationScope =====================

func scopeToDTO(s datasims.GenerationScope) GenerationScopeResponseDTO {
	dto := GenerationScopeResponseDTO{
		Kind: string(s.Kind),
	}
	if s.AgentUIDs != nil {
		dto.AgentUIDs = append([]string(nil), s.AgentUIDs...)
	}
	if s.Parts != nil {
		dto.Parts = make([]string, 0, len(s.Parts))
		for _, p := range s.Parts {
			dto.Parts = append(dto.Parts, string(p))
		}
	}
	return dto
}

func scopeFromDTO(d GenerationScopeDTO) datasims.GenerationScope {
	s := datasims.GenerationScope{
		Kind:      datasims.GenerationScopeKind(d.Kind),
		AgentUIDs: d.AgentUIDs,
	}
	if d.Parts != nil {
		s.Parts = make([]datasims.GenerationPart, 0, len(d.Parts))
		for _, p := range d.Parts {
			s.Parts = append(s.Parts, datasims.GenerationPart(p))
		}
	}
	return s
}

// ===================== GenerationRun =====================

func generationRunsToDTO(runs []datasims.GenerationRun) []GenerationRunDTO {
	if runs == nil {
		return nil
	}
	out := make([]GenerationRunDTO, 0, len(runs))
	for _, r := range runs {
		dto := GenerationRunDTO{
			Seq:        r.Seq,
			Seed:       r.Seed,
			Scope:      scopeToDTO(r.Scope),
			Status:     string(r.Status),
			StartedAt:  r.StartedAt,
			FinishedAt: r.FinishedAt,
			Error:      r.Error,
			Requests:   r.Requests,
		}
		if r.Warnings != nil {
			dto.Warnings = make([]GenerationWarningDTO, 0, len(r.Warnings))
			for _, w := range r.Warnings {
				dto.Warnings = append(dto.Warnings, GenerationWarningDTO{Path: w.Path, Reason: w.Reason})
			}
		}
		out = append(out, dto)
	}
	return out
}

// ===================== TokenUsage =====================

func tokenUsageToDTO(u common.TokenUsage) TokenUsageDTO {
	return TokenUsageDTO{
		ProviderUID:      u.ProviderUID,
		ModelUID:         u.ModelUID,
		Effort:           string(u.Effort),
		PromptTokens:     u.PromptTokens,
		CompletionTokens: u.CompletionTokens,
		ReasoningTokens:  u.ReasoningTokens,
		TotalTokens:      u.TotalTokens,
		Calls:            u.Calls,
		LatencyMs:        u.LatencyMs,
	}
}

func simulationTokenUsageToDTO(u *datasims.SimulationTokenUsage) *SimulationTokenUsageDTO {
	if u == nil {
		return nil
	}
	dto := &SimulationTokenUsageDTO{
		Total: tokenUsageToDTO(u.Total),
	}
	if u.ByModel != nil {
		dto.ByModel = make([]TokenUsageDTO, 0, len(u.ByModel))
		for _, m := range u.ByModel {
			dto.ByModel = append(dto.ByModel, tokenUsageToDTO(m))
		}
	}
	if u.ByAgent != nil {
		dto.ByAgent = make([]AgentTokenUsageDTO, 0, len(u.ByAgent))
		for _, a := range u.ByAgent {
			dto.ByAgent = append(dto.ByAgent, AgentTokenUsageDTO{AgentUID: a.AgentUID, TokenUsageDTO: tokenUsageToDTO(a.TokenUsage)})
		}
	}
	if u.ByPurpose != nil {
		dto.ByPurpose = make([]PurposeTokenUsageDTO, 0, len(u.ByPurpose))
		for _, p := range u.ByPurpose {
			dto.ByPurpose = append(dto.ByPurpose, PurposeTokenUsageDTO{Purpose: string(p.Purpose), TokenUsageDTO: tokenUsageToDTO(p.TokenUsage)})
		}
	}
	if u.ByGeneration != nil {
		dto.ByGeneration = make([]TokenUsageDTO, 0, len(u.ByGeneration))
		for _, g := range u.ByGeneration {
			dto.ByGeneration = append(dto.ByGeneration, tokenUsageToDTO(g))
		}
	}
	if u.ByRun != nil {
		dto.ByRun = make([]TokenUsageDTO, 0, len(u.ByRun))
		for _, r := range u.ByRun {
			dto.ByRun = append(dto.ByRun, tokenUsageToDTO(r))
		}
	}
	return dto
}

// ===================== Status history =====================

func statusHistoryToDTO(history []datasims.SimulationStatusChange) []StatusChangeDTO {
	if history == nil {
		return nil
	}
	out := make([]StatusChangeDTO, 0, len(history))
	for _, h := range history {
		out = append(out, StatusChangeDTO{Status: string(h.Status), At: h.At, By: h.By})
	}
	return out
}

// ===================== Model snapshot =====================

func modelSnapshotToDTO(m *datamodels.Model) *ModelSnapshotDTO {
	if m == nil {
		return nil
	}
	dto := &ModelSnapshotDTO{
		Name:        m.Name,
		Description: m.Description,
		IsSpatial:   m.IsSpatial,
	}
	if m.Agents != nil {
		dto.Agents = make([]SnapshotAgentDTO, 0, len(m.Agents))
		for _, a := range m.Agents {
			dto.Agents = append(dto.Agents, snapshotAgentToDTO(a))
		}
	}
	if m.Environment != nil {
		env := snapshotEnvironmentToDTO(*m.Environment)
		dto.Environment = &env
	}
	return dto
}

func snapshotAgentToDTO(a datamodels.Agent) SnapshotAgentDTO {
	dto := SnapshotAgentDTO{
		UID:            a.UID,
		Name:           a.Name,
		Type:           string(a.Type),
		MoveType:       string(a.MoveType),
		States:         a.States,
		Initialization: snapshotInitToDTO(a.Initialization),
	}
	if a.Properties != nil {
		dto.Properties = make([]SnapshotPropertyDTO, 0, len(a.Properties))
		for _, p := range a.Properties {
			dto.Properties = append(dto.Properties, snapshotPropertyToDTO(p))
		}
	}
	dto.Footprint = snapshotFootprintToDTO(a.Footprint)
	dto.Display = snapshotDisplayToDTO(a.Display)
	dto.Collidable = a.Collidable
	return dto
}

func snapshotFootprintToDTO(f *datamodels.AgentFootprint) *SnapshotFootprintDTO {
	if f == nil {
		return nil
	}
	dto := &SnapshotFootprintDTO{
		Kind:   string(f.Kind),
		Radius: f.Radius,
		Width:  f.Width,
		Height: f.Height,
	}
	if f.Points != nil {
		dto.Points = make([]PointDTO, 0, len(f.Points))
		for _, p := range f.Points {
			dto.Points = append(dto.Points, pointToDTO(p))
		}
	}
	return dto
}

func snapshotDisplayToDTO(d *datamodels.Display) *SnapshotDisplayDTO {
	if d == nil {
		return nil
	}
	dto := &SnapshotDisplayDTO{
		Kind:  string(d.Kind),
		Scale: d.Scale,
	}
	if d.Image != nil {
		dto.Image = &SnapshotDisplayImageDTO{Data: d.Image.Data, MimeType: d.Image.MimeType, Width: d.Image.Width, Height: d.Image.Height}
	}
	if d.Figure != nil {
		dto.Figure = make([]PointDTO, 0, len(d.Figure))
		for _, p := range d.Figure {
			dto.Figure = append(dto.Figure, pointToDTO(p))
		}
	}
	return dto
}

func snapshotPropertyToDTO(p datamodels.AgentProperty) SnapshotPropertyDTO {
	dto := SnapshotPropertyDTO{
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
	}
	if p.Generation != nil {
		g := snapshotGenerationToDTO(*p.Generation)
		dto.Generation = &g
	}
	return dto
}

func snapshotGenerationToDTO(g datamodels.PropertyGeneration) PropertyGenerationDTO {
	dto := PropertyGenerationDTO{
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

func snapshotInitToDTO(init datamodels.AgentInitialization) SnapshotInitializationDTO {
	dto := SnapshotInitializationDTO{
		PopulationCount: init.PopulationCount,
		MinPopulation:   init.MinPopulation,
		MaxPopulation:   init.MaxPopulation,
		InitialState:    init.InitialState,
		CountMode:       string(init.CountMode),
	}
	if init.Placement != nil {
		p := snapshotPlacementToDTO(*init.Placement)
		dto.Placement = &p
	}
	return dto
}

func snapshotPlacementToDTO(p datamodels.Placement) SnapshotPlacementDTO {
	return SnapshotPlacementDTO{
		Type:          string(p.Type),
		ShapeUID:      p.ShapeUID,
		PromptContext: p.PromptContext,
		Clusters:      p.Clusters,
		ClusterRadius: p.ClusterRadius,
	}
}

func snapshotEnvironmentToDTO(env datamodels.Environment) SnapshotEnvironmentDTO {
	dto := SnapshotEnvironmentDTO{
		UID:           env.UID,
		Width:         env.Width,
		Height:        env.Height,
		Torus:         env.Torus,
		CollisionMode: string(env.CollisionMode),
	}
	if env.GlobalProperties != nil {
		dto.GlobalProperties = make([]SnapshotPropertyDTO, 0, len(env.GlobalProperties))
		for _, p := range env.GlobalProperties {
			dto.GlobalProperties = append(dto.GlobalProperties, snapshotPropertyToDTO(p))
		}
	}
	if env.Shapes != nil {
		dto.Shapes = make([]SnapshotShapeDTO, 0, len(env.Shapes))
		for _, s := range env.Shapes {
			dto.Shapes = append(dto.Shapes, snapshotShapeToDTO(s))
		}
	}
	return dto
}

// scalarSpecToDTO converts a *ScalarSpec to a *ScalarSpecDTO. Returns nil for nil input.
func scalarSpecToDTO(s *datamodels.ScalarSpec) *ScalarSpecDTO {
	if s == nil {
		return nil
	}
	return &ScalarSpecDTO{
		Value: s.Value,
		Min:   s.Min,
		Max:   s.Max,
	}
}

func snapshotShapeToDTO(s datamodels.Shape) SnapshotShapeDTO {
	dto := SnapshotShapeDTO{
		UID:            s.UID,
		Name:           s.Name,
		Description:    s.Description,
		Type:           string(s.Type),
		Layer:          s.Layer,
		Visible:        s.Visible,
		Initialization: snapshotInitToDTO(s.Initialization),
	}
	if s.Properties != nil {
		dto.Properties = make([]SnapshotPropertyDTO, 0, len(s.Properties))
		for _, p := range s.Properties {
			dto.Properties = append(dto.Properties, snapshotPropertyToDTO(p))
		}
	}
	if s.Geometry != nil {
		g := s.Geometry
		geom := &SnapshotGeometryDTO{
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
			geom.Position = &p
		}
		if g.Points != nil {
			geom.Points = make([]PointDTO, 0, len(g.Points))
			for _, p := range g.Points {
				geom.Points = append(geom.Points, pointToDTO(p))
			}
		}
		dto.Geometry = geom
	}
	dto.Display = snapshotDisplayToDTO(s.Display)
	return dto
}

// ===================== Termination condition (U2) =====================

func terminationFromDTO(d *TerminationConditionDTO) *datamodels.Condition {
	if d == nil {
		return nil
	}
	c := datamodels.Condition{
		Type:          datamodels.ConditionType(d.Type),
		PropertyUID:   d.PropertyUID,
		Operator:      datamodels.ConditionOperator(d.Operator),
		Value:         d.Value,
		MinValue:      d.MinValue,
		MaxValue:      d.MaxValue,
		TargetUID:     d.TargetUID,
		TargetUIDs:    d.TargetUIDs,
		LogicOperator: datamodels.LogicOperator(d.LogicOperator),
	}
	if len(d.SubConditions) > 0 {
		c.SubConditions = make([]datamodels.Condition, len(d.SubConditions))
		for i := range d.SubConditions {
			c.SubConditions[i] = *terminationFromDTO(&d.SubConditions[i])
		}
	}
	return &c
}

func terminationToDTO(c *datamodels.Condition) *TerminationConditionDTO {
	if c == nil {
		return nil
	}
	d := TerminationConditionDTO{
		Type:          string(c.Type),
		PropertyUID:   c.PropertyUID,
		Operator:      string(c.Operator),
		Value:         c.Value,
		MinValue:      c.MinValue,
		MaxValue:      c.MaxValue,
		TargetUID:     c.TargetUID,
		TargetUIDs:    c.TargetUIDs,
		LogicOperator: string(c.LogicOperator),
	}
	if len(c.SubConditions) > 0 {
		d.SubConditions = make([]TerminationConditionDTO, len(c.SubConditions))
		for i := range c.SubConditions {
			d.SubConditions[i] = *terminationToDTO(&c.SubConditions[i])
		}
	}
	return &d
}

// ===================== Top-level simulation → response =====================

// simulationToResponse converts a full Simulation entity + a pre-computed
// ValidationResult into the API response DTO.
func simulationToResponse(sim datasims.Simulation, validation ValidationResult) SimulationResponse {
	resp := SimulationResponse{
		ID:             sim.ID,
		Name:           sim.Name,
		Description:    sim.Description,
		ModelID:        sim.ModelID,
		Seed:           sim.Seed,
		Steps:          sim.Steps,
		Termination:    terminationToDTO(sim.Termination),
		Agents:         agentsToDTO(sim.Agents),
		Environment:    environmentToDTO(sim.Environment),
		Widgets:        widgetsToDTO(sim.Widgets),
		Status:         string(sim.Status),
		StatusText:     sim.StatusText,
		StatusHistory:  statusHistoryToDTO(sim.StatusHistory),
		GenerationRuns: generationRunsToDTO(sim.GenerationRuns),
		TokenUsage:     simulationTokenUsageToDTO(sim.TokenUsage),
		ExecutionRuns:  executionRunsToDTO(sim.ExecutionRuns),
		CurrentStep:    sim.CurrentStep,
		CreatedAt:      sim.CreatedAt,
		CreatedBy:      sim.CreatedBy,
		UpdatedAt:      sim.UpdatedAt,
		UpdatedBy:      sim.UpdatedBy,
		StartedAt:      sim.StartedAt,
		FinishedAt:     sim.FinishedAt,
		ModelSnapshot:  modelSnapshotToDTO(sim.ModelSnapshot),
		Validation:     validation,
	}
	if sim.LLMGenerationTargetOverride != nil {
		t := llmTargetToDTO(*sim.LLMGenerationTargetOverride)
		resp.LLMGenerationTarget = &t
	}
	if sim.LLMExecutionTargetOverride != nil {
		t := llmTargetToDTO(*sim.LLMExecutionTargetOverride)
		resp.LLMExecutionTarget = &t
	}
	// Statistics is surfaced as-is (data/simulations.SimulationStatistics shape);
	// nil leaves the field omitted. The entity already carries json snake_case
	// tags, so no per-field DTO is duplicated here.
	if sim.Statistics != nil {
		resp.Statistics = sim.Statistics
	}
	return resp
}

// summaryToDTO converts a SimulationSummary lightweight projection to its DTO.
func summaryToDTO(s datasims.SimulationSummary) SimulationSummaryDTO {
	return SimulationSummaryDTO{
		ID:          s.ID,
		Name:        s.Name,
		Description: s.Description,
		ModelID:     s.ModelID,
		ModelName:   s.ModelName,
		Status:      string(s.Status),
		Steps:       s.Steps,
		AgentCount:  s.AgentCount,
		TotalTokens: s.TotalTokens,
		CreatedAt:   s.CreatedAt,
		UpdatedAt:   s.UpdatedAt,
		CreatedBy:   s.CreatedBy,
	}
}
