package publisher

import "errors"

var (
	ErrInvalidRequest      = errors.New("failed to read request")
	ErrFailedToSaveChannel = errors.New("failed to save channel")
)
