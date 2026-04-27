package managers

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/mercury/pkg/clients/entitlements"
	"github.com/mercury/pkg/instrumentation"
	"github.com/smira/go-statsd"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

var ErrDuplicateEntitlement = errors.New("entitlementid already created")
var ErrEntitlementNotFound = errors.New("entitlement not found")

type CatalogManager interface {
	GetEntitlement(ctx context.Context, entitlementID string, version int) (_ *Entitlement, err error)
	CreateEntitlement(
		ctx context.Context,
		catalogItemID string,
		itemType entitlements.EntitlementType,
		category string,
		price entitlements.EntitlementPrice,
		unique bool,
		metadata map[string]any,
		gameProperties map[string]any,
		tags []string,
		behavior entitlements.CatalogItemBehavior,
		grantResults []entitlements.CatalogGrantResult,
		requirements []string,
	) (_ *Entitlement, err error)
}

type catalogManager struct {
	col *mongo.Collection
}

func NewCatalogManager(mongoAddr string) (CatalogManager, error) {
	client, err := mongo.Connect(options.Client().ApplyURI(mongoAddr))
	if err != nil {
		return nil, err
	}
	// mongo.Connect creates a connection pool managed by the driver.
	// The pool is created once at startup, reused across all requests
	col := client.Database("entitlements").Collection("catalog")

	return &catalogManager{
		col: col,
	}, nil
}

type EntitlementPrice struct {
	Amount   int    `bson:"amount"`
	Currency string `bson:"currency"`
}

type CatalogItemBehavior struct {
	IsTradable bool `bson:"is_tradable"`
	IsGiftable bool `bson:"is_giftable"`
	MaxStack   int  `bson:"max_stack"`
}

type CatalogGrantResult struct {
	GrantType entitlements.CatalogGrantType `bson:"grant_type"`
	TargetID  string                        `bson:"target_id"`
	Amount    int                           `bson:"amount"`
}

type Entitlement struct {
	CatalogItemID string `bson:"_id"` // Name is the entitlement name ID/SKU Unique identifier.
	Version       int    `bson:"version"`
	CommitID      string `bson:"commit_id"` // idempotency
	// Item Type (The System Logic) is a strict Enum.
	// It tells your database and your entitlement engine how to handle the item's lifecycle.
	// The code can branch based on this.
	// For example:
	// if (item.type == CONSUMABLE) { decrement_quantity() }
	ItemType entitlements.EntitlementType `bson:"item_type"`
	// Category (The Gameplay/UI Label) is a flexible string. It tells the game client and the shop
	// where to display the item. for example:
	//  "Cosmetic" for capes, hats, weapon glows.
	//  "Utility" for inventory expansions, name changes.
	//  "Gameplay" for weapons, armor, spells.
	//  "Booster" for XP multipliers, gold find increase.
	// Why it's separate: You can have a "Booster" that is a "Consumable" (use it once) or a
	// "Booster" that is a Durable (a permanent +5% XP passive).
	// The Item Type handles the logic; the Category handles the organization.
	Category string `bson:"category"`
	// Price is contains the price data
	Price EntitlementPrice `bson:"price"`
	// Unique is whether an account can own more than one
	Unique         bool                 `bson:"unique"`
	Metadata       map[string]any       `bson:"metadata,omitempty"`
	GameProperties map[string]any       `bson:"game_properties,omitempty"`
	Tags           []string             `bson:"tags"`
	Behavior       CatalogItemBehavior  `bson:"behavior"`
	GrantResults   []CatalogGrantResult `bson:"grant_results"`
	// Requirements is a list of strings. These strings should be tied to a
	// rules function that checks that the players game state matches the requirements before granting.
	// for example: "min_player_level_10" should map to a function which checks the player is at
	// least level 10 before this item can be granted
	Requirements []string `bson:"grant_requirements"`
}

func (u *catalogManager) CreateEntitlement(
	ctx context.Context,
	catalogItemID string,
	itemType entitlements.EntitlementType,
	category string,
	price entitlements.EntitlementPrice,
	unique bool,
	metadata map[string]any,
	gameProperties map[string]any,
	tags []string,
	behavior entitlements.CatalogItemBehavior,
	grantResults []entitlements.CatalogGrantResult,
	requirements []string,
) (_ *Entitlement, err error) {

	t := instrumentation.NewMetricsTimer(ctx, "entmgr.dur", statsd.StringTag("op", "create_entitlement"))
	defer func() { t.Done(err) }()

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	gResults := make([]CatalogGrantResult, 0, len(grantResults))
	for _, v := range grantResults {
		gResults = append(gResults, CatalogGrantResult{
			GrantType: v.GrantType,
			TargetID:  v.TargetID,
			Amount:    v.Amount,
		})
	}

	entitlement := &Entitlement{
		CatalogItemID: catalogItemID,
		CommitID:      uuid.New().String(),
		Version:       1,
		ItemType:      itemType,
		Category:      category,
		Price: EntitlementPrice{
			Amount:   price.Amount,
			Currency: price.Currency,
		},
		Unique:         unique,
		Metadata:       metadata,
		GameProperties: gameProperties,
		Tags:           tags,
		Behavior: CatalogItemBehavior{
			IsTradable: behavior.IsTradable,
			IsGiftable: behavior.IsGiftable,
			MaxStack:   behavior.MaxStack,
		},
		GrantResults: gResults,
		Requirements: requirements,
	}

	_, err = u.col.InsertOne(ctx, entitlement)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return nil, ErrDuplicateEntitlement
		}
		return nil, err
	}

	return entitlement, nil
}

func (u *catalogManager) GetEntitlement(ctx context.Context, entitlementID string, version int) (_ *Entitlement, err error) {

	t := instrumentation.NewMetricsTimer(ctx, "entmgr.dur", statsd.StringTag("op", "get_entitlement"))
	defer func() { t.Done(err) }()

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	filter := bson.M{
		"_id":     entitlementID,
		"version": version,
	}

	doc := &Entitlement{}
	if err := u.col.FindOne(ctx, filter).Decode(doc); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrEntitlementNotFound
		}
		return nil, err
	}

	return doc, nil
}
