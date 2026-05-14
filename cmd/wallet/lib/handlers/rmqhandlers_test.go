package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/mercury/cmd/wallet/lib/managers"
	"github.com/mercury/pkg/clients/wallet"
	"github.com/mercury/pkg/ids"
)

// --- mock ---

type mockWalletManager struct {
	wallet    *managers.Wallet
	grantErr  error
	getErr    error
}

func (m *mockWalletManager) GetWallet(_ context.Context, playerID string) (*managers.Wallet, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	if m.wallet != nil {
		return m.wallet, nil
	}
	return &managers.Wallet{PlayerID: playerID, Currencies: map[string]int{}}, nil
}

func (m *mockWalletManager) Grant(_ context.Context, playerID, _ string, _ int, _ string) (*managers.Wallet, error) {
	if m.grantErr != nil {
		return nil, m.grantErr
	}
	if m.wallet != nil {
		return m.wallet, nil
	}
	return &managers.Wallet{PlayerID: playerID, Currencies: map[string]int{}}, nil
}

// --- helpers ---

func newHandlers(mgr managers.WalletManager) RMQHandlers {
	return NewRMQHandlers(mgr)
}

func validOrderID() string {
	return ids.NewOrderID()
}

func marshalJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return b
}

// --- convertDBCurrencyToRMQCurrency ---

func TestConvertDBCurrencyToRMQCurrency_empty(t *testing.T) {
	result := convertDBCurrencyToRMQCurrency(map[string]int{})
	if len(result) != 0 {
		t.Fatalf("expected empty slice, got %v", result)
	}
}

func TestConvertDBCurrencyToRMQCurrency_mapsCorrectly(t *testing.T) {
	result := convertDBCurrencyToRMQCurrency(map[string]int{"gold": 100, "gems": 5})
	if len(result) != 2 {
		t.Fatalf("expected 2 currencies, got %d", len(result))
	}
	seen := map[string]int{}
	for _, c := range result {
		seen[c.CurrencyType] = c.Amount
	}
	if seen["gold"] != 100 {
		t.Errorf("expected gold=100, got %d", seen["gold"])
	}
	if seen["gems"] != 5 {
		t.Errorf("expected gems=5, got %d", seen["gems"])
	}
}

// --- AddCurrency ---

func TestAddCurrency_invalidJSON_returnsErrInvalidRequest(t *testing.T) {
	h := newHandlers(&mockWalletManager{})
	_, err := h.AddCurrency(context.Background(), []byte("not json"))
	if !errors.Is(err, wallet.ErrInvalidRequest) {
		t.Fatalf("expected ErrInvalidRequest, got %v", err)
	}
}

func TestAddCurrency_invalidOrderID_returnsErrInvalidRequest(t *testing.T) {
	h := newHandlers(&mockWalletManager{})
	body := marshalJSON(t, wallet.AddCurrencyRequest{
		PlayerID:   "player-1",
		CurrencyID: "gold",
		Amount:     100,
		OrderID:    "not-a-ulid",
	})
	_, err := h.AddCurrency(context.Background(), body)
	if !errors.Is(err, wallet.ErrInvalidRequest) {
		t.Fatalf("expected ErrInvalidRequest, got %v", err)
	}
}

func TestAddCurrency_grantError_returnsErrFailedToGrantCurrency(t *testing.T) {
	h := newHandlers(&mockWalletManager{grantErr: errors.New("db down")})
	body := marshalJSON(t, wallet.AddCurrencyRequest{
		PlayerID:   "player-1",
		CurrencyID: "gold",
		Amount:     100,
		OrderID:    validOrderID(),
	})
	_, err := h.AddCurrency(context.Background(), body)
	if !errors.Is(err, wallet.ErrFailedToGrantCurrency) {
		t.Fatalf("expected ErrFailedToGrantCurrency, got %v", err)
	}
}

func TestAddCurrency_success_returnsWalletResponse(t *testing.T) {
	mgr := &mockWalletManager{
		wallet: &managers.Wallet{
			PlayerID:   "player-1",
			Currencies: map[string]int{"gold": 100},
		},
	}
	h := newHandlers(mgr)
	body := marshalJSON(t, wallet.AddCurrencyRequest{
		PlayerID:   "player-1",
		CurrencyID: "gold",
		Amount:     100,
		OrderID:    validOrderID(),
	})
	resp, err := h.AddCurrency(context.Background(), body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var got wallet.GetWalletResponse
	if err := json.Unmarshal(resp, &got); err != nil {
		t.Fatalf("invalid response JSON: %v", err)
	}
	if got.PlayerID != "player-1" {
		t.Errorf("expected player-1, got %q", got.PlayerID)
	}
	if len(got.Currencies) != 1 || got.Currencies[0].CurrencyType != "gold" || got.Currencies[0].Amount != 100 {
		t.Errorf("unexpected currencies: %v", got.Currencies)
	}
}

// --- GetWallet ---

func TestGetWallet_invalidJSON_returnsErrInvalidRequest(t *testing.T) {
	h := newHandlers(&mockWalletManager{})
	_, err := h.GetWallet(context.Background(), []byte("not json"))
	if !errors.Is(err, wallet.ErrInvalidRequest) {
		t.Fatalf("expected ErrInvalidRequest, got %v", err)
	}
}

func TestGetWallet_notFound_returnsErrWalletDoesNotExist(t *testing.T) {
	h := newHandlers(&mockWalletManager{getErr: managers.ErrWalletNotFound})
	body := marshalJSON(t, wallet.GetWalletRequest{PlayerID: "ghost"})
	_, err := h.GetWallet(context.Background(), body)
	if !errors.Is(err, wallet.ErrWalletDoesNotExist) {
		t.Fatalf("expected ErrWalletDoesNotExist, got %v", err)
	}
}

func TestGetWallet_otherDBError_returnsErrFailedToGetWallet(t *testing.T) {
	h := newHandlers(&mockWalletManager{getErr: errors.New("connection reset")})
	body := marshalJSON(t, wallet.GetWalletRequest{PlayerID: "player-1"})
	_, err := h.GetWallet(context.Background(), body)
	if !errors.Is(err, wallet.ErrFailedToGetWallet) {
		t.Fatalf("expected ErrFailedToGetWallet, got %v", err)
	}
}

func TestGetWallet_success_returnsWalletResponse(t *testing.T) {
	mgr := &mockWalletManager{
		wallet: &managers.Wallet{
			PlayerID:   "player-1",
			Currencies: map[string]int{"gems": 50, "gold": 200},
		},
	}
	h := newHandlers(mgr)
	body := marshalJSON(t, wallet.GetWalletRequest{PlayerID: "player-1"})
	resp, err := h.GetWallet(context.Background(), body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var got wallet.GetWalletResponse
	if err := json.Unmarshal(resp, &got); err != nil {
		t.Fatalf("invalid response JSON: %v", err)
	}
	if got.PlayerID != "player-1" {
		t.Errorf("expected player-1, got %q", got.PlayerID)
	}
	if len(got.Currencies) != 2 {
		t.Errorf("expected 2 currencies, got %d", len(got.Currencies))
	}
}

func TestGetWallet_success_emptyWallet(t *testing.T) {
	mgr := &mockWalletManager{
		wallet: &managers.Wallet{
			PlayerID:   "player-1",
			Currencies: map[string]int{},
		},
	}
	h := newHandlers(mgr)
	body := marshalJSON(t, wallet.GetWalletRequest{PlayerID: "player-1"})
	resp, err := h.GetWallet(context.Background(), body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var got wallet.GetWalletResponse
	if err := json.Unmarshal(resp, &got); err != nil {
		t.Fatalf("invalid response JSON: %v", err)
	}
	if len(got.Currencies) != 0 {
		t.Errorf("expected empty currencies, got %v", got.Currencies)
	}
}
