package handlers

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/mercury/cmd/messages/lib/managers"
	"github.com/mercury/pkg/clients/publisher"
	"github.com/mercury/pkg/clients/query"
	"github.com/mercury/pkg/clients/worker"
	"github.com/mercury/pkg/middleware"
	"github.com/mercury/pkg/server"
	"github.com/redis/go-redis/v9"
)

type MessageHandlers interface {
	GetMessages(c echo.Context) error
	SendMessage(c echo.Context) error
	RefreshMessages(c echo.Context) error
}

type messageHandlers struct {
	cassandraClient managers.CassandraClient
	publisherClient publisher.Client
	workerClient    worker.WorkerClient
	redisClient     *redis.Client
}

func NewMessageHandlers(
	cassandraClient managers.CassandraClient,
	publisherClient publisher.Client,
	workerClient worker.WorkerClient,
	redisClient *redis.Client,
) MessageHandlers {
	return &messageHandlers{
		cassandraClient: cassandraClient,
		publisherClient: publisherClient,
		workerClient:    workerClient,
		redisClient:     redisClient,
	}
}

func (h *messageHandlers) GetMessages(c echo.Context) error {
	conversationID := c.QueryParam("conversation_id")
	if conversationID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "conversation_id query param required",
		})
	}
	pageSizeStr := c.QueryParam("page_size")
	pageSize := 10
	if pageSizeStr != "" {
		parsed, parseErr := strconv.Atoi(pageSizeStr)
		if parseErr != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"error": "Invalid page_size",
			})
		}
		pageSize = parsed
	}

	// Assume 'tokenStr' comes from your API request
	nextToken := c.QueryParam("next_token")
	pagingState := []byte(nil)
	if nextToken != "" {
		parsed, parsedErr := base64.StdEncoding.DecodeString(nextToken)
		if parsedErr != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"error": "Invalid next_token",
			})
		}
		pagingState = parsed
	}

	messages, err := h.cassandraClient.GetMessages(conversationID, pageSize, pagingState)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to fetch messages",
		})
	}
	msgs := make([]query.MessageResponse, len(messages.Messages))
	for i, msg := range messages.Messages {
		msgs[i] = query.MessageResponse{
			ConversationID: msg.ConversationID,
			MessageID:      msg.MessageID,
			Body:           msg.Body,
			User:           msg.User,
			CreatedAt:      msg.CreatedAt,
		}
	}

	respNextToken := ""
	if len(messages.Next) > 0 {
		respNextToken = base64.StdEncoding.EncodeToString(messages.Next)
	}

	return c.JSON(http.StatusOK, &query.GetMessagesResponse{
		Messages:  msgs,
		NextToken: respNextToken,
	})
}

func (h *messageHandlers) SendMessage(c echo.Context) error {
	logger := middleware.GetLogger(c)
	ctx := c.Request().Context()

	var req query.SendMessageRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid request",
		})
	}
	if req.ConversationID == "" || req.Body == "" || req.User == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "conversation_id, user and body required",
		})
	}
	user := req.User

	if !server.Limit(
		h.redisClient,
		ctx,
		200,
		time.Hour,
		user,
		req.ConversationID,
	) {
		return c.JSON(http.StatusTooManyRequests, map[string]string{
			"error": "too many requests",
		})
	}

	// if direct message this tells the recievers to subscribe to the
	// pubsub associated with the chat to get updates
	for _, userID := range req.To {
		resp, err := h.publisherClient.SendNotification(ctx,
			fmt.Sprintf("client:%s", userID),
			publisher.COMMAND,
			map[string]any{
				"cmd":      "subscribe",
				"channels": []string{fmt.Sprintf("conversation:%s", req.ConversationID)},
			},
		)
		if err != nil {
			logger.WithError(err).Errorf("failed to notify user %s to subscribe", userID)
			continue
		}
		if resp.Notified == 0 {
			h.workerClient.SendChatMessage(
				ctx, user, "system", fmt.Sprintf("user %s is offline", userID))
		}
	}
	msgID, err := h.workerClient.SendChatMessage(
		ctx, req.ConversationID, user, req.Body)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to enqueue message",
		})
	}
	return c.JSON(http.StatusOK, query.SendMessageResponse{
		Status:    "queued",
		MessageID: msgID,
	})
}

func (h *messageHandlers) RefreshMessages(c echo.Context) error {
	conversationID := c.QueryParam("conversation_id")
	if conversationID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "conversation_id query param required",
		})
	}
	messageID := c.QueryParam("message_id")

	messages, err := h.cassandraClient.RefreshMessages(conversationID, messageID)
	if err != nil {
		// logger.WithError(err).Error("cassandra: get messages failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to fetch messages",
		})
	}
	msgs := make([]query.MessageResponse, len(messages.Messages))
	for i, msg := range messages.Messages {
		msgs[i] = query.MessageResponse{
			ConversationID: msg.ConversationID,
			MessageID:      msg.MessageID,
			Body:           msg.Body,
			User:           msg.User,
			CreatedAt:      msg.CreatedAt,
		}
	}

	respNextToken := ""
	if len(messages.Next) > 0 {
		respNextToken = base64.StdEncoding.EncodeToString(messages.Next)
	}

	return c.JSON(http.StatusOK, &query.GetMessagesResponse{
		Messages:  msgs,
		NextToken: respNextToken,
	})
}
