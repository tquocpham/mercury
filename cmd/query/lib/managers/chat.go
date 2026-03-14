package managers

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type ChatsManager interface {
	GetUserChats(ctx context.Context, userID string) (_ *UserChats, err error)
	AddUserChat(ctx context.Context, userID, conversationID string) (_ *UserChats, err error)
}

type chatsManager struct {
	col *mongo.Collection
}

func NewChatsManager(mongoAddr string) (ChatsManager, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client, err := mongo.Connect(options.Client().ApplyURI(mongoAddr))
	if err != nil {
		return nil, err
	}
	col := client.Database("msgs").Collection("userchats")
	_, err = col.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "username", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
	})
	if err != nil {
		return nil, err
	}

	return &chatsManager{col: col}, nil
}

type UserChats struct{}

func (m *chatsManager) GetUserChats(ctx context.Context, userID string) (_ *UserChats, err error) {
	return nil, nil
}

func (m *chatsManager) AddUserChat(ctx context.Context, userID, conversationID string) (_ *UserChats, err error) {
	return nil, nil
}
