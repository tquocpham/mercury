package handlers

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/mercury/pkg/clients/publisher"
	"github.com/mercury/pkg/rmq"
	"github.com/redis/go-redis/v9"
)

type RMQHandlers interface {
	SendNotification(ctx context.Context, body []byte) ([]byte, error)
	Subscribe(ctx context.Context, body []byte) ([]byte, error)
}

type rmqHanders struct {
	redisClient *redis.Client
}

func NewRMQHandlers(redisClient *redis.Client) RMQHandlers {
	return &rmqHanders{
		redisClient: redisClient,
	}
}

func (h *rmqHanders) SendNotification(ctx context.Context, body []byte) ([]byte, error) {
	logger := rmq.GetLogger(ctx)
	request := &publisher.SendNotificationRequest{}
	if err := json.Unmarshal(body, request); err != nil {
		logger.WithError(err).Error("failed to unmarshal send notification request")
		return nil, publisher.ErrInvalidRequest
	}
	bts, err := json.Marshal(request)
	if err != nil {
		logger.WithError(err).Error("failed to marshal send notification request")
		return nil, publisher.ErrInvalidRequest
	}
	notified := h.redisClient.Publish(ctx, request.Channel, bts)
	resp, err := json.Marshal(publisher.SendNotificationResponse{
		Notified: notified.Val(),
	})
	if err != nil {
		return nil, publisher.ErrFailedToCreateResponse
	}
	return resp, nil
}

func (h *rmqHanders) Subscribe(ctx context.Context, body []byte) ([]byte, error) {
	logger := rmq.GetLogger(ctx)
	request := &publisher.SubscribeRequest{}
	if err := json.Unmarshal(body, request); err != nil {
		logger.WithError(err).Error("failed to unmarshal send notification request")
		return nil, publisher.ErrInvalidRequest
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
		return nil, publisher.ErrFailedToSaveChannel
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
		resp, err := json.Marshal(response)
		if err != nil {
			return nil, publisher.ErrFailedToCreateResponse
		}
		return resp, nil
	}
	referenceID := uuid.New().String()
	bts, err := json.Marshal(&publisher.SendNotificationRequest{
		Channel:     userChannel,
		Type:        publisher.SUBSCRIBE,
		Payload:     payload,
		ReferenceID: referenceID,
	})
	if err != nil {
		logger.WithError(err).Info("failed to marshal SendNotificationRequest")
		return json.Marshal(response)
	}
	h.redisClient.Publish(ctx, userChannel, bts)
	resp, err := json.Marshal(response)
	if err != nil {
		return nil, publisher.ErrFailedToCreateResponse
	}
	return resp, nil
}
