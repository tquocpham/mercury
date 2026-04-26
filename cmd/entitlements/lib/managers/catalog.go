package managers

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
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
		ctx context.Context, name, description, category string, price int, currency string,
		unique bool, metadata map[string]any) (_ *Entitlement, err error)
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

type Entitlement struct {
	EntitlementID string           `bson:"_id"`
	Version       int              `bson:"version"`
	Name          string           `bson:"name"`
	Description   string           `bson:"description"`
	Category      string           `bson:"category"`
	Price         EntitlementPrice `bson:"price"`
	Unique        bool             `bson:"unique"`
	Metadata      map[string]any   `bson:"metadata,omitempty"`
}

func (u *catalogManager) CreateEntitlement(
	ctx context.Context, name, description, category string, price int, currency string,
	unique bool, metadata map[string]any) (_ *Entitlement, err error) {

	t := instrumentation.NewMetricsTimer(ctx, "entmgr.dur", statsd.StringTag("op", "create_entitlement"))
	defer func() { t.Done(err) }()

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	entitlement := &Entitlement{
		EntitlementID: uuid.New().String(),
		Version:       1,
		Name:          name,
		Description:   description,
		Category:      category,
		Metadata:      metadata,
		Unique:        unique,
		Price: EntitlementPrice{
			Amount:   price,
			Currency: currency,
		},
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
