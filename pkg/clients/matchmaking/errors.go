package matchmaking

import (
	"errors"
)

var (
	ErrInvalidRequest             = errors.New("failed to read request")
	ErrFailedToQueueParty         = errors.New("failed to queue party")
	ErrFailedToRegisterGameserver = errors.New("failed to register gameserver")
)
