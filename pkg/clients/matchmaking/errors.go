package matchmaking

import "github.com/mercury/pkg/rmq"

var (
	ErrInvalidRequest             = rmq.NewError(4000, "failed to read request")
	ErrFailedToQueueParty         = rmq.NewError(4001, "failed to queue party")
	ErrFailedToRegisterGameserver = rmq.NewError(4002, "failed to register gameserver")
)
