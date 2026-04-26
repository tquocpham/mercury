package entitlements

import "github.com/mercury/pkg/rmq"

var (
	ErrInvalidRequest            = rmq.NewError(5001, "failed to read request")
	ErrFailedToCreateResponse    = rmq.NewError(5002, "failed to create response")
	ErrEntitlementNotFound       = rmq.NewError(5003, "cannot find entitlement")
	ErrFailedToGetEntitlement    = rmq.NewError(5004, "failed to get entitlement")
	ErrDuplicateGrant            = rmq.NewError(5005, "entitlement already granted")
	ErrFailedToGrantEntitlement  = rmq.NewError(5006, "failed to grant entitlement")
	ErrFailedToCreateEntitlement = rmq.NewError(5007, "failed to create entitlement")
)
