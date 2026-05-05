package archiver

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type ArchivedOrder struct {
	OrderID    string    `bson:"_id"`
	Source     string    `bson:"source"`
	PlayerID   string    `bson:"player_id"`
	Details    bson.M    `bson:"details"`
	ArchivedAt time.Time `bson:"archived_at"`
}

// Archiver prunes embedded processed_orders from a collection and writes
// them to an archive collection. Archive-before-delete ensures no data loss
// if the pull step fails — duplicates on retry are ignored.
type Archiver struct {
	source      *mongo.Collection
	archive     *mongo.Collection
	ordersField string
	sourceName  string
	retention   time.Duration
	interval    time.Duration
	batchSize   int32
}

func New(
	mongoAddr, sourceDB, sourceColl, archiveDB, archiveColl, ordersField, sourceName string,
	retention, interval time.Duration,
	batchSize int,
) (*Archiver, error) {
	client, err := mongo.Connect(options.Client().ApplyURI(mongoAddr))
	if err != nil {
		return nil, err
	}
	return &Archiver{
		source:      client.Database(sourceDB).Collection(sourceColl),
		archive:     client.Database(archiveDB).Collection(archiveColl),
		ordersField: ordersField,
		sourceName:  sourceName,
		retention:   retention,
		interval:    interval,
		batchSize:   int32(batchSize),
	}, nil
}

func (a *Archiver) Run(ctx context.Context, logger *logrus.Logger) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(a.interval):
			if err := a.archiveStale(ctx, logger); err != nil {
				logger.WithError(err).Error("archiver run failed")
			}
		}
	}
}

// ArchivePlayer immediately archives all processed orders for a specific player
// regardless of age. Intended for testing and manual operations.
func (a *Archiver) ArchivePlayer(ctx context.Context, playerID string) (int, error) {
	var raw bson.M
	if err := a.source.FindOne(ctx, bson.M{"player_id": playerID}).Decode(&raw); err != nil {
		return 0, err
	}

	docPlayerID, _ := raw["player_id"].(string)
	ordersRaw, _ := raw[a.ordersField].(bson.A)
	var toArchive []any
	for _, o := range ordersRaw {
		order, ok := o.(bson.M)
		if !ok {
			continue
		}
		orderID, _ := order["order_id"].(string)
		toArchive = append(toArchive, ArchivedOrder{
			OrderID:    orderID,
			Source:     a.sourceName,
			PlayerID:   docPlayerID,
			Details:    order,
			ArchivedAt: time.Now(),
		})
	}

	if len(toArchive) == 0 {
		return 0, nil
	}

	_, err := a.archive.InsertMany(ctx, toArchive, options.InsertMany().SetOrdered(false))
	if err != nil && !mongo.IsDuplicateKeyError(err) {
		return 0, err
	}

	_, err = a.source.UpdateOne(ctx,
		bson.M{"player_id": playerID},
		bson.M{"$set": bson.M{a.ordersField: bson.A{}}},
	)
	if err != nil {
		return 0, err
	}
	return len(toArchive), nil
}

func (a *Archiver) archiveStale(ctx context.Context, logger *logrus.Logger) error {
	cutoff := time.Now().Add(-a.retention)

	cursor, err := a.source.Find(ctx,
		bson.M{
			a.ordersField: bson.M{
				"$elemMatch": bson.M{
					"created_at": bson.M{
						"$lt": cutoff,
					},
				},
			},
		},
		options.Find().SetBatchSize(a.batchSize),
	)
	if err != nil {
		return err
	}
	defer cursor.Close(ctx)

	for cursor.Next(ctx) {
		var raw bson.M
		if err := cursor.Decode(&raw); err != nil {
			logger.WithError(err).Warn("archiver: failed to decode document, skipping")
			continue
		}

		docPlayerID, _ := raw["player_id"].(string)
		ordersRaw, _ := raw[a.ordersField].(bson.A)
		var toArchive []any
		for _, o := range ordersRaw {
			order, ok := o.(bson.M)
			if !ok {
				continue
			}
			createdAt, _ := order["created_at"].(time.Time)
			if !createdAt.Before(cutoff) {
				continue
			}
			orderID, _ := order["order_id"].(string)
			toArchive = append(toArchive, ArchivedOrder{
				OrderID:    orderID,
				Source:     a.sourceName,
				PlayerID:   docPlayerID,
				Details:    order,
				ArchivedAt: time.Now(),
			})
		}

		if len(toArchive) == 0 {
			continue
		}

		_, err := a.archive.InsertMany(ctx, toArchive, options.InsertMany().SetOrdered(false))
		if err != nil && !mongo.IsDuplicateKeyError(err) {
			logger.WithError(err).Error("archiver: failed to insert archived orders")
			continue
		}

		_, err = a.source.UpdateOne(ctx,
			bson.M{"_id": raw["_id"]},
			bson.M{"$pull": bson.M{
				a.ordersField: bson.M{"created_at": bson.M{"$lt": cutoff}},
			}},
		)
		if err != nil {
			logger.WithError(err).Error("archiver: failed to pull stale orders from document")
		} else {
			logger.WithField("count", len(toArchive)).Info("archiver: orders archived")
		}
	}

	return cursor.Err()
}
