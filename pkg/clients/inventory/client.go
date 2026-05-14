package inventory

import (
	"context"

	"github.com/mercury/pkg/rmq"
)

type RMQClient interface {
	Close()
	CreateInventory(ctx context.Context, playerID string) (*GetInventoryResponse, error)
	GetInventory(ctx context.Context, playerID string) (*GetInventoryResponse, error)
	AddItem(ctx context.Context, playerID, itemID, orderID string, amount, maxStack int) (*GetInventoryResponse, error)
	AddItemToSlot(ctx context.Context, playerID, itemID, orderID string, slotID, amount, maxStack int) (*GetInventoryResponse, error)
}

type client struct {
	publisher *rmq.Publisher
}

func NewClient(amqpURL string) (RMQClient, error) {
	publisher, err := rmq.NewPublisher(amqpURL)
	if err != nil {
		return nil, err
	}
	return &client{publisher: publisher}, nil
}

func (c *client) Close() {
	c.publisher.Close()
}

type Item struct {
	ItemID string `json:"item_id"`
	Amount int    `json:"amount"`
}

type GetInventoryRequest struct {
	PlayerID string `json:"player_id"`
}

type AddItemRequest struct {
	PlayerID string `json:"player_id"`
	ItemID   string `json:"item_id"`
	Amount   int    `json:"amount"`
	MaxStack int    `json:"max_stack"`
	OrderID  string `json:"order_id"`
}

type AddItemToSlotRequest struct {
	PlayerID string `json:"player_id"`
	ItemID   string `json:"item_id"`
	SlotID   int    `json:"slot_id"`
	Amount   int    `json:"amount"`
	MaxStack int    `json:"max_stack"`
	OrderID  string `json:"order_id"`
}

type GetInventoryResponse struct {
	PlayerID  string `json:"player_id"`
	Inventory []Item `json:"inventory"`
}

func (c *client) CreateInventory(ctx context.Context, playerID string) (*GetInventoryResponse, error) {
	return rmq.Request[GetInventoryRequest, GetInventoryResponse](ctx, c.publisher, "inventory.v1.createinventory", GetInventoryRequest{
		PlayerID: playerID,
	})
}

func (c *client) GetInventory(ctx context.Context, playerID string) (*GetInventoryResponse, error) {
	return rmq.Request[GetInventoryRequest, GetInventoryResponse](ctx, c.publisher, "inventory.v1.getinventory", GetInventoryRequest{
		PlayerID: playerID,
	})
}

func (c *client) AddItem(ctx context.Context, playerID, itemID, orderID string, amount, maxStack int) (*GetInventoryResponse, error) {
	return rmq.Request[AddItemRequest, GetInventoryResponse](ctx, c.publisher, "inventory.v1.additem", AddItemRequest{
		PlayerID: playerID,
		ItemID:   itemID,
		Amount:   amount,
		MaxStack: maxStack,
		OrderID:  orderID,
	})
}

func (c *client) AddItemToSlot(ctx context.Context, playerID, itemID, orderID string, slotID, amount, maxStack int) (*GetInventoryResponse, error) {
	return rmq.Request[AddItemToSlotRequest, GetInventoryResponse](ctx, c.publisher, "inventory.v1.additemtoslot", AddItemToSlotRequest{
		PlayerID: playerID,
		ItemID:   itemID,
		SlotID:   slotID,
		Amount:   amount,
		MaxStack: maxStack,
		OrderID:  orderID,
	})
}
