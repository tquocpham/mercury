package handlers

import (
	"context"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/mercury/cmd/auth/lib/hash"
	"github.com/mercury/cmd/auth/lib/managers"
	"github.com/mercury/pkg/clients/auth"
	"github.com/mercury/pkg/config"
	"github.com/mercury/pkg/middleware"
	"github.com/mercury/pkg/rmq"
	"github.com/sirupsen/logrus"
)

type RMQHandlers interface {
	Login(ctx context.Context, body []byte) ([]byte, error)
	Refresh(ctx context.Context, body []byte) ([]byte, error)
	Revoke(ctx context.Context, body []byte) ([]byte, error)
	CreateAccount(ctx context.Context, body []byte) ([]byte, error)
	ActivateAccount(ctx context.Context, body []byte) ([]byte, error)
	GetSession(ctx context.Context, body []byte) ([]byte, error)
	RefreshSession(ctx context.Context, body []byte) ([]byte, error)
	DeleteSession(ctx context.Context, body []byte) ([]byte, error)
}

type rmqHanders struct {
	accountsManager managers.AccountsManager
	tokenExp        time.Duration
	privKey         *rsa.PrivateKey
	pubKey          *rsa.PublicKey
	signer          jwt.SigningMethod
	sessionsManager managers.SessionsManager
}

func NewRMQHandlers(
	accountsManager managers.AccountsManager,
	sessionsManager managers.SessionsManager,
	tokenExp time.Duration,
	keys *config.Keys,
) RMQHandlers {
	return &rmqHanders{
		accountsManager: accountsManager,
		sessionsManager: sessionsManager,
		tokenExp:        tokenExp,
		privKey:         keys.Private,
		pubKey:          keys.Public,
		signer:          jwt.GetSigningMethod("RS256"),
	}
}

func (h *rmqHanders) Login(ctx context.Context, body []byte) ([]byte, error) {
	request := &auth.LoginRequest{}
	if err := json.Unmarshal(body, request); err != nil {
		return nil, auth.ErrInvalidRequest
	}
	creds := request.Credentials
	// get account info and check if the passwords match
	account, err := h.accountsManager.GetAccountByUsername(ctx, creds.Username)
	if err != nil {
		if errors.Is(err, managers.ErrAccountNotFound) {
			return nil, auth.ErrUnauthorized
		}
		return nil, auth.ErrFailedToQueryAccount
	}
	if !hash.CheckPasswordHash(creds.Password, account.Salt, account.Password) {
		return nil, auth.ErrUnauthorized
	}
	expirationTime := time.Now().Add(h.tokenExp)

	rs := make([]string, len(account.Roles))
	for i, r := range account.Roles {
		rs[i] = string(r)
	}

	session, err := h.sessionsManager.Create(ctx, account.ID, account.Username, rs, h.tokenExp)
	if err != nil {
		return nil, auth.ErrSessionCreationFailed
	}

	clms := &middleware.Claims{
		Username:  creds.Username,
		UserID:    account.ID,
		Roles:     rs,
		SessionID: session.SessionID,
		StandardClaims: jwt.StandardClaims{
			IssuedAt:  time.Now().Unix(),
			ExpiresAt: expirationTime.Unix(), // In JWT, the expiry time is expressed as unix milliseconds
		},
	}
	token := jwt.NewWithClaims(h.signer, clms)
	signedToken, err := token.SignedString(h.privKey)
	if err != nil {
		return nil, auth.ErrTokenSignatureFailed
	}
	return json.Marshal(auth.TokenResponse{
		Token: signedToken,
	})
}

func (h *rmqHanders) Refresh(ctx context.Context, body []byte) ([]byte, error) {
	request := &auth.RefreshRequest{}
	if err := json.Unmarshal(body, request); err != nil {
		return nil, auth.ErrInvalidRequest
	}
	claims, err := middleware.ValidateToken(request.Token, h.pubKey)
	if err != nil {
		return nil, auth.ErrUnauthorized
	}
	expirationTime := time.Now().Add(h.tokenExp)
	claims.ExpiresAt = expirationTime.Unix()
	claims.IssuedAt = time.Now().Unix()

	if err := h.sessionsManager.Refresh(ctx, claims.SessionID, h.tokenExp); err != nil {
		return nil, auth.ErrSessionCreationFailed
	}

	token := jwt.NewWithClaims(h.signer, claims)
	signedToken, err := token.SignedString(h.privKey)
	if err != nil {
		return nil, auth.ErrTokenSignatureFailed
	}
	return json.Marshal(auth.RefreshResponse{
		Token: signedToken,
	})
}

func (h *rmqHanders) Revoke(ctx context.Context, body []byte) ([]byte, error) {
	logger := rmq.GetLogger(ctx)
	logger.Debug("not implemented")
	return []byte{}, nil
}

func (h *rmqHanders) CreateAccount(ctx context.Context, body []byte) ([]byte, error) {
	logger := rmq.GetLogger(ctx)
	request := &auth.AccountCreationRequest{}
	if err := json.Unmarshal(body, request); err != nil {
		return nil, auth.ErrInvalidRequest
	}
	account, err := h.accountsManager.CreateAccount(ctx, request.Username, request.Email, request.Password, []auth.Role{
		auth.UserRole,
	})
	if err != nil {
		if errors.Is(err, managers.ErrDuplicateAccount) {
			return nil, auth.ErrAccountDuplicate
		}
		return nil, auth.ErrAccountCreationFailed
	}
	logger.
		WithFields(logrus.Fields{
			"accountID": account.ID,
		}).
		Info("account created")
	return json.Marshal(auth.AccountCreationResponse{
		AccountID: account.ID,
	})
}
func (h *rmqHanders) ActivateAccount(ctx context.Context, body []byte) ([]byte, error) {
	request := &auth.ActivateAccountRequest{}
	if err := json.Unmarshal(body, request); err != nil {
		return nil, auth.ErrInvalidRequest
	}
	accountID := request.AccountID
	if err := h.accountsManager.ActivateAccount(ctx, accountID); err != nil {
		return nil, auth.ErrAccountActivationFailed
	}
	return json.Marshal(auth.ActivateAccountResponse{
		AccountID: accountID,
	})
}

func (h *rmqHanders) GetSession(ctx context.Context, body []byte) ([]byte, error) {
	request := &auth.GetSessionRequest{}
	if err := json.Unmarshal(body, request); err != nil {
		return nil, auth.ErrInvalidRequest
	}
	sessionID := request.SessionID
	session, err := h.sessionsManager.Get(ctx, sessionID)
	if err != nil {
		return nil, auth.ErrNoSessionFound
	}
	return json.Marshal(auth.SessionResponse{
		SessionID: session.SessionID,
		UserID:    session.UserID,
		Username:  session.Username,
		Roles:     session.Roles,
	})
}

func (h *rmqHanders) RefreshSession(ctx context.Context, body []byte) ([]byte, error) {
	request := &auth.RefreshSessionRequest{}
	if err := json.Unmarshal(body, request); err != nil {
		return nil, auth.ErrInvalidRequest
	}
	sessionID := request.SessionID
	if err := h.sessionsManager.Refresh(ctx, sessionID, h.tokenExp); err != nil {
		return nil, auth.ErrSessionExtensionFailed
	}
	session, err := h.sessionsManager.Get(ctx, sessionID)
	if err != nil {
		return nil, auth.ErrNoSessionFound
	}
	return json.Marshal(auth.SessionResponse{
		SessionID: session.SessionID,
		UserID:    session.UserID,
		Username:  session.Username,
		Roles:     session.Roles,
	})
}

func (h *rmqHanders) DeleteSession(ctx context.Context, body []byte) ([]byte, error) {
	request := &auth.DeleteSessionRequest{}
	if err := json.Unmarshal(body, request); err != nil {
		return nil, auth.ErrInvalidRequest
	}
	sessionID := request.SessionID
	if err := h.sessionsManager.Delete(ctx, sessionID); err != nil {
		return nil, auth.ErrSessionDeletionFailed
	}
	return json.Marshal(auth.DeleteSessionResponse{
		SessionID: sessionID,
	})
}
