package managers

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mercury/pkg/instrumentation"
	"github.com/smira/go-statsd"
)

var ErrWalletNotFound = errors.New("wallet not found")

type Wallet struct {
	PlayerID   string
	Currencies map[string]int
}

type WalletManager interface {
	GetWallet(ctx context.Context, playerID string) (*Wallet, error)
	Grant(ctx context.Context, playerID, currencyID string, amount int, orderID string) (*Wallet, error)
}

type postgresWalletManager struct {
	pool         *pgxpool.Pool
	statsdClient *statsd.Client
}

func NewPostgresWalletManager(dsn string, statsdClient *statsd.Client) (WalletManager, error) {
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		return nil, err
	}
	return &postgresWalletManager{pool: pool, statsdClient: statsdClient}, nil
}

func (s *postgresWalletManager) GetWallet(ctx context.Context, playerID string) (_ *Wallet, err error) {
	t := instrumentation.NewMetricsTimer(ctx, "walletmgr.dur", statsd.StringTag("op", "get_wallet"))
	defer func() { t.Done(err) }()

	var exists bool
	if err := s.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM wallets WHERE player_id = $1)`, playerID,
	).Scan(&exists); err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrWalletNotFound
	}

	rows, err := s.pool.Query(ctx,
		`SELECT currency_id, amount FROM wallet_currencies WHERE player_id = $1`,
		playerID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	wallet := &Wallet{
		PlayerID:   playerID,
		Currencies: make(map[string]int),
	}
	for rows.Next() {
		var currencyID string
		var amount int
		if err := rows.Scan(&currencyID, &amount); err != nil {
			return nil, err
		}
		wallet.Currencies[currencyID] = amount
	}
	return wallet, rows.Err()
}

func (s *postgresWalletManager) Grant(ctx context.Context, playerID, currencyID string, amount int, orderID string) (_ *Wallet, err error) {
	t := instrumentation.NewMetricsTimer(ctx, "walletmgr.dur", statsd.StringTag("op", "grant"))
	defer func() { t.Done(err) }()

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	// Idempotency check
	tag, err := tx.Exec(ctx,
		`INSERT INTO wallet_processed_orders (order_id, player_id, currency_id, amount, created_at)
		 VALUES ($1, $2, $3, $4, $5) ON CONFLICT (order_id) DO NOTHING`,
		orderID, playerID, currencyID, amount, time.Now(),
	)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		if err := tx.Rollback(ctx); err != nil {
			return nil, err
		}
		return s.GetWallet(ctx, playerID)
	}

	// Upsert wallet row
	_, err = tx.Exec(ctx,
		`INSERT INTO wallets (player_id, created_at) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		playerID, time.Now(),
	)
	if err != nil {
		return nil, err
	}

	// Upsert currency balance
	_, err = tx.Exec(ctx,
		`INSERT INTO wallet_currencies (player_id, currency_id, amount)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (player_id, currency_id) DO UPDATE SET amount = wallet_currencies.amount + excluded.amount`,
		playerID, currencyID, amount,
	)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.GetWallet(ctx, playerID)
}
