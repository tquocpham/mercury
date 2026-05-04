package trade

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type OutboxStatus string

const (
	OutboxStatusDraft     OutboxStatus = "DRAFT"
	OutboxStatusPending   OutboxStatus = "PENDING"
	OutboxStatusPartial   OutboxStatus = "PARTIAL"
	OutboxStatusCompleted OutboxStatus = "COMPLETED"
	OutboxStatusFailed    OutboxStatus = "FAILED"
)

type GrantItem struct {
	PlayerID  string    `bson:"player_id"`
	Type      GrantType `bson:"type"`
	TargetID  string    `bson:"target_id"`
	Amount    int       `bson:"amount"`
	Delivered bool      `bson:"delivered"`
	OrderID   string    `bson:"order_id"`
}

type OutboxEvent struct {
	ID          primitive.ObjectID `bson:"_id"`
	OrderID     string             `bson:"order_id"`
	InitiatorID string             `bson:"initiator_id"`
	Grants      []GrantItem        `bson:"grants"`
	Status      OutboxStatus       `bson:"status"`
	Attempts    int                `bson:"attempts"`
	LockedAt    *time.Time         `bson:"locked_at"` // For worker concurrency

	// DRAFTS
	GrantsByPlayer     map[string][]GrantItem `bson:"grants_by_player,omitempty"`
	Created            time.Time            `bson:"created"`
	Modified           time.Time            `bson:"modified"`
	CommitID           string               `bson:"commit_id"`
	ContractingParties []string             `bson:"contracting_parties"`
	Signatures         []string             `bson:"signatures"`
}
