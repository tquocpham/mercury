package courier

import (
	"context"
	"errors"
	"time"

	"github.com/mercury/pkg/clients/trade"
	"github.com/mercury/pkg/instrumentation"
	"github.com/smira/go-statsd"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

var ErrNoPendingEvents = errors.New("no pending outbox events")

type OutboxManager interface {
	LockNext(ctx context.Context) (*trade.OutboxEvent, error)
	Finalize(ctx context.Context, event trade.OutboxEvent, success bool) error
}

type outboxManager struct {
	col          *mongo.Collection
	statsdClient *statsd.Client
	lockTimeout  time.Duration
	maxAttempts  int
}

func NewOutboxManager(mongoAddr string, statsdClient *statsd.Client, lockTimeout time.Duration, maxAttempts int) (OutboxManager, error) {
	client, err := mongo.Connect(options.Client().ApplyURI(mongoAddr))
	if err != nil {
		return nil, err
	}
	col := client.Database("trade").Collection("outbox")
	return &outboxManager{
		col:          col,
		statsdClient: statsdClient,
		lockTimeout:  lockTimeout,
		maxAttempts:  maxAttempts,
	}, nil
}

func (m *outboxManager) LockNext(ctx context.Context) (_ *trade.OutboxEvent, err error) {
	t := instrumentation.NewMetricsTimer(ctx, "courier.dur", statsd.StringTag("op", "locknext"))
	defer func() { t.Done(err) }()

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	filter := bson.M{
		"status": bson.M{"$in": []trade.OutboxStatus{
			trade.OutboxStatusPending,
			trade.OutboxStatusPartial,
		}},
		"locked_at": bson.M{"$lt": time.Now().Add(-m.lockTimeout)},
		"attempts":  bson.M{"$lt": m.maxAttempts},
	}
	update := bson.M{
		"$set": bson.M{"locked_at": time.Now()},
	}
	var event trade.OutboxEvent
	if err := m.col.FindOneAndUpdate(ctx, filter, update).Decode(&event); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, ErrNoPendingEvents
		}
		return nil, err
	}
	return &event, nil
}

func (m *outboxManager) Finalize(ctx context.Context, event trade.OutboxEvent, success bool) (err error) {
	t := instrumentation.NewMetricsTimer(ctx, "courier.dur", statsd.StringTag("op", "finalize"))
	defer func() { t.Done(err) }()

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	newStatus := trade.OutboxStatusPartial
	if success {
		newStatus = trade.OutboxStatusCompleted
	}
	_, dberr := m.col.UpdateOne(ctx,
		bson.M{"_id": event.ID},
		bson.M{
			"$set": bson.M{
				"status":    newStatus,
				"grants":    event.Grants,
				"locked_at": time.Time{}, // Release lock
			},
			"$inc": bson.M{"attempts": 1},
		},
	)
	return dberr
}
