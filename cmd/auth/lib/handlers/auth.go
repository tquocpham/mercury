package handlers

import (
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/labstack/echo/v4"
	"github.com/mercury/cmd/auth/lib/hash"
	"github.com/mercury/cmd/auth/lib/managers"
	"github.com/mercury/pkg/clients/auth"
	"github.com/mercury/pkg/instrumentation"
	"github.com/mercury/pkg/middleware"
)

type AuthHandlers interface {
	Signin(c echo.Context) error
	Refresh(c echo.Context) error
}

type authHandlers struct {
	userManager managers.UsersManager
	tokenExp    time.Duration
	privKey     *rsa.PrivateKey
	pubKey      *rsa.PublicKey
	signer      jwt.SigningMethod
}

func NewAuthHandler(
	userManager managers.UsersManager, tokenExp time.Duration,
	privKey *rsa.PrivateKey, pubKey *rsa.PublicKey) AuthHandlers {

	return &authHandlers{
		userManager: userManager,
		tokenExp:    tokenExp,
		privKey:     privKey,
		pubKey:      pubKey,
		signer:      jwt.GetSigningMethod("RS256"),
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

	// get user and check if the passwords match
	user, err := h.userManager.GetUserByUsername(ctx, creds.Username)
	if err != nil {
		return echo.ErrUnauthorized
	}
	if !hash.CheckPasswordHash(creds.Password, user.Salt, user.Password) {
		return echo.ErrUnauthorized
	}

	expirationTime := time.Now().Add(h.tokenExp)

	rs := make([]string, len(user.Roles))
	for i, r := range user.Roles {
		rs[i] = string(r)
	}

	clms := &middleware.Claims{
		Username: creds.Username,
		Roles:    rs,
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: expirationTime.Unix(), // In JWT, the expiry time is expressed as unix milliseconds
		},
	}

	token := jwt.NewWithClaims(h.signer, clms)
	signedToken, err := token.SignedString(h.privKey)
	if err != nil {
		return echo.ErrUnauthorized
	}

	// Return the token in the response body so the login page can
	// redirect to the calling service's callback URL with the token.
	return c.JSON(http.StatusOK, auth.TokenResponse{
		Token: signedToken,
	})
}

func (h *authHandlers) Refresh(c echo.Context) error {
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

	return c.JSON(http.StatusOK, auth.TokenResponse{
		Token: signedToken,
	})
}
