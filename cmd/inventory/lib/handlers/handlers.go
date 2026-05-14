package handlers

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/mercury/cmd/inventory/lib/managers"
	"github.com/mercury/pkg/clients/inventory"
	"github.com/mercury/pkg/rmq"
)

type RMQHandlers interface {
	CreateInventory(ctx context.Context, body []byte) ([]byte, error)
	GetInventory(ctx context.Context, body []byte) ([]byte, error)
	AddItem(ctx context.Context, body []byte) ([]byte, error)
	AddItemToSlot(ctx context.Context, body []byte) ([]byte, error)
}

type rmqHanders struct {
	inventoryManager managers.InventoryManager
}

func NewRMQHandlers(inventoryManager managers.InventoryManager) RMQHandlers {
	return &rmqHanders{
		inventoryManager: inventoryManager,
	}
}

func slotsToItems(inv *managers.Inventory) []inventory.Item {
	items := make([]inventory.Item, 0, len(inv.Slots))
	for _, slot := range inv.Slots {
		if slot.ItemID != "" {
			items = append(items, inventory.Item{ItemID: slot.ItemID, Amount: slot.Amount})
		}
	}
	return items
}

func (h *rmqHanders) CreateInventory(ctx context.Context, body []byte) ([]byte, error) {
	logger := rmq.GetLogger(ctx)
	request := &inventory.GetInventoryRequest{}
	if err := json.Unmarshal(body, request); err != nil {
		logger.WithError(err).Error("failed to parse create inventory request")
		return nil, inventory.ErrInvalidRequest
	}

	inv, err := h.inventoryManager.CreateInventory(ctx, request.PlayerID)
	if err != nil {
		return nil, inventory.ErrFailedToGetInventory
	}

	bts, err := json.Marshal(inventory.GetInventoryResponse{
		PlayerID:  inv.PlayerID,
		Inventory: slotsToItems(inv),
	})
	if err != nil {
		return nil, inventory.ErrFailedToCreateResponse
	}
	return bts, nil
}

func (h *rmqHanders) GetInventory(ctx context.Context, body []byte) ([]byte, error) {
	logger := rmq.GetLogger(ctx)
	request := &inventory.GetInventoryRequest{}
	if err := json.Unmarshal(body, request); err != nil {
		logger.WithError(err).Error("failed to parse get inventory request")
		return nil, inventory.ErrInvalidRequest
	}

	inv, err := h.inventoryManager.GetInventory(ctx, request.PlayerID)
	if err != nil {
		if errors.Is(err, managers.ErrInventoryNotFound) {
			return nil, inventory.ErrInventoryDoesNotExist
		}
		return nil, inventory.ErrFailedToGetInventory
	}

	bts, err := json.Marshal(inventory.GetInventoryResponse{
		PlayerID:  inv.PlayerID,
		Inventory: slotsToItems(inv),
	})
	if err != nil {
		return nil, inventory.ErrFailedToCreateResponse
	}
	return bts, nil
}

func (h *rmqHanders) AddItem(ctx context.Context, body []byte) ([]byte, error) {
	logger := rmq.GetLogger(ctx)
	request := &inventory.AddItemRequest{}
	if err := json.Unmarshal(body, request); err != nil {
		logger.WithError(err).Error("failed to parse add item request")
		return nil, inventory.ErrInvalidRequest
	}

	inv, err := h.inventoryManager.AddItem(ctx, request.PlayerID, request.ItemID, request.OrderID, request.Amount, request.MaxStack)
	if err != nil {
		if errors.Is(err, managers.ErrInventoryNotFound) {
			return nil, inventory.ErrInventoryDoesNotExist
		}
		if errors.Is(err, managers.ErrInventoryFull) {
			return nil, inventory.ErrInventoryFull
		}
		return nil, inventory.ErrFailedToAddItem
	}

	bts, err := json.Marshal(inventory.GetInventoryResponse{
		PlayerID:  inv.PlayerID,
		Inventory: slotsToItems(inv),
	})
	if err != nil {
		return nil, inventory.ErrFailedToCreateResponse
	}
	return bts, nil
}

func (h *rmqHanders) AddItemToSlot(ctx context.Context, body []byte) ([]byte, error) {
	logger := rmq.GetLogger(ctx)
	request := &inventory.AddItemToSlotRequest{}
	if err := json.Unmarshal(body, request); err != nil {
		logger.WithError(err).Error("failed to parse add item to slot request")
		return nil, inventory.ErrInvalidRequest
	}

	inv, err := h.inventoryManager.AddItemToSlot(ctx, request.PlayerID, request.ItemID, request.OrderID, request.SlotID, request.Amount, request.MaxStack)
	if err != nil {
		if errors.Is(err, managers.ErrSlotNotAvailable) {
			return nil, inventory.ErrSlotNotAvailable
		}
		return nil, inventory.ErrFailedToAddItem
	}

	bts, err := json.Marshal(inventory.GetInventoryResponse{
		PlayerID:  inv.PlayerID,
		Inventory: slotsToItems(inv),
	})
	if err != nil {
		return nil, inventory.ErrFailedToCreateResponse
	}
	return bts, nil
}
