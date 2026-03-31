package managers

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

var (
	ErrVersionConflict = errors.New("version conflict, data was modified during solve")
)

// GameserverState determines the state of the game server
type GameserverState string

const (
	Available GameserverState = "available"
	Draining  GameserverState = "draining"
	Drained   GameserverState = "drained"
	Offline   GameserverState = "offline"
	Deleted   GameserverState = "deleted"
)

// PartyStatus determines the matchmaking status of a party
type PartyStatus string

const (
	PartyPending   PartyStatus = "pending"
	PartyAssigned  PartyStatus = "assigned"
	PartyCancelled PartyStatus = "cancelled"
)

type GameserverInfo struct {
	ServerID     string          `bson:"_id"`
	Port         int             `bson:"port"`
	Capacity     int             `bson:"capacity"`
	IPAddress    string          `bson:"ip_address"`
	RegisterTime time.Time       `bson:"register_time"`
	Status       GameserverState `bson:"status"`
	Version      int             `bson:"version"`
}

type PartyInfo struct {
	PartyID          string      `bson:"_id"`
	PlayerIDs        []string    `bson:"player_ids"`
	AssignedServerID string      `bson:"server_id"`
	RegisterTime     time.Time   `bson:"register_time"`
	Status           PartyStatus `bson:"status"`
	Version          int         `bson:"version"`
}

type MatchmakingManager interface {
	RegisterServer(ctx context.Context, serverID, ipAddress string, port, capacity int) error
	UpdateServerState(ctx context.Context, serverID string, version int, state GameserverState) error
	QueueParty(ctx context.Context, partyID string, playerIDs []string) (string, error)
	GetQueue(ctx context.Context, partyID string) (*PartyInfo, error)
	UpdatePartyState(ctx context.Context, partyID string, version int, state PartyStatus) error
	GetGameservers() ([]*GameserverInfo, error)
	GetPendingParties() ([]*PartyInfo, error)
	GetServerOccupancies(ctx context.Context) (map[string]int, error)
	UpdateMMState(ctx context.Context, parties []*PartyInfo) error
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
func (m *matchmakingManager) RegisterServer(ctx context.Context, serverID, ipAddress string, port, capacity int) error {
	_, err := m.col.Database().Collection("gameservers").InsertOne(ctx, &GameserverInfo{
		ServerID:     serverID,
		IPAddress:    ipAddress,
		Port:         port,
		Capacity:     capacity,
		RegisterTime: time.Now(),
		Status:       Available,
		Version:      0,
	})
	return err
}

// UpdateServerState transitions a game server to the given state.
// Version must match to prevent clobbering concurrent modifications.
func (m *matchmakingManager) UpdateServerState(ctx context.Context, serverID string, version int, state GameserverState) error {
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
	document, err := m.col.Database().Collection("pending_parties").InsertOne(ctx, &PartyInfo{
		PartyID:      partyID,
		PlayerIDs:    playerIDs,
		RegisterTime: time.Now(),
		Status:       PartyPending,
		Version:      0,
	})
	if err != nil {
		return "", err
	}
	queueID := document.InsertedID.(string)
	return queueID, nil
}

// GetQueue returns a party by its queue ID.
func (m *matchmakingManager) GetQueue(ctx context.Context, partyID string) (*PartyInfo, error) {
	var party PartyInfo
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

// UpdatePartyState transitions a party to the given status.
// Version must match to prevent clobbering concurrent modifications.
func (m *matchmakingManager) UpdatePartyState(ctx context.Context, partyID string, version int, state PartyStatus) error {
	result, err := m.col.Database().Collection("pending_parties").UpdateOne(ctx,
		bson.M{"_id": partyID, "version": version},
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

// GetGameservers returns all available game servers.
func (m *matchmakingManager) GetGameservers() ([]*GameserverInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cursor, err := m.col.Database().Collection("gameservers").Find(ctx,
		bson.M{"status": Available},
	)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var gameservers []*GameserverInfo
	if err := cursor.All(ctx, &gameservers); err != nil {
		return nil, err
	}
	return gameservers, nil
}

// GetPendingParties returns all unassigned parties, ordered oldest first.
func (m *matchmakingManager) GetPendingParties() ([]*PartyInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cursor, err := m.col.Database().Collection("pending_parties").Find(ctx,
		bson.M{"status": PartyPending},
		options.Find().SetSort(bson.M{"register_time": 1}),
	)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var parties []*PartyInfo
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
		{{Key: "$match", Value: bson.M{"status": PartyAssigned, "server_id": bson.M{"$ne": ""}}}},
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
func (m *matchmakingManager) UpdateMMState(ctx context.Context, parties []*PartyInfo) error {
	partyCol := m.col.Database().Collection("pending_parties")
	for _, party := range parties {
		result, err := partyCol.UpdateOne(ctx,
			bson.M{"_id": party.PartyID, "version": party.Version, "status": PartyPending},
			bson.M{"$set": bson.M{
				"status":    PartyAssigned,
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
