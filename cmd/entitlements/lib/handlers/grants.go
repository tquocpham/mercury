package handlers

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/mercury/cmd/entitlements/lib/managers"
	"github.com/mercury/pkg/clients/entitlements"
)

type GrantHandlers interface {
	Check(ctx context.Context, body []byte) ([]byte, error)
	Grant(ctx context.Context, body []byte) ([]byte, error)
	Revoke(ctx context.Context, body []byte) ([]byte, error)
}

type grantHandlers struct {
	grantsManager  managers.GrantsManager
	catalogManager managers.CatalogManager
}

func NewGrantHandlers(grantsManager managers.GrantsManager,
	catalogManager managers.CatalogManager) GrantHandlers {

	return &grantHandlers{
		grantsManager:  grantsManager,
		catalogManager: catalogManager,
	}
}

func (h *grantHandlers) Check(ctx context.Context, body []byte) ([]byte, error) {
	return nil, nil
}

func (h *grantHandlers) Grant(ctx context.Context, body []byte) ([]byte, error) {
	request := &entitlements.GrantRequest{}
	if err := json.Unmarshal(body, request); err != nil {
		return nil, entitlements.ErrInvalidRequest
	}

	entitlement, err := h.catalogManager.GetEntitlement(ctx, request.EntitlementID, request.Version)
	if err != nil {
		if errors.Is(err, managers.ErrEntitlementNotFound) {
			return nil, entitlements.ErrEntitlementNotFound
		}
		return nil, entitlements.ErrFailedToGetEntitlement
	}

	grant, err := h.grantsManager.Grant(ctx, request.AccountID, entitlement.EntitlementID, entitlement.Version, entitlement.Unique)
	if err != nil {
		if errors.Is(err, managers.ErrDuplicateGrant) {
			return nil, entitlements.ErrDuplicateGrant
		}
		return nil, entitlements.ErrFailedToGrantEntitlement
	}

	return json.Marshal(entitlements.GrantResponse{
		ID:            grant.ID,
		EntitlementID: grant.EntitlementID,
		CommitID:      grant.CommitID,
		Count:         grant.Count,
		AccountID:     grant.AccountID,
		State:         string(grant.State),
		GrantedAt:     grant.GrantedAt,
		RevokedAt:     grant.RevokedAt,
		ExpiresAt:     grant.ExpiresAt,
		Metadata:      grant.Metadata,
	})
}

func (h *grantHandlers) Revoke(ctx context.Context, body []byte) ([]byte, error) {
	return nil, nil
}
