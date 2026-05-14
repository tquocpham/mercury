CREATE TABLE IF NOT EXISTS inventories (
    player_id      TEXT PRIMARY KEY,
    unlocked_slots INT         NOT NULL DEFAULT 20,
    created_at     TIMESTAMPTZ NOT NULL,
    updated_at     TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS inventory_slots (
    player_id TEXT NOT NULL REFERENCES inventories (player_id),
    slot_id   INT  NOT NULL,
    item_id   TEXT NOT NULL DEFAULT '',
    amount    INT  NOT NULL DEFAULT 0,
    PRIMARY KEY (player_id, slot_id)
);

CREATE TABLE IF NOT EXISTS inventory_processed_orders (
    order_id   TEXT PRIMARY KEY,
    player_id  TEXT        NOT NULL,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_inventory_processed_orders_player ON inventory_processed_orders (player_id);
