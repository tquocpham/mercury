package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/mercury/pkg/clients/auth"
	"github.com/mercury/pkg/instrumentation"
)

type AuthHandlers interface {
	Login(c echo.Context) error
	Refresh(c echo.Context) error
	Revoke(c echo.Context) error
	CreateAccount(c echo.Context) error
	ActivateAccount(c echo.Context) error
}

type authHandlers struct {
	authClient auth.RMQClient
}

func NewAuthHandlers(
	authClient auth.RMQClient,
) AuthHandlers {
	return &authHandlers{
		authClient: authClient,
	}
}
func (h *authHandlers) Login(c echo.Context) error {
	ctx := instrumentation.ToContext(c)
	request := &auth.LoginRequest{}
	if err := json.NewDecoder(c.Request().Body).Decode(request); err != nil {
		return echo.ErrUnauthorized
	}
	creds := request.Credentials
	response, err := h.authClient.Login(ctx, creds.Username, creds.Password)
	if err != nil {
		return auth.ConvertHttpError(err)
	}
	return c.JSON(http.StatusOK, response)
}

func (h *authHandlers) Refresh(c echo.Context) error {
	ctx := instrumentation.ToContext(c)
	cookie, err := c.Cookie("session")
	if err != nil || cookie.Value == "" {
		return echo.ErrUnauthorized
	}
	response, err := h.authClient.Refresh(ctx, cookie.Value)
	if err != nil {
		return auth.ConvertHttpError(err)
	}
	return c.JSON(http.StatusOK, response)
}

func (h *authHandlers) Revoke(c echo.Context) error {
	ctx := instrumentation.ToContext(c)
	logger := instrumentation.LoggerFromContext(ctx)
	logger.Debug("not implemented")
	return nil
}

func (h *authHandlers) CreateAccount(c echo.Context) error {
	ctx := instrumentation.ToContext(c)
	request := &auth.AccountCreationRequest{}
	if err := json.NewDecoder(c.Request().Body).Decode(request); err != nil {
		return echo.ErrUnauthorized
	}
	response, err := h.authClient.CreateAccount(ctx, request.Username, request.Email, request.Password)
	if err != nil {
		return auth.ConvertHttpError(err)
	}
	return c.JSON(http.StatusOK, response)
}

func (h *authHandlers) ActivateAccount(c echo.Context) error {
	ctx := instrumentation.ToContext(c)
	accountID := c.Param("accountid")
	response, err := h.authClient.ActivateAccount(ctx, accountID)
	if err != nil {
		return auth.ConvertHttpError(err)
	}
	return c.JSON(http.StatusOK, response)
}
