package simulations

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	datamodels "intelli.sim.core/data/models"
	datasims "intelli.sim.core/data/simulations"
)

// fakeStepsRepo implements only the analysis query subset used by evidence assembly.
type fakeStepsRepo struct {
	datasims.SimulationStepsRepository // embed nil; only override what we use
	min, max                           int64
	props                              []datasims.PropertyInfo
	pop                                []datasims.PopulationPoint
	series                             []datasims.TimeSeriesPoint
	shapeProps                         []datasims.PropertyInfo
	shapeSeries                        []datasims.TimeSeriesPoint
	shapeCalls                         [][]string // records shapeUIDs per AggregateShapePropertyByStep call
	agentCalls                         [][]string // records agentUIDs per AggregatePropertyByStep call
}

func (f *fakeStepsRepo) GetStepRange(_ context.Context, _ string) (int64, int64, error) {
	return f.min, f.max, nil
}
func (f *fakeStepsRepo) FindDistinctProperties(_ context.Context, _ string) ([]datasims.PropertyInfo, error) {
	return f.props, nil
}
func (f *fakeStepsRepo) CountByAgentTypePerStep(_ context.Context, _ string, _ *datasims.StepRange) ([]datasims.PopulationPoint, error) {
	return f.pop, nil
}
func (f *fakeStepsRepo) AggregatePropertyByStep(_ context.Context, _, _ string, agentUIDs []string, _ *datasims.StepRange, _ datasims.Aggregation) ([]datasims.TimeSeriesPoint, error) {
	f.agentCalls = append(f.agentCalls, agentUIDs)
	return f.series, nil
}
func (f *fakeStepsRepo) FindSpatialAtStep(_ context.Context, _ string, _ int64, _ []string) ([]datasims.SpatialAgentData, error) {
	return nil, nil
}
func (f *fakeStepsRepo) FindDistinctShapeProperties(_ context.Context, _ string) ([]datasims.PropertyInfo, error) {
	return f.shapeProps, nil
}
func (f *fakeStepsRepo) AggregateShapePropertyByStep(_ context.Context, _, _ string, shapeUIDs []string, _ *datasims.StepRange, _ datasims.Aggregation) ([]datasims.TimeSeriesPoint, error) {
	f.shapeCalls = append(f.shapeCalls, shapeUIDs)
	return f.shapeSeries, nil
}

func testModelWithLake() *datamodels.Model {
	m := testModel()
	m.Environment = &datamodels.Environment{
		Width: 40, Height: 30,
		Shapes: []datamodels.Shape{{
			UID: "lake", Name: "Lake", Type: datamodels.ShapeTypeCircle,
			Properties: []datamodels.AgentProperty{
				{UID: "water_level", Name: "Water level", Type: datamodels.PropertyTypeNumber},
				{UID: "label", Name: "Label", Type: datamodels.PropertyTypeString},
			},
		}},
	}
	return m
}

func TestBuildEvidence_ShapeSeries(t *testing.T) {
	repo := &fakeStepsRepo{
		min: 0, max: 5,
		props:       []datasims.PropertyInfo{{UID: "energy", DataType: "number"}},
		shapeProps:  []datasims.PropertyInfo{{UID: "water_level", DataType: "float"}},
		pop:         []datasims.PopulationPoint{{Step: 0, Count: map[string]int{"prey": 3}}},
		series:      []datasims.TimeSeriesPoint{{Step: 0, Value: 10.0}},
		shapeSeries: []datasims.TimeSeriesPoint{{Step: 0, Value: 50.0}, {Step: 5, Value: 36.0}},
	}
	sim := &datasims.Simulation{
		ID: "sim1", Name: "Run A", ModelSnapshot: testModelWithLake(), Steps: 5,
		Status: datasims.SimulationStatusFinished,
	}
	pack, covered, err := buildEvidence(context.Background(), repo, sim, datasims.AnalysisSelection{}, AnalysisConfig{MaxSeriesPoints: 60})
	require.NoError(t, err)
	require.Len(t, pack.ShapePropertySeries, 1) // string property excluded (numeric-only)
	sps := pack.ShapePropertySeries[0]
	require.Equal(t, "lake", sps.ShapeUID)
	require.Equal(t, "Lake", sps.ShapeName)
	require.Equal(t, "water_level", sps.PropertyUID)
	require.Equal(t, "Water level", sps.PropertyName)
	require.Len(t, sps.Points, 2)
	require.Equal(t, [][]string{{"lake"}}, repo.shapeCalls) // one call, filtered to this shape
	require.Equal(t, uint64(2), covered.Count)              // shape series folds into the point count
}

func TestBuildEvidence_NoShapes_NoShapeSeries(t *testing.T) {
	repo := &fakeStepsRepo{
		min: 0, max: 5,
		props: []datasims.PropertyInfo{{UID: "energy", DataType: "number"}},
		pop:   []datasims.PopulationPoint{{Step: 0, Count: map[string]int{"prey": 3}}},
	}
	sim := &datasims.Simulation{ID: "s", Name: "R", ModelSnapshot: testModel(), Steps: 5, Status: datasims.SimulationStatusFinished}
	pack, _, err := buildEvidence(context.Background(), repo, sim, datasims.AnalysisSelection{}, AnalysisConfig{MaxSeriesPoints: 60})
	require.NoError(t, err)
	require.Empty(t, pack.ShapePropertySeries)
	require.Empty(t, repo.shapeCalls) // shape queries not even issued
}

func TestBuildEvidence_ShapeSeries_HonorsPropertyFilter(t *testing.T) {
	repo := &fakeStepsRepo{
		min: 0, max: 5,
		shapeProps:  []datasims.PropertyInfo{{UID: "water_level", DataType: "float"}},
		shapeSeries: []datasims.TimeSeriesPoint{{Step: 0, Value: 50.0}},
	}
	sim := &datasims.Simulation{ID: "s", Name: "R", ModelSnapshot: testModelWithLake(), Steps: 5, Status: datasims.SimulationStatusFinished}
	sel := datasims.AnalysisSelection{PropertyUIDs: []string{"energy"}} // excludes water_level
	pack, _, err := buildEvidence(context.Background(), repo, sim, sel, AnalysisConfig{MaxSeriesPoints: 60})
	require.NoError(t, err)
	require.Empty(t, pack.ShapePropertySeries)
}

func testModel() *datamodels.Model {
	return &datamodels.Model{
		Name:      "Predator-Prey",
		IsSpatial: true,
		Agents: []datamodels.Agent{
			{UID: "prey", Name: "Prey", Type: datamodels.AgentTypeArchetype, Properties: []datamodels.AgentProperty{{UID: "energy", Name: "Energy", Type: datamodels.PropertyTypeNumber}}},
			{UID: "predator", Name: "Predator", Type: datamodels.AgentTypeArchetype, Properties: []datamodels.AgentProperty{{UID: "energy", Name: "Energy", Type: datamodels.PropertyTypeNumber}}},
		},
		Rules: []datamodels.Rule{{ID: "r1", Name: "hunt"}},
	}
}

func TestBuildEvidence_WholeRun(t *testing.T) {
	repo := &fakeStepsRepo{
		min: 0, max: 100,
		props:  []datasims.PropertyInfo{{UID: "energy", DataType: "number"}},
		pop:    []datasims.PopulationPoint{{Step: 0, Count: map[string]int{"prey": 50, "predator": 10}}, {Step: 100, Count: map[string]int{"prey": 5, "predator": 2}}},
		series: []datasims.TimeSeriesPoint{{Step: 0, Value: 10.0}, {Step: 100, Value: 2.0}},
	}
	sim := &datasims.Simulation{
		ID: "sim1", Name: "Run A", ModelSnapshot: testModel(), Steps: 100,
		Status:     datasims.SimulationStatusFinished,
		Statistics: &datasims.SimulationStatistics{StepsCompleted: 100, TotalAgents: 7},
	}
	cfg := AnalysisConfig{MaxSeriesPoints: 60}

	pack, covered, err := buildEvidence(context.Background(), repo, sim, datasims.AnalysisSelection{}, cfg)
	require.NoError(t, err)
	require.Equal(t, uint64(0), covered.From)
	require.Equal(t, uint64(100), covered.To)
	require.Equal(t, "Predator-Prey", pack.Model.Name)
	require.Len(t, pack.Model.Archetypes, 2)
	require.NotEmpty(t, pack.Population)
	require.NotEmpty(t, pack.PropertySeries)
}

func TestBuildEvidence_AppliesSampling(t *testing.T) {
	repo := &fakeStepsRepo{
		min: 0, max: 4,
		props: []datasims.PropertyInfo{{UID: "energy", DataType: "number"}},
		pop: []datasims.PopulationPoint{
			{Step: 0, Count: map[string]int{"prey": 5}},
			{Step: 1, Count: map[string]int{"prey": 4}},
			{Step: 2, Count: map[string]int{"prey": 3}},
			{Step: 3, Count: map[string]int{"prey": 2}},
			{Step: 4, Count: map[string]int{"prey": 1}},
		},
	}
	sim := &datasims.Simulation{ID: "sim1", Name: "Run A", ModelSnapshot: testModel(), Steps: 4,
		Status: datasims.SimulationStatusFinished}
	cfg := AnalysisConfig{MaxSeriesPoints: 60}

	sampling := uint64(2)
	pack, covered, err := buildEvidence(context.Background(), repo, sim, datasims.AnalysisSelection{Sampling: &sampling}, cfg)
	require.NoError(t, err)
	// Only even steps survive the stride.
	steps := make([]int64, 0, len(pack.Population))
	for _, p := range pack.Population {
		steps = append(steps, p.Step)
	}
	require.Equal(t, []int64{0, 2, 4}, steps)
	require.Equal(t, uint64(2), covered.Sampling)
	require.Equal(t, uint64(3), covered.Count) // actual retained points
}

func TestBuildEvidence_RecordedNumericTypeFallback(t *testing.T) {
	// "vitality" is recorded as "integer" and is NOT in the model snapshot, so it
	// must still resolve as numeric and yield a property series.
	repo := &fakeStepsRepo{
		min: 0, max: 10,
		props:  []datasims.PropertyInfo{{UID: "vitality", DataType: "integer"}},
		series: []datasims.TimeSeriesPoint{{Step: 0, Value: 9.0}, {Step: 10, Value: 1.0}},
	}
	sim := &datasims.Simulation{ID: "sim1", Name: "Run A", ModelSnapshot: testModel(), Steps: 10,
		Status: datasims.SimulationStatusFinished}
	cfg := AnalysisConfig{MaxSeriesPoints: 60}

	pack, _, err := buildEvidence(context.Background(), repo, sim, datasims.AnalysisSelection{}, cfg)
	require.NoError(t, err)
	require.Len(t, pack.PropertySeries, 1)
	require.Equal(t, "vitality", pack.PropertySeries[0].PropertyUID)
}
