package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
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

type NotifierHandlers interface {
	NotifyClient(c echo.Context) error
}

type notifierHandlers struct {
	redisClient *redis.Client
}

func NewNotifierHandlers(redisClient *redis.Client) NotifierHandlers {
	return &notifierHandlers{
		redisClient: redisClient,
	}
}

// TODO validate token
type WebSocketRequest struct {
	Token    string   `json:"token"`
	Channels []string `json:"channels"`
}

func (h *notifierHandlers) NotifyClient(c echo.Context) error {
	logger := middleware.GetLogger(c)
	w := c.Response()
	r := c.Request()

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.WithError(err).Error("Error upgrading connection")
		return echo.ErrInternalServerError
	}
	defer conn.Close()

	_, raw, err := conn.ReadMessage()
	if err != nil {
		logger.WithError(err).Error("Error reading websocket message")
		return nil
	}

	var request WebSocketRequest
	if err := json.Unmarshal(raw, &request); err != nil {
		logger.WithError(err).Error("Error parsing WebSocketRequest")
		conn.WriteMessage(websocket.TextMessage, []byte(`{"error":"Error parsing WebSocketRequest"}`))
		return nil
	}
	if len(request.Channels) == 0 {
		conn.WriteMessage(websocket.TextMessage, []byte(`{"error":"no channels"}`))
		return nil
	}

	// subscribe to Redis and stream
	pubsub := h.redisClient.Subscribe(r.Context(), request.Channels...)
	defer pubsub.Close()

	go func() {
		for msg := range pubsub.Channel() {
			notification := &publisher.Notification{}
			if err := json.Unmarshal([]byte(msg.Payload), notification); err != nil {
				logger.Error("failed to parse message")
				continue
			}
			if err := conn.WriteMessage(websocket.TextMessage, []byte(msg.Payload)); err != nil {
				return
			}
		}
	}()

	// block until disconnect
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			return nil
		}
	}
}
