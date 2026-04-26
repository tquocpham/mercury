package matchmaking

import "time"

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
