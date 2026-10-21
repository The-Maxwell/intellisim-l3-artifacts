package simulations

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.uber.org/zap"

	"intelli.sim.core/data/common"
	mongocommon "intelli.sim.core/data/mongodb/common"
	"intelli.sim.core/data/simulations"
	"intelli.sim.core/pkg/mongodb"
)

type simulationsRepository struct {
	logger                *zap.Logger
	simulationsCollection *mongo.Collection
}

// NewSimulationsRepository returns the MongoDB implementation of
// simulations.SimulationsRepository.
func NewSimulationsRepository(logger *zap.Logger, mongodbClient *mongodb.MongoDBClient) simulations.SimulationsRepository {
	return &simulationsRepository{
		logger:                logger.Named("simulations-repository"),
		simulationsCollection: mongodbClient.GetCollection("simulations"),
	}
}

func (r *simulationsRepository) Create(ctx context.Context, simulation simulations.Simulation) (string, error) {
	doc, err := SimulationToDocument(simulation)
	if err != nil {
		return "", err
	}
	res, err := r.simulationsCollection.InsertOne(ctx, doc)
	if err != nil {
		return "", fmt.Errorf("insert simulation: %w", err)
	}
	oid, ok := res.InsertedID.(bson.ObjectID)
	if !ok {
		return "", fmt.Errorf("insert simulation: unexpected InsertedID type %T", res.InsertedID)
	}
	return mongocommon.FormatID(oid), nil
}

func (r *simulationsRepository) GetByID(ctx context.Context, id string) (*simulations.Simulation, error) {
	oid, err := mongocommon.ParseID(id)
	if err != nil {
		return nil, err
	}
	var doc SimulationDocument
	if err := r.simulationsCollection.FindOne(ctx, bson.M{"_id": oid}).Decode(&doc); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, fmt.Errorf("simulation %s: %w", id, common.ErrNotFound)
		}
		return nil, fmt.Errorf("find simulation %s: %w", id, err)
	}
	sim := SimulationFromDocument(doc)
	return &sim, nil
}

func (r *simulationsRepository) Update(ctx context.Context, id string, simulation simulations.Simulation) error {
	oid, err := mongocommon.ParseID(id)
	if err != nil {
		return err
	}
	doc, err := SimulationToDocument(simulation)
	if err != nil {
		return err
	}
	if _, err := r.simulationsCollection.ReplaceOne(ctx, bson.M{"_id": oid}, doc); err != nil {
		return fmt.Errorf("update simulation %s: %w", id, err)
	}
	return nil
}

func (r *simulationsRepository) Delete(ctx context.Context, id string) error {
	oid, err := mongocommon.ParseID(id)
	if err != nil {
		return err
	}
	if _, err := r.simulationsCollection.DeleteOne(ctx, bson.M{"_id": oid}); err != nil {
		return fmt.Errorf("delete simulation %s: %w", id, err)
	}
	return nil
}

const (
	defaultPageSize = 20
	maxPageSize     = 100
)

// List returns a page of SimulationSummary records (updated_at desc) and whether
// more pages exist. It uses a MongoDB aggregation pipeline to compute agent_count
// and surface model_snapshot.name without loading the full agent slice, rules,
// archetypes, or images.
//
// Pagination: page is clamped to >=1; pageSize is clamped to [1, 100], default 20.
func (r *simulationsRepository) List(ctx context.Context, f simulations.ListFilter) ([]simulations.SimulationSummary, bool, error) {
	page := f.Page
	if page < 1 {
		page = 1
	}
	pageSize := f.PageSize
	switch {
	case pageSize < 1:
		pageSize = defaultPageSize
	case pageSize > maxPageSize:
		pageSize = maxPageSize
	}

	// ── match + pipeline ──────────────────────────────────────────────────────
	match, err := listMatch(f)
	if err != nil {
		return nil, false, err
	}

	// $addFields computes agent_count before $sort so it is sortable; $project
	// then avoids transmitting model_snapshot internals (rules, images, full
	// agent definitions) across the wire — only name is needed.
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: match}},
		{{Key: "$addFields", Value: bson.D{
			{Key: "agent_count", Value: bson.D{{Key: "$size", Value: bson.D{{Key: "$ifNull", Value: bson.A{"$agents", bson.A{}}}}}}},
		}}},
		{{Key: "$sort", Value: simSort(f)}},
		{{Key: "$skip", Value: int64(page-1) * int64(pageSize)}},
		{{Key: "$limit", Value: int64(pageSize + 1)}}, // +1 → has_more detection
		{{Key: "$project", Value: bson.D{
			{Key: "_id", Value: 1},
			{Key: "name", Value: 1},
			{Key: "description", Value: 1},
			{Key: "model_id", Value: 1},
			{Key: "model_snapshot_name", Value: "$model_snapshot.name"},
			{Key: "status", Value: 1},
			{Key: "steps", Value: 1},
			{Key: "agent_count", Value: 1},
			{Key: "total_tokens", Value: "$token_usage.total.total_tokens"},
			{Key: "created_at", Value: 1},
			{Key: "updated_at", Value: 1},
			{Key: "created_by", Value: 1},
		}}},
	}

	cursor, err := r.simulationsCollection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, false, fmt.Errorf("list simulations: %w", err)
	}
	defer cursor.Close(ctx)

	// Lightweight projected row — only fields surfaced by the $project above.
	type row struct {
		ID                bson.ObjectID                `bson:"_id"`
		Name              string                       `bson:"name"`
		Description       string                       `bson:"description"`
		ModelID           bson.ObjectID                `bson:"model_id"`
		ModelSnapshotName string                       `bson:"model_snapshot_name"`
		Status            simulations.SimulationStatus `bson:"status"`
		Steps             uint64                       `bson:"steps"`
		AgentCount        int                          `bson:"agent_count"`
		TotalTokens       uint64                       `bson:"total_tokens"`
		CreatedAt         time.Time                    `bson:"created_at"`
		UpdatedAt         time.Time                    `bson:"updated_at"`
		CreatedBy         string                       `bson:"created_by"`
	}

	var rows []row
	if err := cursor.All(ctx, &rows); err != nil {
		return nil, false, fmt.Errorf("decode simulation list: %w", err)
	}

	hasMore := len(rows) > pageSize
	if hasMore {
		rows = rows[:pageSize]
	}

	summaries := make([]simulations.SimulationSummary, len(rows))
	for i, row := range rows {
		summaries[i] = simulations.SimulationSummary{
			ID:          mongocommon.FormatID(row.ID),
			Name:        row.Name,
			Description: row.Description,
			ModelID:     mongocommon.FormatID(row.ModelID),
			ModelName:   row.ModelSnapshotName,
			Status:      row.Status,
			Steps:       row.Steps,
			AgentCount:  row.AgentCount,
			TotalTokens: row.TotalTokens,
			CreatedAt:   row.CreatedAt,
			UpdatedAt:   row.UpdatedAt,
			CreatedBy:   row.CreatedBy,
		}
	}
	return summaries, hasMore, nil
}

// Count returns the total number of simulations matching the filter (owner +
// model), ignoring paging and sort.
func (r *simulationsRepository) Count(ctx context.Context, f simulations.ListFilter) (int64, error) {
	match, err := listMatch(f)
	if err != nil {
		return 0, err
	}
	n, err := r.simulationsCollection.CountDocuments(ctx, match)
	if err != nil {
		return 0, fmt.Errorf("count simulations: %w", err)
	}
	return n, nil
}

// listMatch builds the $match document shared by List and Count: an optional
// owner filter plus an optional model filter (exact match for one id, $in for
// several). Empty ids are skipped.
func listMatch(f simulations.ListFilter) (bson.D, error) {
	match := bson.D{}
	if f.CreatedBy != "" {
		match = append(match, bson.E{Key: "created_by", Value: f.CreatedBy})
	}
	if len(f.ModelIDs) > 0 {
		oids := make([]bson.ObjectID, 0, len(f.ModelIDs))
		for _, id := range f.ModelIDs {
			if id == "" {
				continue
			}
			oid, err := mongocommon.ParseID(id)
			if err != nil {
				return nil, err
			}
			oids = append(oids, oid)
		}
		switch len(oids) {
		case 0:
			// all ids were empty → no model constraint
		case 1:
			match = append(match, bson.E{Key: "model_id", Value: oids[0]})
		default:
			match = append(match, bson.E{Key: "model_id", Value: bson.D{{Key: "$in", Value: oids}}})
		}
	}
	switch len(f.Statuses) {
	case 0:
		// no status constraint
	case 1:
		match = append(match, bson.E{Key: "status", Value: string(f.Statuses[0])})
	default:
		vals := make(bson.A, 0, len(f.Statuses))
		for _, s := range f.Statuses {
			vals = append(vals, string(s))
		}
		match = append(match, bson.E{Key: "status", Value: bson.D{{Key: "$in", Value: vals}}})
	}
	return match, nil
}

// simSortFields whitelists user-sortable list columns → the document path used
// in $sort. agent_count is materialized by an $addFields stage before the sort.
var simSortFields = map[string]string{
	"name":         "name",
	"model_name":   "model_snapshot.name",
	"status":       "status",
	"steps":        "steps",
	"agent_count":  "agent_count",
	"total_tokens": "token_usage.total.total_tokens",
	"updated_at":   "updated_at",
	"created_by":   "created_by",
}

// simSort maps the filter's sort token to a deterministic bson sort. Unknown
// tokens fall back to updated_at; a non-"asc" order is descending. The _id
// tie-break keeps the +1 has-more window stable across pages.
func simSort(f simulations.ListFilter) bson.D {
	field, ok := simSortFields[f.SortBy]
	if !ok {
		field = "updated_at"
	}
	dir := -1
	if f.SortOrder == "asc" {
		dir = 1
	}
	return bson.D{{Key: field, Value: dir}, {Key: "_id", Value: -1}}
}
