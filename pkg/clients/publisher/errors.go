package publisher

import "github.com/mercury/pkg/rmq"

var (
	ErrInvalidRequest      = rmq.NewError(3000, "failed to read request")
	ErrFailedToSaveChannel = rmq.NewError(3001, "failed to save channel")
)
