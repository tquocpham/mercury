package handlers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"github.com/mercury/cmd/messages/lib/managers"
	"github.com/mercury/pkg/clients/messages"
	"github.com/mercury/pkg/clients/publisher"
	"github.com/mercury/pkg/clients/worker"
	"github.com/mercury/pkg/rmq"
	"github.com/mercury/pkg/server"
	"github.com/redis/go-redis/v9"
)

type RMQHandlers interface {
	GetMessages(ctx context.Context, body []byte) ([]byte, error)
	SendMessage(ctx context.Context, body []byte) ([]byte, error)
	RefreshMessages(ctx context.Context, body []byte) ([]byte, error)
}
type rmqHanders struct {
	cassandraClient managers.CassandraClient
	publisherClient publisher.RMQClient
	workerClient    worker.WorkerClient
	redisClient     *redis.Client
}

func NewRMQHandlers(
	cassandraClient managers.CassandraClient,
	publisherClient publisher.RMQClient,
	workerClient worker.WorkerClient,
	redisClient *redis.Client,
) RMQHandlers {
	return &rmqHanders{
		cassandraClient: cassandraClient,
		publisherClient: publisherClient,
		workerClient:    workerClient,
		redisClient:     redisClient,
	}
}

func (h *rmqHanders) GetMessages(ctx context.Context, body []byte) ([]byte, error) {
	logger := rmq.GetLogger(ctx)
	request := &messages.GetMessagesRequest{}
	if err := json.Unmarshal(body, request); err != nil {
		logger.WithError(err).Error("failed to unmarshal get messages request")
		return nil, messages.ErrInvalidRequest
	}
	pagingState := []byte(nil)
	nextToken := request.NextToken
	if nextToken != "" {
		parsed, parsedErr := base64.StdEncoding.DecodeString(nextToken)
		if parsedErr != nil {
			logger.WithError(parsedErr).Error("failed to decode next token")
			return nil, messages.ErrInvalidNextToken
		}
		pagingState = parsed
	}
	msgHistory, err := h.cassandraClient.GetMessages(request.ConversationID, request.Limit, pagingState)
	if err != nil {
		logger.WithError(err).Error("failed to get messages from cassandra")
		return nil, messages.ErrFailedToGetMessages
	}
	msgs := make([]messages.MessageResponse, len(msgHistory.Messages))
	for i, msg := range msgHistory.Messages {
		msgs[i] = messages.MessageResponse{
			ConversationID: msg.ConversationID,
			MessageID:      msg.MessageID,
			Body:           msg.Body,
			User:           msg.User,
			CreatedAt:      msg.CreatedAt,
		}
	}
	respNextToken := ""
	if len(msgHistory.Next) > 0 {
		respNextToken = base64.StdEncoding.EncodeToString(msgHistory.Next)
	}
	return json.Marshal(messages.GetMessagesResponse{
		Messages:  msgs,
		NextToken: respNextToken,
	})
}

func (h *rmqHanders) SendMessage(ctx context.Context, body []byte) ([]byte, error) {
	logger := rmq.GetLogger(ctx)
	request := &messages.SendMessageRequest{}
	if err := json.Unmarshal(body, request); err != nil {
		logger.WithError(err).Error("failed to unmarshal send message request")
		return nil, messages.ErrInvalidRequest
	}
	user := request.User
	if !server.Limit(
		h.redisClient,
		ctx,
		200,
		time.Hour,
		user,
		request.ConversationID,
	) {
		return nil, messages.ErrTooManyMessages
	}
	// if direct message this tells the recievers to subscribe to the
	// pubsub associated with the chat to get updates
	for _, userID := range request.To {
		channel := fmt.Sprintf("conversation:%s", request.ConversationID)
		_, err := h.publisherClient.Subscribe(ctx, userID, []string{channel})
		if err != nil {
			logger.WithError(err).Errorf("failed to notify user %s to subscribe", userID)
			// TODO: build a mechanims for eventual consistency here
			continue
		}
	}
	msgID, err := h.workerClient.SendChatMessage(
		ctx, request.ConversationID, user, request.Body)
	if err != nil {
		logger.WithError(err).Error("failed to send chat message")
		return nil, messages.ErrFailedToSendMessage
	}
	return json.Marshal(messages.SendMessageResponse{
		Status:    "queued",
		MessageID: msgID,
	})
}

func (h *rmqHanders) RefreshMessages(ctx context.Context, body []byte) ([]byte, error) {
	logger := rmq.GetLogger(ctx)
	request := &messages.RefreshMessagesRequest{}
	if err := json.Unmarshal(body, request); err != nil {
		logger.WithError(err).Error("failed to unmarshal refresh messages request")
		return nil, messages.ErrInvalidRequest
	}
	msgHistory, err := h.cassandraClient.RefreshMessages(request.ConversationID, request.MessageID)
	if err != nil {
		logger.WithError(err).Error("get messages failed")
		return nil, messages.ErrFailedToGetMessages
	}
	msgs := make([]messages.MessageResponse, len(msgHistory.Messages))
	for i, msg := range msgHistory.Messages {
		msgs[i] = messages.MessageResponse{
			ConversationID: msg.ConversationID,
			MessageID:      msg.MessageID,
			Body:           msg.Body,
			User:           msg.User,
			CreatedAt:      msg.CreatedAt,
		}
	}

	return json.Marshal(messages.RefreshMessagesResponse{
		Messages: msgs,
	})
}
