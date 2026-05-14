package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/mercury/pkg/clients/inventory"
	"github.com/mercury/pkg/instrumentation"
)

type InventoryHandlers interface {
	CreateInventory(c echo.Context) error
	GetInventory(c echo.Context) error
	AddItem(c echo.Context) error
	AddItemToSlot(c echo.Context) error
}

type inventoryHandlers struct {
	inventoryClient inventory.RMQClient
}

func NewInventoryHandlers(inventoryClient inventory.RMQClient) InventoryHandlers {
	return &inventoryHandlers{inventoryClient: inventoryClient}
}

func (h *inventoryHandlers) CreateInventory(c echo.Context) error {
	ctx := instrumentation.ToContext(c)
	request := &inventory.GetInventoryRequest{}
	if err := json.NewDecoder(c.Request().Body).Decode(request); err != nil {
		return echo.ErrBadRequest
	}
	response, err := h.inventoryClient.CreateInventory(ctx, request.PlayerID)
	if err != nil {
		return inventory.ConvertHttpError(err)
	}
	return c.JSON(http.StatusOK, response)
}

func (h *inventoryHandlers) GetInventory(c echo.Context) error {
	ctx := instrumentation.ToContext(c)
	playerID := c.Param("playerid")
	response, err := h.inventoryClient.GetInventory(ctx, playerID)
	if err != nil {
		return inventory.ConvertHttpError(err)
	}
	return c.JSON(http.StatusOK, response)
}

func (h *inventoryHandlers) AddItem(c echo.Context) error {
	ctx := instrumentation.ToContext(c)
	request := &inventory.AddItemRequest{}
	if err := json.NewDecoder(c.Request().Body).Decode(request); err != nil {
		return echo.ErrBadRequest
	}
	response, err := h.inventoryClient.AddItem(ctx, request.PlayerID, request.ItemID, request.OrderID, request.Amount, request.MaxStack)
	if err != nil {
		return inventory.ConvertHttpError(err)
	}
	return c.JSON(http.StatusOK, response)
}

func (h *inventoryHandlers) AddItemToSlot(c echo.Context) error {
	ctx := instrumentation.ToContext(c)
	request := &inventory.AddItemToSlotRequest{}
	if err := json.NewDecoder(c.Request().Body).Decode(request); err != nil {
		return echo.ErrBadRequest
	}
	response, err := h.inventoryClient.AddItemToSlot(ctx, request.PlayerID, request.ItemID, request.OrderID, request.SlotID, request.Amount, request.MaxStack)
	if err != nil {
		return inventory.ConvertHttpError(err)
	}
	return c.JSON(http.StatusOK, response)
}
