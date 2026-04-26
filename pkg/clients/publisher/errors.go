package publisher

import "github.com/mercury/pkg/rmq"

var (
	ErrInvalidRequest         = rmq.NewError(4001, "failed to read request")
	ErrFailedToCreateResponse = rmq.NewError(4002, "failed to create response")
	ErrFailedToSaveChannel    = rmq.NewError(4003, "failed to save channel")
)
