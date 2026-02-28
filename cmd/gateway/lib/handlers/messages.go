package handlers

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
	"github.com/mercury/pkg/clients/query"
	"github.com/mercury/pkg/clients/worker"
	"github.com/mercury/pkg/instrumentation"
	"github.com/mercury/pkg/middleware"
)

type MessageHandlers interface {
	SendMessage(c echo.Context) error
	GetMessages(c echo.Context) error
	RefreshMessages(c echo.Context) error
}

type messageHandlers struct {
	workerClient worker.WorkerClient
	queryClient  query.Client
}

func NewMessageHandlers(workerClient worker.WorkerClient, queryClient query.Client) MessageHandlers {
	return &messageHandlers{
		workerClient: workerClient,
		queryClient:  queryClient,
	}
}

type MessageRequest struct {
	ConversationID string `json:"conversation_id"`
	Body           string `json:"body"`
	User           string `json:"user"`
}

func (h *messageHandlers) SendMessage(c echo.Context) error {
	var req MessageRequest
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
	msgID, err := h.workerClient.SendChatMessage(c.Request().Context(), req.ConversationID, req.User, req.Body)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to enqueue message",
		})
	}
	return c.JSON(http.StatusOK, map[string]string{
		"status":     "queued",
		"message_id": msgID,
	})
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
	messages, err := h.queryClient.GetMessages(ctx, conversationID, query.GetMessagesProps{
		PageSize:  pageSize,
		NextToken: nextToken,
	})
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
	response, err := h.queryClient.RefreshMessages(ctx, conversationID, messageID)
	if err != nil {
		logger.WithError(err).Error("cassandra: get messages failed")
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to fetch messages",
		})
	}

	return c.JSON(http.StatusOK, response)
}
