package handlers

import (
	"context"
	"encoding/json"

	"github.com/mercury/cmd/entitlements/lib/managers"
	"github.com/mercury/pkg/clients/entitlements"
)

type CatalogHandlers interface {
	AddItems(ctx context.Context, body []byte) ([]byte, error)
	UpdateItems(ctx context.Context, body []byte) ([]byte, error)
	GetItems(ctx context.Context, body []byte) ([]byte, error)
	ArchiveItems(ctx context.Context, body []byte) ([]byte, error)
}

type catalogHandlers struct {
	catalogManager managers.CatalogManager
}

func NewCatalogHandlers(catalogManager managers.CatalogManager) CatalogHandlers {
	return &catalogHandlers{
		catalogManager: catalogManager,
	}
}

// AddItem adds an item to the catalog
func (h *catalogHandlers) AddItems(ctx context.Context, body []byte) ([]byte, error) {
	request := &entitlements.CreateItemRequest{}
	if err := json.Unmarshal(body, request); err != nil {
		return nil, entitlements.ErrInvalidRequest
	}
	grantResults := make([]managers.CatalogGrantResult, len(request.Item.GrantResults))
	for i, gr := range request.Item.GrantResults {
		grantResults[i] = managers.CatalogGrantResult{
			GrantType: gr.GrantType,
			TargetID:  gr.TargetID,
			Amount:    gr.Amount,
		}
	}
	behavior := managers.CatalogItemBehavior{
		IsTradable: request.Item.Behavior.IsTradable,
		IsGiftable: request.Item.Behavior.IsGiftable,
		MaxStack:   request.Item.Behavior.MaxStack,
	}
	entitlement, err := h.catalogManager.CreateEntitlement(
		ctx,
		request.Item.CatalogItemID,
		request.Item.ItemType,
		request.Item.Category,
		request.Item.Price,
		request.Item.Unique,
		request.Item.Metadata,
		request.Item.GameProperties,
		request.Item.Tags,
		behavior,
		grantResults,
		request.Item.Requirements,
	)
	if err != nil {
		return nil, entitlements.ErrFailedToCreateEntitlement
	}
	bts, err := json.Marshal(entitlements.CreateItemResponse{
		Version:  entitlement.Version,
		CommitID: entitlement.CommitID,
		Item:     request.Item,
	})
	if err != nil {
		return nil, entitlements.ErrFailedToCreateResponse
	}
	return bts, nil
}

// AddItem adds an item to the catalog
func (h *catalogHandlers) UpdateItems(ctx context.Context, body []byte) ([]byte, error) {
	return nil, nil
}

// RetrieveItems gets catalog items
func (h *catalogHandlers) GetItems(ctx context.Context, body []byte) ([]byte, error) {
	return nil, nil
}

// ArchiveItems removes an item from current catalog
func (h *catalogHandlers) ArchiveItems(ctx context.Context, body []byte) ([]byte, error) {
	return nil, nil
}
