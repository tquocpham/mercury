package trade

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/mercury/pkg/rmq"
)

var (
	ErrInvalidRequest         = rmq.NewError("invalid_request", "failed to read request")
	ErrFailedToCreateResponse = rmq.NewError("failed_to_create_response", "failed to create response")
	ErrFailedToCreateTrade    = rmq.NewError("failed_to_create_trade", "failed to create trade")
	ErrFailedToGetTradeStatus = rmq.NewError("failed_to_get_trade_status", "failed to get trade status")
	ErrOrderNotFound          = rmq.NewError("order_not_found", "order not found")
	ErrTradeConflict          = rmq.NewError("trade_conflict", "trade conflict")
	ErrFailedToUpdateTrade    = rmq.NewError("failed_to_update_trade", "failed to update trade")
)

func ConvertHttpError(err error) error {
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
