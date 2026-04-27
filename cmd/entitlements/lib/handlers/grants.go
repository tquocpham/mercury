package handlers

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/mercury/cmd/entitlements/lib/managers"
	"github.com/mercury/pkg/clients/entitlements"
	"github.com/mercury/pkg/clients/inventory"
	"github.com/mercury/pkg/clients/trade"
	"github.com/mercury/pkg/clients/wallet"
	"github.com/mercury/pkg/ids"
	"github.com/mercury/pkg/rmq"
)

type GrantHandlers interface {
	Check(ctx context.Context, body []byte) ([]byte, error)
	Grant(ctx context.Context, body []byte) ([]byte, error)
	Revoke(ctx context.Context, body []byte) ([]byte, error)
}

type grantHandlers struct {
	grantsManager   managers.GrantsManager
	catalogManager  managers.CatalogManager
	walletClient    wallet.RMQClient
	inventoryClient inventory.RMQClient
	tradeClient     trade.RMQClient
}

func NewGrantHandlers(
	grantsManager managers.GrantsManager, catalogManager managers.CatalogManager,
	walletClient wallet.RMQClient, inventoryClient inventory.RMQClient,
	tradeClient trade.RMQClient,
) GrantHandlers {

	return &grantHandlers{
		grantsManager:   grantsManager,
		catalogManager:  catalogManager,
		walletClient:    walletClient,
		inventoryClient: inventoryClient,
		tradeClient:     tradeClient,
	}
}

func catalogGrantTypeToTradeGrantType(t entitlements.CatalogGrantType) (trade.GrantType, error) {
	switch t {
	case entitlements.CatalogGrantTypeEntitlement:
		return trade.GrantTypeEntitlement, nil
	case entitlements.CatalogGrantTypeCurrency:
		return trade.GrantTypeCurrency, nil
	default:
		return "", entitlements.ErrInvalidRequest
	}
}

func (h *grantHandlers) Check(ctx context.Context, body []byte) ([]byte, error) {
	return nil, nil
}

func (h *grantHandlers) Grant(ctx context.Context, body []byte) ([]byte, error) {
	logger := rmq.GetLogger(ctx)

	request := &entitlements.GrantRequest{}
	if err := json.Unmarshal(body, request); err != nil {
		logger.WithError(err).Error("failed to parse grant request")
		return nil, entitlements.ErrInvalidRequest
	}
	if !ids.ValidateOrderID(request.OrderID) {
		logger.WithField("order_id", request.OrderID).Error("order_id must be a valid ULID")
		return nil, entitlements.ErrInvalidRequest
	}

	entitlement, err := h.catalogManager.GetEntitlement(ctx, request.EntitlementID, request.Version)
	if err != nil {
		if errors.Is(err, managers.ErrEntitlementNotFound) {
			return nil, entitlements.ErrEntitlementNotFound
		}
		logger.WithError(err).Error("failed to get entitlement")
		return nil, entitlements.ErrFailedToGetEntitlement
	}
	grants := make([]trade.TradeGrant, 0, len(entitlement.GrantResults))
	for _, grant := range entitlement.GrantResults {
		grantType, err := catalogGrantTypeToTradeGrantType(grant.GrantType)
		if err != nil {
			logger.WithError(err).Error("unknown catalog grant type")
			return nil, entitlements.ErrInvalidRequest
		}
		grants = append(grants, trade.TradeGrant{
			PlayerID: request.PlayerID,
			Type:     grantType,
			TargetID: grant.TargetID,
			Amount:   grant.Amount,
		})
	}
	grant, err := h.grantsManager.CreateGrant(
		ctx, request.AccountID, request.EntitlementID,
		request.OrderID, request.Version)
	if err != nil {
		logger.WithError(err).Error("failed to record grant")
		return nil, entitlements.ErrFailedToGrantEntitlement
	}

	tradeResp, err := h.tradeClient.Trade(ctx, request.OrderID, "system", grants)
	if err != nil {
		logger.WithError(err).Error("failed to submit trade for grant delivery")
		return nil, entitlements.ErrFailedToGrantEntitlement
	}

	bts, err := json.Marshal(entitlements.GrantResponse{
		OrderID: tradeResp.OrderID,
		GrantID: grant.ID,
	})
	if err != nil {
		return nil, entitlements.ErrFailedToCreateResponse
	}
	return bts, nil
}

func (h *grantHandlers) Revoke(ctx context.Context, body []byte) ([]byte, error) {
	return nil, nil
}
