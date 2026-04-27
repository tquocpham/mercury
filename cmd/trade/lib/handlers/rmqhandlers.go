package handlers

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/mercury/cmd/trade/lib/managers"
	"github.com/mercury/pkg/clients/trade"
	"github.com/mercury/pkg/ids"
	"github.com/mercury/pkg/rmq"
)

type RMQHandlers interface {
	Trade(ctx context.Context, body []byte) ([]byte, error)
	TradeStatus(ctx context.Context, body []byte) ([]byte, error)
}

type rmqHanders struct {
	outboxManager managers.OutboxManager
}

func NewRMQHandlers(outboxManager managers.OutboxManager) RMQHandlers {
	return &rmqHanders{
		outboxManager: outboxManager,
	}
}

func (h *rmqHanders) Trade(ctx context.Context, body []byte) ([]byte, error) {
	logger := rmq.GetLogger(ctx)
	request := &trade.TradeRequest{}
	if err := json.Unmarshal(body, request); err != nil {
		logger.WithError(err).Error("Failed to parse trade request")
		return nil, trade.ErrInvalidRequest
	}
	if !ids.ValidateOrderID(request.OrderID) {
		logger.WithField("order_id", request.OrderID).Error("invalid order_id. Must be a valid ULID")
		return nil, trade.ErrInvalidRequest
	}
	grants := make([]trade.GrantItem, 0, len(request.Grants))
	for _, grant := range request.Grants {
		grants = append(grants, trade.GrantItem{
			PlayerID:  grant.PlayerID,
			Type:      grant.Type,
			TargetID:  grant.TargetID,
			Amount:    grant.Amount,
			Delivered: false,
		})
	}

	err := h.outboxManager.CreateOutbox(ctx, request.OrderID, request.InitiatorID, grants)
	if err != nil {
		return nil, trade.ErrFailedToCreateTrade
	}
	bts, err := json.Marshal(trade.TradeResponse{
		OrderID: request.OrderID,
	})
	if err != nil {
		return nil, trade.ErrFailedToCreateResponse
	}
	return bts, nil
}

func (h *rmqHanders) TradeStatus(ctx context.Context, body []byte) ([]byte, error) {
	logger := rmq.GetLogger(ctx)
	request := &trade.TradeStatusRequest{}
	if err := json.Unmarshal(body, request); err != nil {
		logger.WithError(err).Error("Failed to parse tradestatus request")
		return nil, trade.ErrInvalidRequest
	}
	outbox, err := h.outboxManager.GetOutboxStatus(ctx, request.OrderID)
	if errors.Is(err, managers.ErrOrderNotFound) {
		return nil, trade.ErrOrderNotFound
	}
	if err != nil {
		return nil, trade.ErrFailedToGetTradeStatus
	}
	bts, err := json.Marshal(trade.TradeStatusResponse{
		OrderID: request.OrderID,
		Status:  outbox.Status,
	})
	if err != nil {
		return nil, trade.ErrFailedToCreateResponse
	}
	return bts, nil
}
