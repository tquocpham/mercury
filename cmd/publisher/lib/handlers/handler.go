package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/mercury/pkg/clients/publisher"
	"github.com/redis/go-redis/v9"
)

type PublisherHandlers interface {
	SendNotification(c echo.Context) error
}

type publisherHandlers struct {
	redisClient *redis.Client
}

func NewPublisherHandlers(redisClient *redis.Client) PublisherHandlers {
	return &publisherHandlers{
		redisClient: redisClient,
	}
}

type WebSocketRequest struct {
	// TODO validate token
	Token    string   `json:"token"`
	Channels []string `json:"channels"`
}

func (h *publisherHandlers) SendNotification(c echo.Context) error {
	request := &publisher.SendNotificationRequest{}
	if err := json.NewDecoder(c.Request().Body).Decode(request); err != nil {
		return echo.ErrBadRequest
	}

	channel := request.Channel
	notified := h.redisClient.Publish(c.Request().Context(), channel, []byte(request.Payload))
	return c.JSON(http.StatusOK, &publisher.SendNotificationResponse{
		Notified: notified.Val(),
	})
}
