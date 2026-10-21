package mongodb

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.uber.org/zap"
)

func (client *MongoDBClient) EnsureIndex(collection *mongo.Collection, indexName string, indexModel mongo.IndexModel) {
	client.logger.Info("Ensuring index ", zap.String("index", indexName))

	listCtx, listCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer listCancel()

	// Get indexes
	indexView := collection.Indexes()
	cursor, err := indexView.List(listCtx)
	if err != nil {
		client.logger.Error("Failed to get indexes", zap.Error(err))
		return
	}

	closeCtx, closeCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer closeCancel()
	defer cursor.Close(closeCtx)

	exists := false

	nextCtx, nextCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer nextCancel()

	for cursor.Next(nextCtx) {
		var index bson.M
		if err := cursor.Decode(&index); err != nil {
			client.logger.Error("Failed to decode index", zap.Error(err))
			continue
		}

		client.logger.Debug("Index", zap.Any("index", index["name"]))

		if index["name"] == indexName {
			exists = true
			break
		}
	}

	if err := cursor.Err(); err != nil {
		return
	}

	if exists {
		client.logger.Info("Index already exists", zap.String("index", indexName))
		return
	}

	createCtx, createCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer createCancel()

	if indexes, err := collection.Indexes().CreateOne(createCtx, indexModel); err != nil {
		client.logger.Error("Failed to create index", zap.String("index", indexName), zap.Error(err))
	} else {
		client.logger.Info("Created index", zap.String("index", indexes))
	}
}

// EnsureIndexes creates all required indexes for IntelliSim collections.
// It is idempotent and safe to call on every startup.
func (client *MongoDBClient) EnsureIndexes() {
	// chats collection
	chats := client.GetCollection("chats")
	client.EnsureIndex(chats, "chats_created_by_updated_at", mongo.IndexModel{
		Keys:    bson.D{{Key: "created_by", Value: 1}, {Key: "updated_at", Value: -1}},
		Options: options.Index().SetName("chats_created_by_updated_at"),
	})
	client.EnsureIndex(chats, "chats_created_at", mongo.IndexModel{
		Keys:    bson.D{{Key: "created_at", Value: -1}},
		Options: options.Index().SetName("chats_created_at"),
	})

	// chat_messages collection — supports the (chat_id, created_at, _id) cursor.
	chatMessages := client.GetCollection("chat_messages")
	client.EnsureIndex(chatMessages, "chat_messages_chat_id_created_at_id", mongo.IndexModel{
		Keys:    bson.D{{Key: "chat_id", Value: 1}, {Key: "created_at", Value: -1}, {Key: "_id", Value: -1}},
		Options: options.Index().SetName("chat_messages_chat_id_created_at_id"),
	})

	// models collection
	models := client.GetCollection("models")
	client.EnsureIndex(models, "models_created_at", mongo.IndexModel{
		Keys:    bson.D{{Key: "created_at", Value: -1}},
		Options: options.Index().SetName("models_created_at"),
	})
	// owner-scoped, recency-sorted listing (List with CreatedBy filter)
	client.EnsureIndex(models, "models_created_by_updated_at", mongo.IndexModel{
		Keys:    bson.D{{Key: "created_by", Value: 1}, {Key: "updated_at", Value: -1}},
		Options: options.Index().SetName("models_created_by_updated_at"),
	})

	// simulations collection
	simulations := client.GetCollection("simulations")
	client.EnsureIndex(simulations, "simulations_model_id", mongo.IndexModel{
		Keys:    bson.D{{Key: "model_id", Value: 1}},
		Options: options.Index().SetName("simulations_model_id"),
	})
	client.EnsureIndex(simulations, "simulations_chat_id", mongo.IndexModel{
		Keys:    bson.D{{Key: "chat_id", Value: 1}},
		Options: options.Index().SetName("simulations_chat_id"),
	})
	client.EnsureIndex(simulations, "simulations_status", mongo.IndexModel{
		Keys:    bson.D{{Key: "status", Value: 1}},
		Options: options.Index().SetName("simulations_status"),
	})
	client.EnsureIndex(simulations, "simulations_created_at", mongo.IndexModel{
		Keys:    bson.D{{Key: "created_at", Value: -1}},
		Options: options.Index().SetName("simulations_created_at"),
	})
}
