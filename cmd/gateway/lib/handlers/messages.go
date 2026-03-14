package handlers

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
	"github.com/mercury/pkg/clients/messages"
	"github.com/mercury/pkg/instrumentation"
	"github.com/mercury/pkg/middleware"
)

type MessageHandlers interface {
	SendMessage(c echo.Context) error
	GetMessages(c echo.Context) error
	RefreshMessages(c echo.Context) error
}

type messageHandlers struct {
	messagesClient messages.RMQClient
}

func NewMessageHandlers(
	messagesClient messages.RMQClient,
) MessageHandlers {
	return &messageHandlers{
		messagesClient: messagesClient,
	}
}

type MessageRequest struct {
	ConversationID string   `json:"conversation_id"`
	Body           string   `json:"body"`
	To             []string `json:"to"`
}

func (h *messageHandlers) SendMessage(c echo.Context) error {
	ctx := instrumentation.ToContext(c)
	var req MessageRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid request",
		})
	}
	if req.ConversationID == "" || req.Body == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "conversation_id, user and body required",
		})
	}

	claims := middleware.GetClaims(c)
	if claims == nil {
		return c.JSON(http.StatusUnauthorized, echo.Map{"error": "cannot get user information"})
	}

	response, err := h.messagesClient.SendMessage(ctx,
		req.ConversationID,
		req.Body,
		claims.Username,
		claims.UserID,
		req.To,
	)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to send messages",
		})
	}

	return c.JSON(http.StatusOK, response)
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
		if parsed <= 0 || parsed >= 1000000 {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"error": "Invalid page_size",
			})
		}
		pageSize = parsed
	}

	nextToken := c.QueryParam("next_token")
	ctx := instrumentation.ToContext(c)
	messages, err := h.messagesClient.GetMessages(ctx, conversationID, pageSize, nextToken)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to fetch messages",
		})
	}
	return c.JSON(http.StatusOK, messages)
}

func (h *messageHandlers) RefreshMessages(c echo.Context) error {
	logger := middleware.GetLogger(c)
	conversationID := c.QueryParam("conversation_id")
	if conversationID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "conversation_id query param required",
		})
	}
	messageID := c.QueryParam("message_id")
	if messageID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "Invalid message_id",
		})
	}
	ctx := instrumentation.ToContext(c)
	response, err := h.messagesClient.RefreshMessages(ctx, conversationID, messageID)
	if err != nil {
		logger.WithError(err).Error("cassandra: get messages failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to fetch messages",
		})
	}

	return c.JSON(http.StatusOK, response)
}
