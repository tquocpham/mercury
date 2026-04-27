package managers

import (
	"context"
	"time"

	"github.com/mercury/pkg/instrumentation"
	"github.com/smira/go-statsd"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type WalletManager interface {
	Grant(ctx context.Context, playerID string, currencyID string, amount int, orderID string) (*Wallet, error)
}

type Wallet struct {
	PlayerID   string         `bson:"player_id"`
	Currencies map[string]int `bson:"currencies"`
}

type walletManager struct {
	client          *mongo.Client
	wallets         *mongo.Collection
	processedOrders *mongo.Collection
	statsdClient    *statsd.Client
}

func NewWalletManager(mongoAddr string, statsdClient *statsd.Client) (WalletManager, error) {
	client, err := mongo.Connect(options.Client().ApplyURI(mongoAddr))
	if err != nil {
		return nil, err
	}
	db := client.Database("wallet")
	return &walletManager{
		client:          client,
		wallets:         db.Collection("wallets"),
		processedOrders: db.Collection("processed_orders"),
		statsdClient:    statsdClient,
	}, nil
}

func (s *walletManager) Grant(ctx context.Context, playerID string, currencyID string, amount int, orderID string) (_ *Wallet, err error) {
	t := instrumentation.NewMetricsTimer(ctx, "walletmgr.dur", statsd.StringTag("op", "grant"))
	defer func() { t.Done(err) }()

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	session, err := s.client.StartSession()
	if err != nil {
		return nil, err
	}
	defer session.EndSession(ctx)

	var wallet Wallet
	_, err = session.WithTransaction(ctx, func(sessCtx context.Context) (any, error) {
		// 1. Attempt to insert the order ID into a uniqueness table
		// This will FAIL if the order was already processed
		_, err := s.processedOrders.InsertOne(sessCtx, bson.M{
			"_id":        orderID,
			"player_id":  playerID,
			"created_at": time.Now(),
		})

		if mongo.IsDuplicateKeyError(err) {
			return nil, s.wallets.FindOne(sessCtx, bson.M{"player_id": playerID}).Decode(&wallet)
		}
		if err != nil {
			return nil, err
		}

		// 2. Increment the specific currency balance and return the updated wallet
		return nil, s.wallets.FindOneAndUpdate(
			sessCtx,
			bson.M{"player_id": playerID},
			bson.M{
				"$inc":         bson.M{"currencies." + currencyID: amount},
				"$setOnInsert": bson.M{"player_id": playerID},
			},
			options.FindOneAndUpdate().SetReturnDocument(options.After).SetUpsert(true),
		).Decode(&wallet)
	})
	if err != nil {
		return nil, err
	}
	return &wallet, nil
}
