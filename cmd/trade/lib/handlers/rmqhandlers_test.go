package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/mercury/cmd/trade/lib/managers"
	"github.com/mercury/pkg/clients/trade"
	"github.com/mercury/pkg/ids"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type mockOutboxManager struct {
	statusEvent *trade.OutboxEvent
	statusErr   error
	insertErr   error
	updateEvent *trade.OutboxEvent
	updateErr   error
	lockEvent   *trade.OutboxEvent
	lockErr     error
	unlockEvent *trade.OutboxEvent
	unlockErr   error
	createErr   error
}

func (m *mockOutboxManager) GetOutboxStatus(_ context.Context, _ string) (*trade.OutboxEvent, error) {
	return m.statusEvent, m.statusErr
}

func (m *mockOutboxManager) InsertTrade(_ context.Context, _ string, _ *trade.OutboxEvent) error {
	return m.insertErr
}

func (m *mockOutboxManager) UpdateTradeGrants(_ context.Context, _, _, _ string, _ []trade.GrantItem) (*trade.OutboxEvent, error) {
	return m.updateEvent, m.updateErr
}

func (m *mockOutboxManager) LockTrade(_ context.Context, _, _, _ string) (*trade.OutboxEvent, error) {
	return m.lockEvent, m.lockErr
}

func (m *mockOutboxManager) UnlockTrade(_ context.Context, _, _, _ string) (*trade.OutboxEvent, error) {
	return m.unlockEvent, m.unlockErr
}

func (m *mockOutboxManager) CreateOutbox(_ context.Context, _, _ string, _ []trade.GrantItem) error {
	return m.createErr
}

func newHandlers(mgr managers.OutboxManager) RMQHandlers {
	return NewRMQHandlers(mgr)
}

func marshalJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return b
}

func validOrderID() string { return ids.NewOrderID() }

func stubEvent(orderID string) *trade.OutboxEvent {
	return &trade.OutboxEvent{
		ID:                 primitive.NewObjectID(),
		OrderID:            orderID,
		InitiatorID:        "init-1",
		ContractingParties: []string{"player-1", "player-2"},
		Signatures:         []string{},
		CommitID:           "commit-abc",
		Status:             trade.OutboxStatusDraft,
		GrantsByPlayer:     map[string][]trade.GrantItem{},
	}
}

func TestDraftTrade_invalidJSON(t *testing.T) {
	_, err := newHandlers(&mockOutboxManager{}).DraftTrade(context.Background(), []byte("bad"))
	if !errors.Is(err, trade.ErrInvalidRequest) {
		t.Fatalf("expected ErrInvalidRequest, got %v", err)
	}
}

func TestDraftTrade_invalidOrderID(t *testing.T) {
	body := marshalJSON(t, trade.DraftTradeRequest{OrderID: "not-ulid", PlayerID: "p1"})
	_, err := newHandlers(&mockOutboxManager{}).DraftTrade(context.Background(), body)
	if !errors.Is(err, trade.ErrInvalidRequest) {
		t.Fatalf("expected ErrInvalidRequest, got %v", err)
	}
}

func TestDraftTrade_missingPlayerID(t *testing.T) {
	body := marshalJSON(t, trade.DraftTradeRequest{OrderID: validOrderID()})
	_, err := newHandlers(&mockOutboxManager{}).DraftTrade(context.Background(), body)
	if !errors.Is(err, trade.ErrInvalidRequest) {
		t.Fatalf("expected ErrInvalidRequest, got %v", err)
	}
}

func TestDraftTrade_newOrder_insertFails(t *testing.T) {
	mgr := &mockOutboxManager{
		statusErr: managers.ErrOrderNotFound,
		insertErr: errors.New("db down"),
	}
	body := marshalJSON(t, trade.DraftTradeRequest{OrderID: validOrderID(), PlayerID: "p1"})
	_, err := newHandlers(mgr).DraftTrade(context.Background(), body)
	if !errors.Is(err, trade.ErrFailedToCreateTrade) {
		t.Fatalf("expected ErrFailedToCreateTrade, got %v", err)
	}
}

func TestDraftTrade_getStatusError(t *testing.T) {
	mgr := &mockOutboxManager{statusErr: errors.New("mongo down")}
	body := marshalJSON(t, trade.DraftTradeRequest{OrderID: validOrderID(), PlayerID: "p1"})
	_, err := newHandlers(mgr).DraftTrade(context.Background(), body)
	if !errors.Is(err, trade.ErrFailedToGetTradeStatus) {
		t.Fatalf("expected ErrFailedToGetTradeStatus, got %v", err)
	}
}

func TestDraftTrade_updateGrantsConflict(t *testing.T) {
	orderID := validOrderID()
	mgr := &mockOutboxManager{
		statusErr: managers.ErrOrderNotFound,
		updateErr: managers.ErrOrderNotFound,
	}
	body := marshalJSON(t, trade.DraftTradeRequest{OrderID: orderID, PlayerID: "p1"})
	_, err := newHandlers(mgr).DraftTrade(context.Background(), body)
	if !errors.Is(err, trade.ErrTradeConflict) {
		t.Fatalf("expected ErrTradeConflict, got %v", err)
	}
}

func TestDraftTrade_updateGrantsError(t *testing.T) {
	mgr := &mockOutboxManager{
		statusErr: managers.ErrOrderNotFound,
		updateErr: errors.New("write failed"),
	}
	body := marshalJSON(t, trade.DraftTradeRequest{OrderID: validOrderID(), PlayerID: "p1"})
	_, err := newHandlers(mgr).DraftTrade(context.Background(), body)
	if !errors.Is(err, trade.ErrFailedToUpdateTrade) {
		t.Fatalf("expected ErrFailedToUpdateTrade, got %v", err)
	}
}

func TestDraftTrade_newOrder_success(t *testing.T) {
	orderID := validOrderID()
	event := stubEvent(orderID)
	mgr := &mockOutboxManager{
		statusErr:   managers.ErrOrderNotFound,
		updateEvent: event,
	}
	body := marshalJSON(t, trade.DraftTradeRequest{OrderID: orderID, PlayerID: "p1", InitiatorID: "init-1"})
	resp, err := newHandlers(mgr).DraftTrade(context.Background(), body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var got trade.DraftTradeResponse
	if err := json.Unmarshal(resp, &got); err != nil {
		t.Fatalf("invalid response JSON: %v", err)
	}
	if got.OrderID != orderID {
		t.Errorf("expected order_id %q, got %q", orderID, got.OrderID)
	}
	if got.TransactionID != event.CommitID {
		t.Errorf("expected transaction_id %q, got %q", event.CommitID, got.TransactionID)
	}
}

func TestDraftTrade_existingOrder_success(t *testing.T) {
	orderID := validOrderID()
	event := stubEvent(orderID)
	mgr := &mockOutboxManager{
		statusEvent: event,
		updateEvent: event,
	}
	body := marshalJSON(t, trade.DraftTradeRequest{OrderID: orderID, PlayerID: "p1", TransactionID: "commit-abc"})
	resp, err := newHandlers(mgr).DraftTrade(context.Background(), body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var got trade.DraftTradeResponse
	if err := json.Unmarshal(resp, &got); err != nil {
		t.Fatalf("invalid response JSON: %v", err)
	}
	if got.OrderID != orderID {
		t.Errorf("expected order_id %q, got %q", orderID, got.OrderID)
	}
}

func TestLockTrade_invalidJSON(t *testing.T) {
	_, err := newHandlers(&mockOutboxManager{}).LockTrade(context.Background(), []byte("bad"))
	if !errors.Is(err, trade.ErrInvalidRequest) {
		t.Fatalf("expected ErrInvalidRequest, got %v", err)
	}
}

func TestLockTrade_invalidOrderID(t *testing.T) {
	body := marshalJSON(t, trade.LockTradeRequest{OrderID: "bad", PlayerID: "p1", TransactionID: "tx"})
	_, err := newHandlers(&mockOutboxManager{}).LockTrade(context.Background(), body)
	if !errors.Is(err, trade.ErrInvalidRequest) {
		t.Fatalf("expected ErrInvalidRequest, got %v", err)
	}
}

func TestLockTrade_missingPlayerID(t *testing.T) {
	body := marshalJSON(t, trade.LockTradeRequest{OrderID: validOrderID(), TransactionID: "tx"})
	_, err := newHandlers(&mockOutboxManager{}).LockTrade(context.Background(), body)
	if !errors.Is(err, trade.ErrInvalidRequest) {
		t.Fatalf("expected ErrInvalidRequest, got %v", err)
	}
}

func TestLockTrade_missingTransactionID(t *testing.T) {
	body := marshalJSON(t, trade.LockTradeRequest{OrderID: validOrderID(), PlayerID: "p1"})
	_, err := newHandlers(&mockOutboxManager{}).LockTrade(context.Background(), body)
	if !errors.Is(err, trade.ErrInvalidRequest) {
		t.Fatalf("expected ErrInvalidRequest, got %v", err)
	}
}

func TestLockTrade_conflict(t *testing.T) {
	mgr := &mockOutboxManager{lockErr: managers.ErrOrderNotFound}
	body := marshalJSON(t, trade.LockTradeRequest{OrderID: validOrderID(), PlayerID: "p1", TransactionID: "tx"})
	_, err := newHandlers(mgr).LockTrade(context.Background(), body)
	if !errors.Is(err, trade.ErrTradeConflict) {
		t.Fatalf("expected ErrTradeConflict, got %v", err)
	}
}

func TestLockTrade_otherError(t *testing.T) {
	mgr := &mockOutboxManager{lockErr: errors.New("db down")}
	body := marshalJSON(t, trade.LockTradeRequest{OrderID: validOrderID(), PlayerID: "p1", TransactionID: "tx"})
	_, err := newHandlers(mgr).LockTrade(context.Background(), body)
	if !errors.Is(err, trade.ErrFailedToUpdateTrade) {
		t.Fatalf("expected ErrFailedToUpdateTrade, got %v", err)
	}
}

func TestLockTrade_success(t *testing.T) {
	orderID := validOrderID()
	event := stubEvent(orderID)
	event.Status = trade.OutboxStatusPending
	event.Signatures = []string{"p1"}
	mgr := &mockOutboxManager{lockEvent: event}
	body := marshalJSON(t, trade.LockTradeRequest{OrderID: orderID, PlayerID: "p1", TransactionID: "tx"})
	resp, err := newHandlers(mgr).LockTrade(context.Background(), body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var got trade.LockTradeResponse
	if err := json.Unmarshal(resp, &got); err != nil {
		t.Fatalf("invalid response JSON: %v", err)
	}
	if got.OrderID != orderID {
		t.Errorf("expected order_id %q, got %q", orderID, got.OrderID)
	}
	if got.Status != trade.OutboxStatusPending {
		t.Errorf("expected status PENDING, got %q", got.Status)
	}
}

func TestUnlockTrade_invalidJSON(t *testing.T) {
	_, err := newHandlers(&mockOutboxManager{}).UnlockTrade(context.Background(), []byte("bad"))
	if !errors.Is(err, trade.ErrInvalidRequest) {
		t.Fatalf("expected ErrInvalidRequest, got %v", err)
	}
}

func TestUnlockTrade_missingPlayerID(t *testing.T) {
	body := marshalJSON(t, trade.UnlockTradeRequest{OrderID: validOrderID(), TransactionID: "tx"})
	_, err := newHandlers(&mockOutboxManager{}).UnlockTrade(context.Background(), body)
	if !errors.Is(err, trade.ErrInvalidRequest) {
		t.Fatalf("expected ErrInvalidRequest, got %v", err)
	}
}

func TestUnlockTrade_missingTransactionID(t *testing.T) {
	body := marshalJSON(t, trade.UnlockTradeRequest{OrderID: validOrderID(), PlayerID: "p1"})
	_, err := newHandlers(&mockOutboxManager{}).UnlockTrade(context.Background(), body)
	if !errors.Is(err, trade.ErrInvalidRequest) {
		t.Fatalf("expected ErrInvalidRequest, got %v", err)
	}
}

func TestUnlockTrade_conflict(t *testing.T) {
	mgr := &mockOutboxManager{unlockErr: managers.ErrOrderNotFound}
	body := marshalJSON(t, trade.UnlockTradeRequest{OrderID: validOrderID(), PlayerID: "p1", TransactionID: "tx"})
	_, err := newHandlers(mgr).UnlockTrade(context.Background(), body)
	if !errors.Is(err, trade.ErrTradeConflict) {
		t.Fatalf("expected ErrTradeConflict, got %v", err)
	}
}

func TestUnlockTrade_success(t *testing.T) {
	orderID := validOrderID()
	event := stubEvent(orderID)
	event.Signatures = []string{}
	mgr := &mockOutboxManager{unlockEvent: event}
	body := marshalJSON(t, trade.UnlockTradeRequest{OrderID: orderID, PlayerID: "p1", TransactionID: "tx"})
	resp, err := newHandlers(mgr).UnlockTrade(context.Background(), body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var got trade.UnlockTradeResponse
	if err := json.Unmarshal(resp, &got); err != nil {
		t.Fatalf("invalid response JSON: %v", err)
	}
	if got.OrderID != orderID {
		t.Errorf("expected order_id %q, got %q", orderID, got.OrderID)
	}
}

func TestDispatchGrants_invalidJSON(t *testing.T) {
	_, err := newHandlers(&mockOutboxManager{}).DispatchGrants(context.Background(), []byte("bad"))
	if !errors.Is(err, trade.ErrInvalidRequest) {
		t.Fatalf("expected ErrInvalidRequest, got %v", err)
	}
}

func TestDispatchGrants_invalidOrderID(t *testing.T) {
	body := marshalJSON(t, trade.DispatchGrantsRequest{OrderID: "bad"})
	_, err := newHandlers(&mockOutboxManager{}).DispatchGrants(context.Background(), body)
	if !errors.Is(err, trade.ErrInvalidRequest) {
		t.Fatalf("expected ErrInvalidRequest, got %v", err)
	}
}

func TestDispatchGrants_createOutboxError(t *testing.T) {
	mgr := &mockOutboxManager{createErr: errors.New("db down")}
	body := marshalJSON(t, trade.DispatchGrantsRequest{OrderID: validOrderID()})
	_, err := newHandlers(mgr).DispatchGrants(context.Background(), body)
	if !errors.Is(err, trade.ErrFailedToCreateTrade) {
		t.Fatalf("expected ErrFailedToCreateTrade, got %v", err)
	}
}

func TestDispatchGrants_success(t *testing.T) {
	orderID := validOrderID()
	body := marshalJSON(t, trade.DispatchGrantsRequest{
		OrderID:     orderID,
		InitiatorID: "init-1",
		Grants: []trade.TradeGrant{
			{PlayerID: "p1", Type: "currency", TargetID: "gold", Amount: 100},
		},
	})
	resp, err := newHandlers(&mockOutboxManager{}).DispatchGrants(context.Background(), body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var got trade.TradeResponse
	if err := json.Unmarshal(resp, &got); err != nil {
		t.Fatalf("invalid response JSON: %v", err)
	}
	if got.OrderID != orderID {
		t.Errorf("expected order_id %q, got %q", orderID, got.OrderID)
	}
}

func TestTradeStatus_invalidJSON(t *testing.T) {
	_, err := newHandlers(&mockOutboxManager{}).TradeStatus(context.Background(), []byte("bad"))
	if !errors.Is(err, trade.ErrInvalidRequest) {
		t.Fatalf("expected ErrInvalidRequest, got %v", err)
	}
}

func TestTradeStatus_notFound(t *testing.T) {
	mgr := &mockOutboxManager{statusErr: managers.ErrOrderNotFound}
	body := marshalJSON(t, trade.TradeStatusRequest{OrderID: validOrderID()})
	_, err := newHandlers(mgr).TradeStatus(context.Background(), body)
	if !errors.Is(err, trade.ErrOrderNotFound) {
		t.Fatalf("expected ErrOrderNotFound, got %v", err)
	}
}

func TestTradeStatus_otherError(t *testing.T) {
	mgr := &mockOutboxManager{statusErr: errors.New("db down")}
	body := marshalJSON(t, trade.TradeStatusRequest{OrderID: validOrderID()})
	_, err := newHandlers(mgr).TradeStatus(context.Background(), body)
	if !errors.Is(err, trade.ErrFailedToGetTradeStatus) {
		t.Fatalf("expected ErrFailedToGetTradeStatus, got %v", err)
	}
}

func TestTradeStatus_success(t *testing.T) {
	orderID := validOrderID()
	mgr := &mockOutboxManager{statusEvent: &trade.OutboxEvent{
		OrderID: orderID,
		Status:  trade.OutboxStatusPending,
	}}
	body := marshalJSON(t, trade.TradeStatusRequest{OrderID: orderID})
	resp, err := newHandlers(mgr).TradeStatus(context.Background(), body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var got trade.TradeStatusResponse
	if err := json.Unmarshal(resp, &got); err != nil {
		t.Fatalf("invalid response JSON: %v", err)
	}
	if got.Status != trade.OutboxStatusPending {
		t.Errorf("expected PENDING, got %q", got.Status)
	}
}
