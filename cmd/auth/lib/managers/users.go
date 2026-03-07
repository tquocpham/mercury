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

type UsersManager interface {
	// GetUser looks up a user by username or email.
	GetUserByUsername(ctx context.Context, email string) (_ *UserInformation, err error)
	AddUser(username, email, password string, roles []auth.Role) error
}

type usersManager struct {
	col *mongo.Collection
}

// UserInformation is the in-memory representation returned to callers.
type UserInformation struct {
	ID       string
	Username string
	Email    string
	Password []byte
	Salt     []byte
	Roles    []auth.Role
}

// userDocument is the MongoDB storage representation.
type userDocument struct {
	ID       string      `bson:"_id"`
	Username string      `bson:"username"`
	Email    string      `bson:"email"`
	Password []byte      `bson:"password"`
	Salt     []byte      `bson:"salt"`
	Roles    []auth.Role `bson:"roles"`
}

func NewUsersManager(mongoAddr string) (UsersManager, error) {
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

	return &usersManager{col: col}, nil
}

// GetUser finds a user by username or email.
func (u *usersManager) GetUserByUsername(ctx context.Context, username string) (_ *UserInformation, err error) {
	t := instrumentation.NewMetricsTimer(ctx, "usrmgr.dur", statsd.StringTag("op", "get_user_by_un"))
	defer func() { t.Done(err) }()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	filter := bson.M{"$or": bson.A{
		bson.M{"username": username},
	}}

	var doc userDocument
	if err := u.col.FindOne(ctx, filter).Decode(&doc); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, errors.New("user not found")
		}
		return nil, err
	}

	return &UserInformation{
		ID:       doc.ID,
		Username: doc.Username,
		Email:    doc.Email,
		Password: doc.Password,
		Salt:     doc.Salt,
		Roles:    doc.Roles,
	}, nil
}

// AddUser creates a new user with a hashed password and a generated UUID.
func (u *usersManager) AddUser(username, email, password string, roles []auth.Role) error {
	salt, err := hash.GenerateSalt(14)
	if err != nil {
		return err
	}

	pwhash, err := hash.Hash(password, salt)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = u.col.InsertOne(ctx, userDocument{
		ID:       uuid.New().String(),
		Username: username,
		Email:    email,
		Password: pwhash,
		Salt:     salt,
		Roles:    roles,
	})
	return err
}
