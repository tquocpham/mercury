package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
	"github.com/mercury/pkg/clients/auth"
	"github.com/mercury/pkg/clients/publisher"
	"github.com/mercury/pkg/instrumentation"
	"github.com/mercury/pkg/middleware"
	"github.com/redis/go-redis/v9"
)

// Upgrader is used to upgrade HTTP connections to WebSocket connections.
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type NotifierHandlers interface {
	NotifyClient(c echo.Context) error
}

type notifierHandlers struct {
	authClient  auth.RMQClient
	redisClient *redis.Client
}

func NewNotifierHandlers(authClient auth.RMQClient, redisClient *redis.Client) NotifierHandlers {
	return &notifierHandlers{
		authClient:  authClient,
		redisClient: redisClient,
	}
}

type NotificationEnvelope struct {
	Type    publisher.NotificationName `json:"type"`
	Payload any                        `json:"payload"`
}

func (h *notifierHandlers) NotifyClient(c echo.Context) error {
	ctx := instrumentation.ToContext(c)
	logger := middleware.GetLogger(c)
	w := c.Response()
	r := c.Request()

	// Validate before upgrading — returning HTTP errors after Upgrade() is not possible
	// because the connection has been hijacked.
	claims := middleware.GetClaims(c)
	if claims == nil {
		return echo.ErrUnauthorized
	}
	if _, err := h.authClient.GetSession(r.Context(), claims.SessionID); err != nil {
		logger.WithError(err).Error("session validation failed")
		return echo.ErrUnauthorized
	}
	logger.Info("Starting client ws connection")

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.WithError(err).Error("Error upgrading connection")
		return echo.ErrInternalServerError
	}
	defer conn.Close()

	// TODO: POPULATE THIS FROM REDIS
	subscribed := []string{
		publisher.UserChannel(claims.UserID),          // for sending pubsub commands
		fmt.Sprintf("conversation:%s", claims.UserID), // for sending system chat
		"conversation:abc123123",                      // testing channel
	}
	key := fmt.Sprintf("user:%s:channels", claims.UserID)
	channel := h.redisClient.SMembers(ctx, key)
	subbed, err := channel.Result()
	if err != nil {
		return echo.ErrInternalServerError
	}
	mergedsubs := append(subscribed, subbed...)

	pubsub := h.redisClient.Subscribe(r.Context(),
		mergedsubs...,
	)
	defer pubsub.Close()

	// poll Redis. Close connection if session has been revoked
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	go func() {
		for range ticker.C {
			if _, err := h.authClient.GetSession(r.Context(), claims.SessionID); err != nil {
				conn.WriteControl(
					websocket.CloseMessage,
					websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "session expired"),
					time.Now().Add(time.Second),
				)
				conn.Close()
				return
			}
		}
	}()

	go func() {
		for msg := range pubsub.Channel() {
			notification := &publisher.SendNotificationRequest{}
			if err := json.Unmarshal([]byte(msg.Payload), notification); err != nil {
				logger.Error("failed to parse message")
				continue
			}
			switch notification.Type {
			case publisher.SUBSCRIBE:
				payload := &publisher.SubscribePayload{}
				if err := json.Unmarshal(notification.Payload, payload); err != nil {
					logger.Error("failed to parse subscribe payload")
					continue
				}
				// TODO: Check here to make sure that we're not already subscribed
				channels := payload.Channels
				if len(channels) > 0 {
					if err := pubsub.Subscribe(r.Context(), channels...); err != nil {
						logger.WithError(err).Error("failed to subscribe to additional channels")
					}
				}
			case publisher.DISCONNECT:
				conn.WriteControl(
					websocket.CloseMessage,
					websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "session terminated"),
					time.Now().Add(time.Second),
				)
				conn.Close()
			case publisher.UNSUBSCRIBE:
				payload := &publisher.UnsubscribePayload{}
				if err := json.Unmarshal(notification.Payload, payload); err != nil {
					logger.Error("failed to parse subscribe payload")
					continue
				}
				// TODO: Check here to make sure that we're not already subscribed
				channels := payload.Channels
				if len(channels) > 0 {
					if err := pubsub.Unsubscribe(r.Context(), channels...); err != nil {
						logger.WithError(err).Error("failed to unsubscribe to additional channels")
					}
				}
			case publisher.MESSAGE:
				payload := &publisher.MessagePayload{}
				if err := json.Unmarshal(notification.Payload, payload); err != nil {
					logger.Error("failed to parse message payload")
					continue
				}
				notification, err := json.Marshal(&NotificationEnvelope{
					Type:    publisher.MESSAGE,
					Payload: payload,
				})
				if err != nil {
					logger.Error("failed to parse message payload")
					continue
				}
				if err := conn.WriteMessage(websocket.TextMessage, notification); err != nil {
					logger.WithError(err).Error("failed to send message to client")
					return
				}
			case publisher.TOAST:
				logger.Error("Not implemented")
			}
		}
	}()

	// block until disconnect
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			return nil
		}
	}
}
