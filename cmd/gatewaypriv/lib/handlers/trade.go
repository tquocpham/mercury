package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/mercury/pkg/clients/trade"
	"github.com/mercury/pkg/instrumentation"
)

type TradeHandlers interface {
	Trade(c echo.Context) error
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

func (h *tradeHandlers) Trade(c echo.Context) error {
	ctx := instrumentation.ToContext(c)
	request := &trade.TradeRequest{}
	if err := json.NewDecoder(c.Request().Body).Decode(request); err != nil {
		return echo.ErrBadRequest
	}
	response, err := h.tradeClient.Trade(ctx, request.OrderID, request.InitiatorID, request.Grants)
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
