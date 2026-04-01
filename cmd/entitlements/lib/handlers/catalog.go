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
	request := &entitlements.CreateEntitlementRequest{}
	if err := json.Unmarshal(body, request); err != nil {
		return nil, entitlements.ErrInvalidRequest
	}
	h.catalogManager.CreateEntitlement(ctx, request.Name, request.Description, request.Category, request.Price.Amount, request.Price.Currency, request.Metadata)
	return nil, nil
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
