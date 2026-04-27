package wallet

import "errors"

var (
	ErrInvalidRequest         = errors.New("failed to read request")
	ErrFailedToCreateResponse = errors.New("failed to create response")
	ErrFailedToGrantCurrency  = errors.New("failed to grant currency")
)
