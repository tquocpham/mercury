package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/mercury/cmd/entitlements/lib/managers"
	"github.com/mercury/pkg/instrumentation"
	"github.com/mercury/pkg/middleware"
)

type EntitlementHandlers interface {
	Check(c echo.Context) error
	Grant(c echo.Context) error
	Revoke(c echo.Context) error
}

type entitlementHandlers struct {
	grantsManager  managers.GrantsManager
	catalogManager managers.EntitlementsManager
}

func NewEntitlementHandlers(grantsManager managers.GrantsManager,
	catalogManager managers.EntitlementsManager) EntitlementHandlers {

	return &entitlementHandlers{
		grantsManager:  grantsManager,
		catalogManager: catalogManager,
	}
}

func (h *entitlementHandlers) Check(c echo.Context) error {
	return nil
}

type GrantRequest struct {
	AccountID     string    `json:"account_id"`
	EntitlementID string    `json:"entitlement_id"`
	ExpiresAt     time.Time `json:"expires_at,omitempty"`
}

func (h *entitlementHandlers) Grant(c echo.Context) error {
	ctx := instrumentation.ToContext(c)
	request := &GrantRequest{}
	if err := json.NewDecoder(c.Request().Body).Decode(request); err != nil {
		return echo.ErrUnauthorized
	}

	claims := middleware.GetClaims(c)
	if claims == nil {
		return c.JSON(http.StatusUnauthorized, echo.Map{"error": "cannot get user information"})
	}

	entitlement, err := h.catalogManager.GetEntitlement(ctx, request.EntitlementID)
	if err != nil {
		if errors.Is(err, managers.ErrEntitlementNotFound) {
			return c.JSON(http.StatusBadRequest, echo.Map{"error": "cannot find entitlement_id"})
		}
		return c.JSON(http.StatusInternalServerError, echo.Map{"error": "failed to get entitlement"})
	}

	// TODO: add expiry here. It should take the lesser of two expiries:
	// if the entitlement definition has an expiry it needs to expire it then.
	// if the user explicitly defintes an expiry time then we should choose the minimum of the two.
	// wait... if we set the expiry based on the entitlement definition, and then somebody
	// updates the definition with a diferent expiry then it will throw everything off...
	h.grantsManager.Grant(ctx, request.AccountID, entitlement.EntitlementID, claims.Username)

	return nil
}

func (h *entitlementHandlers) Revoke(c echo.Context) error {
	return nil
}
