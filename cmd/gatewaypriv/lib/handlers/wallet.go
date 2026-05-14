package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/mercury/pkg/clients/wallet"
	"github.com/mercury/pkg/instrumentation"
)

type WalletHandlers interface {
	AddCurrency(c echo.Context) error
	GetWallet(c echo.Context) error
}

type walletHandlers struct {
	walletClient wallet.RMQClient
}

func NewWalletHandlers(
	walletClient wallet.RMQClient,
) WalletHandlers {
	return &walletHandlers{
		walletClient: walletClient,
	}
}

func (h *walletHandlers) AddCurrency(c echo.Context) error {
	ctx := instrumentation.ToContext(c)
	request := &wallet.AddCurrencyRequest{}
	if err := json.NewDecoder(c.Request().Body).Decode(request); err != nil {
		return echo.ErrBadRequest
	}
	response, err := h.walletClient.AddCurrency(ctx, request.PlayerID, request.CurrencyID, request.Amount, request.OrderID)
	if err != nil {
		return wallet.ConvertHttpError(err)
	}
	return c.JSON(http.StatusOK, response)
}

func (h *walletHandlers) GetWallet(c echo.Context) error {
	ctx := instrumentation.ToContext(c)
	playerID := c.Param("playerid")
	response, err := h.walletClient.GetWallet(ctx, playerID)
	if err != nil {
		return wallet.ConvertHttpError(err)
	}
	return c.JSON(http.StatusOK, response)
}
