package middleware

import (
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/labstack/echo/v4"
)

const ContextKeyClaims = "Claims"
const SessionCookieName = "session"

// Claims is a struct that will be encoded to a JWT.
type Claims struct {
	Username string   `json:"username"`
	Roles    []string `json:"roles"`
	jwt.StandardClaims
}

// UseAuth validates a JWT from the session cookie and stores claims in context.
func UseAuth(pubKey *rsa.PublicKey, requirements ...Requirement) echo.MiddlewareFunc {
	reqs := append(requirements, EnforceTimes)

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			cookie, err := c.Cookie(SessionCookieName)
			if err != nil {
				return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
			}

			claims, err := ValidateToken(cookie.Value, pubKey)
			if err != nil {
				return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
			}

			for _, r := range reqs {
				if err := r(claims); err != nil {
					return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
				}
			}

			c.Set(ContextKeyClaims, claims)
			return next(c)
		}
	}
}

// GetClaims retrieves the JWT claims from the Echo context.
func GetClaims(c echo.Context) *Claims {
	v := c.Get(ContextKeyClaims)
	if v == nil {
		return nil
	}
	claims, ok := v.(*Claims)
	if !ok {
		return nil
	}
	return claims
}

// Requirement is a function that validates claims.
type Requirement func(claims *Claims) error

// EnforceTimes checks that the token is not expired and not issued in the future.
func EnforceTimes(claims *Claims) error {
	now := time.Now()
	issued := time.Unix(claims.StandardClaims.IssuedAt, 0)
	if issued.After(now) {
		return errors.New("invalid auth token")
	}
	expiration := time.Unix(claims.StandardClaims.ExpiresAt, 0)
	if expiration.Before(now) {
		return errors.New("expired auth token")
	}
	return nil
}

// EnforceRoles returns a requirement that checks the user has one of the given roles.
func EnforceRoles(roles ...string) Requirement {
	return func(claims *Claims) error {
		for _, role := range roles {
			for _, claimRole := range claims.Roles {
				if role == string(claimRole) {
					return nil
				}
			}
		}
		return errors.New("invalid role")
	}
}

// UseAuthRedirect redirects unauthenticated users to the given login URL.
// API routes (prefixed with "api/") are skipped and handled by UseAuth instead.
func UseAuthRedirect(pubKey *rsa.PublicKey, loginURL string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			path := c.Request().URL.Path
			// Skip API routes - they return JSON 401s via UseAuth
			if strings.HasPrefix(path, "/api/") || strings.HasPrefix(path, "api/") {
				return next(c)
			}
			// Skip static assets
			if strings.HasPrefix(path, "/static/") {
				return next(c)
			}

			// Build full URL including scheme and host so the auth service
			// can redirect back to this service after login.
			scheme := "http"
			if c.Request().TLS != nil {
				scheme = "https"
			}
			fullURL := scheme + "://" + c.Request().Host + c.Request().URL.String()

			cookie, err := c.Cookie(SessionCookieName)
			if err != nil || cookie.Value == "" {
				redirect := loginURL + "?redirect=" + url.QueryEscape(fullURL)
				return c.Redirect(http.StatusFound, redirect)
			}

			_, err = ValidateToken(cookie.Value, pubKey)
			if err != nil {
				redirect := loginURL + "?redirect=" + url.QueryEscape(fullURL)
				return c.Redirect(http.StatusFound, redirect)
			}

			return next(c)
		}
	}
}

// AuthCallbackHandler returns a handler that reads a JWT from the "token" query param,
// validates it, sets the session cookie on this service's domain, and redirects to "/".
func AuthCallbackHandler(pubKey *rsa.PublicKey) echo.HandlerFunc {
	return func(c echo.Context) error {
		tokenStr := c.QueryParam("token")
		if tokenStr == "" {
			return echo.NewHTTPError(http.StatusBadRequest, "missing token")
		}

		claims, err := ValidateToken(tokenStr, pubKey)
		if err != nil {
			return echo.NewHTTPError(http.StatusUnauthorized, "invalid token")
		}

		c.SetCookie(&http.Cookie{
			Name:    SessionCookieName,
			Value:   tokenStr,
			Path:    "/",
			Expires: time.Unix(claims.ExpiresAt, 0),
		})

		return c.Redirect(http.StatusFound, "/videos")
	}
}

// ValidateToken parses and validates a JWT token string.
func ValidateToken(tokenString string, pubKey *rsa.PublicKey) (*Claims, error) {
	if tokenString == "" {
		return nil, errors.New("invalid token")
	}
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (any, error) {
		return pubKey, nil
	})
	if err != nil {
		fmt.Println(err.Error())
		return nil, err
	}
	if !token.Valid {
		return nil, errors.New("invalid token")
	}
	claims, ok := token.Claims.(*Claims)
	if !ok {
		return nil, errors.New("invalid token")
	}
	return claims, nil
}

// extractClaims returns claims from context (set by UseAuth) if present,
// or decodes the session cookie JWT without signature verification.
// Signature verification is intentionally skipped — this is for identification only.
// Auth middleware handles actual verification.
func extractClaims(c echo.Context) *Claims {
	// if UseAuth func was called before this just get claims from context
	if claims := GetClaims(c); claims != nil {
		return claims
	}

	cookie, err := c.Cookie(SessionCookieName)
	if err != nil || cookie.Value == "" {
		return nil
	}
	parts := strings.SplitN(cookie.Value, ".", 3)
	if len(parts) != 3 {
		return nil
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil
	}
	var claims Claims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil
	}
	return &claims
}
