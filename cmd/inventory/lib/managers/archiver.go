package managers

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type ArchivedOrder struct {
	ID         string    `bson:"_id"`
	PlayerID   string    `bson:"player_id"`
	CreatedAt  time.Time `bson:"created_at"`
	ArchivedAt time.Time `bson:"archived_at"`
}

type Archiver struct {
	processedOrders *mongo.Collection
	archivedOrders  *mongo.Collection
	retention       time.Duration
	interval        time.Duration
	batchSize       int
}

func NewArchiver(mongoAddr string, retention, interval time.Duration, batchSize int) (*Archiver, error) {
	client, err := mongo.Connect(options.Client().ApplyURI(mongoAddr))
	if err != nil {
		return nil, err
	}
	db := client.Database("inventory")
	return &Archiver{
		processedOrders: db.Collection("processed_orders"),
		archivedOrders:  db.Collection("archived_orders"),
		retention:       retention,
		interval:        interval,
		batchSize:       batchSize,
	}, nil
}

func (a *Archiver) Run(ctx context.Context, logger *logrus.Logger) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(a.interval):
			if err := a.archive(ctx, logger); err != nil {
				logger.WithError(err).Error("archiver failed")
			}
		}
	}
}

func (a *Archiver) archive(ctx context.Context, logger *logrus.Logger) error {
	cutoff := time.Now().Add(-a.retention)

	cursor, err := a.processedOrders.Find(ctx,
		bson.M{"created_at": bson.M{"$lt": cutoff}},
		options.Find().SetBatchSize(int32(a.batchSize)),
	)
	if err != nil {
		return err
	}
	defer cursor.Close(ctx)

	var batch []interface{}
	var ids []string

	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		// Archive first — ignore duplicates in case a previous run was interrupted
		_, err := a.archivedOrders.InsertMany(ctx, batch,
			options.InsertMany().SetOrdered(false),
		)
		if err != nil && !mongo.IsDuplicateKeyError(err) {
			return err
		}
		// Delete only after successful archive
		_, err = a.processedOrders.DeleteMany(ctx, bson.M{"_id": bson.M{"$in": ids}})
		if err != nil {
			return err
		}
		logger.WithField("count", len(batch)).Info("archived processed orders")
		batch = batch[:0]
		ids = ids[:0]
		return nil
	}

	for cursor.Next(ctx) {
		var order struct {
			ID        string    `bson:"_id"`
			PlayerID  string    `bson:"player_id"`
			CreatedAt time.Time `bson:"created_at"`
		}
		if err := cursor.Decode(&order); err != nil {
			logger.WithError(err).Warn("failed to decode processed order, skipping")
			continue
		}
		batch = append(batch, ArchivedOrder{
			ID:         order.ID,
			PlayerID:   order.PlayerID,
			CreatedAt:  order.CreatedAt,
			ArchivedAt: time.Now(),
		})
		ids = append(ids, order.ID)

		if len(batch) >= a.batchSize {
			if err := flush(); err != nil {
				return err
			}
		}
	}
	if err := cursor.Err(); err != nil {
		return err
	}
	return flush()
}
