package trade

import (
	"context"

	"github.com/mercury/pkg/rmq"
)

type RMQClient interface {
	Close()
	Trade(
		ctx context.Context, orderID string, initiatorID string,
		grants []TradeGrant) (*TradeResponse, error)
}
type rmqClient struct {
	Publisher *rmq.Publisher
}

func NewClient(amqpURL string) (RMQClient, error) {
	publisher, err := rmq.NewPublisher(amqpURL)
	if err != nil {
		return nil, err
	}
	return &rmqClient{
		Publisher: publisher,
	}, nil
}

func (c *rmqClient) Close() {
	c.Publisher.Close()
}

type GrantType string

const (
	GrantTypeCurrency    = "CURRENCY"
	GrantTypeItem        = "ITEM"
	GrantTypeEntitlement = "ENTITLEMENT"
)

type TradeGrant struct {
	PlayerID string    `json:"player_id"`
	Type     GrantType `json:"type"`
	TargetID string    `json:"target_id"`
	Amount   int       `json:"amount"`
}

type TradeRequest struct {
	OrderID     string       `json:"order_id"`
	InitiatorID string       `json:"initiator_id"`
	Grants      []TradeGrant `json:"grants"`
}

type TradeResponse struct {
	OrderID string `json:"order_id"`
}

func (c *rmqClient) Trade(
	ctx context.Context, orderID string, initiatorID string,
	grants []TradeGrant,
) (*TradeResponse, error) {

	return rmq.Request[TradeRequest, TradeResponse](ctx, c.Publisher, "trade.v1.trade", TradeRequest{
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
	return rmq.Request[TradeStatusRequest, TradeStatusResponse](ctx, c.Publisher, "trade.v1.status", TradeStatusRequest{
		OrderID: orderID,
	})
}
