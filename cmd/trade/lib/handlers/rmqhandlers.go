package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/mercury/cmd/trade/lib/managers"
	"github.com/mercury/pkg/clients/trade"
	"github.com/mercury/pkg/ids"
	"github.com/mercury/pkg/rmq"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type RMQHandlers interface {
	DraftTrade(ctx context.Context, body []byte) ([]byte, error)
	LockTrade(ctx context.Context, body []byte) ([]byte, error)
	UnlockTrade(ctx context.Context, body []byte) ([]byte, error)
	ExecuteTrade(ctx context.Context, body []byte) ([]byte, error)
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

func (h *rmqHanders) DraftTrade(ctx context.Context, body []byte) ([]byte, error) {
	logger := rmq.GetLogger(ctx)
	request := &trade.DraftTradeRequest{}
	if err := json.Unmarshal(body, request); err != nil {
		logger.WithError(err).Error("Failed to parse create trade request")
		return nil, trade.ErrInvalidRequest
	}
	if !ids.ValidateOrderID(request.OrderID) {
		logger.WithField("order_id", request.OrderID).Error("order_id must be a valid ULID")
		return nil, trade.ErrInvalidRequest
	}
	if request.PlayerID == "" {
		logger.Error("player_id is required")
		return nil, trade.ErrInvalidRequest
	}
	commitID := request.TransactionID
	_, err := h.outboxManager.GetOutboxStatus(ctx, request.OrderID)
	if errors.Is(err, managers.ErrOrderNotFound) {
		logger.WithField("order_id", request.OrderID).Info("order not found, creating new draft")
		commitID = managers.NewCommitID()
		outbox := &trade.OutboxEvent{
			ID:                 primitive.NewObjectID(),
			OrderID:            request.OrderID,
			InitiatorID:        request.InitiatorID,
			ContractingParties: request.ContractingParties,
			Signatures:         []string{},
			Status:             trade.OutboxStatusDraft,
			Attempts:           0,
			LockedAt:           &time.Time{},
			CommitID:           commitID,
		}
		if err := h.outboxManager.InsertTrade(ctx, request.OrderID, outbox); err != nil {
			return nil, trade.ErrFailedToCreateTrade
		}
	} else if err != nil {
		return nil, trade.ErrFailedToGetTradeStatus
	}

	grants := make([]trade.GrantItem, 0, len(request.Grants))
	for _, g := range request.Grants {
		grants = append(grants, trade.GrantItem{
			PlayerID: g.PlayerID,
			Type:     g.Type,
			TargetID: g.TargetID,
			Amount:   g.Amount,
		})
	}

	updated, err := h.outboxManager.UpdateTradeGrants(ctx, request.OrderID, commitID, request.PlayerID, grants)
	if errors.Is(err, managers.ErrOrderNotFound) {
		return nil, trade.ErrTradeConflict
	}
	if err != nil {
		return nil, trade.ErrFailedToUpdateTrade
	}

	grantsByPlayer := make(map[string][]trade.TradeGrant, len(updated.GrantsByPlayer))
	for pid, playerGrants := range updated.GrantsByPlayer {
		tg := make([]trade.TradeGrant, 0, len(playerGrants))
		for _, g := range playerGrants {
			tg = append(tg, trade.TradeGrant{
				PlayerID: g.PlayerID,
				Type:     g.Type,
				TargetID: g.TargetID,
				Amount:   g.Amount,
			})
		}
		grantsByPlayer[pid] = tg
	}

	bts, err := json.Marshal(trade.DraftTradeResponse{
		OrderID:            updated.OrderID,
		TransactionID:      updated.CommitID,
		InitiatorID:        updated.InitiatorID,
		ContractingParties: updated.ContractingParties,
		GrantsByPlayer:     grantsByPlayer,
		Signatures:         updated.Signatures,
	})
	if err != nil {
		return nil, trade.ErrFailedToCreateResponse
	}
	return bts, nil
}

func (h *rmqHanders) LockTrade(ctx context.Context, body []byte) ([]byte, error) {
	logger := rmq.GetLogger(ctx)
	request := &trade.LockTradeRequest{}
	if err := json.Unmarshal(body, request); err != nil {
		logger.WithError(err).Error("Failed to parse lock trade request")
		return nil, trade.ErrInvalidRequest
	}
	if !ids.ValidateOrderID(request.OrderID) {
		logger.WithField("order_id", request.OrderID).Error("order_id must be a valid ULID")
		return nil, trade.ErrInvalidRequest
	}
	if request.PlayerID == "" {
		logger.Error("player_id is required")
		return nil, trade.ErrInvalidRequest
	}
	if request.TransactionID == "" {
		logger.Error("transaction_id is required")
		return nil, trade.ErrInvalidRequest
	}

	updated, err := h.outboxManager.LockTrade(ctx, request.OrderID, request.TransactionID, request.PlayerID)
	if errors.Is(err, managers.ErrOrderNotFound) {
		return nil, trade.ErrTradeConflict
	}
	if err != nil {
		return nil, trade.ErrFailedToUpdateTrade
	}

	bts, err := json.Marshal(trade.LockTradeResponse{
		OrderID:            updated.OrderID,
		TransactionID:      updated.CommitID,
		Status:             updated.Status,
		ContractingParties: updated.ContractingParties,
		Signatures:         updated.Signatures,
	})
	if err != nil {
		return nil, trade.ErrFailedToCreateResponse
	}
	return bts, nil
}

func (h *rmqHanders) UnlockTrade(ctx context.Context, body []byte) ([]byte, error) {
	logger := rmq.GetLogger(ctx)
	request := &trade.UnlockTradeRequest{}
	if err := json.Unmarshal(body, request); err != nil {
		logger.WithError(err).Error("Failed to parse unlock trade request")
		return nil, trade.ErrInvalidRequest
	}
	if !ids.ValidateOrderID(request.OrderID) {
		logger.WithField("order_id", request.OrderID).Error("order_id must be a valid ULID")
		return nil, trade.ErrInvalidRequest
	}
	if request.PlayerID == "" {
		logger.Error("player_id is required")
		return nil, trade.ErrInvalidRequest
	}
	if request.TransactionID == "" {
		logger.Error("transaction_id is required")
		return nil, trade.ErrInvalidRequest
	}

	updated, err := h.outboxManager.UnlockTrade(ctx, request.OrderID, request.TransactionID, request.PlayerID)
	if errors.Is(err, managers.ErrOrderNotFound) {
		return nil, trade.ErrTradeConflict
	}
	if err != nil {
		return nil, trade.ErrFailedToUpdateTrade
	}

	bts, err := json.Marshal(trade.UnlockTradeResponse{
		OrderID:            updated.OrderID,
		TransactionID:      updated.CommitID,
		Status:             updated.Status,
		ContractingParties: updated.ContractingParties,
		Signatures:         updated.Signatures,
	})
	if err != nil {
		return nil, trade.ErrFailedToCreateResponse
	}
	return bts, nil
}

func (h *rmqHanders) ExecuteTrade(ctx context.Context, body []byte) ([]byte, error) {
	logger := rmq.GetLogger(ctx)
	request := &trade.ExecuteTradeRequest{}
	if err := json.Unmarshal(body, request); err != nil {
		logger.WithError(err).Error("Failed to parse execute trade request")
		return nil, trade.ErrInvalidRequest
	}
	if !ids.ValidateOrderID(request.OrderID) {
		logger.WithField("order_id", request.OrderID).Error("order_id must be a valid ULID")
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
			OrderID:   ids.NewOrderID(),
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
