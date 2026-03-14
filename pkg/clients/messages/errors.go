package messages

import "errors"

var (
	ErrInvalidNextToken    = errors.New("invalid next token")
	ErrFailedToGetMessages = errors.New("failed to get messages")
	ErrTooManyMessages     = errors.New("top many messages")
)
