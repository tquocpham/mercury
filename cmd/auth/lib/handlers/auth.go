package handlers

import (
	"crypto/rsa"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/labstack/echo/v4"
	"github.com/mercury/cmd/auth/lib/hash"
	"github.com/mercury/cmd/auth/lib/managers"
	"github.com/mercury/pkg/clients/auth"
	"github.com/mercury/pkg/config"
	"github.com/mercury/pkg/instrumentation"
	"github.com/mercury/pkg/middleware"
	"github.com/sirupsen/logrus"
)

type AuthHandlers interface {
	Signin(c echo.Context) error
	Refresh(c echo.Context) error
	Revoke(c echo.Context) error
	CreateAccount(c echo.Context) error
	ActivateAccount(c echo.Context) error
	GetSession(c echo.Context) error
	ExtendSession(c echo.Context) error
	DeleteSession(c echo.Context) error
}

type authHandlers struct {
	accountsManager managers.AccountsManager
	tokenExp        time.Duration
	privKey         *rsa.PrivateKey
	pubKey          *rsa.PublicKey
	signer          jwt.SigningMethod
	sessionsManager managers.SessionsManager
}

func NewAuthHandler(
	accountsManager managers.AccountsManager,
	sessionsManager managers.SessionsManager,
	tokenExp time.Duration,
	keys *config.Keys) AuthHandlers {

	return &authHandlers{
		accountsManager: accountsManager,
		sessionsManager: sessionsManager,
		tokenExp:        tokenExp,
		privKey:         keys.Private,
		pubKey:          keys.Public,
		signer:          jwt.GetSigningMethod("RS256"),
	}
}

// Signin handler
func (h *authHandlers) Signin(c echo.Context) error {
	ctx := instrumentation.ToContext(c)
	request := &auth.SigninRequest{}
	if err := json.NewDecoder(c.Request().Body).Decode(request); err != nil {
		return echo.ErrUnauthorized
	}
	creds := request.Credentials

	// get account info and check if the passwords match
	account, err := h.accountsManager.GetAccountByUsername(ctx, creds.Username)
	if err != nil {
		return echo.ErrUnauthorized
	}
	if !hash.CheckPasswordHash(creds.Password, account.Salt, account.Password) {
		return echo.ErrUnauthorized
	}

	expirationTime := time.Now().Add(h.tokenExp)

	rs := make([]string, len(account.Roles))
	for i, r := range account.Roles {
		rs[i] = string(r)
	}

	session, err := h.sessionsManager.Create(ctx, account.ID, account.Username, rs, h.tokenExp)
	if err != nil {
		return echo.ErrInternalServerError
		//
	}

	clms := &middleware.Claims{
		Username:  creds.Username,
		UserID:    account.ID,
		Roles:     rs,
		SessionID: session.SessionID,
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: expirationTime.Unix(), // In JWT, the expiry time is expressed as unix milliseconds
		},
	}

	token := jwt.NewWithClaims(h.signer, clms)
	signedToken, err := token.SignedString(h.privKey)
	if err != nil {
		return echo.ErrInternalServerError
	}

	// Return the token in the response body so the login page can
	// redirect to the calling service's callback URL with the token.
	return c.JSON(http.StatusOK, auth.TokenResponse{
		Token: signedToken,
	})
}

func (h *authHandlers) Refresh(c echo.Context) error {
	ctx := instrumentation.ToContext(c)

	cookie, err := c.Cookie("session")
	if err != nil || cookie.Value == "" {
		return echo.ErrUnauthorized
	}

	claims, err := middleware.ValidateToken(cookie.Value, h.pubKey)
	if err != nil {
		return echo.ErrUnauthorized
	}

	expirationTime := time.Now().Add(h.tokenExp)
	claims.ExpiresAt = expirationTime.Unix()
	claims.IssuedAt = time.Now().Unix()

	token := jwt.NewWithClaims(h.signer, claims)
	signedToken, err := token.SignedString(h.privKey)
	if err != nil {
		return echo.ErrInternalServerError
	}
	if err := h.sessionsManager.Refresh(ctx, claims.SessionID, h.tokenExp); err != nil {
		return echo.ErrInternalServerError
	}

	return c.JSON(http.StatusOK, auth.TokenResponse{
		Token: signedToken,
	})
}

func (h *authHandlers) Revoke(c echo.Context) error {
	ctx := instrumentation.ToContext(c)
	logger := instrumentation.LoggerFromContext(ctx)
	logger.Debug("not implemented")
	return nil
}

func (h *authHandlers) CreateAccount(c echo.Context) error {
	ctx := instrumentation.ToContext(c)
	request := &auth.AccountCreationRequest{}
	if err := json.NewDecoder(c.Request().Body).Decode(request); err != nil {
		return echo.ErrUnauthorized
	}

	account, err := h.accountsManager.CreateAccount(ctx, request.Username, request.Email, request.Password, []auth.Role{
		auth.UserRole,
	})
	if err != nil {
		if errors.Is(err, managers.ErrDuplicateAccount) {
			return echo.NewHTTPError(http.StatusConflict, "username or email already taken")
		}
		return echo.ErrInternalServerError
	}

	logger := instrumentation.LoggerFromContext(ctx)
	logger.
		WithFields(logrus.Fields{
			"accountID": account.ID,
		}).
		Info("account created")

	return c.JSON(http.StatusOK, auth.AccountCreationResponse{
		AccountID: account.ID,
	})
}

func (h *authHandlers) ActivateAccount(c echo.Context) error {
	ctx := instrumentation.ToContext(c)
	accountID := c.Param("accountid")
	if err := h.accountsManager.ActivateAccount(ctx, accountID); err != nil {
		// todo: update this with correct error
		return echo.ErrUnauthorized
	}

	return c.JSON(http.StatusOK, auth.TokenResponse{})
}

func (h *authHandlers) GetSession(c echo.Context) error {
	ctx := instrumentation.ToContext(c)
	sessionID := c.Param("sessionid")
	session, err := h.sessionsManager.Get(ctx, sessionID)
	if err != nil {
		return echo.ErrNonExistentKey
	}
	return c.JSON(http.StatusOK, auth.SessionResponse{
		SessionID: session.SessionID,
		UserID:    session.UserID,
		Username:  session.Username,
		Roles:     session.Roles,
	})
}

func (h *authHandlers) ExtendSession(c echo.Context) error {
	ctx := instrumentation.ToContext(c)
	sessionID := c.Param("sessionid")
	if err := h.sessionsManager.Refresh(ctx, sessionID, h.tokenExp); err != nil {
		return echo.ErrInternalServerError
	}
	session, err := h.sessionsManager.Get(ctx, sessionID)
	if err != nil {
		return echo.ErrNonExistentKey
	}
	return c.JSON(http.StatusOK, auth.SessionResponse{
		SessionID: session.SessionID,
		UserID:    session.UserID,
		Username:  session.Username,
		Roles:     session.Roles,
	})
}

func (h *authHandlers) DeleteSession(c echo.Context) error {
	ctx := instrumentation.ToContext(c)
	sessionID := c.Param("sessionid")
	if err := h.sessionsManager.Delete(ctx, sessionID); err != nil {
		return echo.ErrInternalServerError
	}
	return c.JSON(http.StatusOK, auth.DeleteSessionResponse{
		SessionID: sessionID,
	})
}
