package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/mercury/pkg/clients/trade"
	"github.com/mercury/pkg/instrumentation"
)

type TradeHandlers interface {
	DraftTrade(c echo.Context) error
	LockTrade(c echo.Context) error
	UnlockTrade(c echo.Context) error
	DispatchGrants(c echo.Context) error
	GetTradeStatus(c echo.Context) error
}

type tradeHandlers struct {
	tradeClient trade.RMQClient
}

func NewTradeHandlers(
	tradeClient trade.RMQClient,
) TradeHandlers {
	return &tradeHandlers{
		tradeClient: tradeClient,
	}
}

func (h *tradeHandlers) DraftTrade(c echo.Context) error {
	ctx := instrumentation.ToContext(c)
	request := &trade.DraftTradeRequest{}
	if err := json.NewDecoder(c.Request().Body).Decode(request); err != nil {
		return echo.ErrBadRequest
	}
	response, err := h.tradeClient.DraftTrade(ctx,
		request.OrderID,
		request.PlayerID,
		request.InitiatorID,
		request.TransactionID,
		request.ContractingParties,
		request.Grants,
	)
	if err != nil {
		return trade.ConvertHttpError(err)
	}
	return c.JSON(http.StatusOK, response)
}

func (h *tradeHandlers) LockTrade(c echo.Context) error {
	ctx := instrumentation.ToContext(c)
	request := &trade.LockTradeRequest{}
	if err := json.NewDecoder(c.Request().Body).Decode(request); err != nil {
		return echo.ErrBadRequest
	}
	response, err := h.tradeClient.LockTrade(ctx, request.OrderID, request.PlayerID, request.TransactionID)
	if err != nil {
		return trade.ConvertHttpError(err)
	}
	return c.JSON(http.StatusOK, response)
}

func (h *tradeHandlers) UnlockTrade(c echo.Context) error {
	ctx := instrumentation.ToContext(c)
	request := &trade.UnlockTradeRequest{}
	if err := json.NewDecoder(c.Request().Body).Decode(request); err != nil {
		return echo.ErrBadRequest
	}
	response, err := h.tradeClient.UnlockTrade(ctx, request.OrderID, request.PlayerID, request.TransactionID)
	if err != nil {
		return trade.ConvertHttpError(err)
	}
	return c.JSON(http.StatusOK, response)
}

func (h *tradeHandlers) DispatchGrants(c echo.Context) error {
	ctx := instrumentation.ToContext(c)
	request := &trade.DispatchGrantsRequest{}
	if err := json.NewDecoder(c.Request().Body).Decode(request); err != nil {
		return echo.ErrBadRequest
	}
	response, err := h.tradeClient.DispatchGrants(ctx, request.OrderID, request.InitiatorID, request.Grants)
	if err != nil {
		return trade.ConvertHttpError(err)
	}
	return c.JSON(http.StatusOK, response)
}

func (h *tradeHandlers) GetTradeStatus(c echo.Context) error {
	ctx := instrumentation.ToContext(c)
	orderID := c.Param("orderid")
	response, err := h.tradeClient.TradeStatus(ctx, orderID)
	if err != nil {
		return trade.ConvertHttpError(err)
	}
	return c.JSON(http.StatusOK, response)
}
