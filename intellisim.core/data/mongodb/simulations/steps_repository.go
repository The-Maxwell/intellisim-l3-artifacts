package simulations

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.uber.org/zap"

	"intelli.sim.core/data/common"
	mongocommon "intelli.sim.core/data/mongodb/common"
	"intelli.sim.core/data/simulations"
	"intelli.sim.core/pkg/mongodb"
)

type simulationStepsRepository struct {
	logger     *zap.Logger
	collection *mongo.Collection
}

// NewSimulationStepsRepository returns the MongoDB implementation of
// simulations.SimulationStepsRepository, backed by the "simulation_steps"
// timeseries collection. Index creation is idempotent and best-effort.
func NewSimulationStepsRepository(logger *zap.Logger, mongodbClient *mongodb.MongoDBClient) simulations.SimulationStepsRepository {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	r := &simulationStepsRepository{
		logger:     logger.Named("simulation-steps-repository"),
		collection: mongodbClient.GetTimeSeriesCollection(ctx, "simulation_steps"),
	}
	r.ensureIndexes(ctx)
	return r
}

func (r *simulationStepsRepository) ensureIndexes(ctx context.Context) {
	indexes := []mongo.IndexModel{
		{Keys: bson.D{{Key: "metadata.simulation_id", Value: 1}, {Key: "metadata.step", Value: 1}}},
		{Keys: bson.D{{Key: "metadata.simulation_id", Value: 1}, {Key: "metadata.type", Value: 1}, {Key: "metadata.step", Value: 1}}},
		{Keys: bson.D{{Key: "timestamp", Value: 1}}},
	}
	if _, err := r.collection.Indexes().CreateMany(ctx, indexes); err != nil {
		r.logger.Warn("failed to ensure simulation_steps indexes", zap.Error(err))
	}
}

// ---- persistence ----

func (r *simulationStepsRepository) SaveSteps(ctx context.Context, steps []simulations.SimulationStep) error {
	if len(steps) == 0 {
		return nil
	}
	docs := make([]SimulationStepDocument, len(steps))
	for i, step := range steps {
		doc, err := StepToDocument(step)
		if err != nil {
			return err
		}
		docs[i] = doc
	}
	opts := options.InsertMany().SetOrdered(false)
	if _, err := r.collection.InsertMany(ctx, docs, opts); err != nil {
		r.logger.Error("failed to insert step batch", zap.Int("count", len(steps)), zap.Error(err))
		return fmt.Errorf("insert %d steps: %w", len(steps), err)
	}
	return nil
}

func (r *simulationStepsRepository) DeleteBySimulation(ctx context.Context, simulationID string) error {
	oid, err := mongocommon.ParseID(simulationID)
	if err != nil {
		return err
	}
	if _, err := r.collection.DeleteMany(ctx, bson.M{"metadata.simulation_id": oid}); err != nil {
		return fmt.Errorf("delete steps by simulation: %w", err)
	}
	return nil
}

// ---- finders ----

func (r *simulationStepsRepository) GetStepRange(ctx context.Context, simulationID string) (int64, int64, error) {
	simOID, err := mongocommon.ParseID(simulationID)
	if err != nil {
		return 0, 0, err
	}
	filter := bson.M{"metadata.simulation_id": simOID}

	optsAsc := options.FindOne().SetSort(bson.D{{Key: "metadata.step", Value: 1}})
	var minDoc SimulationStepDocument
	if err := r.collection.FindOne(ctx, filter, optsAsc).Decode(&minDoc); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return 0, 0, fmt.Errorf("step range of simulation %s: %w", simulationID, common.ErrNotFound)
		}
		return 0, 0, fmt.Errorf("find min step: %w", err)
	}

	optsDesc := options.FindOne().SetSort(bson.D{{Key: "metadata.step", Value: -1}})
	var maxDoc SimulationStepDocument
	if err := r.collection.FindOne(ctx, filter, optsDesc).Decode(&maxDoc); err != nil {
		return 0, 0, fmt.Errorf("find max step: %w", err)
	}
	return minDoc.Metadata.Step, maxDoc.Metadata.Step, nil
}

// GetSteps returns persisted steps for a simulation in [from, to] (inclusive),
// ordered by metadata.step asc, paginated. page is 1-based; pageSize bounds the
// number of returned rows. It fetches pageSize+1 rows to determine hasMore without
// a separate count, and trims the extra row before returning.
func (r *simulationStepsRepository) GetSteps(ctx context.Context, simID string, from, to, page, pageSize uint64) ([]simulations.SimulationStep, bool, error) {
	simOID, err := mongocommon.ParseID(simID)
	if err != nil {
		return nil, false, err
	}
	if page == 0 {
		page = 1
	}
	if pageSize == 0 {
		pageSize = 50
	}

	// Paginate by LOGICAL STEP number, not by raw document. A single step holds one
	// document per agent (+ optional environment row), so limiting raw documents
	// would slice a step in half and make page_size mean "≈ docs/agents steps". The
	// page therefore covers a window of step numbers [windowStart, windowEnd] and
	// returns EVERY document in that window so each step is complete.
	windowStart := from + (page-1)*pageSize
	if windowStart > to {
		return nil, false, nil
	}
	windowEnd := windowStart + pageSize - 1
	if windowEnd > to {
		windowEnd = to
	}

	filter := bson.D{
		{Key: "metadata.simulation_id", Value: simOID},
		{Key: "metadata.step", Value: bson.D{
			{Key: "$gte", Value: windowStart},
			{Key: "$lte", Value: windowEnd},
		}},
	}
	opts := options.Find().
		SetSort(bson.D{{Key: "metadata.step", Value: 1}, {Key: "metadata.reference_id", Value: 1}})

	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, false, fmt.Errorf("find steps page: %w", err)
	}
	defer cursor.Close(ctx)

	var docs []SimulationStepDocument
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, false, fmt.Errorf("decode steps page: %w", err)
	}

	// hasMore reflects whether any step exists beyond this window (not raw docs).
	hasMore := false
	if windowEnd < to {
		moreFilter := bson.D{
			{Key: "metadata.simulation_id", Value: simOID},
			{Key: "metadata.step", Value: bson.D{
				{Key: "$gt", Value: windowEnd},
				{Key: "$lte", Value: to},
			}},
		}
		n, err := r.collection.CountDocuments(ctx, moreFilter, options.Count().SetLimit(1))
		if err != nil {
			return nil, false, fmt.Errorf("count remaining steps: %w", err)
		}
		hasMore = n > 0
	}
	return mongocommon.MapSlice(docs, StepFromDocument), hasMore, nil
}

// ---- analysis queries ----

// stepRangeFilter appends the optional [start..end] bound on metadata.step.
func stepRangeFilter(match bson.D, stepRange *simulations.StepRange) bson.D {
	if stepRange == nil {
		return match
	}
	bounds := bson.D{}
	if stepRange.Start != nil {
		bounds = append(bounds, bson.E{Key: "$gte", Value: *stepRange.Start})
	}
	if stepRange.End != nil {
		bounds = append(bounds, bson.E{Key: "$lte", Value: *stepRange.End})
	}
	if len(bounds) > 0 {
		match = append(match, bson.E{Key: "metadata.step", Value: bounds})
	}
	return match
}

func (r *simulationStepsRepository) AggregatePropertyByStep(ctx context.Context, simulationID, propertyUID string, agentUIDs []string, stepRange *simulations.StepRange, aggregation simulations.Aggregation) ([]simulations.TimeSeriesPoint, error) {
	return r.aggregatePropertyByStep(ctx, simulationID, propertyUID, agentUIDs, stepRange, aggregation, simulations.MetadataTypeAgent)
}

func (r *simulationStepsRepository) AggregateShapePropertyByStep(ctx context.Context, simulationID, propertyUID string, shapeUIDs []string, stepRange *simulations.StepRange, aggregation simulations.Aggregation) ([]simulations.TimeSeriesPoint, error) {
	return r.aggregatePropertyByStep(ctx, simulationID, propertyUID, shapeUIDs, stepRange, aggregation, simulations.MetadataTypeShape)
}

func (r *simulationStepsRepository) aggregatePropertyByStep(
	ctx context.Context,
	simulationID, propertyUID string,
	uids []string,
	stepRange *simulations.StepRange,
	aggregation simulations.Aggregation,
	entity simulations.MetadataType,
) ([]simulations.TimeSeriesPoint, error) {
	simOID, err := mongocommon.ParseID(simulationID)
	if err != nil {
		return nil, err
	}
	propField := fmt.Sprintf("$properties.%s", propertyUID)

	match := bson.D{
		{Key: "metadata.simulation_id", Value: simOID},
		{Key: "metadata.type", Value: entity},
	}
	match = stepRangeFilter(match, stepRange)
	// Filter by definition UIDs of the given entity type (empty = all).
	if len(uids) > 0 {
		match = append(match, bson.E{Key: "metadata.uid", Value: bson.D{{Key: "$in", Value: uids}}})
	}

	var accumulator bson.D
	switch aggregation {
	case simulations.AggregationSum:
		accumulator = bson.D{{Key: "$sum", Value: propField}}
	case simulations.AggregationMin:
		accumulator = bson.D{{Key: "$min", Value: propField}}
	case simulations.AggregationMax:
		accumulator = bson.D{{Key: "$max", Value: propField}}
	case simulations.AggregationCount:
		accumulator = bson.D{{Key: "$sum", Value: 1}}
	default: // simulations.AggregationAvg
		accumulator = bson.D{{Key: "$avg", Value: propField}}
	}

	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: match}},
		{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: "$metadata.step"},
			{Key: "value", Value: accumulator},
		}}},
		{{Key: "$sort", Value: bson.D{{Key: "_id", Value: 1}}}},
	}

	cursor, err := r.collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("aggregate property by step: %w", err)
	}
	defer cursor.Close(ctx)

	var raw []struct {
		ID    int64       `bson:"_id"`
		Value interface{} `bson:"value"`
	}
	if err := cursor.All(ctx, &raw); err != nil {
		return nil, fmt.Errorf("decode aggregation: %w", err)
	}

	points := make([]simulations.TimeSeriesPoint, len(raw))
	for i, item := range raw {
		points[i] = simulations.TimeSeriesPoint{Step: item.ID, Value: mongocommon.NormalizeValue(item.Value)}
	}
	return points, nil
}

func (r *simulationStepsRepository) CountByAgentTypePerStep(
	ctx context.Context,
	simulationID string,
	stepRange *simulations.StepRange,
) ([]simulations.PopulationPoint, error) {
	simOID, err := mongocommon.ParseID(simulationID)
	if err != nil {
		return nil, err
	}
	match := bson.D{
		{Key: "metadata.simulation_id", Value: simOID},
		{Key: "metadata.type", Value: simulations.MetadataTypeAgent},
	}
	match = stepRangeFilter(match, stepRange)

	// First grouping: count occurrences of each (step, definition UID) pair.
	// Second grouping: accumulate per-UID counts into one entry list per step.
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: match}},
		{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: bson.D{
				{Key: "step", Value: "$metadata.step"},
				{Key: "uid", Value: "$metadata.uid"},
			}},
			{Key: "count", Value: bson.D{{Key: "$sum", Value: 1}}},
		}}},
		{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: "$_id.step"},
			{Key: "entries", Value: bson.D{{Key: "$push", Value: bson.D{
				{Key: "k", Value: "$_id.uid"},
				{Key: "v", Value: "$count"},
			}}}},
		}}},
		{{Key: "$sort", Value: bson.D{{Key: "_id", Value: 1}}}},
	}

	cursor, err := r.collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("count by agent type: %w", err)
	}
	defer cursor.Close(ctx)

	var raw []struct {
		ID      int64 `bson:"_id"`
		Entries []struct {
			K string `bson:"k"`
			V int    `bson:"v"`
		} `bson:"entries"`
	}
	if err := cursor.All(ctx, &raw); err != nil {
		return nil, fmt.Errorf("decode counts: %w", err)
	}

	points := make([]simulations.PopulationPoint, len(raw))
	for i, item := range raw {
		counts := make(map[string]int, len(item.Entries))
		for _, e := range item.Entries {
			counts[e.K] = e.V
		}
		points[i] = simulations.PopulationPoint{Step: item.ID, Count: counts}
	}
	return points, nil
}

func (r *simulationStepsRepository) FindSpatialAtStep(
	ctx context.Context,
	simulationID string,
	step int64,
	propertyUIDs []string,
) ([]simulations.SpatialAgentData, error) {
	simOID, err := mongocommon.ParseID(simulationID)
	if err != nil {
		return nil, err
	}
	filter := bson.D{
		{Key: "metadata.simulation_id", Value: simOID},
		{Key: "metadata.step", Value: step},
		{Key: "metadata.type", Value: simulations.MetadataTypeAgent},
	}

	var findOpts []options.Lister[options.FindOptions]
	if len(propertyUIDs) > 0 {
		proj := bson.D{
			{Key: "metadata", Value: 1},
			{Key: "position", Value: 1},
		}
		for _, p := range propertyUIDs {
			proj = append(proj, bson.E{Key: fmt.Sprintf("properties.%s", p), Value: 1})
		}
		findOpts = append(findOpts, options.Find().SetProjection(proj))
	}

	cursor, err := r.collection.Find(ctx, filter, findOpts...)
	if err != nil {
		return nil, fmt.Errorf("find spatial state: %w", err)
	}
	defer cursor.Close(ctx)

	var docs []SimulationStepDocument
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("decode spatial state: %w", err)
	}

	result := make([]simulations.SpatialAgentData, 0, len(docs))
	for _, doc := range docs {
		sd := simulations.SpatialAgentData{
			ReferenceID: doc.Metadata.ReferenceID,
			UID:         doc.Metadata.UID,
			Type:        doc.Metadata.Type,
			Position:    mongocommon.PointPtrFromDocument(doc.Position),
			Properties:  mongocommon.NormalizeMap(doc.Properties),
		}
		if stateVal, ok := sd.Properties["state"]; ok {
			if s, ok := stateVal.(string); ok {
				sd.State = s
			}
		}
		result = append(result, sd)
	}
	return result, nil
}

func (r *simulationStepsRepository) FindDistinctProperties(ctx context.Context, simulationID string) ([]simulations.PropertyInfo, error) {
	return r.findDistinctProperties(ctx, simulationID, simulations.MetadataTypeAgent)
}

func (r *simulationStepsRepository) FindDistinctShapeProperties(ctx context.Context, simulationID string) ([]simulations.PropertyInfo, error) {
	return r.findDistinctProperties(ctx, simulationID, simulations.MetadataTypeShape)
}

func (r *simulationStepsRepository) findDistinctProperties(
	ctx context.Context,
	simulationID string,
	entity simulations.MetadataType,
) ([]simulations.PropertyInfo, error) {
	simOID, err := mongocommon.ParseID(simulationID)
	if err != nil {
		return nil, err
	}
	// Sample up to 100 documents; collecting more does not meaningfully improve
	// coverage but keeps the operation cheap.
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.D{
			{Key: "metadata.simulation_id", Value: simOID},
			{Key: "metadata.type", Value: entity},
		}}},
		{{Key: "$limit", Value: 100}},
		{{Key: "$project", Value: bson.D{{Key: "properties", Value: 1}}}},
	}

	cursor, err := r.collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("sample properties: %w", err)
	}
	defer cursor.Close(ctx)

	var docs []struct {
		Properties map[string]interface{} `bson:"properties"`
	}
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("decode property sample: %w", err)
	}

	seen := make(map[string]string)
	for _, doc := range docs {
		for k, v := range doc.Properties {
			if _, already := seen[k]; already {
				continue
			}
			seen[k] = inferDataType(v)
		}
	}

	infos := make([]simulations.PropertyInfo, 0, len(seen))
	for uid, dt := range seen {
		infos = append(infos, simulations.PropertyInfo{UID: uid, DataType: dt})
	}
	return infos, nil
}

// ---- helpers ----

// inferDataType returns a simple type label for a property value.
func inferDataType(v interface{}) string {
	if v == nil {
		return "unknown"
	}
	switch v.(type) {
	case bool:
		return "boolean"
	case int, int32, int64:
		return "integer"
	case float32, float64:
		return "float"
	case string:
		return "string"
	default:
		return "object"
	}
}
