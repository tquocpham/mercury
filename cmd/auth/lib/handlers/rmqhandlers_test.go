package handlers_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/mercury/cmd/auth/lib/hash"
	"github.com/mercury/cmd/auth/lib/handlers"
	"github.com/mercury/cmd/auth/lib/managers"
	"github.com/mercury/pkg/clients/auth"
	"github.com/mercury/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Mocks
// ---------------------------------------------------------------------------

type mockAccountsManager struct {
	account *managers.AccountInformation
	err     error
}

func (m *mockAccountsManager) GetAccountByUsername(_ context.Context, _ string) (*managers.AccountInformation, error) {
	return m.account, m.err
}
func (m *mockAccountsManager) CreateAccount(_ context.Context, _, _, _ string, _ []auth.Role) (*managers.AccountInformation, error) {
	return nil, errors.New("not implemented")
}
func (m *mockAccountsManager) ActivateAccount(_ context.Context, _ string) error {
	return errors.New("not implemented")
}

type mockSessionsManager struct {
	session *managers.Session
	err     error
}

func (m *mockSessionsManager) Create(_ context.Context, _, _ string, _ []string, _ time.Duration) (*managers.Session, error) {
	return m.session, m.err
}
func (m *mockSessionsManager) Get(_ context.Context, _ string) (*managers.Session, error) {
	return nil, errors.New("not implemented")
}
func (m *mockSessionsManager) Refresh(_ context.Context, _ string, _ time.Duration) error {
	return errors.New("not implemented")
}
func (m *mockSessionsManager) Delete(_ context.Context, _ string) error {
	return errors.New("not implemented")
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func newTestHandler(t *testing.T, accounts managers.AccountsManager, sessions managers.SessionsManager) handlers.RMQHandlers {
	t.Helper()
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	keys := &config.Keys{
		Private: privKey,
		Public:  &privKey.PublicKey,
	}
	return handlers.NewRMQHandlers(accounts, sessions, time.Hour, keys)
}

func loginBody(t *testing.T, username, password string) []byte {
	t.Helper()
	b, err := json.Marshal(auth.LoginRequest{
		Credentials: auth.Credentials{Username: username, Password: password},
	})
	require.NoError(t, err)
	return b
}

func makeAccount(t *testing.T, password string) *managers.AccountInformation {
	t.Helper()
	salt, err := hash.GenerateSalt(14)
	require.NoError(t, err)
	hashed, err := hash.Hash(password, salt)
	require.NoError(t, err)
	return &managers.AccountInformation{
		ID:       "test-user-id",
		Username: "testuser",
		Salt:     salt,
		Password: hashed,
		Roles:    []auth.Role{auth.UserRole},
	}
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestLogin_InvalidJSON(t *testing.T) {
	h := newTestHandler(t, &mockAccountsManager{}, &mockSessionsManager{})
	resp, err := h.Login(context.Background(), []byte("not-json"))
	assert.Nil(t, resp)
	assert.ErrorIs(t, err, auth.ErrInvalidRequest)
}

func TestLogin_AccountNotFound(t *testing.T) {
	accounts := &mockAccountsManager{err: managers.ErrAccountNotFound}
	h := newTestHandler(t, accounts, &mockSessionsManager{})

	resp, err := h.Login(context.Background(), loginBody(t, "ghost", "password"))
	assert.Nil(t, resp)
	assert.ErrorIs(t, err, auth.ErrUnauthorized)
}

func TestLogin_AccountQueryError(t *testing.T) {
	accounts := &mockAccountsManager{err: errors.New("db down")}
	h := newTestHandler(t, accounts, &mockSessionsManager{})

	resp, err := h.Login(context.Background(), loginBody(t, "testuser", "password"))
	assert.Nil(t, resp)
	assert.ErrorIs(t, err, auth.ErrFailedToQueryAccount)
}

func TestLogin_WrongPassword(t *testing.T) {
	account := makeAccount(t, "correctpassword")
	accounts := &mockAccountsManager{account: account}
	h := newTestHandler(t, accounts, &mockSessionsManager{})

	resp, err := h.Login(context.Background(), loginBody(t, "testuser", "wrongpassword"))
	assert.Nil(t, resp)
	assert.ErrorIs(t, err, auth.ErrUnauthorized)
}

func TestLogin_SessionCreationFails(t *testing.T) {
	account := makeAccount(t, "password")
	accounts := &mockAccountsManager{account: account}
	sessions := &mockSessionsManager{err: errors.New("redis down")}
	h := newTestHandler(t, accounts, sessions)

	resp, err := h.Login(context.Background(), loginBody(t, "testuser", "password"))
	assert.Nil(t, resp)
	assert.ErrorIs(t, err, auth.ErrSessionCreationFailed)
}

func TestLogin_Success(t *testing.T) {
	account := makeAccount(t, "password")
	accounts := &mockAccountsManager{account: account}
	sessions := &mockSessionsManager{
		session: &managers.Session{
			SessionID: "test-session-id",
			UserID:    account.ID,
			Username:  account.Username,
			Roles:     []string{string(auth.UserRole)},
		},
	}
	h := newTestHandler(t, accounts, sessions)

	resp, err := h.Login(context.Background(), loginBody(t, "testuser", "password"))
	require.NoError(t, err)
	require.NotNil(t, resp)

	var tokenResp auth.TokenResponse
	require.NoError(t, json.Unmarshal(resp, &tokenResp))
	assert.NotEmpty(t, tokenResp.Token)
}
