package trade

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type OutboxStatus string

const (
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
}

type OutboxEvent struct {
	ID          primitive.ObjectID `bson:"_id"`
	OrderID     string             `bson:"order_id"`
	InitiatorID string             `bson:"initiator_id"`
	Grants      []GrantItem        `bson:"grants"`
	Status      OutboxStatus       `bson:"status"`
	Attempts    int                `bson:"attempts"`
	LockedAt    *time.Time         `bson:"locked_at"` // For worker concurrency
}
