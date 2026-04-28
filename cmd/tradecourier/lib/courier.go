package courier

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/mercury/pkg/clients/inventory"
	"github.com/mercury/pkg/clients/trade"
	"github.com/mercury/pkg/clients/wallet"
	"github.com/mercury/pkg/instrumentation"
	"github.com/sirupsen/logrus"
	"github.com/smira/go-statsd"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type Courier interface {
	Run(ctx context.Context, logger *logrus.Logger)
}

type courier struct {
	interval        time.Duration
	maxRunTime      time.Duration
	col             *mongo.Collection
	inventoryClient inventory.RMQClient
	walletClient    wallet.RMQClient
	statsdClient    *statsd.Client
}

func NewCourier(
	interval time.Duration,
	mongoAddr string,
	inventoryClient inventory.RMQClient,
	walletClient wallet.RMQClient,
	statsdClient *statsd.Client,
) (Courier, error) {

	client, err := mongo.Connect(options.Client().ApplyURI(mongoAddr))
	if err != nil {
		return nil, err
	}
	col := client.Database("trade").Collection("outbox")
	return &courier{
		interval:        interval,
		col:             col,
		inventoryClient: inventoryClient,
		walletClient:    walletClient,
		maxRunTime:      10 * time.Minute,
		statsdClient:    statsdClient,
	}, nil
}

func (c *courier) Run(ctx context.Context, logger *logrus.Logger) {
	interval := c.interval
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(interval):
			t := instrumentation.NewMetricsTimer(ctx, "courier.dur", statsd.StringTag("op", "run"))
			runID := uuid.New().String()
			runCtx, cancel := context.WithTimeout(ctx, c.maxRunTime)
			entry := logger.WithFields(logrus.Fields{
				"run_id": runID,
			})
			hasOutbox, err := c.processBatch(runCtx, entry)
			cancel()
			if hasOutbox {
				interval = 500 * time.Millisecond
			} else {
				interval = c.interval
			}
			t.Done(err)
		}
	}
}

func (c *courier) processBatch(ctx context.Context, logger *logrus.Entry) (bool, error) {
	// 1. Find and Lock a batch of messages
	// We use findOneAndUpdate to ensure only ONE worker gets this specific message
	filter := bson.M{
		"status":    bson.M{"$in": []trade.OutboxStatus{trade.OutboxStatusPending, trade.OutboxStatusPartial}},
		"locked_at": bson.M{"$lt": time.Now().Add(-1 * time.Minute)}, // Simple lock timeout
		"attempts":  bson.M{"$lt": 5},
	}

	update := bson.M{
		"$set": bson.M{"locked_at": time.Now()},
	}

	var event trade.OutboxEvent
	err := c.col.FindOneAndUpdate(ctx, filter, update).Decode(&event)
	if err == mongo.ErrNoDocuments {
		return false, err
	}
	if err != nil {
		logger.WithError(err).Error("courier failed to lock outbox event")
		return false, err
	}

	logger = logger.WithFields(logrus.Fields{
		"event_id": event.ID,
		"order_id": event.OrderID,
	})

	// 2. Process the Grants
	allSucceeded := true
	for i, grant := range event.Grants {
		if grant.Delivered {
			continue // Skip already done (Partial Success case)
		}

		var grantErr error
		switch grant.Type {
		case trade.GrantTypeCurrency:
			_, grantErr = c.walletClient.AddCurrency(ctx, grant.PlayerID, grant.TargetID, grant.Amount, event.OrderID)
		case trade.GrantTypeItem, trade.GrantTypeEntitlement:
			_, grantErr = c.inventoryClient.AddItem(grant.PlayerID, grant.TargetID, grant.Amount, event.OrderID)
		default:
			logger.WithFields(logrus.Fields{
				"grant_type": grant.Type,
			}).Warn("unknown grant type")
			allSucceeded = false
			continue
		}
		if grantErr == nil {
			event.Grants[i].Delivered = true
		} else {
			logger.Printf("Grant failed for order %s: %v", event.OrderID, err)
			logger.
				WithError(grantErr).
				WithFields(logrus.Fields{
					"grant_type": grant.Type,
				}).Warn("unknown grant type")
			allSucceeded = false
		}
	}

	return true, c.finalize(ctx, logger, event, allSucceeded)
}

func (c *courier) finalize(ctx context.Context, logger *logrus.Entry, event trade.OutboxEvent, success bool) error {
	newStatus := trade.OutboxStatusPartial
	if success {
		newStatus = trade.OutboxStatusCompleted
	}

	_, err := c.col.UpdateOne(ctx,
		bson.M{"_id": event.ID},
		bson.M{
			"$set": bson.M{
				"status":    newStatus,
				"grants":    event.Grants,
				"locked_at": time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC), // Release lock
			},
			"$inc": bson.M{"attempts": 1},
		},
	)
	if err != nil {
		logger.
			WithError(err).
			Error("Failed to finalize outbox item")
		return err
	}
	return nil
}
