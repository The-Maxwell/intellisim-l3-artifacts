package simulations

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	datamodels "intelli.sim.core/data/models"
	datasims "intelli.sim.core/data/simulations"
)

// newTestAnalysisService builds a minimal analysisService for unit tests.
// The simsRepo is seeded with a ready simulation; stepsRepo controls what
// properties/series are "recorded" in the step timeseries.
func newTestAnalysisService(simsRepo datasims.SimulationsRepository, stepsRepo datasims.SimulationStepsRepository) *analysisService {
	hub := NewEventsHub()
	return &analysisService{
		repo:     simsRepo,
		analyses: &fakeAnalysesRepo{},
		files:    &fakeAnalysisFileStore{},
		steps:    stepsRepo,
		llm:      &fakeSvcLLM{},
		hub:      hub,
		cfg:      AnalysisConfig{MaxSeriesPoints: 60},
		logger:   zap.NewNop().Named("analysis-service"),
	}
}

// TestResultsSeries_InvalidAggregation verifies that a non-empty, unknown
// aggregation value (e.g. "median") returns a ValidationError with path
// "aggregation" instead of silently defaulting to avg.
func TestResultsSeries_InvalidAggregation(t *testing.T) {
	model := readyModel("m1", "alice")
	sim := readySim("s1", "alice", model)
	sim.Status = datasims.SimulationStatusFinished

	simsRepo := newFakeSimsRepo()
	simsRepo.store["s1"] = sim

	// stepsRepo reports one known property so we can reach the aggregation check.
	stepsRepo := &fakeStepsRepo{
		min:   0,
		max:   50,
		props: []datasims.PropertyInfo{{UID: "energy", DataType: "number"}},
	}

	svc := newTestAnalysisService(simsRepo, stepsRepo)
	ctx := ctxAs("alice", false)

	invalidValues := []string{"median", "average", "typo", "AVG", "SUM"}
	for _, bad := range invalidValues {
		t.Run("bad="+bad, func(t *testing.T) {
			_, err := svc.ResultsSeries(ctx, "s1", ResultsSeriesQuery{
				PropertyUID: "energy",
				Aggregation: bad,
			})
			require.Error(t, err, "expected error for aggregation %q", bad)

			var ve *ValidationError
			require.ErrorAs(t, err, &ve, "expected ValidationError for aggregation %q", bad)
			require.Len(t, ve.Result.Errors, 1)
			require.Equal(t, "aggregation", ve.Result.Errors[0].Path)
			require.Contains(t, ve.Result.Errors[0].Reason, bad)
		})
	}
}

// TestResultsSeries_ValidAggregation verifies that all five known aggregation
// values (avg, sum, min, max, count) are accepted without error.
func TestResultsSeries_ValidAggregation(t *testing.T) {
	model := readyModel("m1", "alice")
	sim := readySim("s1", "alice", model)
	sim.Status = datasims.SimulationStatusFinished

	simsRepo := newFakeSimsRepo()
	simsRepo.store["s1"] = sim

	stepsRepo := &fakeStepsRepo{
		min:   0,
		max:   50,
		props: []datasims.PropertyInfo{{UID: "energy", DataType: "number"}},
		series: []datasims.TimeSeriesPoint{
			{Step: 0, Value: 10.0},
			{Step: 50, Value: 5.0},
		},
	}

	svc := newTestAnalysisService(simsRepo, stepsRepo)
	ctx := ctxAs("alice", false)

	validAggs := []string{
		string(datasims.AggregationAvg),
		string(datasims.AggregationSum),
		string(datasims.AggregationMin),
		string(datasims.AggregationMax),
		string(datasims.AggregationCount),
	}
	for _, agg := range validAggs {
		t.Run("agg="+agg, func(t *testing.T) {
			result, err := svc.ResultsSeries(ctx, "s1", ResultsSeriesQuery{
				PropertyUID: "energy",
				Aggregation: agg,
			})
			require.NoError(t, err, "unexpected error for valid aggregation %q", agg)
			require.NotNil(t, result)
		})
	}
}

// TestResultsSeries_EmptyAggregation verifies that an empty aggregation string
// is accepted (defaults to avg internally) — only non-empty unknown values error.
func TestResultsSeries_EmptyAggregation(t *testing.T) {
	model := readyModel("m1", "alice")
	sim := readySim("s1", "alice", model)
	sim.Status = datasims.SimulationStatusFinished

	simsRepo := newFakeSimsRepo()
	simsRepo.store["s1"] = sim

	stepsRepo := &fakeStepsRepo{
		min:    0,
		max:    50,
		props:  []datasims.PropertyInfo{{UID: "energy", DataType: "number"}},
		series: []datasims.TimeSeriesPoint{{Step: 0, Value: 7.5}},
	}

	svc := newTestAnalysisService(simsRepo, stepsRepo)
	ctx := ctxAs("alice", false)

	result, err := svc.ResultsSeries(ctx, "s1", ResultsSeriesQuery{
		PropertyUID: "energy",
		Aggregation: "", // empty → default avg
	})
	require.NoError(t, err)
	require.NotNil(t, result)
}

// TestResultsSeries_PopulationPath_NoAggregation verifies the population path
// (no propertyUID) is unaffected by the aggregation param (it's ignored there).
func TestResultsSeries_PopulationPath_NoAggregation(t *testing.T) {
	model := readyModel("m1", "alice")
	sim := readySim("s1", "alice", model)
	sim.Status = datasims.SimulationStatusFinished

	simsRepo := newFakeSimsRepo()
	simsRepo.store["s1"] = sim

	stepsRepo := &fakeStepsRepo{
		min: 0,
		max: 50,
		pop: []datasims.PopulationPoint{{Step: 0, Count: map[string]int{"prey": 50}}},
	}

	svc := newTestAnalysisService(simsRepo, stepsRepo)
	ctx := ctxAs("alice", false)

	// Unknown aggregation with no propertyUID: population path → no error.
	result, err := svc.ResultsSeries(ctx, "s1", ResultsSeriesQuery{
		PropertyUID: "",
		Aggregation: "median", // ignored on the population branch
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, result.Population, 1)
}

// TestResultsSeries_InvalidEntityType verifies an unknown entity_type value
// returns a ValidationError with path "entity_type".
func TestResultsSeries_InvalidEntityType(t *testing.T) {
	model := readyModel("m1", "alice")
	sim := readySim("s1", "alice", model)
	sim.Status = datasims.SimulationStatusFinished

	simsRepo := newFakeSimsRepo()
	simsRepo.store["s1"] = sim

	stepsRepo := &fakeStepsRepo{
		min:   0,
		max:   50,
		props: []datasims.PropertyInfo{{UID: "energy", DataType: "number"}},
	}

	svc := newTestAnalysisService(simsRepo, stepsRepo)
	ctx := ctxAs("alice", false)

	_, err := svc.ResultsSeries(ctx, "s1", ResultsSeriesQuery{
		PropertyUID: "energy",
		EntityType:  "banana",
	})
	require.Error(t, err)

	var ve *ValidationError
	require.ErrorAs(t, err, &ve)
	require.Len(t, ve.Result.Errors, 1)
	require.Equal(t, "entity_type", ve.Result.Errors[0].Path)
	require.Contains(t, ve.Result.Errors[0].Reason, "banana")
}

// TestResultsSeries_ShapeRequiresProperty verifies that entity_type=shape
// without a property returns a ValidationError with path "property".
func TestResultsSeries_ShapeRequiresProperty(t *testing.T) {
	model := readyModel("m1", "alice")
	sim := readySim("s1", "alice", model)
	sim.Status = datasims.SimulationStatusFinished

	simsRepo := newFakeSimsRepo()
	simsRepo.store["s1"] = sim

	stepsRepo := &fakeStepsRepo{min: 0, max: 50}

	svc := newTestAnalysisService(simsRepo, stepsRepo)
	ctx := ctxAs("alice", false)

	_, err := svc.ResultsSeries(ctx, "s1", ResultsSeriesQuery{EntityType: "shape"})
	require.Error(t, err)

	var ve *ValidationError
	require.ErrorAs(t, err, &ve)
	require.Len(t, ve.Result.Errors, 1)
	require.Equal(t, "property", ve.Result.Errors[0].Path)
}

// TestResultsSeries_ShapeDispatch verifies that entity_type=shape routes to the
// shape aggregate query (never the agent one) and the DTO carries its points.
func TestResultsSeries_ShapeDispatch(t *testing.T) {
	model := readyModel("m1", "alice")
	sim := readySim("s1", "alice", model)
	sim.Status = datasims.SimulationStatusFinished

	simsRepo := newFakeSimsRepo()
	simsRepo.store["s1"] = sim

	stepsRepo := &fakeStepsRepo{
		min:        0,
		max:        50,
		shapeProps: []datasims.PropertyInfo{{UID: "water_level", DataType: "number"}},
		shapeSeries: []datasims.TimeSeriesPoint{
			{Step: 0, Value: 10.0},
			{Step: 50, Value: 5.0},
		},
	}

	svc := newTestAnalysisService(simsRepo, stepsRepo)
	ctx := ctxAs("alice", false)

	out, err := svc.ResultsSeries(ctx, "s1", ResultsSeriesQuery{
		PropertyUID: "water_level",
		EntityType:  "shape",
		AgentUIDs:   []string{"lake"},
	})
	require.NoError(t, err)
	require.Len(t, out.Series, 2)
	require.Equal(t, [][]string{{"lake"}}, stepsRepo.shapeCalls)
	require.Empty(t, stepsRepo.agentCalls)
}

// TestResultsSummary_TagsEntityType verifies every property entry carries an
// entity_type, and shape entries resolve name/type from the model snapshot.
func TestResultsSummary_TagsEntityType(t *testing.T) {
	model := readyModel("m1", "alice")
	model.Environment = &datamodels.Environment{
		Width: 40, Height: 30,
		Shapes: []datamodels.Shape{{
			UID: "lake", Name: "Lake", Type: datamodels.ShapeTypeCircle,
			Properties: []datamodels.AgentProperty{
				{UID: "water_level", Name: "Water level", Type: datamodels.PropertyTypeNumber},
			},
		}},
	}
	sim := readySim("s1", "alice", model)
	sim.Status = datasims.SimulationStatusFinished

	simsRepo := newFakeSimsRepo()
	simsRepo.store["s1"] = sim

	stepsRepo := &fakeStepsRepo{
		min:        0,
		max:        50,
		props:      []datasims.PropertyInfo{{UID: "energy", DataType: "number"}},
		shapeProps: []datasims.PropertyInfo{{UID: "water_level", DataType: "number"}},
	}

	svc := newTestAnalysisService(simsRepo, stepsRepo)
	ctx := ctxAs("alice", false)

	out, err := svc.ResultsSummary(ctx, "s1")
	require.NoError(t, err)

	byUID := map[string]ResultsPropertyDTO{}
	for _, p := range out.Properties {
		byUID[p.UID] = p
	}
	require.Equal(t, "agent", byUID["energy"].EntityType)
	require.Equal(t, "shape", byUID["water_level"].EntityType)
	require.Equal(t, "Water level", byUID["water_level"].Name)
	require.Equal(t, "number", byUID["water_level"].Type)
}
