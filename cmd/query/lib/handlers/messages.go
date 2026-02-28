package handlers

import (
	"encoding/base64"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
	"github.com/mercury/cmd/query/lib/managers"
	"github.com/mercury/pkg/clients/query"
)

type MessageHandlers interface {
	GetMessages(c echo.Context) error
	RefreshMessages(c echo.Context) error
}

type messageHandlers struct {
	cassandraClient managers.CassandraClient
}

func NewMessageHandlers(cassandraClient managers.CassandraClient) MessageHandlers {
	return &messageHandlers{
		cassandraClient: cassandraClient,
	}
}

type MessageRequest struct {
	ConversationID string `json:"conversation_id"`
	Body           string `json:"body"`
	User           string `json:"user"`
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
