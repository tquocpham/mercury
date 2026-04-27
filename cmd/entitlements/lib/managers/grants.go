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

var ErrDuplicateGrant = errors.New("entitlementid and accountid already created")

type GrantsManager interface {
	CreateGrant(
		ctx context.Context, accountID, entitlementID, orderID string, entVersion int) (_ *Grant, err error)
	Update(ctx context.Context, accountID string, entitlementID string) (_ *Grant, err error)
}

type grantsManager struct {
	col *mongo.Collection
}

func NewGrantsManager(mongoAddr string) (GrantsManager, error) {
	client, err := mongo.Connect(options.Client().ApplyURI(mongoAddr))
	if err != nil {
		return nil, err
	}
	// mongo.Connect creates a connection pool managed by the driver.
	// The pool is created once at startup, reused across all requests
	col := client.Database("entitlements").Collection("grants")

	return &grantsManager{
		col: col,
	}, nil
}

type GrantState string

const (
	EntitlementStateActive  GrantState = "active"
	EntitlementStateRevoked GrantState = "revoked"
)

type Grant struct {
	ID                 string         `bson:"_id"`
	OrderID            string         `bson:"order_id"`
	EntitlementID      string         `bson:"entitlement_id"`
	EntitlementVersion int            `bson:"entitlement_version"`
	Count              int            `bson:"count"`
	AccountID          string         `bson:"account_id"`
	State              GrantState     `bson:"state"`
	GrantedAt          time.Time      `bson:"granted_at"`
	RevokedAt          time.Time      `bson:"revoked_at"`
	ExpiresAt          time.Time      `bson:"expires_at"`
	Metadata           map[string]any `bson:"metadata,omitempty"`
}

func (u *grantsManager) CreateGrant(
	ctx context.Context, accountID, entitlementID, orderID string,
	entVersion int,
) (_ *Grant, err error) {

	t := instrumentation.NewMetricsTimer(ctx, "grntmgr.dur", statsd.StringTag("op", "grant"))
	defer func() { t.Done(err) }()

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Atomic upsert — $setOnInsert only writes on insert, so concurrent retries
	// with the same orderID will all get back the same grant document.
	var grant Grant
	err = u.col.FindOneAndUpdate(ctx,
		bson.M{"order_id": orderID},
		bson.M{"$setOnInsert": bson.M{
			"_id":                 uuid.New().String(),
			"order_id":            orderID,
			"entitlement_id":      entitlementID,
			"entitlement_version": entVersion,
			"account_id":          accountID,
			"state":               EntitlementStateActive,
			"granted_at":          time.Now(),
		}},
		options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After),
	).Decode(&grant)
	if err != nil {
		return nil, err
	}

	return &grant, nil
}

func (u *grantsManager) Update(ctx context.Context, accountID string, entitlementID string) (_ *Grant, err error) {
	t := instrumentation.NewMetricsTimer(ctx, "grntmgr.dur", statsd.StringTag("op", "update"))
	defer func() { t.Done(err) }()

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	return nil, nil
}

func (u *grantsManager) Check(ctx context.Context, accountID string, entitlementID string) (_ *Grant, err error) {
	t := instrumentation.NewMetricsTimer(ctx, "grntmgr.dur", statsd.StringTag("op", "check"))
	defer func() { t.Done(err) }()

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	return nil, nil
}

func (u *grantsManager) Revoke(ctx context.Context, accountID string, entitlementID string) (_ *Grant, err error) {
	t := instrumentation.NewMetricsTimer(ctx, "grntmgr.dur", statsd.StringTag("op", "revoke"))
	defer func() { t.Done(err) }()

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	return nil, nil
}
