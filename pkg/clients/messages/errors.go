package messages

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/mercury/pkg/rmq"
)

var (
	ErrInvalidRequest      = rmq.NewError(2000, "failed to read request")
	ErrFailedToSendMessage = rmq.NewError(2001, "failed to send message")
	ErrInvalidNextToken    = rmq.NewError(2002, "invalid next token")
	ErrFailedToGetMessages = rmq.NewError(2003, "failed to get messages")
	ErrTooManyMessages     = rmq.NewError(2004, "too many messages")
)

func ConvertHttpError(err error) error {
	if converted := rmq.ConvertHttpError(err); converted != nil {
		return converted
	}
	switch {
	case errors.Is(err, ErrInvalidRequest),
		errors.Is(err, ErrInvalidNextToken):
		return echo.ErrBadRequest
	case errors.Is(err, ErrTooManyMessages):
		return echo.NewHTTPError(http.StatusTooManyRequests)
	default:
		return echo.ErrInternalServerError
	}
}
