package entitlements

import (
	"context"

	"github.com/mercury/pkg/rmq"
)

// EntitlementType defines the type of an entitlement.
type EntitlementType string

const (
	// Durable: Items that persist forever (Skins, DLC, Characters).
	CategoryDurable EntitlementType = "durable"
	// Consumable: Items that are used up (Potions, Revives, Loot Boxes).
	CategoryConsumable EntitlementType = "consumable"
	// Currency: Values that increment a balance (Gold, Gems, Seasonal Tokens).
	CategoryCurrency EntitlementType = "currency"
	// Bundle: A "container" that grants multiple other items upon purchase.
	CategoryBundle EntitlementType = "bundle"
	// Subscription: Items with an expiration date that require renewal.
	CategorySubscription EntitlementType = "subscription"
)

type CatalogGrantType string

const (
	CatalogGrantTypeEntitlement CatalogGrantType = "ENTITLEMENT"
	CatalogGrantTypeCurrency    CatalogGrantType = "CURRENCY"
)

type RMQClient interface {
	Close()
	GrantEntitlement(
		ctx context.Context,
		accountID, playerID, entitlementID, orderID string,
		version int,
	) (*GrantResponse, error)
	CreateItem(
		ctx context.Context,
		catalogItemID string,
		itemType EntitlementType,
		category string,
		price EntitlementPrice,
		unique bool,
		metadata map[string]any,
		gameProperties map[string]any,
		tags []string,
		behavior CatalogItemBehavior,
		grantResults []CatalogGrantResult,
		requirements []string,
	) (*CreateItemResponse, error)
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

func (c *client) Close() {
	c.Publisher.Close()
}

type GrantRequest struct {
	AccountID     string `json:"account_id"`
	PlayerID      string `json:"player_id"`
	EntitlementID string `json:"entitlement_id"`
	OrderID       string `json:"order_id"`
	Version       int    `json:"version"`
}

type GrantResponse struct {
	OrderID string `json:"order_id"`
	GrantID string `json:"grant_id"`
}

func (c *client) GrantEntitlement(
	ctx context.Context,
	accountID, playerID, entitlementID, orderID string,
	version int,
) (*GrantResponse, error) {

	return rmq.Request[GrantRequest, GrantResponse](ctx, c.Publisher, "ent.v1.grant", GrantRequest{
		AccountID:     accountID,
		PlayerID:      playerID,
		EntitlementID: entitlementID,
		OrderID:       orderID,
		Version:       version,
	})
}

type EntitlementPrice struct {
	// Amount is the cost amount
	Amount int `json:"amount"`
	// Currency is the type of currency the entitlement cost, "gold", "USD" etc.
	Currency string `json:"currency"`
}

type CatalogItemBehavior struct {
	IsTradable bool `json:"is_tradable"`
	IsGiftable bool `json:"is_giftable"`
	MaxStack   int  `json:"max_stack"`
}

type CatalogGrantResult struct {
	GrantType CatalogGrantType `json:"grant_type"`
	TargetID  string           `json:"target_id"`
	Amount    int              `json:"amount"`
}

type CatalogItem struct {
	// Name is the entitlement name ID/SKU Unique identifier.
	CatalogItemID string `json:"catalog_item_id"`
	// Item Type (The System Logic) is a strict Enum.
	// It tells your database and your entitlement engine how to handle the item's lifecycle.
	// The code can branch based on this.
	// For example:
	// if (item.type == CONSUMABLE) { decrement_quantity() }
	ItemType EntitlementType `json:"item_type"`
	// Category (The Gameplay/UI Label) is a flexible string. It tells the game client and the shop
	// where to display the item. for example:
	//  "Cosmetic" for capes, hats, weapon glows.
	//  "Utility" for inventory expansions, name changes.
	//  "Gameplay" for weapons, armor, spells.
	//  "Booster" for XP multipliers, gold find increase.
	// Why it's separate: You can have a "Booster" that is a "Consumable" (use it once) or a
	// "Booster" that is a Durable (a permanent +5% XP passive).
	// The Item Type handles the logic; the Category handles the organization.
	Category string `json:"category"`
	// Price is contains the price data
	Price EntitlementPrice `json:"price"`
	// Unique is whether an account can own more than one
	Unique         bool                 `json:"unique"`
	Metadata       map[string]any       `json:"metadata,omitempty"`
	GameProperties map[string]any       `json:"game_properties,omitempty"`
	Tags           []string             `json:"tags"`
	Behavior       CatalogItemBehavior  `json:"behavior"`
	GrantResults   []CatalogGrantResult `json:"grant_results"`
	// Requirements is a list of strings. These strings should be tied to a
	// rules function that checks that the players game state matches the requirements before granting.
	// for example: "min_player_level_10" should map to a function which checks the player is at
	// least level 10 before this item can be granted
	Requirements []string `json:"grant_requirements"`
}

type CreateEntitlementRequest = CreateItemRequest

type CreateItemRequest struct {
	Item CatalogItem `json:"catalog_item"`
}

type CreateItemResponse struct {
	Item     CatalogItem `json:"catalog_item"`
	CommitID string      `json:"commit_id"`
	Version  int         `json:"version"`
}

func (c *client) CreateItem(
	ctx context.Context,
	catalogItemID string,
	itemType EntitlementType,
	category string,
	price EntitlementPrice,
	unique bool,
	metadata map[string]any,
	gameProperties map[string]any,
	tags []string,
	behavior CatalogItemBehavior,
	grantResults []CatalogGrantResult,
	requirements []string,
) (*CreateItemResponse, error) {

	return rmq.Request[CreateItemRequest, CreateItemResponse](ctx, c.Publisher, "cat.v1.additems", CreateItemRequest{
		Item: CatalogItem{
			CatalogItemID:  catalogItemID,
			ItemType:       itemType,
			Category:       category,
			Price:          price,
			Unique:         unique,
			Metadata:       metadata,
			GameProperties: gameProperties,
			Tags:           tags,
			Behavior:       behavior,
			GrantResults:   grantResults,
			Requirements:   requirements,
		},
	})
}
