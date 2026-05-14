CREATE TABLE IF NOT EXISTS wallets (
    player_id  TEXT PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS wallet_currencies (
    player_id   TEXT   NOT NULL REFERENCES wallets (player_id),
    currency_id TEXT   NOT NULL,
    amount      BIGINT NOT NULL DEFAULT 0,
    PRIMARY KEY (player_id, currency_id)
);

CREATE TABLE IF NOT EXISTS wallet_processed_orders (
    order_id    TEXT PRIMARY KEY,
    player_id   TEXT        NOT NULL,
    currency_id TEXT        NOT NULL,
    amount      INT         NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_wallet_processed_orders_player ON wallet_processed_orders (player_id);
