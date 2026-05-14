package inventory

import (
	"errors"

	"github.com/labstack/echo/v4"
	"github.com/mercury/pkg/rmq"
)

var (
	ErrInvalidRequest         = rmq.NewError(8000, "failed to read request")
	ErrFailedToCreateResponse = rmq.NewError(8001, "failed to create response")
	ErrFailedToAddItem        = rmq.NewError(8002, "failed to add item")
	ErrFailedToGetInventory   = rmq.NewError(8003, "failed to get inventory")
	ErrInventoryDoesNotExist  = rmq.NewError(8004, "inventory does not exist")
	ErrInventoryFull          = rmq.NewError(8005, "inventory full")
	ErrSlotNotAvailable       = rmq.NewError(8006, "slot not available")
)

func ConvertRPCError(err error) error {
	switch {
	case errors.Is(err, ErrInventoryDoesNotExist):
		return errors.New("inventory not found")
	case errors.Is(err, ErrInventoryFull):
		return errors.New("inventory full")
	case errors.Is(err, ErrSlotNotAvailable):
		return errors.New("slot not available")
	default:
		return errors.New("internal error")
	}
}

func ConvertHttpError(err error) error {
	if converted := rmq.ConvertHttpError(err); converted != nil {
		return converted
	}
	switch {
	case errors.Is(err, ErrInvalidRequest):
		return echo.ErrBadRequest
	case errors.Is(err, ErrInventoryDoesNotExist):
		return echo.ErrNotFound
	case errors.Is(err, ErrInventoryFull),
		errors.Is(err, ErrSlotNotAvailable):
		return echo.NewHTTPError(409, err.Error())
	case errors.Is(err, ErrFailedToGetInventory),
		errors.Is(err, ErrFailedToAddItem),
		errors.Is(err, ErrFailedToCreateResponse):
		return echo.ErrInternalServerError
	default:
		return echo.ErrInternalServerError
	}
}
