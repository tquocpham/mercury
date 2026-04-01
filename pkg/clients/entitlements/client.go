package entitlements

import "time"

type GrantRequest struct {
	AccountID     string    `json:"account_id"`
	EntitlementID string    `json:"entitlement_id"`
	Version       int       `json:"version"`
	ExpiresAt     time.Time `json:"expires_at,omitempty"`
}

type EntitlementPrice struct {
	// Amount is the cost amount
	Amount int `json:"amount"`
	// Currency is the type of currency the entitlement cost, "gold", "USD" etc.
	Currency string `json:"currency"`
}

type CreateEntitlementRequest struct {
	// Description is the description of the entitlement.
	Description string `json:"description"`
	// Name is the entitlement name
	Name string `json:"name"`
	// Category is a string allowing creators to denote entitlement type such as "skin", "consumable", "modifier" etc.
	// Catalog categories provide a basic structure for organizing catalog items, assisting in organizing the inventory.
	// and providing a user-friendly shopping experience
	Category string `json:"type"`
	// Price is contains the price data
	Price    EntitlementPrice       `json:"price"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

type GrantResponse struct {
	ID            string                 `json:"id"`
	EntitlementID string                 `json:"entitlement_id"`
	CommitID      string                 `json:"commit_id"` // idempotency
	Count         int                    `json:"count"`
	AccountID     string                 `json:"account_id"`
	State         string                 `json:"state"`
	GrantedAt     time.Time              `json:"granted_at"`
	RevokedAt     time.Time              `json:"revoked_at"`
	ExpiresAt     time.Time              `json:"expires_at"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}
