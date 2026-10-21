package models

import (
	"context"
	"errors"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.uber.org/zap"

	"intelli.sim.core/data/common"
	"intelli.sim.core/data/models"
	mongocommon "intelli.sim.core/data/mongodb/common"
	"intelli.sim.core/pkg/mongodb"
)

type modelsRepository struct {
	logger           *zap.Logger
	modelsCollection *mongo.Collection
}

// NewModelsRepository returns the MongoDB implementation of models.ModelsRepository.
func NewModelsRepository(logger *zap.Logger, mongodbClient *mongodb.MongoDBClient) models.ModelsRepository {
	return &modelsRepository{
		logger:           logger.Named("models-repository"),
		modelsCollection: mongodbClient.GetCollection("models"),
	}
}

func (r *modelsRepository) Create(ctx context.Context, model models.Model) (string, error) {
	doc, err := ModelToDocument(model)
	if err != nil {
		return "", err
	}
	res, err := r.modelsCollection.InsertOne(ctx, doc)
	if err != nil {
		return "", fmt.Errorf("insert model: %w", err)
	}
	oid, ok := res.InsertedID.(bson.ObjectID)
	if !ok {
		return "", fmt.Errorf("insert model: unexpected InsertedID type %T", res.InsertedID)
	}
	return mongocommon.FormatID(oid), nil
}

func (r *modelsRepository) GetByID(ctx context.Context, id string) (*models.Model, error) {
	oid, err := mongocommon.ParseID(id)
	if err != nil {
		return nil, err
	}
	var doc ModelDocument
	if err := r.modelsCollection.FindOne(ctx, bson.M{"_id": oid}).Decode(&doc); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, fmt.Errorf("model %s: %w", id, common.ErrNotFound)
		}
		return nil, fmt.Errorf("find model %s: %w", id, err)
	}
	model := ModelFromDocument(doc)
	return &model, nil
}

func (r *modelsRepository) Update(ctx context.Context, id string, model models.Model) error {
	oid, err := mongocommon.ParseID(id)
	if err != nil {
		return err
	}
	doc, err := ModelToDocument(model)
	if err != nil {
		return err
	}
	if _, err := r.modelsCollection.ReplaceOne(ctx, bson.M{"_id": oid}, doc); err != nil {
		return fmt.Errorf("update model %s: %w", id, err)
	}
	return nil
}

func (r *modelsRepository) Delete(ctx context.Context, id string) error {
	oid, err := mongocommon.ParseID(id)
	if err != nil {
		return err
	}
	if _, err := r.modelsCollection.DeleteOne(ctx, bson.M{"_id": oid}); err != nil {
		return fmt.Errorf("delete model %s: %w", id, err)
	}
	return nil
}

// modelSortFields whitelists the user-sortable list columns → stored document
// fields. Columns whose value is computed at mapping time (agent_count,
// simulation_ready) are intentionally absent: the list collection is a plain
// Find (no $size), so those stay non-sortable on the server.
var modelSortFields = map[string]string{
	"name":       "name",
	"updated_at": "updated_at",
	"created_by": "created_by",
}

// modelSort maps the filter's sort token to a deterministic bson sort. Unknown
// tokens fall back to updated_at; a non-"asc" order is descending. The _id
// tie-break keeps the +1 has-more window stable across pages.
func modelSort(filter models.ListFilter) bson.D {
	field, ok := modelSortFields[filter.SortBy]
	if !ok {
		field = "updated_at"
	}
	dir := -1
	if filter.SortOrder == "asc" {
		dir = 1
	}
	return bson.D{{Key: field, Value: dir}, {Key: "_id", Value: -1}}
}

// modelsQuery builds the match document shared by List and Count.
func modelsQuery(filter models.ListFilter) bson.M {
	query := bson.M{}
	if filter.CreatedBy != "" {
		query["created_by"] = filter.CreatedBy
	}
	return query
}

func (r *modelsRepository) List(ctx context.Context, filter models.ListFilter) ([]models.Model, bool, error) {
	page := filter.Page
	if page < 1 {
		page = 1
	}
	pageSize := filter.PageSize
	if pageSize < 1 {
		pageSize = 20
	}

	opts := options.Find().
		SetSort(modelSort(filter)).
		SetSkip(int64(page-1) * int64(pageSize)). // widen before multiply to avoid int overflow on large pages
		SetLimit(int64(pageSize + 1))             // fetch one extra to detect has-more

	cursor, err := r.modelsCollection.Find(ctx, modelsQuery(filter), opts)
	if err != nil {
		return nil, false, fmt.Errorf("list models: %w", err)
	}
	defer cursor.Close(ctx)

	var docs []ModelDocument
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, false, fmt.Errorf("decode models: %w", err)
	}

	hasMore := len(docs) > pageSize
	if hasMore {
		docs = docs[:pageSize]
	}
	out := make([]models.Model, 0, len(docs))
	for _, doc := range docs {
		out = append(out, ModelFromDocument(doc))
	}
	return out, hasMore, nil
}

func (r *modelsRepository) Count(ctx context.Context, filter models.ListFilter) (int64, error) {
	n, err := r.modelsCollection.CountDocuments(ctx, modelsQuery(filter))
	if err != nil {
		return 0, fmt.Errorf("count models: %w", err)
	}
	return n, nil
}

// ListOptions returns the id+name of every model for the owner (empty = all),
// sorted by name, with no paging — a lightweight projection for selector UIs.
func (r *modelsRepository) ListOptions(ctx context.Context, createdBy string) ([]models.ModelOption, error) {
	query := bson.M{}
	if createdBy != "" {
		query["created_by"] = createdBy
	}
	opts := options.Find().
		SetProjection(bson.D{{Key: "name", Value: 1}}).
		SetSort(bson.D{{Key: "name", Value: 1}})

	cursor, err := r.modelsCollection.Find(ctx, query, opts)
	if err != nil {
		return nil, fmt.Errorf("list model options: %w", err)
	}
	defer cursor.Close(ctx)

	var docs []struct {
		ID   bson.ObjectID `bson:"_id"`
		Name string        `bson:"name"`
	}
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("decode model options: %w", err)
	}
	out := make([]models.ModelOption, len(docs))
	for i, d := range docs {
		out[i] = models.ModelOption{ID: mongocommon.FormatID(d.ID), Name: d.Name}
	}
	return out, nil
}
