package mongodb

import (
	"context"
	"sync"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.uber.org/zap"
)

var collectionCreationMutex sync.Mutex

// GetTimeSeriesCollection returns a handle to a MongoDB time-series collection.
// No retention policy is applied or managed at the DB level.
func (client *MongoDBClient) GetTimeSeriesCollection(
	ctx context.Context,
	timeSeriesCollectionName string,
) *mongo.Collection {

	collectionCreationMutex.Lock()
	defer collectionCreationMutex.Unlock()

	sugar := client.logger.Sugar()
	db := client.GetDatabase()

	sugar.Infof("Ensuring time-series collection: %s", timeSeriesCollectionName)

	collectionsSpecs, err := db.ListCollectionSpecifications(
		ctx,
		bson.M{"name": timeSeriesCollectionName},
	)
	if err != nil {
		sugar.Fatal("Failed to list collection specifications", zap.Error(err))
	}

	collectionExists := false

	for _, spec := range collectionsSpecs {
		if spec.Name != timeSeriesCollectionName {
			continue
		}

		if spec.Type == "timeseries" {
			sugar.Infof("%s already exists and is a time-series collection", timeSeriesCollectionName)
			collectionExists = true
			break
		}

		// Exists but NOT time-series → drop and recreate
		sugar.Warnf(
			"%s exists but is not a time-series collection, dropping it",
			timeSeriesCollectionName,
		)

		if err := db.Collection(timeSeriesCollectionName).Drop(ctx); err != nil {
			sugar.Fatal(
				"Failed to drop non-time-series collection",
				zap.String("collection", timeSeriesCollectionName),
				zap.Error(err),
			)
		}

		sugar.Infof("%s dropped successfully", timeSeriesCollectionName)
		break
	}

	if !collectionExists {
		createOptions := options.CreateCollection().SetTimeSeriesOptions(
			options.TimeSeries().
				SetTimeField("timestamp").
				SetMetaField("metadata").
				SetGranularity("seconds"),
		)

		sugar.Infof("Creating time-series collection: %s", timeSeriesCollectionName)

		if err := db.CreateCollection(ctx, timeSeriesCollectionName, createOptions); err != nil {
			sugar.Fatal(
				"Failed to create time-series collection",
				zap.String("collection", timeSeriesCollectionName),
				zap.Error(err),
			)
		}

		sugar.Infof("%s created successfully", timeSeriesCollectionName)
	}

	return db.Collection(timeSeriesCollectionName)
}
