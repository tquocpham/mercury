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

type GetInventoryResponse struct {
	PlayerID  string `json:"player_id"`
	Inventory []Item `json:"inventory"`
	CommitID  string `json:"commit_id"`
}

func (c *client) GetInventory() (*GetInventoryResponse, error) {
	return nil, nil
}

func (c *client) AddItem(playerID string, itemID string, amount int, orderID string) (*GetInventoryResponse, error) {
	return nil, nil
}
