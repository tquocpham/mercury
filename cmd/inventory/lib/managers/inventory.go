package managers

import (
	"context"
	"errors"
	"time"

	"github.com/mercury/pkg/instrumentation"
	"github.com/smira/go-statsd"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const (
	defaultUnlockedSlots = 20
)

var (
	ErrDuplicateOrder    = errors.New("duplicate order")
	ErrInventoryNotFound = errors.New("inventory not found")
	ErrInventoryFull     = errors.New("inventory full")
	ErrSlotNotAvailable  = errors.New("slot not available")
)

type InventorySlot struct {
	SlotID int    `bson:"slot_id"`
	ItemID string `bson:"item_id"` // empty string = empty slot
	Amount int    `bson:"amount"`
}

type Inventory struct {
	PlayerID      string          `bson:"player_id"`
	Slots         []InventorySlot `bson:"slots"`
	UnlockedSlots int             `bson:"unlocked_slots"`
	CreatedAt     time.Time       `bson:"created_at"`
	UpdatedAt     time.Time       `bson:"updated_at"`
}

type InventoryManager interface {
	GetInventory(ctx context.Context, playerID string) (*Inventory, error)
	CreateInventory(ctx context.Context, playerID string) (*Inventory, error)
	AddItem(ctx context.Context, playerID, itemID, orderID string, amount, maxStack int) (*Inventory, error)
	AddItemToSlot(ctx context.Context, playerID, itemID, orderID string, slotID, amount, maxStack int) (*Inventory, error)
	UnlockSlots(ctx context.Context, playerID string, count int) (*Inventory, error)
}

type inventoryManager struct {
	client          *mongo.Client
	inventory       *mongo.Collection
	processedOrders *mongo.Collection
	statsdClient    *statsd.Client
}

func NewInventoryManager(mongoAddr string, statsdClient *statsd.Client) (InventoryManager, error) {
	client, err := mongo.Connect(options.Client().ApplyURI(mongoAddr))
	if err != nil {
		return nil, err
	}
	db := client.Database("inventory")
	return &inventoryManager{
		client:          client,
		inventory:       db.Collection("inventory"),
		processedOrders: db.Collection("processed_orders"),
		statsdClient:    statsdClient,
	}, nil
}

func initialSlots(count int) []InventorySlot {
	slots := make([]InventorySlot, count)
	for i := range slots {
		slots[i] = InventorySlot{SlotID: i}
	}
	return slots
}

func (s *inventoryManager) GetInventory(ctx context.Context, playerID string) (_ *Inventory, err error) {
	t := instrumentation.NewMetricsTimer(ctx, "invmgr.dur", statsd.StringTag("op", "get_inventory"))
	defer func() { t.Done(err) }()

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var inv Inventory
	if err := s.inventory.FindOne(ctx, bson.M{"player_id": playerID}).Decode(&inv); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrInventoryNotFound
		}
		return nil, err
	}
	return &inv, nil
}

func (s *inventoryManager) CreateInventory(ctx context.Context, playerID string) (_ *Inventory, err error) {
	t := instrumentation.NewMetricsTimer(ctx, "invmgr.dur", statsd.StringTag("op", "create_inventory"))
	defer func() { t.Done(err) }()

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	now := time.Now()
	_, err = s.inventory.UpdateOne(ctx,
		bson.M{"player_id": playerID},
		bson.M{"$setOnInsert": bson.M{
			"player_id":      playerID,
			"slots":          initialSlots(defaultUnlockedSlots),
			"unlocked_slots": defaultUnlockedSlots,
			"created_at":     now,
			"updated_at":     now,
		}},
		options.UpdateOne().SetUpsert(true),
	)
	if err != nil {
		return nil, err
	}
	return s.GetInventory(ctx, playerID)
}

func (s *inventoryManager) AddItem(ctx context.Context, playerID, itemID, orderID string, amount, maxStack int) (_ *Inventory, err error) {
	t := instrumentation.NewMetricsTimer(ctx, "invmgr.dur", statsd.StringTag("op", "add_item"))
	defer func() { t.Done(err) }()

	session, err := s.client.StartSession()
	if err != nil {
		return nil, err
	}
	defer session.EndSession(ctx)

	var inv Inventory
	err = mongo.WithSession(ctx, session, func(sessCtx context.Context) error {
		sessCtx, cancel := context.WithTimeout(sessCtx, 10*time.Second)
		defer cancel()

		if err := session.StartTransaction(); err != nil {
			return err
		}

		_, err := s.processedOrders.InsertOne(sessCtx, bson.M{
			"_id":        orderID,
			"player_id":  playerID,
			"created_at": time.Now(),
		})
		if err != nil {
			session.AbortTransaction(sessCtx)
			if mongo.IsDuplicateKeyError(err) {
				return ErrDuplicateOrder
			}
			return err
		}

		// Try to add to an existing stack of the same item with room
		err = s.inventory.FindOneAndUpdate(sessCtx,
			bson.M{
				"player_id": playerID,
				"slots": bson.M{"$elemMatch": bson.M{
					"item_id": itemID,
					"amount":  bson.M{"$lte": maxStack - amount},
				}},
			},
			bson.M{
				"$inc": bson.M{"slots.$.amount": amount},
				"$set": bson.M{"updated_at": time.Now()},
			},
			options.FindOneAndUpdate().SetReturnDocument(options.After),
		).Decode(&inv)
		if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
			session.AbortTransaction(sessCtx)
			return err
		}

		// No existing stack with room — find the first empty slot
		if errors.Is(err, mongo.ErrNoDocuments) {
			err = s.inventory.FindOneAndUpdate(sessCtx,
				bson.M{
					"player_id": playerID,
					"slots":     bson.M{"$elemMatch": bson.M{"item_id": ""}},
				},
				bson.M{
					"$set": bson.M{
						"slots.$.item_id": itemID,
						"slots.$.amount":  amount,
						"updated_at":      time.Now(),
					},
				},
				options.FindOneAndUpdate().SetReturnDocument(options.After),
			).Decode(&inv)
			if errors.Is(err, mongo.ErrNoDocuments) {
				session.AbortTransaction(sessCtx)
				var check Inventory
				if cerr := s.inventory.FindOne(sessCtx, bson.M{"player_id": playerID}).Decode(&check); errors.Is(cerr, mongo.ErrNoDocuments) {
					return ErrInventoryNotFound
				}
				return ErrInventoryFull
			}
			if err != nil {
				session.AbortTransaction(sessCtx)
				return err
			}
		}

		return session.CommitTransaction(sessCtx)
	})
	if err != nil {
		if errors.Is(err, ErrDuplicateOrder) {
			return s.GetInventory(ctx, playerID)
		}
		return nil, err
	}
	return &inv, nil
}

func (s *inventoryManager) AddItemToSlot(ctx context.Context, playerID, itemID, orderID string, slotID, amount, maxStack int) (_ *Inventory, err error) {
	t := instrumentation.NewMetricsTimer(ctx, "invmgr.dur", statsd.StringTag("op", "add_item_to_slot"))
	defer func() { t.Done(err) }()

	session, err := s.client.StartSession()
	if err != nil {
		return nil, err
	}
	defer session.EndSession(ctx)

	var inv Inventory
	err = mongo.WithSession(ctx, session, func(sessCtx context.Context) error {
		sessCtx, cancel := context.WithTimeout(sessCtx, 10*time.Second)
		defer cancel()

		if err := session.StartTransaction(); err != nil {
			return err
		}

		_, err := s.processedOrders.InsertOne(sessCtx, bson.M{
			"_id":        orderID,
			"player_id":  playerID,
			"created_at": time.Now(),
		})
		if err != nil {
			session.AbortTransaction(sessCtx)
			if mongo.IsDuplicateKeyError(err) {
				return ErrDuplicateOrder
			}
			return err
		}

		// Slot must be empty or contain the same item with room
		err = s.inventory.FindOneAndUpdate(sessCtx,
			bson.M{
				"player_id": playerID,
				"slots": bson.M{"$elemMatch": bson.M{
					"slot_id": slotID,
					"$or": bson.A{
						bson.M{"item_id": ""},
						bson.M{"item_id": itemID, "amount": bson.M{"$lte": maxStack - amount}},
					},
				}},
			},
			bson.M{
				"$inc": bson.M{"slots.$.amount": amount},
				"$set": bson.M{
					"slots.$.item_id": itemID,
					"updated_at":      time.Now(),
				},
			},
			options.FindOneAndUpdate().SetReturnDocument(options.After),
		).Decode(&inv)
		if errors.Is(err, mongo.ErrNoDocuments) {
			session.AbortTransaction(sessCtx)
			return ErrSlotNotAvailable
		}
		if err != nil {
			session.AbortTransaction(sessCtx)
			return err
		}

		return session.CommitTransaction(sessCtx)
	})
	if err != nil {
		if errors.Is(err, ErrDuplicateOrder) {
			return s.GetInventory(ctx, playerID)
		}
		return nil, err
	}
	return &inv, nil
}

func (s *inventoryManager) UnlockSlots(ctx context.Context, playerID string, count int) (_ *Inventory, err error) {
	t := instrumentation.NewMetricsTimer(ctx, "invmgr.dur", statsd.StringTag("op", "unlock_slots"))
	defer func() { t.Done(err) }()

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	inv, err := s.GetInventory(ctx, playerID)
	if err != nil {
		return nil, err
	}

	if count == 0 {
		return inv, nil
	}

	newSlots := make([]InventorySlot, count)
	for i := range newSlots {
		newSlots[i] = InventorySlot{SlotID: inv.UnlockedSlots + i}
	}

	var updated Inventory
	err = s.inventory.FindOneAndUpdate(ctx,
		bson.M{"player_id": playerID},
		bson.M{
			"$push": bson.M{"slots": bson.M{"$each": newSlots}},
			"$inc":  bson.M{"unlocked_slots": count},
			"$set":  bson.M{"updated_at": time.Now()},
		},
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	).Decode(&updated)
	if err != nil {
		return nil, err
	}
	return &updated, nil
}
