package entitlements

import (
	"context"
	"time"

	"github.com/mercury/pkg/rmq"
)

type RMQClient interface {
	CreateItem(
		ctx context.Context, name string, description string, category string, cost int,
		currency string, unique bool, metadata map[string]any) (*CreateItemResponse, error)
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

type GrantRequest struct {
	AccountID     string    `json:"account_id"`
	EntitlementID string    `json:"entitlement_id"`
	Version       int       `json:"version"`
	ExpiresAt     time.Time `json:"expires_at,omitempty"`
}

type EntitlementPrice struct {
	// Amount is the cost amount
	Amount int `json:"amount"`
	// Currency is the type of currency the entitlement cost, "gold", "USD" etc.
	Currency string `json:"currency"`
}

type CreateItemRequest struct {
	// Description is the description of the entitlement.
	Description string `json:"description"`
	// Name is the entitlement name
	Name string `json:"name"`
	// Category is a string allowing creators to denote entitlement type such as "skin", "consumable", "modifier" etc.
	// Catalog categories provide a basic structure for organizing catalog items, assisting in organizing the inventory.
	// and providing a user-friendly shopping experience
	Category string `json:"type"`
	// Price is contains the price data
	Price EntitlementPrice `json:"price"`
	// Unique is whether an account can own more than one
	Unique   bool           `json:"unique"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

type CreateItemResponse struct {
	EntitlementID string `json:"entitlement_id"`
	Version       int    `json:"version"`
}

func (c *client) CreateItem(
	ctx context.Context, name string, description string, category string, cost int,
	currency string, unique bool, metadata map[string]any) (*CreateItemResponse, error) {

	return rmq.Request[CreateItemRequest, CreateItemResponse](ctx, c.Publisher, "cat.v1.additems", CreateItemRequest{
		Name:        name,
		Description: description,
		Category:    category,
		Price: EntitlementPrice{
			Amount:   cost,
			Currency: currency,
		},
		Unique:   unique,
		Metadata: metadata,
	})
}

type GrantResponse struct {
	ID            string                 `json:"id"`
	EntitlementID string                 `json:"entitlement_id"`
	CommitID      string                 `json:"commit_id"` // idempotency
	Count         int                    `json:"count"`
	AccountID     string                 `json:"account_id"`
	State         string                 `json:"state"`
	GrantedAt     time.Time              `json:"granted_at"`
	RevokedAt     time.Time              `json:"revoked_at"`
	ExpiresAt     time.Time              `json:"expires_at"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}
