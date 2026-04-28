package managers

import (
	"context"
	"errors"
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

var errDuplicateOrder = errors.New("duplicate order")

func (s *walletManager) Grant(ctx context.Context, playerID string, currencyID string, amount int, orderID string) (_ *Wallet, err error) {
	t := instrumentation.NewMetricsTimer(ctx, "walletmgr.dur", statsd.StringTag("op", "grant"))
	defer func() { t.Done(err) }()

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	session, err := s.client.StartSession()
	if err != nil {
		return nil, err
	}
	defer session.EndSession(ctx)

	var wallet Wallet
	_, err = session.WithTransaction(ctx, func(sessCtx context.Context) (any, error) {
		_, err := s.processedOrders.InsertOne(sessCtx, bson.M{
			"_id":        orderID,
			"player_id":  playerID,
			"created_at": time.Now(),
		})
		if mongo.IsDuplicateKeyError(err) {
			return nil, errDuplicateOrder
		}
		if err != nil {
			return nil, err
		}

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

	if errors.Is(err, errDuplicateOrder) {
		if err := s.wallets.FindOne(ctx, bson.M{"player_id": playerID}).Decode(&wallet); err != nil {
			return nil, err
		}
		return &wallet, nil
	}
	if err != nil {
		return nil, err
	}
	return &wallet, nil
}
