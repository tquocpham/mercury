package wallet

import (
	"context"

	"github.com/mercury/pkg/rmq"
)

type RMQClient interface {
	GetWallet(ctx context.Context, playerID string) (*GetWalletResponse, error)
	AddCurrency(ctx context.Context, playerID string, currencyID string, amount int, orderID string) (*GetWalletResponse, error)
}

type client struct {
	Publisher *rmq.Publisher
}

// NewClient creates a new query client
func NewClient(amqpURL string) (RMQClient, error) {
	publisher, err := rmq.NewPublisher(amqpURL)
	if err != nil {
		return nil, err
	}
	return &client{
		Publisher: publisher,
	}, nil
}

type Currency struct {
	CurrencyType string `json:"currency_type"`
	Amount       int    `json:"amount"`
}

type GetWalletResponse struct {
	PlayerID   string     `json:"player_id"`
	Currencies []Currency `json:"currencies"`
}

type GetWalletRequest struct {
	PlayerID string `json:"player_id"`
}

func (c *client) GetWallet(ctx context.Context, playerID string) (*GetWalletResponse, error) {
	return rmq.Request[GetWalletRequest, GetWalletResponse](ctx, c.Publisher, "wallet.v1.get_wallet", GetWalletRequest{
		PlayerID: playerID,
	})
}

type AddCurrencyRequest struct {
	PlayerID   string `json:"player_id"`
	CurrencyID string `json:"currency_id"`
	Amount     int    `json:"amount"`
	OrderID    string `json:"order_id"`
}

func (c *client) AddCurrency(ctx context.Context, playerID string, currencyID string, amount int, orderID string) (*GetWalletResponse, error) {
	return rmq.Request[AddCurrencyRequest, GetWalletResponse](ctx, c.Publisher, "wallet.v1.add_currency", AddCurrencyRequest{
		PlayerID:   playerID,
		CurrencyID: currencyID,
		Amount:     amount,
		OrderID:    orderID,
	})
}
