package auth

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/mercury/pkg/rmq"
)

var (
	ErrInvalidRequest          = rmq.NewError(1000, "failed to read request")
	ErrUnauthorized            = rmq.NewError(1001, "unauthorized")
	ErrFailedToQueryAccount    = rmq.NewError(1002, "failed to query account")
	ErrSessionCreationFailed   = rmq.NewError(1003, "session failed to create")
	ErrTokenSignatureFailed    = rmq.NewError(1004, "failed to create signed token")
	ErrAccountDuplicate        = rmq.NewError(1005, "username or email already taken")
	ErrAccountCreationFailed   = rmq.NewError(1006, "failed to create account")
	ErrAccountActivationFailed = rmq.NewError(1007, "failed to activate account")
	ErrNoSessionFound          = rmq.NewError(1008, "failed to find session")
	ErrSessionExtensionFailed  = rmq.NewError(1009, "failed to extend session")
	ErrSessionDeletionFailed   = rmq.NewError(1010, "failed to delete session")
)

func ConvertHttpError(err error) error {
	if converted := rmq.ConvertHttpError(err); converted != nil {
		return converted
	}
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
