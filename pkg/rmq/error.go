package rmq

// Error is a serializable error that travels over RMQ.
// Sentinels defined with NewError are matched via the Code field,
// so errors.Is works correctly after deserialization.
type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func NewError(code, message string) *Error {
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
