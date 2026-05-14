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
	GetGameservers() ([]*matchmaking.GameserverInfo, error)
	GetPendingParties() ([]*matchmaking.PartyInfo, error)
	GetServerOccupancies(ctx context.Context) (map[string]int, error)
	UpdateMMState(ctx context.Context, parties []*matchmaking.PartyInfo) error
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

// GetGameservers returns all available game servers.
func (m *matchmakingManager) GetGameservers() ([]*matchmaking.GameserverInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cursor, err := m.col.Database().Collection("gameservers").Find(ctx,
		bson.M{"status": matchmaking.Available},
	)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var gameservers []*matchmaking.GameserverInfo
	if err := cursor.All(ctx, &gameservers); err != nil {
		return nil, err
	}
	return gameservers, nil
}

// GetPendingParties returns all unassigned parties, ordered oldest first.
func (m *matchmakingManager) GetPendingParties() ([]*matchmaking.PartyInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cursor, err := m.col.Database().Collection("pending_parties").Find(ctx,
		bson.M{"status": matchmaking.PartyPending},
		options.Find().SetSort(bson.M{"register_time": 1}),
	)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var parties []*matchmaking.PartyInfo
	if err := cursor.All(ctx, &parties); err != nil {
		return nil, err
	}
	return parties, nil
}

// GetServerOccupancies returns the number of currently assigned players per server,
// derived from the pending_parties collection. Used by the solver to compute
// remaining capacity without storing player_ids on the server document.
func (m *matchmakingManager) GetServerOccupancies(ctx context.Context) (map[string]int, error) {
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{"status": matchmaking.PartyAssigned, "server_id": bson.M{"$ne": ""}}}},
		{{Key: "$project", Value: bson.M{"server_id": 1, "player_count": bson.M{"$size": "$player_ids"}}}},
		{{Key: "$group", Value: bson.M{"_id": "$server_id", "total": bson.M{"$sum": "$player_count"}}}},
	}
	cursor, err := m.col.Database().Collection("pending_parties").Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var results []struct {
		ServerID string `bson:"_id"`
		Total    int    `bson:"total"`
	}
	if err := cursor.All(ctx, &results); err != nil {
		return nil, err
	}
	occupancies := make(map[string]int, len(results))
	for _, r := range results {
		occupancies[r.ServerID] = r.Total
	}
	return occupancies, nil
}

// UpdateMMState writes party assignments to the database. Each party is updated
// independently — no transaction required since only one collection is written.
// Version check on each party prevents clobbering concurrent modifications.
// Returns ErrVersionConflict if any party was modified since it was read.
func (m *matchmakingManager) UpdateMMState(ctx context.Context, parties []*matchmaking.PartyInfo) error {
	partyCol := m.col.Database().Collection("pending_parties")
	for _, party := range parties {
		result, err := partyCol.UpdateOne(ctx,
			bson.M{"_id": party.PartyID, "version": party.Version, "status": matchmaking.PartyPending},
			bson.M{"$set": bson.M{
				"status":    matchmaking.PartyAssigned,
				"server_id": party.AssignedServerID,
				"version":   party.Version + 1,
			}},
		)
		if err != nil {
			return err
		}
		if result.MatchedCount == 0 {
			return ErrVersionConflict
		}
	}
	return nil
}
