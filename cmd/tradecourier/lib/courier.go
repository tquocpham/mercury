package courier

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/mercury/pkg/clients/inventory"
	"github.com/mercury/pkg/clients/trade"
	"github.com/mercury/pkg/clients/wallet"
	"github.com/mercury/pkg/instrumentation"
	"github.com/sirupsen/logrus"
	"github.com/smira/go-statsd"
)

type Courier interface {
	Run(ctx context.Context, logger *logrus.Logger)
}

type courier struct {
	checkInterval   time.Duration
	workInterval    time.Duration
	maxRunTime      time.Duration
	outboxManager   OutboxManager
	inventoryClient inventory.RMQClient
	walletClient    wallet.RMQClient
	statsdClient    *statsd.Client
}

func NewCourier(
	checkInterval time.Duration,
	workInterval time.Duration,
	outboxManager OutboxManager,
	inventoryClient inventory.RMQClient,
	walletClient wallet.RMQClient,
	statsdClient *statsd.Client,
) Courier {
	return &courier{
		checkInterval:   checkInterval,
		workInterval:    workInterval,
		outboxManager:   outboxManager,
		inventoryClient: inventoryClient,
		walletClient:    walletClient,
		maxRunTime:      10 * time.Minute,
		statsdClient:    statsdClient,
	}
}

func (c *courier) Run(ctx context.Context, logger *logrus.Logger) {
	interval := c.checkInterval
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
				interval = c.workInterval
			} else {
				interval = c.checkInterval
			}
			t.Done(err)
		}
	}
}

func (c *courier) processBatch(ctx context.Context, logger *logrus.Entry) (bool, error) {
	event, err := c.outboxManager.LockNext(ctx)
	if errors.Is(err, ErrNoPendingEvents) {
		return false, nil
	}
	if err != nil {
		logger.WithError(err).Error("courier failed to lock outbox event")
		return false, err
	}

	logger = logger.WithFields(logrus.Fields{
		"event_id": event.ID,
		"order_id": event.OrderID,
	})

	// Process the Grants
	allSucceeded := true
	for i, grant := range event.Grants {
		if grant.Delivered {
			continue // Skip already done (Partial Success case)
		}

		var grantErr error
		switch grant.Type {
		case trade.GrantTypeCurrency:
			_, grantErr = c.walletClient.AddCurrency(
				ctx, grant.PlayerID, grant.TargetID, grant.Amount, grant.OrderID)
		case trade.GrantTypeItem, trade.GrantTypeEntitlement:
			_, grantErr = c.inventoryClient.AddItem(
				ctx, grant.PlayerID, grant.TargetID, grant.OrderID, grant.Amount, 250)
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
			logger.
				WithError(grantErr).
				WithFields(logrus.Fields{
					"grant_type": grant.Type,
				}).Warn("unknown grant type")
			allSucceeded = false
		}
	}

	return true, c.finalize(ctx, logger, *event, allSucceeded)
}

func (c *courier) finalize(ctx context.Context, logger *logrus.Entry, event trade.OutboxEvent, success bool) error {
	if err := c.outboxManager.Finalize(ctx, event, success); err != nil {
		logger.WithError(err).Error("failed to finalize outbox item")
		return err
	}
	if success {
		logger.Info("outbox event delivered successfully")
	}
	return nil
}
