package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/mercury/pkg/clients/auth"
	"github.com/mercury/pkg/clients/matchmaking"
	"github.com/mercury/pkg/instrumentation"
)

type MatchmakingHandlers interface {
	QueueParty(c echo.Context) error
	GetQueue(c echo.Context) error
	RegisterGameserver(c echo.Context) error
}

type matchmakingHandlers struct {
	mmClient matchmaking.RMQClient
}

func NewMatchmakingHandlers(
	mmClient matchmaking.RMQClient,
) MatchmakingHandlers {
	return &matchmakingHandlers{
		mmClient: mmClient,
	}
}

func (h *matchmakingHandlers) QueueParty(c echo.Context) error {
	ctx := instrumentation.ToContext(c)
	request := &matchmaking.MatchmakingQueueRequest{}
	if err := json.NewDecoder(c.Request().Body).Decode(request); err != nil {
		return echo.ErrUnauthorized
	}
	response, err := h.mmClient.MatchmakingQueue(ctx, request.PartyID, request.PlayerIDs)
	if err != nil {
		return auth.ConvertHttpError(err)
	}
	return c.JSON(http.StatusOK, response)
}

func (h *matchmakingHandlers) GetQueue(c echo.Context) error {
	ctx := instrumentation.ToContext(c)
	partyID := c.Param("partyid")
	response, err := h.mmClient.GetQueue(ctx, partyID)
	if err != nil {
		return auth.ConvertHttpError(err)
	}
	return c.JSON(http.StatusOK, response)
}

func (h *matchmakingHandlers) RegisterGameserver(c echo.Context) error {
	ctx := instrumentation.ToContext(c)
	request := &matchmaking.GSRegisterRequest{}
	if err := json.NewDecoder(c.Request().Body).Decode(request); err != nil {
		return echo.ErrUnauthorized
	}
	response, err := h.mmClient.GameserverRegister(ctx, request.ServerID, request.IPAddress, request.Port, request.Capacity)
	if err != nil {
		return auth.ConvertHttpError(err)
	}
	return c.JSON(http.StatusOK, response)
}
