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

var ErrWalletNotFound = errors.New("wallet not found")

type ProcessedOrder struct {
	OrderID    string    `bson:"order_id"`
	CurrencyID string    `bson:"currency_id"`
	Amount     int       `bson:"amount"`
	CreatedAt  time.Time `bson:"created_at"`
}

type Wallet struct {
	PlayerID        string           `bson:"player_id"`
	Currencies      map[string]int   `bson:"currencies"`
	ProcessedOrders []ProcessedOrder `bson:"processed_orders"`
}

type WalletManager interface {
	Grant(ctx context.Context, playerID string, currencyID string, amount int, orderID string) (*Wallet, error)
	GetWallet(ctx context.Context, playerID string) (*Wallet, error)
}

type walletManager struct {
	wallets      *mongo.Collection
	statsdClient *statsd.Client
}

func NewWalletManager(mongoAddr string, statsdClient *statsd.Client) (WalletManager, error) {
	client, err := mongo.Connect(options.Client().ApplyURI(mongoAddr))
	if err != nil {
		return nil, err
	}
	db := client.Database("wallet")
	return &walletManager{
		wallets:      db.Collection("wallets"),
		statsdClient: statsdClient,
	}, nil
}

func (s *walletManager) GetWallet(ctx context.Context, playerID string) (_ *Wallet, err error) {
	t := instrumentation.NewMetricsTimer(ctx, "walletmgr.dur", statsd.StringTag("op", "get_wallet"))
	defer func() { t.Done(err) }()

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var wallet Wallet
	if err := s.wallets.FindOne(ctx, bson.M{"player_id": playerID}).Decode(&wallet); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrWalletNotFound
		}
		return nil, err
	}
	return &wallet, nil
}

func (s *walletManager) Grant(
	ctx context.Context, playerID, currencyID string, amount int, orderID string,
) (_ *Wallet, err error) {
	t := instrumentation.NewMetricsTimer(ctx, "walletmgr.dur", statsd.StringTag("op", "grant"))
	defer func() { t.Done(err) }()

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	var wallet Wallet
	err = s.wallets.FindOneAndUpdate(ctx,
		bson.M{
			"player_id":               playerID,
			"processed_orders.order_id": bson.M{"$ne": orderID},
		},
		bson.M{
			"$inc":         bson.M{"currencies." + currencyID: amount},
			"$push":        bson.M{"processed_orders": ProcessedOrder{OrderID: orderID, CurrencyID: currencyID, Amount: amount, CreatedAt: time.Now()}},
			"$setOnInsert": bson.M{"player_id": playerID},
		},
		options.FindOneAndUpdate().SetReturnDocument(options.After).SetUpsert(true),
	).Decode(&wallet)
	if errors.Is(err, mongo.ErrNoDocuments) {
		// orderID already in processed_orders — idempotent return
		return s.GetWallet(ctx, playerID)
	}
	if err != nil {
		return nil, err
	}
	return &wallet, nil
}
