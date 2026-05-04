package trade

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/mercury/pkg/rmq"
)

var (
	ErrInvalidRequest         = rmq.NewError(6000, "failed to read request")
	ErrFailedToCreateResponse = rmq.NewError(6001, "failed to create response")
	ErrFailedToCreateTrade    = rmq.NewError(6002, "failed to create trade")
	ErrFailedToGetTradeStatus = rmq.NewError(6003, "failed to get trade status")
	ErrOrderNotFound          = rmq.NewError(6004, "order not found")
	ErrTradeConflict          = rmq.NewError(6005, "trade conflict")
	ErrFailedToUpdateTrade    = rmq.NewError(6006, "failed to update trade")
)

func ConvertHttpError(err error) error {
	if converted := rmq.ConvertHttpError(err); converted != nil {
		return converted
	}
	switch {
	case errors.Is(err, ErrInvalidRequest):
		return echo.ErrBadRequest
	case errors.Is(err, ErrOrderNotFound):
		return echo.NewHTTPError(http.StatusNotFound)
	case errors.Is(err, ErrTradeConflict):
		return echo.NewHTTPError(http.StatusConflict)
	case errors.Is(err, ErrFailedToCreateTrade),
		errors.Is(err, ErrFailedToGetTradeStatus),
		errors.Is(err, ErrFailedToUpdateTrade),
		errors.Is(err, ErrFailedToCreateResponse):
		return echo.ErrInternalServerError
	default:
		return echo.ErrInternalServerError
	}
}
