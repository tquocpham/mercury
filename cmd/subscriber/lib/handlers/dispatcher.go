package handlers

import (
	"context"
	"encoding/json"

	"github.com/mercury/pkg/clients/publisher"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
)

func OnSubscribe(
	ctx context.Context, logger *logrus.Entry, notification *publisher.SendNotificationRequest,
	pubsub *redis.PubSub, send chan<- []byte, done chan<- struct{}) error {

	payload := &publisher.SubscribePayload{}
	if err := json.Unmarshal(notification.Payload, payload); err != nil {
		return err
	}
	// TODO: Check here to make sure that we're not already subscribed
	channels := payload.Channels
	if len(channels) > 0 {
		if err := pubsub.Subscribe(ctx, channels...); err != nil {
			return err
		}
	}
	return nil
}

func OnUnsubscribe(
	ctx context.Context, logger *logrus.Entry, notification *publisher.SendNotificationRequest,
	pubsub *redis.PubSub, send chan<- []byte, done chan<- struct{}) error {

	payload := &publisher.UnsubscribePayload{}
	if err := json.Unmarshal(notification.Payload, payload); err != nil {
		return err
	}
	// TODO: Check here to make sure that we're not already subscribed
	channels := payload.Channels
	if len(channels) > 0 {
		if err := pubsub.Unsubscribe(ctx, channels...); err != nil {
			return err
		}
	}
	return nil
}

func OnDisconnect(
	ctx context.Context, logger *logrus.Entry, notification *publisher.SendNotificationRequest,
	pubsub *redis.PubSub, send chan<- []byte, done chan<- struct{}) error {

	close(done)
	return nil
}

func OnMessage(
	ctx context.Context, logger *logrus.Entry, notification *publisher.SendNotificationRequest,
	pubsub *redis.PubSub, send chan<- []byte, done chan<- struct{}) error {

	payload := &publisher.MessagePayload{}
	if err := json.Unmarshal(notification.Payload, payload); err != nil {
		return err
	}
	envelope, err := json.Marshal(&NotificationEnvelope{
		Type:    publisher.MESSAGE,
		Payload: payload,
	})
	if err != nil {
		return err
	}
	send <- envelope
	return nil
}

func OnToast(
	ctx context.Context, logger *logrus.Entry, notification *publisher.SendNotificationRequest,
	pubsub *redis.PubSub, send chan<- []byte, done chan<- struct{}) error {

	logger.Error("Not implemented")
	return nil
}
