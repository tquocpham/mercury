package messages

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"
)

var (
	ErrInvalidRequest      = errors.New("failed to read request")
	ErrFailedToSendMessage = errors.New("failed to send message")
	ErrInvalidNextToken    = errors.New("invalid next token")
	ErrFailedToGetMessages = errors.New("failed to get messages")
	ErrTooManyMessages     = errors.New("too many messages")
)

func ConvertHttpError(err error) error {
	switch {
	case errors.Is(err, ErrInvalidRequest),
		errors.Is(err, ErrInvalidNextToken):
		return echo.ErrBadRequest
	case errors.Is(err, ErrTooManyMessages):
		return echo.NewHTTPError(http.StatusTooManyRequests)
	case errors.Is(err, ErrFailedToGetMessages),
		errors.Is(err, ErrFailedToSendMessage):
		return echo.ErrInternalServerError
	default:
		return echo.ErrInternalServerError
	}
}
