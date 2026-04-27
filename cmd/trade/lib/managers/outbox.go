package managers

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

var (
	ErrOrderNotFound = errors.New("order not found")
)

type OutboxManager interface {
	CreateOutbox(ctx context.Context, orderID, initiatorID string, grants []trade.GrantItem) error
	GetOutboxStatus(ctx context.Context, orderID string) (*trade.OutboxEvent, error)
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
	return &outboxManager{
		col:          col,
		statsdClient: statsdClient,
	}, nil
}

func (m *outboxManager) CreateOutbox(ctx context.Context, orderID, initiatorID string, grants []trade.GrantItem) (_ error) {
	t := instrumentation.NewMetricsTimer(ctx, "trademgr.dur", statsd.StringTag("op", "create_outbox"))
	defer func() { t.Done(nil) }()

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	zeroTime := time.Time{}
	_, err := m.col.InsertOne(ctx, &trade.OutboxEvent{
		OrderID:     orderID,
		InitiatorID: initiatorID,
		Grants:      grants,
		Status:      trade.OutboxStatusPending,
		Attempts:    0,
		LockedAt:    &zeroTime,
	})
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
