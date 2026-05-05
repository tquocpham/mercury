package inventory

import "github.com/mercury/pkg/rmq"

type RMQClient interface {
	GetInventory() (*GetInventoryResponse, error)
	AddItem(playerID string, itemID string, amount int, orderID string) (*GetInventoryResponse, error)
}
type client struct {
	Publisher *rmq.Publisher
}

// NewClient creates a new query client
func NewClient(amqpURL string) (RMQClient, error) {
	publisher, err := rmq.NewPublisher(amqpURL)
	if err != nil {
		return nil, err
	}
	return &client{
		Publisher: publisher,
	}, nil
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

func (c *client) GetInventory() (*GetInventoryResponse, error) {
	return nil, nil
}

func (c *client) AddItem(playerID string, itemID string, amount int, orderID string) (*GetInventoryResponse, error) {
	return nil, nil
}
