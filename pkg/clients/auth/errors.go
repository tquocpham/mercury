package auth

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/mercury/pkg/rmq"
)

var (
	ErrInvalidRequest          = rmq.NewError(1001, "failed to read request")
	ErrFailedToCreateResponse  = rmq.NewError(1002, "failed to create response")
	ErrUnauthorized            = rmq.NewError(1003, "unauthorized")
	ErrFailedToQueryAccount    = rmq.NewError(1004, "failed to query account")
	ErrSessionCreationFailed   = rmq.NewError(1005, "session failed to create")
	ErrTokenSignatureFailed    = rmq.NewError(1006, "failed to create signed token")
	ErrAccountDuplicate        = rmq.NewError(1007, "username or email already taken")
	ErrAccountCreationFailed   = rmq.NewError(1008, "failed to create account")
	ErrAccountActivationFailed = rmq.NewError(1009, "failed to activate account")
	ErrNoSessionFound          = rmq.NewError(1010, "failed to find session")
	ErrSessionExtensionFailed  = rmq.NewError(1011, "failed to extend session")
	ErrSessionDeletionFailed   = rmq.NewError(1012, "failed to delete session")
)

func ConvertHttpError(err error) error {
	switch {
	case errors.Is(err, ErrInvalidRequest):
		return echo.ErrBadRequest
	case errors.Is(err, ErrUnauthorized):
		return echo.ErrUnauthorized
	case errors.Is(err, ErrAccountDuplicate):
		return echo.NewHTTPError(http.StatusConflict)
	default:
		return echo.ErrInternalServerError
	}
}
