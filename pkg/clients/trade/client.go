package trade

import (
	"context"

	"github.com/mercury/pkg/rmq"
	"github.com/sirupsen/logrus"
)

type RMQClient interface {
	Close()
	DraftTrade(ctx context.Context, orderID, playerID, initiatorID, transactionID string, contractingParties []string, grants []TradeGrant) (*DraftTradeResponse, error)
	LockTrade(ctx context.Context, orderID, playerID, transactionID string) (*LockTradeResponse, error)
	UnlockTrade(ctx context.Context, orderID, playerID, transactionID string) (*UnlockTradeResponse, error)
	DispatchGrants(ctx context.Context, orderID string, initiatorID string, grants []TradeGrant) (*TradeResponse, error)
	TradeStatus(ctx context.Context, orderID string) (*TradeStatusResponse, error)
}
type rmqClient struct {
	publisher *rmq.Publisher
	logger    *logrus.Logger
}

func NewClient(logger *logrus.Logger, amqpURL string) (RMQClient, error) {
	publisher, err := rmq.NewPublisher(amqpURL)
	if err != nil {
		return nil, err
	}
	return &rmqClient{
		publisher: publisher,
		logger:    logger,
	}, nil
}

func (c *rmqClient) Close() {
	c.publisher.Close()
}

type GrantType string

const (
	GrantTypeCurrency    = "CURRENCY"
	GrantTypeItem        = "ITEM"
	GrantTypeEntitlement = "ENTITLEMENT"
)

type TradeGrant struct {
	PlayerID string    `json:"player_id"` // Player ID to recieve the the grant
	Type     GrantType `json:"type"`
	TargetID string    `json:"target_id"` // TargetID (item id or currency id)
	Amount   int       `json:"amount"`
}

type DispatchGrantsRequest struct {
	OrderID     string       `json:"order_id"`
	InitiatorID string       `json:"initiator_id"`
	Grants      []TradeGrant `json:"grants"`
}

type TradeResponse struct {
	OrderID string `json:"order_id"`
}

func (c *rmqClient) DispatchGrants(
	ctx context.Context, orderID string, initiatorID string,
	grants []TradeGrant,
) (*TradeResponse, error) {
	return rmq.Request[DispatchGrantsRequest, TradeResponse](ctx, c.publisher, "trade.v1.dispatchgrants", DispatchGrantsRequest{
		OrderID:     orderID,
		InitiatorID: initiatorID,
		Grants:      grants,
	})
}

type TradeStatusRequest struct {
	OrderID string `json:"order_id"`
}

type TradeStatusResponse struct {
	OrderID string       `json:"order_id"`
	Status  OutboxStatus `json:"status"`
}

func (c *rmqClient) TradeStatus(
	ctx context.Context, orderID string,
) (*TradeStatusResponse, error) {
	return rmq.Request[TradeStatusRequest, TradeStatusResponse](ctx, c.publisher, "trade.v1.status", TradeStatusRequest{
		OrderID: orderID,
	})
}

type DraftTradeRequest struct {
	OrderID            string       `json:"order_id"`
	PlayerID           string       `json:"player_id"`
	InitiatorID        string       `json:"initiator_id"`
	ContractingParties []string     `json:"contracting_parties"`
	TransactionID      string       `json:"transaction_id,omitempty"`
	Grants             []TradeGrant `json:"grants"`
}

type DraftTradeResponse struct {
	OrderID            string                  `json:"order_id"`
	TransactionID      string                  `json:"transaction_id"`
	InitiatorID        string                  `json:"initiator_id"`
	ContractingParties []string                `json:"contracting_parties"`
	GrantsByPlayer     map[string][]TradeGrant `json:"grants_by_player"`
	Signatures         []string                `json:"signatures"`
}

func (c *rmqClient) DraftTrade(ctx context.Context, orderID, playerID, initiatorID, transactionID string, contractingParties []string, grants []TradeGrant) (*DraftTradeResponse, error) {
	return rmq.Request[DraftTradeRequest, DraftTradeResponse](ctx, c.publisher, "trade.v1.drafttrade", DraftTradeRequest{
		OrderID:            orderID,
		PlayerID:           playerID,
		InitiatorID:        initiatorID,
		TransactionID:      transactionID,
		ContractingParties: contractingParties,
		Grants:             grants,
	})
}

type LockTradeRequest struct {
	OrderID       string `json:"order_id"`
	PlayerID      string `json:"player_id"`
	TransactionID string `json:"transaction_id"`
}

type LockTradeResponse struct {
	OrderID            string       `json:"order_id"`
	TransactionID      string       `json:"transaction_id"`
	Status             OutboxStatus `json:"status"`
	ContractingParties []string     `json:"contracting_parties"`
	Signatures         []string     `json:"signatures"`
}

func (c *rmqClient) LockTrade(ctx context.Context, orderID, playerID, transactionID string) (*LockTradeResponse, error) {
	return rmq.Request[LockTradeRequest, LockTradeResponse](ctx, c.publisher, "trade.v1.locktrade", LockTradeRequest{
		OrderID:       orderID,
		PlayerID:      playerID,
		TransactionID: transactionID,
	})
}

type UnlockTradeRequest struct {
	OrderID       string `json:"order_id"`
	PlayerID      string `json:"player_id"`
	TransactionID string `json:"transaction_id"`
}

type UnlockTradeResponse struct {
	OrderID            string       `json:"order_id"`
	TransactionID      string       `json:"transaction_id"`
	Status             OutboxStatus `json:"status"`
	ContractingParties []string     `json:"contracting_parties"`
	Signatures         []string     `json:"signatures"`
}

func (c *rmqClient) UnlockTrade(ctx context.Context, orderID, playerID, transactionID string) (*UnlockTradeResponse, error) {
	return rmq.Request[UnlockTradeRequest, UnlockTradeResponse](ctx, c.publisher, "trade.v1.unlocktrade", UnlockTradeRequest{
		OrderID:       orderID,
		PlayerID:      playerID,
		TransactionID: transactionID,
	})
}
