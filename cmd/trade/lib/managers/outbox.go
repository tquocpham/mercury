package managers

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/mercury/pkg/clients/trade"
	"github.com/mercury/pkg/ids"
	"github.com/mercury/pkg/instrumentation"
	"github.com/smira/go-statsd"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

var (
	ErrOrderNotFound = errors.New("order not found")
)

// newCommitID creates a commit id string
func NewCommitID() string {
	return uuid.New().String()
}

type OutboxManager interface {
	CreateOutbox(ctx context.Context, orderID, initiatorID string, grants []trade.GrantItem) (_ error)
	GetOutboxStatus(ctx context.Context, orderID string) (_ *trade.OutboxEvent, _ error)
	InsertTrade(ctx context.Context, orderID string, event *trade.OutboxEvent) (_ error)
	UpdateTradeGrants(ctx context.Context, orderID, commitID, playerID string, grants []trade.GrantItem) (_ *trade.OutboxEvent, _ error)
	LockTrade(ctx context.Context, orderID, commitID, playerID string) (_ *trade.OutboxEvent, _ error)
	UnlockTrade(ctx context.Context, orderID, commitID, playerID string) (_ *trade.OutboxEvent, _ error)
}

type outboxManager struct {
	col          *mongo.Collection
	statsdClient *statsd.Client
}

func NewOutboxManager(mongoAddr string, statsdClient *statsd.Client) (OutboxManager, error) {
	client, err := mongo.Connect(options.Client().ApplyURI(mongoAddr))
	if err != nil {
		return nil, err
	}
	// mongo.Connect creates a connection pool managed by the driver.
	// The pool is created once at startup, reused across all requests
	col := client.Database("trade").Collection("outbox")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err = col.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "order_id", Value: 1}},
		Options: options.Index().SetUnique(true),
	})
	if err != nil {
		return nil, err
	}

	return &outboxManager{
		col:          col,
		statsdClient: statsdClient,
	}, nil
}

func (m *outboxManager) InsertTrade(ctx context.Context, orderID string, event *trade.OutboxEvent) (_ error) {

	t := instrumentation.NewMetricsTimer(ctx, "trademgr.dur", statsd.StringTag("op", "insert_trade"))
	defer func() { t.Done(nil) }()

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	now := time.Now()
	event.Modified = now
	event.Created = now
	_, err := m.col.UpdateOne(ctx,
		bson.M{
			"order_id": orderID,
		},
		bson.M{
			"$setOnInsert": event,
		},
		options.UpdateOne().SetUpsert(true),
	)
	return err
}

func (m *outboxManager) UpdateTradeGrants(
	ctx context.Context, orderID, commitID, playerID string, grants []trade.GrantItem) (_ *trade.OutboxEvent, _ error) {

	t := instrumentation.NewMetricsTimer(ctx, "trademgr.dur", statsd.StringTag("op", "update_trade"))
	defer func() { t.Done(nil) }()

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var updated trade.OutboxEvent
	err := m.col.FindOneAndUpdate(ctx,
		bson.M{
			"order_id":   orderID,
			"commit_id":  commitID,
			"status":     trade.OutboxStatusDraft,
			"signatures": bson.M{"$size": 0},
		},
		bson.M{
			"$set": bson.M{
				"grants_by_player." + playerID: grants,
				"modified":                     time.Now(),
				"commit_id":                    NewCommitID(),
			},
		},
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	).Decode(&updated)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, ErrOrderNotFound
	}
	if err != nil {
		return nil, err
	}
	return &updated, nil
}

func (m *outboxManager) LockTrade(
	ctx context.Context, orderID, commitID, playerID string) (_ *trade.OutboxEvent, _ error) {

	t := instrumentation.NewMetricsTimer(ctx, "trademgr.dur", statsd.StringTag("op", "lock_trade"))
	defer func() { t.Done(nil) }()

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var updated trade.OutboxEvent
	err := m.col.FindOneAndUpdate(ctx,
		bson.M{
			"order_id":            orderID,
			"commit_id":           commitID,
			"status":              trade.OutboxStatusDraft,
			"contracting_parties": playerID,
		},
		bson.M{
			"$addToSet": bson.M{"signatures": playerID},
			"$set": bson.M{
				"modified":  time.Now(),
				"commit_id": NewCommitID(),
			},
		},
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	).Decode(&updated)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, ErrOrderNotFound
	}
	if err != nil {
		return nil, err
	}

	if len(updated.Signatures) == len(updated.ContractingParties) {
		var grants []trade.GrantItem
		for _, playerGrants := range updated.GrantsByPlayer {
			for _, g := range playerGrants {
				g.OrderID = ids.NewOrderID()
				grants = append(grants, g)
			}
		}
		_, err = m.col.UpdateOne(ctx,
			bson.M{"order_id": orderID, "commit_id": updated.CommitID},
			bson.M{"$set": bson.M{
				"status": trade.OutboxStatusPending,
				"grants": grants,
			}},
		)
		if err != nil {
			return nil, err
		}
		updated.Status = trade.OutboxStatusPending
		updated.Grants = grants
	}

	return &updated, nil
}

func (m *outboxManager) UnlockTrade(
	ctx context.Context, orderID, commitID, playerID string) (_ *trade.OutboxEvent, _ error) {

	t := instrumentation.NewMetricsTimer(ctx, "trademgr.dur", statsd.StringTag("op", "unlock_trade"))
	defer func() { t.Done(nil) }()

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var updated trade.OutboxEvent
	err := m.col.FindOneAndUpdate(ctx,
		bson.M{
			"order_id":            orderID,
			"commit_id":           commitID,
			"status":              trade.OutboxStatusDraft,
			"contracting_parties": playerID,
		},
		bson.M{
			"$set": bson.M{
				"signatures": []string{},
				"modified":   time.Now(),
				"commit_id":  NewCommitID(),
			},
		},
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	).Decode(&updated)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, ErrOrderNotFound
	}
	if err != nil {
		return nil, err
	}
	return &updated, nil
}

func (m *outboxManager) CreateOutbox(ctx context.Context, orderID, initiatorID string, grants []trade.GrantItem) (_ error) {
	t := instrumentation.NewMetricsTimer(ctx, "trademgr.dur", statsd.StringTag("op", "create_outbox"))
	defer func() { t.Done(nil) }()

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	event := trade.OutboxEvent{
		ID:          primitive.NewObjectID(), // Manually set to ensure it's an ObjectID
		OrderID:     orderID,
		InitiatorID: initiatorID,
		Grants:      grants,
		Status:      trade.OutboxStatusPending,
		Attempts:    0,
		LockedAt:    &time.Time{}, // Pointer to zero time as per your struct
	}

	_, err := m.col.UpdateOne(ctx,
		bson.M{"order_id": orderID},
		bson.M{
			"$setOnInsert": event,
		},
		options.UpdateOne().SetUpsert(true),
	)
	return err
}

func (m *outboxManager) GetOutboxStatus(ctx context.Context, orderID string) (_ *trade.OutboxEvent, err error) {
	t := instrumentation.NewMetricsTimer(ctx, "trademgr.dur", statsd.StringTag("op", "get_outbox_status"))
	defer func() { t.Done(err) }()

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var event trade.OutboxEvent
	err = m.col.FindOne(ctx, bson.M{"order_id": orderID}).Decode(&event)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, ErrOrderNotFound
	}
	if err != nil {
		return nil, err
	}
	return &event, nil
}
