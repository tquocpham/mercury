package auth

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"
)

var (
	ErrUnauthorized            = errors.New("unauthorized")
	ErrSessionCreationFailed   = errors.New("session failed to create")
	ErrTokenSignatureFailed    = errors.New("failed to create signed token")
	ErrAccountDuplicate        = errors.New("username or email already taken")
	ErrAccountCreationFailed   = errors.New("failed to create account")
	ErrAccountActivationFailed = errors.New("failed to activate account")
	ErrNoSessionFound          = errors.New("failed to find session")
	ErrSessionExtensionFailed  = errors.New("failed to extend session")
	ErrSessionDeletionFailed   = errors.New("failed to delete session")
)

func ConvertHttpError(err error) error {
	switch {
	case errors.Is(err, ErrUnauthorized):
		return echo.ErrUnauthorized
	case errors.Is(err, ErrAccountDuplicate):
		return echo.NewHTTPError(http.StatusConflict)
	case errors.Is(err, ErrAccountCreationFailed),
		errors.Is(err, ErrSessionCreationFailed),
		errors.Is(err, ErrTokenSignatureFailed),
		errors.Is(err, ErrNoSessionFound),
		errors.Is(err, ErrSessionExtensionFailed),
		errors.Is(err, ErrSessionDeletionFailed),
		errors.Is(err, ErrAccountActivationFailed):
		return echo.ErrInternalServerError
	default:
		return echo.ErrInternalServerError
	}
}
