package trade

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"
)

var (
	ErrInvalidRequest         = errors.New("failed to read request")
	ErrFailedToCreateResponse = errors.New("failed to create response")
	ErrFailedToCreateTrade    = errors.New("failed to create trade")
	ErrFailedToGetTradeStatus = errors.New("failed to get trade status")
	ErrOrderNotFound          = errors.New("order not found")
)

func ConvertHttpError(err error) error {
	switch {
	case errors.Is(err, ErrInvalidRequest):
		return echo.ErrBadRequest
	case errors.Is(err, ErrOrderNotFound):
		return echo.NewHTTPError(http.StatusNotFound)
	case errors.Is(err, ErrFailedToCreateTrade),
		errors.Is(err, ErrFailedToGetTradeStatus),
		errors.Is(err, ErrFailedToCreateResponse):
		return echo.ErrInternalServerError
	default:
		return echo.ErrInternalServerError
	}
}
