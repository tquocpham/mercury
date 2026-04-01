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
	Grant(
		ctx context.Context, accountID, entitlementID string, entVersion int, unique bool) (_ *Grant, err error)
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
	ID                 string                 `bson:"_id"`
	EntitlementID      string                 `bson:"entitlement_id"`
	EntitlementVersion int                    `bson:"entitlement_version"`
	CommitID           string                 `bson:"commit_id"` // idempotency
	Count              int                    `bson:"count"`
	AccountID          string                 `bson:"account_id"`
	State              GrantState             `bson:"state"`
	GrantedAt          time.Time              `bson:"granted_at"`
	RevokedAt          time.Time              `bson:"revoked_at"`
	ExpiresAt          time.Time              `bson:"expires_at"`
	Metadata           map[string]any         `bson:"metadata,omitempty"`
}

func (u *grantsManager) Grant(
	ctx context.Context, accountID, entitlementID string, entVersion int, unique bool) (_ *Grant, err error) {
	t := instrumentation.NewMetricsTimer(ctx, "grntmgr.dur", statsd.StringTag("op", "grant"))
	defer func() { t.Done(err) }()

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Design choice of loose coupling with accounts for now.
	// If accounts/auth service is down we don't want to block entitlements unnecessarily.

	if unique {
		var existing Grant
		findErr := u.col.FindOne(ctx, bson.M{
			"account_id":     accountID,
			"entitlement_id": entitlementID,
		}).Decode(&existing)
		if findErr == nil {
			return nil, ErrDuplicateGrant
		}
		if !errors.Is(findErr, mongo.ErrNoDocuments) {
			return nil, findErr
		}
	}

	grant := &Grant{
		ID:                 uuid.New().String(),
		EntitlementID:      entitlementID,
		EntitlementVersion: entVersion,
		AccountID:          accountID,
		State:              EntitlementStateActive,
	}

	_, err = u.col.InsertOne(ctx, grant)
	if err != nil {
		return nil, err
	}

	return grant, nil
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
