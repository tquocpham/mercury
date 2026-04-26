package matchmaking

import "github.com/mercury/pkg/rmq"

var (
	ErrInvalidRequest             = rmq.NewError(3001, "failed to read request")
	ErrFailedToQueueParty         = rmq.NewError(3002, "failed to queue party")
	ErrFailedToRegisterGameserver = rmq.NewError(3003, "failed to register gameserver")
)
