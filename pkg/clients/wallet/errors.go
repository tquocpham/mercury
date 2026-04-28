package wallet

import (
	"errors"

	"github.com/labstack/echo/v4"
)

var (
	ErrInvalidRequest         = errors.New("failed to read request")
	ErrFailedToCreateResponse = errors.New("failed to create response")
	ErrFailedToGrantCurrency  = errors.New("failed to grant currency")
)

func ConvertHttpError(err error) error {
	switch {
	case errors.Is(err, ErrInvalidRequest):
		return echo.ErrBadRequest
	case errors.Is(err, ErrFailedToGrantCurrency),
		errors.Is(err, ErrFailedToCreateResponse):
		return echo.ErrInternalServerError
	default:
		return echo.ErrInternalServerError
	}
}
