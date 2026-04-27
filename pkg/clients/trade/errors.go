package trade

import "errors"

var (
	ErrInvalidRequest         = errors.New("failed to read request")
	ErrFailedToCreateResponse = errors.New("failed to create response")
	ErrFailedToCreateTrade    = errors.New("failed to create trade")
	ErrFailedToGetTradeStatus = errors.New("failed to get trade status")
	ErrOrderNotFound          = errors.New("order not found")
)
