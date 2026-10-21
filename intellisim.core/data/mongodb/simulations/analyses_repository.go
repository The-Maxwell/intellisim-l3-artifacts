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

type analysesRepository struct {
	logger     *zap.Logger
	collection *mongo.Collection
}

// NewAnalysesRepository returns the MongoDB implementation backed by the
// "simulation_analyses" collection. Index creation is idempotent + best-effort.
func NewAnalysesRepository(logger *zap.Logger, mongodbClient *mongodb.MongoDBClient) simulations.AnalysesRepository {
	r := &analysesRepository{
		logger:     logger.Named("analyses-repository"),
		collection: mongodbClient.GetCollection("simulation_analyses"),
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, err := r.collection.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "simulation_id", Value: 1}, {Key: "created_at", Value: -1}}},
		{Keys: bson.D{{Key: "created_by", Value: 1}}},
	})
	if err != nil {
		r.logger.Warn("failed to ensure simulation_analyses indexes", zap.Error(err))
	}
	return r
}

func (r *analysesRepository) Create(ctx context.Context, a simulations.Analysis) (string, error) {
	doc, err := AnalysisToDocument(a)
	if err != nil {
		return "", err
	}
	doc.ID = bson.NilObjectID
	res, err := r.collection.InsertOne(ctx, doc)
	if err != nil {
		return "", fmt.Errorf("insert analysis: %w", err)
	}
	return res.InsertedID.(bson.ObjectID).Hex(), nil
}

func (r *analysesRepository) GetByID(ctx context.Context, id string) (*simulations.Analysis, error) {
	oid, err := mongocommon.ParseID(id)
	if err != nil {
		return nil, common.ErrInvalidID
	}
	var doc AnalysisDocument
	if err := r.collection.FindOne(ctx, bson.M{"_id": oid}).Decode(&doc); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, common.ErrNotFound
		}
		return nil, fmt.Errorf("find analysis: %w", err)
	}
	a := AnalysisFromDocument(doc)
	return &a, nil
}

func (r *analysesRepository) Update(ctx context.Context, id string, a simulations.Analysis) error {
	oid, err := mongocommon.ParseID(id)
	if err != nil {
		return common.ErrInvalidID
	}
	doc, err := AnalysisToDocument(a)
	if err != nil {
		return err
	}
	doc.ID = oid
	_, err = r.collection.ReplaceOne(ctx, bson.M{"_id": oid}, doc)
	if err != nil {
		return fmt.Errorf("update analysis: %w", err)
	}
	return nil
}

func (r *analysesRepository) Delete(ctx context.Context, id string) error {
	oid, err := mongocommon.ParseID(id)
	if err != nil {
		return common.ErrInvalidID
	}
	_, err = r.collection.DeleteOne(ctx, bson.M{"_id": oid})
	if err != nil {
		return fmt.Errorf("delete analysis: %w", err)
	}
	return nil
}

func (r *analysesRepository) ListBySimulation(ctx context.Context, simulationID string) ([]simulations.Analysis, error) {
	simOID, err := mongocommon.ParseID(simulationID)
	if err != nil {
		return nil, common.ErrInvalidID
	}
	cur, err := r.collection.Find(ctx, bson.M{"simulation_id": simOID},
		options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}}))
	if err != nil {
		return nil, fmt.Errorf("list analyses: %w", err)
	}
	defer cur.Close(ctx)
	out := make([]simulations.Analysis, 0)
	for cur.Next(ctx) {
		var doc AnalysisDocument
		if err := cur.Decode(&doc); err != nil {
			return nil, fmt.Errorf("decode analysis: %w", err)
		}
		out = append(out, AnalysisFromDocument(doc))
	}
	return out, cur.Err()
}

func (r *analysesRepository) FindUnfinished(ctx context.Context) ([]simulations.Analysis, error) {
	cur, err := r.collection.Find(ctx, bson.M{"status": bson.M{"$in": []string{"pending", "running"}}})
	if err != nil {
		return nil, fmt.Errorf("find unfinished analyses: %w", err)
	}
	defer cur.Close(ctx)
	out := make([]simulations.Analysis, 0)
	for cur.Next(ctx) {
		var doc AnalysisDocument
		if err := cur.Decode(&doc); err != nil {
			return nil, fmt.Errorf("decode analysis: %w", err)
		}
		out = append(out, AnalysisFromDocument(doc))
	}
	return out, cur.Err()
}

func (r *analysesRepository) DeleteBySimulationID(ctx context.Context, simulationID string) error {
	simOID, err := mongocommon.ParseID(simulationID)
	if err != nil {
		return common.ErrInvalidID
	}
	_, err = r.collection.DeleteMany(ctx, bson.M{"simulation_id": simOID})
	if err != nil {
		return fmt.Errorf("delete analyses by simulation: %w", err)
	}
	return nil
}
