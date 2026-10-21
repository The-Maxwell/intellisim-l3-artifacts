package chats

import (
	"context"
	"errors"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.uber.org/zap"

	"intelli.sim.core/data/chats"
	"intelli.sim.core/data/common"
	mongocommon "intelli.sim.core/data/mongodb/common"
	"intelli.sim.core/pkg/mongodb"
)

type chatsRepository struct {
	logger                *zap.Logger
	chatCollection        *mongo.Collection
	chatMessageCollection *mongo.Collection
}

// NewChatsRepository returns the MongoDB implementation of chats.ChatsRepository.
func NewChatsRepository(logger *zap.Logger, mongodbClient *mongodb.MongoDBClient) chats.ChatsRepository {
	return &chatsRepository{
		logger:                logger.Named("chats-repository"),
		chatCollection:        mongodbClient.GetCollection("chats"),
		chatMessageCollection: mongodbClient.GetCollection("chat_messages"),
	}
}

func (r *chatsRepository) Create(ctx context.Context, chat chats.Chat) (string, error) {
	doc, err := ChatToDocument(chat)
	if err != nil {
		return "", err
	}
	res, err := r.chatCollection.InsertOne(ctx, doc)
	if err != nil {
		return "", fmt.Errorf("insert chat: %w", err)
	}
	oid, ok := res.InsertedID.(bson.ObjectID)
	if !ok {
		return "", fmt.Errorf("insert chat: unexpected InsertedID type %T", res.InsertedID)
	}
	return mongocommon.FormatID(oid), nil
}

func (r *chatsRepository) GetByID(ctx context.Context, id string) (*chats.Chat, error) {
	oid, err := mongocommon.ParseID(id)
	if err != nil {
		return nil, err
	}
	var doc ChatDocument
	if err := r.chatCollection.FindOne(ctx, bson.M{"_id": oid}).Decode(&doc); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, fmt.Errorf("chat %s: %w", id, common.ErrNotFound)
		}
		return nil, fmt.Errorf("find chat %s: %w", id, err)
	}
	chat := ChatFromDocument(doc)
	return &chat, nil
}

func (r *chatsRepository) Update(ctx context.Context, id string, chat chats.Chat) error {
	oid, err := mongocommon.ParseID(id)
	if err != nil {
		return err
	}
	doc, err := ChatToDocument(chat)
	if err != nil {
		return err
	}
	if _, err := r.chatCollection.ReplaceOne(ctx, bson.M{"_id": oid}, doc); err != nil {
		return fmt.Errorf("update chat %s: %w", id, err)
	}
	return nil
}

func (r *chatsRepository) Delete(ctx context.Context, id string) error {
	oid, err := mongocommon.ParseID(id)
	if err != nil {
		return err
	}
	if _, err := r.chatCollection.DeleteOne(ctx, bson.M{"_id": oid}); err != nil {
		return fmt.Errorf("delete chat %s: %w", id, err)
	}
	return nil
}

func (r *chatsRepository) List(ctx context.Context, filter chats.ListFilter) ([]chats.Chat, bool, error) {
	query := bson.M{}
	if filter.CreatedBy != "" {
		query["created_by"] = filter.CreatedBy
	}
	if filter.Type != "" {
		query["type"] = filter.Type
	}

	skip := int64((filter.Page - 1) * filter.PageSize)
	findOptions := options.Find().
		SetSkip(skip).
		SetLimit(int64(filter.PageSize + 1)). // +1 row → has_more
		SetSort(bson.D{{Key: "updated_at", Value: -1}})

	cursor, err := r.chatCollection.Find(ctx, query, findOptions)
	if err != nil {
		return nil, false, fmt.Errorf("list chats: %w", err)
	}
	defer cursor.Close(ctx)

	var docs []ChatDocument
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, false, fmt.Errorf("decode chats: %w", err)
	}
	hasMore := len(docs) > filter.PageSize
	if hasMore {
		docs = docs[:filter.PageSize]
	}
	return mongocommon.MapSlice(docs, ChatFromDocument), hasMore, nil
}

// ListMessagesBefore pages messages newest-first using a (created_at, _id)
// compound cursor resolved from beforeID.
func (r *chatsRepository) ListMessagesBefore(ctx context.Context, chatID string, limit int, beforeID string) ([]chats.ChatMessage, bool, error) {
	chatOID, err := mongocommon.ParseID(chatID)
	if err != nil {
		return nil, false, err
	}

	query := bson.M{"chat_id": chatOID}
	if beforeID != "" {
		beforeOID, err := mongocommon.ParseID(beforeID)
		if err != nil {
			return nil, false, err
		}
		var before ChatMessageDocument
		if err := r.chatMessageCollection.FindOne(ctx, bson.M{"_id": beforeOID}).Decode(&before); err != nil {
			if errors.Is(err, mongo.ErrNoDocuments) {
				return nil, false, fmt.Errorf("cursor message %s: %w", beforeID, common.ErrNotFound)
			}
			return nil, false, fmt.Errorf("resolve cursor %s: %w", beforeID, err)
		}
		query["$or"] = bson.A{
			bson.M{"created_at": bson.M{"$lt": before.CreatedAt}},
			bson.M{"created_at": before.CreatedAt, "_id": bson.M{"$lt": beforeOID}},
		}
	}

	findOptions := options.Find().
		SetLimit(int64(limit + 1)).
		SetSort(bson.D{{Key: "created_at", Value: -1}, {Key: "_id", Value: -1}})

	cursor, err := r.chatMessageCollection.Find(ctx, query, findOptions)
	if err != nil {
		return nil, false, fmt.Errorf("list chat %s messages: %w", chatID, err)
	}
	defer cursor.Close(ctx)

	var docs []ChatMessageDocument
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, false, fmt.Errorf("decode chat %s messages: %w", chatID, err)
	}
	hasMore := len(docs) > limit
	if hasMore {
		docs = docs[:limit]
	}
	return mongocommon.MapSlice(docs, ChatMessageFromDocument), hasMore, nil
}

func (r *chatsRepository) GetMessageByID(ctx context.Context, id string) (*chats.ChatMessage, error) {
	oid, err := mongocommon.ParseID(id)
	if err != nil {
		return nil, err
	}
	var doc ChatMessageDocument
	if err := r.chatMessageCollection.FindOne(ctx, bson.M{"_id": oid}).Decode(&doc); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, fmt.Errorf("chat message %s: %w", id, common.ErrNotFound)
		}
		return nil, fmt.Errorf("find chat message %s: %w", id, err)
	}
	msg := ChatMessageFromDocument(doc)
	return &msg, nil
}

func (r *chatsRepository) DeleteMessagesByChatID(ctx context.Context, chatID string) error {
	oid, err := mongocommon.ParseID(chatID)
	if err != nil {
		return err
	}
	if _, err := r.chatMessageCollection.DeleteMany(ctx, bson.M{"chat_id": oid}); err != nil {
		return fmt.Errorf("delete chat %s messages: %w", chatID, err)
	}
	return nil
}

func (r *chatsRepository) AddMessage(ctx context.Context, chatID string, message chats.ChatMessage) (string, error) {
	oid, err := mongocommon.ParseID(chatID)
	if err != nil {
		return "", err
	}
	doc, err := ChatMessageToDocument(message)
	if err != nil {
		return "", err
	}
	// Always associate the message with its chat so it can be retrieved later.
	// Callers (chat/model/simulation services) don't all set this, and
	// GetChatMessagesByChatID filters on chat_id, so set it here defensively.
	doc.ChatID = oid
	res, err := r.chatMessageCollection.InsertOne(ctx, doc)
	if err != nil {
		return "", fmt.Errorf("insert chat %s message: %w", chatID, err)
	}
	msgOID, ok := res.InsertedID.(bson.ObjectID)
	if !ok {
		return "", fmt.Errorf("insert chat %s message: unexpected InsertedID type %T", chatID, res.InsertedID)
	}
	return mongocommon.FormatID(msgOID), nil
}
