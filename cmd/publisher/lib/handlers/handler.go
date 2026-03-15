package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/mercury/pkg/clients/publisher"
	"github.com/mercury/pkg/instrumentation"
	"github.com/redis/go-redis/v9"
)

type PublisherHandlers interface {
	SendNotification(c echo.Context) error
	Subscribe(c echo.Context) error
}

type publisherHandlers struct {
	redisClient *redis.Client
}

func NewPublisherHandlers(redisClient *redis.Client) PublisherHandlers {
	return &publisherHandlers{
		redisClient: redisClient,
	}
}

func (h *publisherHandlers) SendNotification(c echo.Context) error {
	ctx := instrumentation.ToContext(c)
	request := &publisher.SendNotificationRequest{}
	if err := json.NewDecoder(c.Request().Body).Decode(request); err != nil {
		return echo.ErrBadRequest
	}
	bts, err := json.Marshal(request)
	if err != nil {
		return echo.ErrBadRequest
	}

	notified := h.redisClient.Publish(ctx, request.Channel, bts)
	return c.JSON(http.StatusOK, &publisher.SendNotificationResponse{
		Notified: notified.Val(),
	})
}

func (h *publisherHandlers) Subscribe(c echo.Context) error {
	ctx := instrumentation.ToContext(c)
	logger := instrumentation.LoggerFromContext(ctx)
	request := &publisher.SubscribeRequest{}
	if err := json.NewDecoder(c.Request().Body).Decode(request); err != nil {
		return echo.ErrBadRequest
	}
	userID := request.UserID
	key := fmt.Sprintf("user:%s:channels", userID)
	pipe := h.redisClient.Pipeline()
	channels := make([]interface{}, len(request.Channels))
	for i, ch := range request.Channels {
		channels[i] = ch
	}
	pipe.SAdd(ctx, key, channels...)
	pipe.SMembers(ctx, key)
	results, err := pipe.Exec(ctx)
	if err != nil {
		return echo.ErrInternalServerError
	}
	members := results[1].(*redis.StringSliceCmd).Val()
	response := &publisher.SubscribeResponse{
		Channels: members,
	}

	userChannel := publisher.UserChannel(userID)
	payload, err := json.Marshal(publisher.SubscribePayload{
		Channels: request.Channels,
	})
	if err != nil {
		logger.WithError(err).Info("failed to marshal payload")
		return c.JSON(http.StatusOK, response)
	}
	referenceID := uuid.New().String()
	bts, err := json.Marshal(&publisher.SendNotificationRequest{
		Channel:     userChannel,
		Type:        publisher.SUBSCRIBE,
		Payload:     payload,
		Version:     "1",
		ReferenceID: referenceID,
	})
	if err != nil {
		logger.WithError(err).Info("failed to marshal SendNotificationRequest")
		return c.JSON(http.StatusOK, response)
	}
	h.redisClient.Publish(ctx, userChannel, bts)
	return c.JSON(http.StatusOK, response)
}
