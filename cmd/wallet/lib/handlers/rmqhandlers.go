package handlers

import (
	"context"
	"encoding/json"

	"github.com/mercury/cmd/wallet/lib/managers"
	"github.com/mercury/pkg/clients/wallet"
	"github.com/mercury/pkg/ids"
	"github.com/mercury/pkg/rmq"
)

type RMQHandlers interface {
	AddCurrency(ctx context.Context, body []byte) ([]byte, error)
}

type rmqHanders struct {
	walletManager managers.WalletManager
}

func NewRMQHandlers(walletManager managers.WalletManager) RMQHandlers {
	return &rmqHanders{
		walletManager: walletManager,
	}
}

func (h *rmqHanders) AddCurrency(ctx context.Context, body []byte) ([]byte, error) {
	logger := rmq.GetLogger(ctx)
	request := &wallet.AddCurrencyRequest{}
	if err := json.Unmarshal(body, request); err != nil {
		logger.WithError(err).Error("failed to parse wallet request")
		return nil, wallet.ErrInvalidRequest
	}
	if !ids.ValidateOrderID(request.OrderID) {
		logger.WithField("order_id", request.OrderID).Error("order_id must be a valid ULID")
		return nil, wallet.ErrInvalidRequest
	}

	walletInfo, err := h.walletManager.Grant(
		ctx, request.PlayerID, request.CurrencyID,
		request.Amount, request.OrderID)
	if err != nil {
		return nil, wallet.ErrFailedToGrantCurrency
	}
	currencies := make([]wallet.Currency, 0, len(walletInfo.Currencies))
	for currencyType, amount := range walletInfo.Currencies {
		currencies = append(currencies, wallet.Currency{
			CurrencyType: currencyType,
			Amount:       amount,
		})
	}

	bts, err := json.Marshal(wallet.GetWalletResponse{
		PlayerID:   walletInfo.PlayerID,
		Currencies: currencies,
	})
	if err != nil {
		return nil, wallet.ErrFailedToCreateResponse
	}
	return bts, nil
}
