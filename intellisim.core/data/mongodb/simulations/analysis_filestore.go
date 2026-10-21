package simulations

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.uber.org/zap"

	"intelli.sim.core/data/common"
	"intelli.sim.core/data/simulations"
	"intelli.sim.core/pkg/mongodb"
)

const analysisBucketName = "analysis_files"

type analysisFileStore struct {
	logger *zap.Logger
	bucket *mongo.GridFSBucket
}

// NewAnalysisFileStore returns the GridFS-backed implementation of AnalysisFileStore.
func NewAnalysisFileStore(logger *zap.Logger, mongodbClient *mongodb.MongoDBClient) simulations.AnalysisFileStore {
	db := mongodbClient.GetDatabase()
	bucket := db.GridFSBucket(options.GridFSBucket().SetName(analysisBucketName))
	return &analysisFileStore{logger: logger.Named("analysis-filestore"), bucket: bucket}
}

func (s *analysisFileStore) Upload(ctx context.Context, name, mime, simulationID, analysisID string, data []byte) (string, int64, error) {
	meta := bson.M{
		"content_type":  mime,
		"simulation_id": simulationID,
		"analysis_id":   analysisID,
	}
	fileID, err := s.bucket.UploadFromStream(ctx, name, bytes.NewReader(data),
		options.GridFSUpload().SetMetadata(meta))
	if err != nil {
		return "", 0, fmt.Errorf("gridfs upload: %w", err)
	}
	return fileID.Hex(), int64(len(data)), nil
}

func (s *analysisFileStore) Open(ctx context.Context, gridfsID string) (io.ReadCloser, *simulations.AnalysisFileInfo, error) {
	oid, err := bson.ObjectIDFromHex(gridfsID)
	if err != nil {
		return nil, nil, common.ErrInvalidID
	}
	ds, err := s.bucket.OpenDownloadStream(ctx, oid)
	if err != nil {
		if err == mongo.ErrFileNotFound {
			return nil, nil, common.ErrNotFound
		}
		return nil, nil, fmt.Errorf("gridfs open: %w", err)
	}
	return ds, &simulations.AnalysisFileInfo{Length: ds.GetFile().Length}, nil
}

func (s *analysisFileStore) Delete(ctx context.Context, gridfsID string) error {
	oid, err := bson.ObjectIDFromHex(gridfsID)
	if err != nil {
		return common.ErrInvalidID
	}
	if err := s.bucket.Delete(ctx, oid); err != nil {
		if err == mongo.ErrFileNotFound {
			return nil
		}
		return fmt.Errorf("gridfs delete: %w", err)
	}
	return nil
}

func (s *analysisFileStore) DeleteBySimulation(ctx context.Context, simulationID string) error {
	cur, err := s.bucket.Find(ctx, bson.M{"metadata.simulation_id": simulationID})
	if err != nil {
		return fmt.Errorf("gridfs find by simulation: %w", err)
	}
	defer cur.Close(ctx)
	type fileDoc struct {
		ID bson.ObjectID `bson:"_id"`
	}
	var ids []bson.ObjectID
	for cur.Next(ctx) {
		var fd fileDoc
		if err := cur.Decode(&fd); err != nil {
			return fmt.Errorf("decode gridfs file: %w", err)
		}
		ids = append(ids, fd.ID)
	}
	if err := cur.Err(); err != nil {
		return err
	}
	for _, id := range ids {
		if err := s.bucket.Delete(ctx, id); err != nil && err != mongo.ErrFileNotFound {
			s.logger.Warn("failed to delete gridfs file", zap.String("id", id.Hex()), zap.Error(err))
		}
	}
	return nil
}
