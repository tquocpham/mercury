package handlers

import (
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"slices"
	"time"

	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
	"github.com/mercury/pkg/clients/auth"
	"github.com/mercury/pkg/clients/publisher"
	"github.com/mercury/pkg/middleware"
	"github.com/redis/go-redis/v9"
)

// Upgrader is used to upgrade HTTP connections to WebSocket connections.
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func stringsFromAny(v any) ([]string, bool) {
	slice, ok := v.([]any)
	if !ok {
		return nil, false
	}
	out := make([]string, len(slice))
	for i, s := range slice {
		out[i], ok = s.(string)
		if !ok {
			return nil, false
		}
	}
	return out, true
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

type WebSocketRequest struct {
	Token string `json:"token"`
}

type WebSocketRPC struct {
	Version     string         `json:"ver"`
	Command     string         `json:"cmd"`
	ReferenceID string         `json:"ref"`
	Token       string         `json:"token"`
	Payload     map[string]any `json:"payload"`
	// Channels []string `json:"channels"`
}

func (h *notifierHandlers) NotifyClient(c echo.Context) error {
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

	subscribed := map[string]bool{
		fmt.Sprintf("client:%s", claims.UserID):       true, // for sending pubsub commands
		fmt.Sprintf("conversation:%s", claims.UserID): true, // for sending system chat
		"conversation:abc123123":                      true,
	}
	keys := slices.Collect(maps.Keys(subscribed))
	pubsub := h.redisClient.Subscribe(r.Context(),
		keys...,
	)
	defer pubsub.Close()

	// poll Redis every 30s; close connection if session has been revoked
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
			notification := &publisher.Notification{}
			if err := json.Unmarshal([]byte(msg.Payload), notification); err != nil {
				logger.Error("failed to parse message")
				continue
			}
			switch notification.Type {
			case publisher.COMMAND:
				command, ok := notification.Payload["cmd"]
				if !ok {
					logger.Error("failed to get cmd from payload")
					continue
				}
				switch command {
				case "subscribe":
					channels, ok := stringsFromAny(notification.Payload["channels"])
					if !ok {
						logger.Error("failed to get channels from payload")
						continue
					}
					var newChannels []string
					for _, ch := range channels {
						if !subscribed[ch] {
							newChannels = append(newChannels, ch)
							subscribed[ch] = true
						}
					}
					if len(newChannels) > 0 {
						if err := pubsub.Subscribe(r.Context(), newChannels...); err != nil {
							logger.WithError(err).Error("failed to subscribe to additional channels")
						}
					}
				case "disconnect":
					conn.WriteControl(
						websocket.CloseMessage,
						websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "session terminated"),
						time.Now().Add(time.Second),
					)
					conn.Close()
				}
				continue
			}
			if err := conn.WriteMessage(websocket.TextMessage, []byte(msg.Payload)); err != nil {
				logger.WithError(err).Error("failed to send message to client")
				return
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
