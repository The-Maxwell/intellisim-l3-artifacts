package mongodb

import (
	"context"
	"os"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

type MongoDBClient struct {
	logger *zap.Logger
	client *mongo.Client
	once   sync.Once
}

// Close terminates the connection to MongoDB. This function should be called once at the end of your application.
func (m *MongoDBClient) Close() error {
	if m.client != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		err := m.client.Disconnect(ctx)
		if err != nil {
			m.logger.Fatal("Failed to disconnect MongoDB", zap.Error(err))
			return err
		} else {
			m.logger.Info("MongoDB connection closed.")
		}
	}

	return nil
}

func (m *MongoDBClient) connectToDatabase() error {
	opt := map[string]string{}

	// TOOD: Use config env variable
	if os.Getenv("MONGO_AUTH_SOURCE") != "" {
		opt["authSource"] = os.Getenv("MONGO_AUTH_SOURCE")
	}

	if os.Getenv("MONGO_REPLICA_SET") != "" {
		opt["replicaSet"] = os.Getenv("MONGO_REPLICA_SET")
	}

	if os.Getenv("MONGO_USERNAME") != "" && os.Getenv("MONGO_PASSWORD") != "" {
		opt["username"] = os.Getenv("MONGO_USERNAME")
		opt["password"] = os.Getenv("MONGO_PASSWORD")
	}

	uri := buildURI(map[string]interface{}{
		"host":     os.Getenv("MONGO_HOST"),
		"database": os.Getenv("MONGO_DATABASE"),
		"options":  opt,
	})
	m.logger.Info("Connecting to MongoDB with URI", zap.String("uri", buildURI(map[string]interface{}{
		"host":     os.Getenv("MONGO_HOST"),
		"database": os.Getenv("MONGO_DATABASE"),
		"username": os.Getenv("MONGO_USERNAME"),
		"password": "*****",
		"options":  opt,
	})))

	clientOptions := options.Client().ApplyURI(uri)

	client, clientErr := mongo.Connect(clientOptions)
	if clientErr != nil {
		m.logger.Fatal("Failed to connect to MongoDB", zap.Error(clientErr))
		return clientErr
	}

	// Verify the connection
	pingCtx, pingCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer pingCancel()
	clientErr = client.Ping(pingCtx, nil)
	if clientErr != nil {
		m.logger.Fatal("Failed to ping MongoDB", zap.Error(clientErr))
		return clientErr
	}

	m.client = client
	m.logger.Info("Successfully connected to MongoDB")

	return nil
}

// GetDatabase returns a handle to the MongoDB database specified in the environment variable.
func (m *MongoDBClient) GetDatabase() *mongo.Database {
	m.once.Do(func() {
		if err := m.connectToDatabase(); err != nil {
			panic(err)
		}
	})

	if m.client == nil {
		m.logger.Fatal("MongoDB client is not initialized. Please call Connect first.")
		return nil
	}
	databaseName := os.Getenv("MONGO_DATABASE")
	return m.client.Database(databaseName)
}

// GetCollection returns a handle to a MongoDB collection in the specified database.
func (m *MongoDBClient) GetCollection(collectionName string) *mongo.Collection {
	db := m.GetDatabase() // Get the database from the environment variable
	return db.Collection(collectionName)
}

func NewMongoDBClient(lc fx.Lifecycle, logger *zap.Logger) *MongoDBClient {
	client := &MongoDBClient{
		logger: logger.Named("mongodb"),
		once:   sync.Once{},
	}

	// Lifecycle hooks on MongoDBClient itself
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			return nil
		},
		OnStop: func(ctx context.Context) error {
			return client.Close()
		},
	})

	return client
}
