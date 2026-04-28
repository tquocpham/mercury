package wallet

import (
	"errors"

	"github.com/labstack/echo/v4"
)

var (
	ErrInvalidRequest         = errors.New("failed to read request")
	ErrFailedToCreateResponse = errors.New("failed to create response")
	ErrFailedToGrantCurrency  = errors.New("failed to grant currency")
	ErrFailedToGetWallet      = errors.New("failed to get wallet")
	ErrWalletDoesNotExist     = errors.New("wallet does not exist")
)

func ConvertHttpError(err error) error {
	switch {
	case errors.Is(err, ErrInvalidRequest):
		return echo.ErrBadRequest
	case errors.Is(err, ErrWalletDoesNotExist):
		return echo.ErrNotFound
	case errors.Is(err, ErrFailedToGrantCurrency),
		errors.Is(err, ErrFailedToGetWallet),
		errors.Is(err, ErrFailedToCreateResponse):
		return echo.ErrInternalServerError
	default:
		return echo.ErrInternalServerError
	}
}
