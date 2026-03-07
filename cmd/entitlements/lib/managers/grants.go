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
		ctx context.Context, accountID, entitlementID, grantedBy string) (_ *Grant, err error)
	Update(ctx context.Context, accountID string, entitlementID string) (_ *Grant, err error)
}

type grantsManager struct {
	col *mongo.Collection
}

func NewGrantsManager(mongoAddr string) (GrantsManager, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(options.Client().ApplyURI(mongoAddr))
	if err != nil {
		return nil, err
	}
	// mongo.Connect creates a connection pool managed by the driver.
	// The pool is created once at startup, reused across all requests
	col := client.Database("entitlements").Collection("grants")

	// Unique indexes enforce no duplicate account or entitlement ids at the DB level.
	_, err = col.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "account_id", Value: 1},
				{Key: "entitlement_id", Value: 1},
			},
			Options: options.Index().SetUnique(true),
		},
	})
	if err != nil {
		return nil, err
	}

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
	ID            string                 `bson:"_id"`
	EntitlementID string                 `bson:"entitlement_id"`
	CommitID      string                 `bson:"commit_id"` // idempotency
	Count         int                    `bson:"count"`
	AccountID     string                 `bson:"account_id"`
	State         GrantState             `bson:"state"`
	GrantedBy     string                 `bson:"granted_by"`
	GrantedAt     time.Time              `bson:"granted_at"`
	RevokedAt     time.Time              `bson:"revoked_at"`
	ExpiresAt     time.Time              `bson:"expires_at"`
	Metadata      map[string]interface{} `bson:"metadata,omitempty"`
}

// GrantEntitlement activates a newly created account
func (u *grantsManager) Grant(
	ctx context.Context, accountID, entitlementID, grantedBy string) (_ *Grant, err error) {
	t := instrumentation.NewMetricsTimer(ctx, "grntmgr.dur", statsd.StringTag("op", "grant"))
	defer func() { t.Done(err) }()

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	// Design choice of loose coupling with accounts for now.
	// If accounts/auth service is down we don't want to block entitlments unnecsesarily.

	entitlement := &Grant{
		ID:            uuid.New().String(),
		EntitlementID: entitlementID,
		AccountID:     accountID,
		State:         EntitlementStateActive,
		GrantedBy:     grantedBy,
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

// UpdateEntitlement updates an existing entitlement
func (u *grantsManager) Update(ctx context.Context, accountID string, entitlementID string) (_ *Grant, err error) {
	t := instrumentation.NewMetricsTimer(ctx, "grntmgr.dur", statsd.StringTag("op", "update"))
	defer func() { t.Done(err) }()

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	return nil, nil
}

// CheckEntitlement activates a newly created account
func (u *grantsManager) Check(ctx context.Context, accountID string, entitlementID string) (_ *Grant, err error) {
	t := instrumentation.NewMetricsTimer(ctx, "grntmgr.dur", statsd.StringTag("op", "check"))
	defer func() { t.Done(err) }()

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	return nil, nil
}

// RevokeEntitlement activates a newly created account
func (u *grantsManager) Revoke(ctx context.Context, accountID string, entitlementID string) (_ *Grant, err error) {
	t := instrumentation.NewMetricsTimer(ctx, "grntmgr.dur", statsd.StringTag("op", "revoke"))
	defer func() { t.Done(err) }()

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	return nil, nil
}
