package managers

import (
	"context"
	"errors"
	"time"

	"github.com/mercury/pkg/clients/matchmaking"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

var (
	ErrVersionConflict = errors.New("version conflict, data was modified during solve")
)

type MatchmakingManager interface {
	RegisterServer(ctx context.Context, serverID, ipAddress string, port, capacity int) error
	UpdateServerState(ctx context.Context, serverID string, version int, state matchmaking.GameserverState) error
	QueueParty(ctx context.Context, partyID string, playerIDs []string) (string, error)
	GetQueue(ctx context.Context, partyID string) (*matchmaking.PartyInfo, error)
}

type matchmakingManager struct {
	col *mongo.Collection
}

func NewAMatchmakingManager(mongoAddr string) (MatchmakingManager, error) {
	client, err := mongo.Connect(options.Client().ApplyURI(mongoAddr))
	if err != nil {
		return nil, err
	}
	// mongo.Connect creates a connection pool managed by the driver.
	// The pool is created once at startup, reused across all requests
	col := client.Database("mm").Collection("solver")
	return &matchmakingManager{col: col}, nil
}

// RegisterServer inserts a new game server into the pool with status available.
func (m *matchmakingManager) RegisterServer(
	ctx context.Context, serverID, ipAddress string, port, capacity int,
) error {
	_, err := m.col.Database().Collection("gameservers").InsertOne(ctx, &matchmaking.GameserverInfo{
		ServerID:     serverID,
		IPAddress:    ipAddress,
		Port:         port,
		Capacity:     capacity,
		RegisterTime: time.Now(),
		Status:       matchmaking.Available,
		Version:      0,
	})
	return err
}

// UpdateServerState transitions a game server to the given state.
// Version must match to prevent clobbering concurrent modifications.
func (m *matchmakingManager) UpdateServerState(
	ctx context.Context, serverID string, version int, state matchmaking.GameserverState,
) error {

	result, err := m.col.Database().Collection("gameservers").UpdateOne(ctx,
		bson.M{"_id": serverID, "version": version},
		bson.M{"$set": bson.M{"status": state, "version": version + 1}},
	)
	if err != nil {
		return err
	}
	if result.MatchedCount == 0 {
		return ErrVersionConflict
	}
	return nil
}

// QueueParty inserts a new party into the pending queue.
func (m *matchmakingManager) QueueParty(ctx context.Context, partyID string, playerIDs []string) (string, error) {
	document, err := m.col.Database().Collection("pending_parties").InsertOne(ctx, &matchmaking.PartyInfo{
		PartyID:      partyID,
		PlayerIDs:    playerIDs,
		RegisterTime: time.Now(),
		Status:       matchmaking.PartyPending,
		Version:      0,
	})
	if err != nil {
		return "", err
	}
	queueID := document.InsertedID.(string)
	return queueID, nil
}

// GetQueue returns a party by its queue ID.
func (m *matchmakingManager) GetQueue(ctx context.Context, partyID string) (*matchmaking.PartyInfo, error) {
	var party matchmaking.PartyInfo
	if err := m.col.Database().Collection("pending_parties").FindOne(ctx,
		bson.M{"_id": partyID},
	).Decode(&party); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}
	return &party, nil
}
