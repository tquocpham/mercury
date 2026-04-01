package entitlements

import (
	"errors"
)

var (
	ErrInvalidRequest           = errors.New("failed to read request")
	ErrEntitlementNotFound      = errors.New("cannot find entitlement")
	ErrFailedToGetEntitlement   = errors.New("failed to get entitlement")
	ErrDuplicateGrant           = errors.New("entitlement already granted")
	ErrFailedToGrantEntitlement = errors.New("failed to grant entitlement")
)
