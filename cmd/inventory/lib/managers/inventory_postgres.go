package managers

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mercury/pkg/instrumentation"
	"github.com/smira/go-statsd"
)

// querier is satisfied by both *pgxpool.Pool and pgx.Tx.
type querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

type postgresInventoryManager struct {
	pool         *pgxpool.Pool
	statsdClient *statsd.Client
}

func NewPostgresInventoryManager(dsn string, statsdClient *statsd.Client) (InventoryManager, error) {
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		return nil, err
	}
	return &postgresInventoryManager{pool: pool, statsdClient: statsdClient}, nil
}

func scanInventory(ctx context.Context, q querier, playerID string) (*Inventory, error) {
	var inv Inventory
	err := q.QueryRow(ctx,
		`SELECT player_id, unlocked_slots, created_at, updated_at
		 FROM inventories WHERE player_id = $1`,
		playerID,
	).Scan(&inv.PlayerID, &inv.UnlockedSlots, &inv.CreatedAt, &inv.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrInventoryNotFound
	}
	if err != nil {
		return nil, err
	}

	rows, err := q.Query(ctx,
		`SELECT slot_id, item_id, amount FROM inventory_slots
		 WHERE player_id = $1 ORDER BY slot_id`,
		playerID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var slot InventorySlot
		if err := rows.Scan(&slot.SlotID, &slot.ItemID, &slot.Amount); err != nil {
			return nil, err
		}
		inv.Slots = append(inv.Slots, slot)
	}
	return &inv, rows.Err()
}

func (s *postgresInventoryManager) GetInventory(ctx context.Context, playerID string) (_ *Inventory, err error) {
	t := instrumentation.NewMetricsTimer(ctx, "invmgr.dur", statsd.StringTag("op", "get_inventory"))
	defer func() { t.Done(err) }()

	return scanInventory(ctx, s.pool, playerID)
}

func (s *postgresInventoryManager) CreateInventory(ctx context.Context, playerID string) (_ *Inventory, err error) {
	t := instrumentation.NewMetricsTimer(ctx, "invmgr.dur", statsd.StringTag("op", "create_inventory"))
	defer func() { t.Done(err) }()

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	now := time.Now()
	_, err = tx.Exec(ctx,
		`INSERT INTO inventories (player_id, unlocked_slots, created_at, updated_at)
		 VALUES ($1, $2, $3, $4) ON CONFLICT (player_id) DO NOTHING`,
		playerID, defaultUnlockedSlots, now, now,
	)
	if err != nil {
		return nil, err
	}

	slots := initialSlots(defaultUnlockedSlots)
	for _, slot := range slots {
		_, err = tx.Exec(ctx,
			`INSERT INTO inventory_slots (player_id, slot_id, item_id, amount)
			 VALUES ($1, $2, $3, $4) ON CONFLICT DO NOTHING`,
			playerID, slot.SlotID, "", 0,
		)
		if err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return scanInventory(ctx, s.pool, playerID)
}

func (s *postgresInventoryManager) AddItem(ctx context.Context, playerID, itemID, orderID string, amount, maxStack int) (_ *Inventory, err error) {
	t := instrumentation.NewMetricsTimer(ctx, "invmgr.dur", statsd.StringTag("op", "add_item"))
	defer func() { t.Done(err) }()

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	// Idempotency check
	tag, err := tx.Exec(ctx,
		`INSERT INTO inventory_processed_orders (order_id, player_id, created_at)
		 VALUES ($1, $2, $3) ON CONFLICT (order_id) DO NOTHING`,
		orderID, playerID, time.Now(),
	)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		if err := tx.Rollback(ctx); err != nil {
			return nil, err
		}
		return scanInventory(ctx, s.pool, playerID)
	}

	// Existence check
	var exists bool
	if err := tx.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM inventories WHERE player_id = $1)`, playerID,
	).Scan(&exists); err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrInventoryNotFound
	}

	// Try to stack onto an existing slot with room
	// FOR UPDATE SKIP LOCKED prevents two concurrent requests from claiming the same slot
	tag, err = tx.Exec(ctx,
		`UPDATE inventory_slots SET amount = amount + $1
		 WHERE (player_id, slot_id) = (
		     SELECT player_id, slot_id FROM inventory_slots
		     WHERE player_id = $2 AND item_id = $3 AND amount <= $4 - $1
		     LIMIT 1 FOR UPDATE SKIP LOCKED
		 )`,
		amount, playerID, itemID, maxStack,
	)
	if err != nil {
		return nil, err
	}

	// No existing stack with room — claim the first empty slot
	if tag.RowsAffected() == 0 {
		tag, err = tx.Exec(ctx,
			`UPDATE inventory_slots SET item_id = $1, amount = $2
			 WHERE (player_id, slot_id) = (
			     SELECT player_id, slot_id FROM inventory_slots
			     WHERE player_id = $3 AND item_id = ''
			     LIMIT 1 FOR UPDATE SKIP LOCKED
			 )`,
			itemID, amount, playerID,
		)
		if err != nil {
			return nil, err
		}
		if tag.RowsAffected() == 0 {
			return nil, ErrInventoryFull
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return scanInventory(ctx, s.pool, playerID)
}

func (s *postgresInventoryManager) AddItemToSlot(ctx context.Context, playerID, itemID, orderID string, slotID, amount, maxStack int) (_ *Inventory, err error) {
	t := instrumentation.NewMetricsTimer(ctx, "invmgr.dur", statsd.StringTag("op", "add_item_to_slot"))
	defer func() { t.Done(err) }()

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	// Idempotency check
	tag, err := tx.Exec(ctx,
		`INSERT INTO inventory_processed_orders (order_id, player_id, created_at)
		 VALUES ($1, $2, $3) ON CONFLICT (order_id) DO NOTHING`,
		orderID, playerID, time.Now(),
	)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		if err := tx.Rollback(ctx); err != nil {
			return nil, err
		}
		return scanInventory(ctx, s.pool, playerID)
	}

	// Existence check
	var exists bool
	if err := tx.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM inventories WHERE player_id = $1)`, playerID,
	).Scan(&exists); err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrInventoryNotFound
	}

	tag, err = tx.Exec(ctx,
		`UPDATE inventory_slots
		 SET item_id = $1, amount = amount + $2
		 WHERE player_id = $3 AND slot_id = $4
		   AND (item_id = '' OR (item_id = $1 AND amount <= $5 - $2))`,
		itemID, amount, playerID, slotID, maxStack,
	)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		return nil, ErrSlotNotAvailable
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return scanInventory(ctx, s.pool, playerID)
}

func (s *postgresInventoryManager) UnlockSlots(ctx context.Context, playerID string, count int) (_ *Inventory, err error) {
	t := instrumentation.NewMetricsTimer(ctx, "invmgr.dur", statsd.StringTag("op", "unlock_slots"))
	defer func() { t.Done(err) }()

	if count == 0 {
		return s.GetInventory(ctx, playerID)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	inv, err := scanInventory(ctx, tx, playerID)
	if err != nil {
		return nil, err
	}

	for i := range count {
		_, err = tx.Exec(ctx,
			`INSERT INTO inventory_slots (player_id, slot_id, item_id, amount)
			 VALUES ($1, $2, '', 0)`,
			playerID, inv.UnlockedSlots+i,
		)
		if err != nil {
			return nil, err
		}
	}
	_, err = tx.Exec(ctx,
		`UPDATE inventories SET unlocked_slots = unlocked_slots + $1, updated_at = $2
		 WHERE player_id = $3`,
		count, time.Now(), playerID,
	)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return scanInventory(ctx, s.pool, playerID)
}
