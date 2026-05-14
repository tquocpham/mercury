package rmq

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"
)

// Sentinels defined with NewError are matched via the Code field,
// so errors.Is works correctly after deserialization.
type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func NewError(code int, message string) *Error {
	if code == 0 {
		panic("Error code cannot be 0")
	}
	return &Error{
		Code:    code,
		Message: message,
	}
}

func (e *Error) Error() string { return e.Message }

func (e *Error) Is(target error) bool {
	t, ok := target.(*Error)
	return ok && e.Code == t.Code
}

// ConvertHttpError handles system-level RMQ errors (codes 1-999).
// Returns nil for service-level errors (codes >= 1000) so the caller can handle them.
func ConvertHttpError(err error) error {
	var rmqErr *Error
	if !errors.As(err, &rmqErr) {
		return nil
	}
	if rmqErr.Code >= 1000 {
		return nil
	}
	switch rmqErr.Code {
	case 503:
		return echo.NewHTTPError(http.StatusServiceUnavailable)
	default:
		return echo.ErrInternalServerError
	}
}

type responseType string

const (
	envelopeVersion     = 1
	responseTypeSuccess responseType = "success"
	responseTypeError   responseType = "error"
)

type envelope struct {
	Version  int             `json:"version"`
	Type     responseType    `json:"response_type"`
	Response json.RawMessage `json:"response"`
}

func wrapSuccess(data []byte) ([]byte, error) {
	return json.Marshal(envelope{
		Version:  envelopeVersion,
		Type:     responseTypeSuccess,
		Response: json.RawMessage(data),
	})
}

func wrapError(err *Error) ([]byte, error) {
	errBytes, merr := json.Marshal(err)
	if merr != nil {
		return nil, merr
	}
	return json.Marshal(envelope{
		Version:  envelopeVersion,
		Type:     responseTypeError,
		Response: json.RawMessage(errBytes),
	})
}
