package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/mercury/pkg/clients/auth"
	"github.com/mercury/pkg/clients/matchmaking"
	"github.com/mercury/pkg/instrumentation"
)

type GameserverHandlers interface {
	Register(c echo.Context) error
	Unregister(c echo.Context) error
}

type gameserverHandlers struct {
	mmClient matchmaking.RMQClient
}

func NewGameserverHandlers(mmClient matchmaking.RMQClient) GameserverHandlers {
	return &gameserverHandlers{mmClient: mmClient}
}

func (h *gameserverHandlers) Register(c echo.Context) error {
	ctx := instrumentation.ToContext(c)
	request := &matchmaking.GSRegisterRequest{}
	if err := json.NewDecoder(c.Request().Body).Decode(request); err != nil {
		return echo.ErrBadRequest
	}
	response, err := h.mmClient.GameserverRegister(ctx, request.ServerID, request.IPAddress, request.Port, request.Capacity)
	if err != nil {
		return auth.ConvertHttpError(err)
	}
	return c.JSON(http.StatusOK, response)
}

func (h *gameserverHandlers) Unregister(c echo.Context) error {
	ctx := instrumentation.ToContext(c)
	request := &matchmaking.GSUnregisterRequest{}
	if err := json.NewDecoder(c.Request().Body).Decode(request); err != nil {
		return echo.ErrBadRequest
	}
	response, err := h.mmClient.GameserverUnregister(ctx, request.ServerID, request.Version)
	if err != nil {
		return auth.ConvertHttpError(err)
	}
	return c.JSON(http.StatusOK, response)
}
