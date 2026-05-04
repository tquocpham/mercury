package wallet

import (
	"errors"

	"github.com/labstack/echo/v4"
	"github.com/mercury/pkg/rmq"
)

var (
	ErrInvalidRequest         = rmq.NewError(7000, "failed to read request")
	ErrFailedToCreateResponse = rmq.NewError(7001, "failed to create response")
	ErrFailedToGrantCurrency  = rmq.NewError(7002, "failed to grant currency")
	ErrFailedToGetWallet      = rmq.NewError(7003, "failed to get wallet")
	ErrWalletDoesNotExist     = rmq.NewError(7004, "wallet does not exist")
)

func ConvertHttpError(err error) error {
	if converted := rmq.ConvertHttpError(err); converted != nil {
		return converted
	}
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
