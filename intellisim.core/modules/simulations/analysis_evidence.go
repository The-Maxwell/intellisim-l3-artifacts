package simulations

import (
	"context"
	"slices"

	datamodels "intelli.sim.core/data/models"
	datasims "intelli.sim.core/data/simulations"
)

// EvidencePack is the compact, structured input the analysis prompts + PDF consume.
// It holds aggregates (never raw per-agent frames), keeping prompts token-bounded.
type EvidencePack struct {
	Model               ModelSummary
	Run                 RunSummary
	Statistics          *datasims.SimulationStatistics
	Population          []datasims.PopulationPoint
	PropertySeries      []PropertySeries
	ShapePropertySeries []ShapePropertySeries
	Spatial             bool
	SpatialSnapshots    []SpatialSnapshot
}

// ShapePropertySeries is one shape template's numeric property over time (avg
// across instances of that template, matching the agent-series semantics).
type ShapePropertySeries struct {
	ShapeUID     string
	ShapeName    string
	PropertyUID  string
	PropertyName string
	Points       []datasims.TimeSeriesPoint
}

// SpatialSnapshot is a bounded sample of agent spatial state at one step.
type SpatialSnapshot struct {
	Step   int64
	Agents []datasims.SpatialAgentData
}

type ModelSummary struct {
	Name         string
	Description  string
	IsSpatial    bool
	UpdateMode   string
	Archetypes   []ArchetypeSummary
	RuleCount    int
	Interactions int
	Events       int
	Environment  *EnvironmentSummary
}

type ArchetypeSummary struct {
	UID        string
	Name       string
	Type       string
	States     []string
	Properties []PropertyDescriptor
}

type PropertyDescriptor struct {
	UID  string
	Name string
	Type string
}

type EnvironmentSummary struct {
	Width, Height uint64
	Torus         bool
	GlobalProps   []PropertyDescriptor
	ShapeCount    int
}

type RunSummary struct {
	Name            string
	ConfiguredSteps uint64
	StepsCompleted  uint64
	Seed            uint64
	Status          string
}

type PropertySeries struct {
	PropertyUID string
	Name        string
	Points      []datasims.TimeSeriesPoint
}

// buildEvidence assembles the deterministic evidence pack for a simulation,
// honoring the selection (range/sampling/agents/properties). Returns the pack
// plus the effective window covered.
func buildEvidence(ctx context.Context, steps datasims.SimulationStepsRepository, sim *datasims.Simulation, sel datasims.AnalysisSelection, cfg AnalysisConfig) (EvidencePack, datasims.AnalysisStepsCovered, error) {
	model := sim.ModelSnapshot
	pack := EvidencePack{
		Model:      summarizeModel(model),
		Run:        RunSummary{Name: sim.Name, ConfiguredSteps: sim.Steps, Seed: sim.Seed, Status: string(sim.Status)},
		Statistics: sim.Statistics,
	}
	if model != nil {
		pack.Spatial = model.IsSpatial
	}
	if sim.Statistics != nil {
		pack.Run.StepsCompleted = sim.Statistics.StepsCompleted
	}

	// Resolve the effective step window from the recorded range + selection.
	min, max, err := steps.GetStepRange(ctx, sim.ID)
	if err != nil {
		return EvidencePack{}, datasims.AnalysisStepsCovered{}, err
	}
	from, to := min, max
	if sel.StepFrom != nil && int64(*sel.StepFrom) > from {
		from = int64(*sel.StepFrom)
	}
	if sel.StepTo != nil && int64(*sel.StepTo) < to {
		to = int64(*sel.StepTo)
	}
	if to < from {
		to = from
	}
	sampling := int64(1)
	if sel.Sampling != nil && *sel.Sampling > 1 {
		sampling = int64(*sel.Sampling)
	}
	stepRange := &datasims.StepRange{Start: &from, End: &to}

	// Population over time — always assembled (the always-on emergence section
	// needs it regardless of the selected focus areas). Focus areas only drive
	// section planning, not data assembly.
	if pop, err := steps.CountByAgentTypePerStep(ctx, sim.ID, stepRange); err == nil {
		pop = thinPopulation(pop, sampling)
		pack.Population = downsamplePopulation(pop, cfg.MaxSeriesPoints)
	}

	// Per-property numeric series (selected properties or all numeric) — always assembled.
	propUIDs := selectedNumericProperties(model, steps, ctx, sim.ID, sel)
	for _, p := range propUIDs {
		series, err := steps.AggregatePropertyByStep(ctx, sim.ID, p.UID, sel.AgentUIDs, stepRange, datasims.AggregationAvg)
		if err != nil || len(series) == 0 {
			continue
		}
		series = thinSeries(series, sampling)
		pack.PropertySeries = append(pack.PropertySeries, PropertySeries{
			PropertyUID: p.UID, Name: p.Name, Points: downsampleSeries(series, cfg.MaxSeriesPoints),
		})
	}

	// Shape property series — numeric declared shape properties intersected with
	// what was actually recorded; deterministic order: model shape slice order ×
	// declared property order. Empty/erroring series are skipped (fail-soft).
	if model != nil && model.Environment != nil && len(model.Environment.Shapes) > 0 {
		if recorded, err := steps.FindDistinctShapeProperties(ctx, sim.ID); err == nil && len(recorded) > 0 {
			recordedSet := make(map[string]bool, len(recorded))
			for _, rp := range recorded {
				recordedSet[rp.UID] = true
			}
			for si := range model.Environment.Shapes {
				shp := &model.Environment.Shapes[si]
				for pi := range shp.Properties {
					p := &shp.Properties[pi]
					if !isNumericType(string(p.Type)) || !recordedSet[p.UID] {
						continue
					}
					if len(sel.PropertyUIDs) > 0 && !slices.Contains(sel.PropertyUIDs, p.UID) {
						continue
					}
					series, err := steps.AggregateShapePropertyByStep(ctx, sim.ID, p.UID, []string{shp.UID}, stepRange, datasims.AggregationAvg)
					if err != nil || len(series) == 0 {
						continue
					}
					series = thinSeries(series, sampling)
					name := shp.Name
					if name == "" {
						name = shp.UID
					}
					propName := p.Name
					if propName == "" {
						propName = p.UID
					}
					pack.ShapePropertySeries = append(pack.ShapePropertySeries, ShapePropertySeries{
						ShapeUID: shp.UID, ShapeName: name,
						PropertyUID: p.UID, PropertyName: propName,
						Points: downsampleSeries(series, cfg.MaxSeriesPoints),
					})
				}
			}
		}
	}

	// Spatial snapshots at up to 3 steps (from, midpoint, to) for spatial models.
	if model != nil && model.IsSpatial {
		pack.SpatialSnapshots = buildSpatialSnapshots(ctx, steps, sim.ID, from, to, sel.PropertyUIDs)
	}

	// Effective number of points actually retained after thinning + downsampling.
	count := len(pack.Population)
	for _, ps := range pack.PropertySeries {
		if len(ps.Points) > count {
			count = len(ps.Points)
		}
	}
	for _, sps := range pack.ShapePropertySeries {
		if len(sps.Points) > count {
			count = len(sps.Points)
		}
	}
	covered := datasims.AnalysisStepsCovered{
		From: uint64(from), To: uint64(to), Sampling: uint64(sampling),
		Count: uint64(count),
	}
	return pack, covered, nil
}

// buildSpatialSnapshots samples agent spatial state at the window's start,
// midpoint, and end (deduplicated), bounding each snapshot to 50 agents. Errors
// are ignored per-snapshot (the snapshot is skipped).
func buildSpatialSnapshots(ctx context.Context, steps datasims.SimulationStepsRepository, simID string, from, to int64, propertyUIDs []string) []SpatialSnapshot {
	const maxAgents = 50
	stepsToSample := []int64{from, from + (to-from)/2, to}
	seen := map[int64]bool{}
	var out []SpatialSnapshot
	for _, st := range stepsToSample {
		if seen[st] {
			continue
		}
		seen[st] = true
		agents, err := steps.FindSpatialAtStep(ctx, simID, st, propertyUIDs)
		if err != nil || len(agents) == 0 {
			continue
		}
		if len(agents) > maxAgents {
			agents = agents[:maxAgents]
		}
		out = append(out, SpatialSnapshot{Step: st, Agents: agents})
	}
	return out
}

// thinPopulation keeps only points whose step is divisible by sampling (>1).
func thinPopulation(in []datasims.PopulationPoint, sampling int64) []datasims.PopulationPoint {
	if sampling <= 1 {
		return in
	}
	out := in[:0:0]
	for _, p := range in {
		if p.Step%sampling == 0 {
			out = append(out, p)
		}
	}
	return out
}

// thinSeries keeps only points whose step is divisible by sampling (>1).
func thinSeries(in []datasims.TimeSeriesPoint, sampling int64) []datasims.TimeSeriesPoint {
	if sampling <= 1 {
		return in
	}
	out := in[:0:0]
	for _, p := range in {
		if p.Step%sampling == 0 {
			out = append(out, p)
		}
	}
	return out
}

func focusEnabled(sel datasims.AnalysisSelection, area datasims.AnalysisFocusArea) bool {
	if len(sel.FocusAreas) == 0 {
		return true // none selected = all
	}
	return slices.Contains(sel.FocusAreas, area)
}

func summarizeModel(m *datamodels.Model) ModelSummary {
	if m == nil {
		return ModelSummary{}
	}
	s := ModelSummary{
		Name: m.Name, Description: m.Description, IsSpatial: m.IsSpatial,
		UpdateMode: string(m.SimulationConfig.UpdateMode), RuleCount: len(m.Rules),
		Interactions: len(m.Interactions), Events: len(m.Events),
	}
	for _, a := range m.Agents {
		as := ArchetypeSummary{UID: a.UID, Name: a.Name, Type: string(a.Type), States: a.States}
		for _, p := range a.Properties {
			as.Properties = append(as.Properties, PropertyDescriptor{UID: p.UID, Name: p.Name, Type: string(p.Type)})
		}
		s.Archetypes = append(s.Archetypes, as)
	}
	if m.Environment != nil {
		env := &EnvironmentSummary{
			Width: m.Environment.Width, Height: m.Environment.Height,
			Torus: m.Environment.Torus, ShapeCount: len(m.Environment.Shapes),
		}
		for _, p := range m.Environment.GlobalProperties {
			env.GlobalProps = append(env.GlobalProps, PropertyDescriptor{UID: p.UID, Name: p.Name, Type: string(p.Type)})
		}
		s.Environment = env
	}
	return s
}

// selectedNumericProperties resolves which numeric properties to chart/analyze:
// the user's PropertyUIDs filtered to numeric, or all recorded numeric properties.
func selectedNumericProperties(m *datamodels.Model, steps datasims.SimulationStepsRepository, ctx context.Context, simID string, sel datasims.AnalysisSelection) []PropertyDescriptor {
	// Build a name lookup from the model snapshot.
	names := map[string]string{}
	types := map[string]string{}
	if m != nil {
		for _, a := range m.Agents {
			for _, p := range a.Properties {
				names[p.UID] = p.Name
				types[p.UID] = string(p.Type)
			}
		}
	}
	recorded, _ := steps.FindDistinctProperties(ctx, simID)
	var out []PropertyDescriptor
	for _, rp := range recorded {
		if len(sel.PropertyUIDs) > 0 && !slices.Contains(sel.PropertyUIDs, rp.UID) {
			continue
		}
		// Prefer the model's declared type; fall back to recorded data type.
		// The model vocabulary is "number"; the recorded-inference vocabulary
		// is "integer"/"float" — accept all of them as numeric.
		t := types[rp.UID]
		if t == "" {
			t = rp.DataType
		}
		if !isNumericType(t) {
			continue
		}
		name := names[rp.UID]
		if name == "" {
			name = rp.UID
		}
		out = append(out, PropertyDescriptor{UID: rp.UID, Name: name, Type: "number"})
	}
	return out
}

// isNumericType reports whether a property type label denotes a numeric value,
// spanning both the model vocabulary ("number") and the recorded-inference
// vocabulary ("integer"/"float").
func isNumericType(t string) bool {
	return t == "number" || t == "integer" || t == "float"
}

func downsamplePopulation(in []datasims.PopulationPoint, maxN int) []datasims.PopulationPoint {
	if maxN <= 0 || len(in) <= maxN {
		return in
	}
	stride := len(in) / maxN
	if stride < 1 {
		stride = 1
	}
	out := make([]datasims.PopulationPoint, 0, maxN+1)
	for i := 0; i < len(in); i += stride {
		out = append(out, in[i])
	}
	if last := in[len(in)-1]; len(out) == 0 || out[len(out)-1].Step != last.Step {
		out = append(out, last)
	}
	return out
}

func downsampleSeries(in []datasims.TimeSeriesPoint, maxN int) []datasims.TimeSeriesPoint {
	if maxN <= 0 || len(in) <= maxN {
		return in
	}
	stride := len(in) / maxN
	if stride < 1 {
		stride = 1
	}
	out := make([]datasims.TimeSeriesPoint, 0, maxN+1)
	for i := 0; i < len(in); i += stride {
		out = append(out, in[i])
	}
	if last := in[len(in)-1]; len(out) == 0 || out[len(out)-1].Step != last.Step {
		out = append(out, last)
	}
	return out
}
