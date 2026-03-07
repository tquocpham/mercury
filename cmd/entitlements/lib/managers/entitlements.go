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

type EntitlementsManager interface {
	GetEntitlement(ctx context.Context, entitlementID string) (_ *Entitlement, err error)
}

type entitlementsManager struct {
	col *mongo.Collection
}

func NewentitlementsManager(mongoAddr string) (EntitlementsManager, error) {
	client, err := mongo.Connect(options.Client().ApplyURI(mongoAddr))
	if err != nil {
		return nil, err
	}
	// mongo.Connect creates a connection pool managed by the driver.
	// The pool is created once at startup, reused across all requests
	col := client.Database("entitlements").Collection("catalog")

	return &entitlementsManager{
		col: col,
	}, nil
}

type Entitlement struct {
	EntitlementID string                 `bson:"_id"`
	CommitID      string                 `bson:"commit_id"` // idempotency for updates
	Feature       string                 `bson:"feature"`
	Description   string                 `bson:"description"`
	ExpiresAt     time.Time              `bson:"expires_at,omitempty"`
	Metadata      map[string]interface{} `bson:"metadata,omitempty"`
}

func (u *entitlementsManager) CreateEntitlement(
	ctx context.Context, id, feature, description string, expiresAt time.Time,
	metadata map[string]any) (_ *Entitlement, err error) {

	t := instrumentation.NewMetricsTimer(ctx, "entmgr.dur", statsd.StringTag("op", "create_entitlement"))
	defer func() { t.Done(err) }()

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	entitlement := &Entitlement{
		EntitlementID: id,
		CommitID:      uuid.New().String(),
		Feature:       feature,
		Description:   description,
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

func (u *entitlementsManager) GetEntitlement(ctx context.Context, entitlementID string) (_ *Entitlement, err error) {

	t := instrumentation.NewMetricsTimer(ctx, "entmgr.dur", statsd.StringTag("op", "get_entitlement"))
	defer func() { t.Done(err) }()

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	filter := bson.M{
		"_id": entitlementID,
		"$or": bson.A{
			bson.M{"expires_at": bson.M{"$exists": false}},
			bson.M{"expires_at": bson.M{"$gt": time.Now()}},
		},
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
