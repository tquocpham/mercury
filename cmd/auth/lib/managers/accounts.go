package managers

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/mercury/cmd/auth/lib/hash"
	"github.com/mercury/pkg/clients/auth"
	"github.com/mercury/pkg/instrumentation"
	"github.com/smira/go-statsd"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

var (
	ErrDuplicateAccount = errors.New("username or email already taken")
	ErrAccountNotFound  = errors.New("account not found")
)

type AccountsManager interface {
	GetAccountByUsername(ctx context.Context, username string) (_ *AccountInformation, err error)
	CreateAccount(ctx context.Context, username, email, password string, roles []auth.Role) (_ *AccountInformation, err error)
	ActivateAccount(ctx context.Context, accountID string) (err error)
}

type accountsManager struct {
	col *mongo.Collection
}

// AccountInformation is the in-memory representation returned to callers.
type AccountInformation struct {
	ID       string
	Username string
	Email    string
	Password []byte
	Salt     []byte
	Roles    []auth.Role
}

// accountDocument is the MongoDB storage representation.
type accountDocument struct {
	ID       string      `bson:"_id"`
	Username string      `bson:"username"`
	Email    string      `bson:"email"`
	Password []byte      `bson:"password"`
	Salt     []byte      `bson:"salt"`
	Roles    []auth.Role `bson:"roles"`
	State    string      `bson:"state"`
	Expiry   time.Time   `bson:"expiry"`
}

func NewAccountsManager(mongoAddr string) (AccountsManager, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(options.Client().ApplyURI(mongoAddr))
	if err != nil {
		return nil, err
	}
	// mongo.Connect creates a connection pool managed by the driver.
	// The pool is created once at startup, reused across all requests
	col := client.Database("auth").Collection("users")

	// Unique indexes enforce no duplicate usernames or emails at the DB level.
	_, err = col.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "username", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys:    bson.D{{Key: "email", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
	})
	if err != nil {
		return nil, err
	}

	return &accountsManager{col: col}, nil
}

// GetUser finds a account by username.
func (u *accountsManager) GetAccountByUsername(ctx context.Context, username string) (_ *AccountInformation, err error) {
	t := instrumentation.NewMetricsTimer(ctx, "usrmgr.dur", statsd.StringTag("op", "get_user_by_un"))
	defer func() { t.Done(err) }()
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	filter := bson.M{
		"username": username,
		"state": bson.M{
			"$ne": "pending",
		},
	}

	var doc accountDocument
	if err := u.col.FindOne(ctx, filter).Decode(&doc); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrAccountNotFound
		}
		return nil, err
	}

	return &AccountInformation{
		ID:       doc.ID,
		Username: doc.Username,
		Email:    doc.Email,
		Password: doc.Password,
		Salt:     doc.Salt,
		Roles:    doc.Roles,
	}, nil
}

// AddUser creates a new account with a hashed password and a generated UUID.
func (u *accountsManager) CreateAccount(
	ctx context.Context, username, email, password string, roles []auth.Role) (_ *AccountInformation, err error) {

	t := instrumentation.NewMetricsTimer(ctx, "usrmgr.dur", statsd.StringTag("op", "activate_user"))
	defer func() { t.Done(err) }()

	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	salt, err := hash.GenerateSalt(14)
	if err != nil {
		return nil, err
	}

	pwhash, err := hash.Hash(string(password), salt)
	if err != nil {
		return nil, err
	}

	accountID := uuid.New().String()

	_, err = u.col.InsertOne(ctx, accountDocument{
		ID:       accountID,
		Username: username,
		Email:    email,
		Password: pwhash,
		Salt:     salt,
		Roles:    roles,
		State:    "pending",
		Expiry:   time.Now().Add(1 * time.Hour),
	})
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return nil, ErrDuplicateAccount
		}
		return nil, err
	}

	return &AccountInformation{
		ID:       accountID,
		Username: username,
		Email:    email,
		Password: pwhash,
		Salt:     salt,
		Roles:    roles,
	}, nil
}

// ActivateAccount activates a newly created account
func (u *accountsManager) ActivateAccount(ctx context.Context, accountID string) (err error) {
	t := instrumentation.NewMetricsTimer(ctx, "usrmgr.dur", statsd.StringTag("op", "activate_user"))
	defer func() { t.Done(err) }()

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	filter := bson.M{
		"_id":    accountID,
		"state":  bson.M{"$eq": "pending"},
		"expiry": bson.M{"$gt": time.Now()},
	}

	u.col.FindOneAndUpdate(ctx,
		filter,
		bson.M{
			"$set": bson.M{
				"state": "active",
			},
		},
		options.FindOneAndUpdate(),
	)

	return nil
}
