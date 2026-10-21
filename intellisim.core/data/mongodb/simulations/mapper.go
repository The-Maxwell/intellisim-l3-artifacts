package simulations

import (
	"time"

	"intelli.sim.core/data/common"
	"intelli.sim.core/data/models"
	mongocommon "intelli.sim.core/data/mongodb/common"
	mongomodels "intelli.sim.core/data/mongodb/models"
	"intelli.sim.core/data/simulations"
)

// ---- simulation ----

// SimulationToDocument converts a simulations.Simulation entity to its MongoDB
// document form. Returns an error if Simulation.ID or ModelID is a non-empty,
// malformed hex string.
func SimulationToDocument(s simulations.Simulation) (SimulationDocument, error) {
	id, err := mongocommon.ParseOptionalID(s.ID)
	if err != nil {
		return SimulationDocument{}, err
	}
	modelID, err := mongocommon.ParseOptionalID(s.ModelID)
	if err != nil {
		return SimulationDocument{}, err
	}
	var snapshot *mongomodels.ModelDocument
	if s.ModelSnapshot != nil {
		doc, err := mongomodels.ModelToDocument(*s.ModelSnapshot)
		if err != nil {
			return SimulationDocument{}, err
		}
		snapshot = &doc
	}
	return SimulationDocument{
		ID:                          id,
		Name:                        s.Name,
		Description:                 s.Description,
		ModelID:                     modelID,
		ModelSnapshot:               snapshot,
		Seed:                        s.Seed,
		Steps:                       s.Steps,
		Termination:                 terminationPtrToDocument(s.Termination),
		LLMGenerationTargetOverride: mongocommon.LLMTargetPtrToDocument(s.LLMGenerationTargetOverride),
		LLMExecutionTargetOverride:  mongocommon.LLMTargetPtrToDocument(s.LLMExecutionTargetOverride),
		Agents:                      mongocommon.MapSlice(s.Agents, agentToDocument),
		FinalAgents:                 mongocommon.MapSlice(s.FinalAgents, agentToDocument),
		Environment:                 environmentPtrToDocument(s.Environment),
		Widgets:                     mongocommon.WidgetConfigsToDocuments(s.Widgets),
		Status:                      s.Status,
		StatusText:                  s.StatusText,
		StatusHistory:               mongocommon.MapSlice(s.StatusHistory, statusChangeToDocument),
		GenerationRuns:              mongocommon.MapSlice(s.GenerationRuns, generationRunToDocument),
		ExecutionRuns:               mongocommon.MapSlice(s.ExecutionRuns, executionRunToDocument),
		Statistics:                  statisticsPtrToDocument(s.Statistics),
		TokenUsage:                  tokenUsagePtrToDocument(s.TokenUsage),
		CurrentStep:                 s.CurrentStep,
		CreatedAt:                   s.CreatedAt,
		CreatedBy:                   s.CreatedBy,
		UpdatedAt:                   s.UpdatedAt,
		UpdatedBy:                   s.UpdatedBy,
		StartedAt:                   s.StartedAt,
		FinishedAt:                  s.FinishedAt,
	}, nil
}

// SimulationFromDocument converts a SimulationDocument back to its entity form.
func SimulationFromDocument(d SimulationDocument) simulations.Simulation {
	sim := simulations.Simulation{
		ID:                          mongocommon.FormatOptionalID(d.ID),
		Name:                        d.Name,
		Description:                 d.Description,
		ModelID:                     mongocommon.FormatID(d.ModelID),
		Seed:                        d.Seed,
		Steps:                       d.Steps,
		Termination:                 terminationPtrFromDocument(d.Termination),
		LLMGenerationTargetOverride: mongocommon.LLMTargetPtrFromDocument(d.LLMGenerationTargetOverride),
		LLMExecutionTargetOverride:  mongocommon.LLMTargetPtrFromDocument(d.LLMExecutionTargetOverride),
		Agents:                      mongocommon.MapSlice(d.Agents, agentFromDocument),
		FinalAgents:                 mongocommon.MapSlice(d.FinalAgents, agentFromDocument),
		Environment:                 environmentPtrFromDocument(d.Environment),
		Widgets:                     mongocommon.WidgetConfigsFromDocuments(d.Widgets),
		Status:                      d.Status,
		StatusText:                  d.StatusText,
		StatusHistory:               mongocommon.MapSlice(d.StatusHistory, statusChangeFromDocument),
		GenerationRuns:              mongocommon.MapSlice(d.GenerationRuns, generationRunFromDocument),
		ExecutionRuns:               mongocommon.MapSlice(d.ExecutionRuns, executionRunFromDocument),
		Statistics:                  statisticsPtrFromDocument(d.Statistics),
		TokenUsage:                  tokenUsagePtrFromDocument(d.TokenUsage),
		CurrentStep:                 d.CurrentStep,
		CreatedAt:                   d.CreatedAt,
		CreatedBy:                   d.CreatedBy,
		UpdatedAt:                   d.UpdatedAt,
		UpdatedBy:                   d.UpdatedBy,
		StartedAt:                   d.StartedAt,
		FinishedAt:                  d.FinishedAt,
	}
	if d.ModelSnapshot != nil {
		m := mongomodels.ModelFromDocument(*d.ModelSnapshot)
		sim.ModelSnapshot = &m
	}
	return sim
}

// ---- agents ----

func agentToDocument(a simulations.SimulationAgent) SimulationAgentDocument {
	return SimulationAgentDocument{
		ID:         a.ID,
		AgentUID:   a.AgentUID,
		Name:       a.Name,
		Position:   mongocommon.PointPtrToDocument(a.Position),
		Properties: mongocommon.CopyMap(a.Properties),
		State:      a.State,
		Memory:     mongocommon.MapSlice(a.Memory, memoryToDocument),
	}
}

func agentFromDocument(d SimulationAgentDocument) simulations.SimulationAgent {
	return simulations.SimulationAgent{
		ID:         d.ID,
		AgentUID:   d.AgentUID,
		Name:       d.Name,
		Position:   mongocommon.PointPtrFromDocument(d.Position),
		Properties: mongocommon.NormalizeMap(d.Properties),
		State:      d.State,
		Memory:     mongocommon.MapSlice(d.Memory, memoryFromDocument),
	}
}

func memoryToDocument(m simulations.AgentMemory) AgentMemoryDocument {
	return AgentMemoryDocument{
		Step:      m.Step,
		Type:      m.Type,
		Summary:   m.Summary,
		Details:   m.Details,
		Timestamp: m.Timestamp,
	}
}

func memoryFromDocument(d AgentMemoryDocument) simulations.AgentMemory {
	return simulations.AgentMemory{
		Step:      d.Step,
		Type:      d.Type,
		Summary:   d.Summary,
		Details:   mongocommon.NormalizeValue(d.Details),
		Timestamp: d.Timestamp,
	}
}

// ---- environment / shapes ----

func environmentPtrToDocument(e *simulations.SimulationEnvironment) *SimulationEnvironmentDocument {
	if e == nil {
		return nil
	}
	return &SimulationEnvironmentDocument{
		Width:            e.Width,
		Height:           e.Height,
		Torus:            e.Torus,
		CollisionMode:    e.CollisionMode,
		CollisionConfig:  mongomodels.CollisionConfigPtrToDocument(e.CollisionConfig),
		GlobalProperties: mongocommon.CopyMap(e.GlobalProperties),
		Shapes:           mongocommon.MapSlice(e.Shapes, shapeToDocument),
	}
}

func environmentPtrFromDocument(d *SimulationEnvironmentDocument) *simulations.SimulationEnvironment {
	if d == nil {
		return nil
	}
	return &simulations.SimulationEnvironment{
		Width:            d.Width,
		Height:           d.Height,
		Torus:            d.Torus,
		CollisionMode:    d.CollisionMode,
		CollisionConfig:  mongomodels.CollisionConfigPtrFromDocument(d.CollisionConfig),
		GlobalProperties: mongocommon.NormalizeMap(d.GlobalProperties),
		Shapes:           mongocommon.MapSlice(d.Shapes, shapeFromDocument),
	}
}

func shapeToDocument(s simulations.SimulationShape) SimulationShapeDocument {
	return SimulationShapeDocument{
		ID:          s.ID,
		ShapeUID:    s.ShapeUID,
		Name:        s.Name,
		Type:        s.Type,
		Layer:       s.Layer,
		Visible:     s.Visible,
		Properties:  mongocommon.CopyMap(s.Properties),
		Position:    mongocommon.PointPtrToDocument(&s.Position),
		Width:       s.Width,
		Height:      s.Height,
		Radius:      s.Radius,
		Points:      mongocommon.MapSlice(s.Points, mongocommon.PointToDocument),
		Thickness:   s.Thickness,
		Fill:        s.Fill,
		Stroke:      s.Stroke,
		StrokeWidth: s.StrokeWidth,
	}
}

func shapeFromDocument(d SimulationShapeDocument) simulations.SimulationShape {
	var pos common.Point
	if d.Position != nil {
		pos = mongocommon.PointFromDocument(*d.Position)
	}
	return simulations.SimulationShape{
		ID:          d.ID,
		ShapeUID:    d.ShapeUID,
		Name:        d.Name,
		Type:        d.Type,
		Layer:       d.Layer,
		Visible:     d.Visible,
		Properties:  mongocommon.NormalizeMap(d.Properties),
		Position:    pos,
		Width:       d.Width,
		Height:      d.Height,
		Radius:      d.Radius,
		Points:      mongocommon.MapSlice(d.Points, mongocommon.PointFromDocument),
		Thickness:   d.Thickness,
		Fill:        d.Fill,
		Stroke:      d.Stroke,
		StrokeWidth: d.StrokeWidth,
	}
}

// ---- status change ----

func statusChangeToDocument(c simulations.SimulationStatusChange) SimulationStatusChangeDocument {
	return SimulationStatusChangeDocument{Status: c.Status, At: c.At, By: c.By}
}

func statusChangeFromDocument(d SimulationStatusChangeDocument) simulations.SimulationStatusChange {
	return simulations.SimulationStatusChange{Status: d.Status, At: d.At, By: d.By}
}

// ---- generation runs ----

func generationRunToDocument(r simulations.GenerationRun) GenerationRunDocument {
	return GenerationRunDocument{
		Seq:        r.Seq,
		Seed:       r.Seed,
		Scope:      generationScopeToDocument(r.Scope),
		Status:     r.Status,
		StartedAt:  r.StartedAt,
		FinishedAt: r.FinishedAt,
		Error:      r.Error,
		Warnings:   mongocommon.MapSlice(r.Warnings, generationWarningToDocument),
		Requests:   r.Requests,
	}
}

func generationRunFromDocument(d GenerationRunDocument) simulations.GenerationRun {
	return simulations.GenerationRun{
		Seq:        d.Seq,
		Seed:       d.Seed,
		Scope:      generationScopeFromDocument(d.Scope),
		Status:     d.Status,
		StartedAt:  d.StartedAt,
		FinishedAt: d.FinishedAt,
		Error:      d.Error,
		Warnings:   mongocommon.MapSlice(d.Warnings, generationWarningFromDocument),
		Requests:   d.Requests,
	}
}

func generationScopeToDocument(s simulations.GenerationScope) GenerationScopeDocument {
	return GenerationScopeDocument{
		Kind:      s.Kind,
		AgentUIDs: append([]string(nil), s.AgentUIDs...),
		Parts:     append([]simulations.GenerationPart(nil), s.Parts...),
	}
}

func generationScopeFromDocument(d GenerationScopeDocument) simulations.GenerationScope {
	return simulations.GenerationScope{
		Kind:      d.Kind,
		AgentUIDs: append([]string(nil), d.AgentUIDs...),
		Parts:     append([]simulations.GenerationPart(nil), d.Parts...),
	}
}

func generationWarningToDocument(w simulations.GenerationWarning) GenerationWarningDocument {
	return GenerationWarningDocument{Path: w.Path, Reason: w.Reason}
}

func generationWarningFromDocument(d GenerationWarningDocument) simulations.GenerationWarning {
	return simulations.GenerationWarning{Path: d.Path, Reason: d.Reason}
}

// ---- termination condition (U2) ----

func terminationPtrToDocument(c *models.Condition) *mongomodels.ConditionDocument {
	if c == nil {
		return nil
	}
	d := mongomodels.ConditionToDocument(*c)
	return &d
}

func terminationPtrFromDocument(d *mongomodels.ConditionDocument) *models.Condition {
	if d == nil {
		return nil
	}
	c := mongomodels.ConditionFromDocument(*d)
	return &c
}

// ---- execution runs ----

func executionRunToDocument(r simulations.ExecutionRun) ExecutionRunDocument {
	return ExecutionRunDocument{
		Seq:               r.Seq,
		StartedAt:         r.StartedAt,
		FinishedAt:        r.FinishedAt,
		StepsCompleted:    r.StepsCompleted,
		Status:            r.Status,
		Error:             r.Error,
		ActiveDurationMs:  r.ActiveDuration.Milliseconds(),
		TerminationReason: r.TerminationReason,
	}
}

func executionRunFromDocument(d ExecutionRunDocument) simulations.ExecutionRun {
	return simulations.ExecutionRun{
		Seq:               d.Seq,
		StartedAt:         d.StartedAt,
		FinishedAt:        d.FinishedAt,
		StepsCompleted:    d.StepsCompleted,
		Status:            d.Status,
		Error:             d.Error,
		ActiveDuration:    time.Duration(d.ActiveDurationMs) * time.Millisecond,
		TerminationReason: d.TerminationReason,
	}
}

// ---- token usage ----

func agentTokenUsageToDocument(a simulations.AgentTokenUsage) AgentTokenUsageDocument {
	return AgentTokenUsageDocument{
		AgentUID:           a.AgentUID,
		TokenUsageDocument: mongocommon.TokenUsageToDocument(a.TokenUsage),
	}
}

func agentTokenUsageFromDocument(a AgentTokenUsageDocument) simulations.AgentTokenUsage {
	return simulations.AgentTokenUsage{
		AgentUID:   a.AgentUID,
		TokenUsage: mongocommon.TokenUsageFromDocument(a.TokenUsageDocument),
	}
}

func purposeTokenUsageToDocument(p simulations.PurposeTokenUsage) PurposeTokenUsageDocument {
	return PurposeTokenUsageDocument{
		Purpose:            p.Purpose,
		TokenUsageDocument: mongocommon.TokenUsageToDocument(p.TokenUsage),
	}
}

func purposeTokenUsageFromDocument(p PurposeTokenUsageDocument) simulations.PurposeTokenUsage {
	return simulations.PurposeTokenUsage{
		Purpose:    p.Purpose,
		TokenUsage: mongocommon.TokenUsageFromDocument(p.TokenUsageDocument),
	}
}

func tokenUsagePtrToDocument(u *simulations.SimulationTokenUsage) *SimulationTokenUsageDocument {
	if u == nil {
		return nil
	}
	return &SimulationTokenUsageDocument{
		Total:        mongocommon.TokenUsageToDocument(u.Total),
		ByModel:      mongocommon.MapSlice(u.ByModel, mongocommon.TokenUsageToDocument),
		ByAgent:      mongocommon.MapSlice(u.ByAgent, agentTokenUsageToDocument),
		ByPurpose:    mongocommon.MapSlice(u.ByPurpose, purposeTokenUsageToDocument),
		ByGeneration: mongocommon.MapSlice(u.ByGeneration, mongocommon.TokenUsageToDocument),
		ByRun:        mongocommon.MapSlice(u.ByRun, mongocommon.TokenUsageToDocument),
	}
}

func tokenUsagePtrFromDocument(d *SimulationTokenUsageDocument) *simulations.SimulationTokenUsage {
	if d == nil {
		return nil
	}
	return &simulations.SimulationTokenUsage{
		Total:        mongocommon.TokenUsageFromDocument(d.Total),
		ByModel:      mongocommon.MapSlice(d.ByModel, mongocommon.TokenUsageFromDocument),
		ByAgent:      mongocommon.MapSlice(d.ByAgent, agentTokenUsageFromDocument),
		ByPurpose:    mongocommon.MapSlice(d.ByPurpose, purposeTokenUsageFromDocument),
		ByGeneration: mongocommon.MapSlice(d.ByGeneration, mongocommon.TokenUsageFromDocument),
		ByRun:        mongocommon.MapSlice(d.ByRun, mongocommon.TokenUsageFromDocument),
	}
}

// ---- statistics ----

func statisticsPtrToDocument(s *simulations.SimulationStatistics) *SimulationStatisticsDocument {
	if s == nil {
		return nil
	}
	return &SimulationStatisticsDocument{
		StepsCompleted:   s.StepsCompleted,
		TotalAgents:      s.TotalAgents,
		Population:       mongocommon.MapSlice(s.Population, agentPopulationToDocument),
		AgentsSpawned:    s.AgentsSpawned,
		AgentsRemoved:    s.AgentsRemoved,
		Interactions:     s.Interactions,
		Events:           s.Events,
		RuleExecutions:   s.RuleExecutions,
		MovementDistance: s.MovementDistance,
		ByRule:           mongocommon.MapSlice(s.ByRule, ruleStatisticToDocument),
		ByEvent:          mongocommon.MapSlice(s.ByEvent, eventStatisticToDocument),
		ByInteraction:    mongocommon.MapSlice(s.ByInteraction, interactionStatisticToDocument),
		Properties:       mongocommon.MapSlice(s.Properties, propertyStatisticToDocument),
		Custom:           mongocommon.CopyMap(s.Custom),
	}
}

func statisticsPtrFromDocument(d *SimulationStatisticsDocument) *simulations.SimulationStatistics {
	if d == nil {
		return nil
	}
	return &simulations.SimulationStatistics{
		StepsCompleted:   d.StepsCompleted,
		TotalAgents:      d.TotalAgents,
		Population:       mongocommon.MapSlice(d.Population, agentPopulationFromDocument),
		AgentsSpawned:    d.AgentsSpawned,
		AgentsRemoved:    d.AgentsRemoved,
		Interactions:     d.Interactions,
		Events:           d.Events,
		RuleExecutions:   d.RuleExecutions,
		MovementDistance: d.MovementDistance,
		ByRule:           mongocommon.MapSlice(d.ByRule, ruleStatisticFromDocument),
		ByEvent:          mongocommon.MapSlice(d.ByEvent, eventStatisticFromDocument),
		ByInteraction:    mongocommon.MapSlice(d.ByInteraction, interactionStatisticFromDocument),
		Properties:       mongocommon.MapSlice(d.Properties, propertyStatisticFromDocument),
		Custom:           mongocommon.NormalizeMap(d.Custom),
	}
}

func agentPopulationToDocument(p simulations.AgentPopulation) AgentPopulationDocument {
	return AgentPopulationDocument{
		AgentUID: p.AgentUID,
		Current:  p.Current,
		Peak:     p.Peak,
		Min:      p.Min,
		Spawned:  p.Spawned,
		Removed:  p.Removed,
		ByState:  copyUint64Map(p.ByState),
	}
}

func agentPopulationFromDocument(d AgentPopulationDocument) simulations.AgentPopulation {
	return simulations.AgentPopulation{
		AgentUID: d.AgentUID,
		Current:  d.Current,
		Peak:     d.Peak,
		Min:      d.Min,
		Spawned:  d.Spawned,
		Removed:  d.Removed,
		ByState:  copyUint64Map(d.ByState),
	}
}

func ruleStatisticToDocument(r simulations.RuleStatistic) RuleStatisticDocument {
	return RuleStatisticDocument{RuleID: r.RuleID, Executions: r.Executions}
}

func ruleStatisticFromDocument(d RuleStatisticDocument) simulations.RuleStatistic {
	return simulations.RuleStatistic{RuleID: d.RuleID, Executions: d.Executions}
}

func eventStatisticToDocument(e simulations.EventStatistic) EventStatisticDocument {
	return EventStatisticDocument{EventUID: e.EventUID, Emitted: e.Emitted}
}

func eventStatisticFromDocument(d EventStatisticDocument) simulations.EventStatistic {
	return simulations.EventStatistic{EventUID: d.EventUID, Emitted: d.Emitted}
}

func interactionStatisticToDocument(i simulations.InteractionStatistic) InteractionStatisticDocument {
	return InteractionStatisticDocument{InteractionID: i.InteractionID, Count: i.Count}
}

func interactionStatisticFromDocument(d InteractionStatisticDocument) simulations.InteractionStatistic {
	return simulations.InteractionStatistic{InteractionID: d.InteractionID, Count: d.Count}
}

func propertyStatisticToDocument(p simulations.PropertyStatistic) PropertyStatisticDocument {
	return PropertyStatisticDocument{
		AgentUID:    p.AgentUID,
		PropertyUID: p.PropertyUID,
		Avg:         p.Avg,
		Min:         p.Min,
		Max:         p.Max,
	}
}

func propertyStatisticFromDocument(d PropertyStatisticDocument) simulations.PropertyStatistic {
	return simulations.PropertyStatistic{
		AgentUID:    d.AgentUID,
		PropertyUID: d.PropertyUID,
		Avg:         d.Avg,
		Min:         d.Min,
		Max:         d.Max,
	}
}

// copyUint64Map returns a shallow copy of a uint64-valued map, or nil for nil/empty
// input so omitempty + round-trips stay symmetric.
func copyUint64Map(m map[string]uint64) map[string]uint64 {
	if len(m) == 0 {
		return nil
	}
	out := make(map[string]uint64, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// ---- steps (used by steps_repository.go) ----

// StepToDocument converts a simulations.SimulationStep to its MongoDB document form.
func StepToDocument(s simulations.SimulationStep) (SimulationStepDocument, error) {
	id, err := mongocommon.ParseOptionalID(s.ID)
	if err != nil {
		return SimulationStepDocument{}, err
	}
	simID, err := mongocommon.ParseOptionalID(s.Metadata.SimulationID)
	if err != nil {
		return SimulationStepDocument{}, err
	}
	return SimulationStepDocument{
		ID: id,
		Metadata: SimulationStepDocumentMetadata{
			SimulationID: simID,
			Step:         s.Metadata.Step,
			ReferenceID:  s.Metadata.ReferenceID,
			Type:         s.Metadata.Type,
			UID:          s.Metadata.UID,
		},
		Timestamp:  s.Timestamp,
		Position:   mongocommon.PointPtrToDocument(s.Position),
		Properties: mongocommon.CopyMap(s.Properties),
		State:      s.State,
		Decision:   s.Decision,
		Statistics: statisticsPtrToDocument(s.Statistics),
	}, nil
}

// StepFromDocument converts a SimulationStepDocument back to its entity form.
func StepFromDocument(d SimulationStepDocument) simulations.SimulationStep {
	return simulations.SimulationStep{
		ID: mongocommon.FormatOptionalID(d.ID),
		Metadata: simulations.StepMetadata{
			SimulationID: mongocommon.FormatOptionalID(d.Metadata.SimulationID),
			Step:         d.Metadata.Step,
			ReferenceID:  d.Metadata.ReferenceID,
			Type:         d.Metadata.Type,
			UID:          d.Metadata.UID,
		},
		Timestamp:  d.Timestamp,
		Position:   mongocommon.PointPtrFromDocument(d.Position),
		Properties: mongocommon.NormalizeMap(d.Properties),
		State:      d.State,
		Decision:   d.Decision,
		Statistics: statisticsPtrFromDocument(d.Statistics),
	}
}
