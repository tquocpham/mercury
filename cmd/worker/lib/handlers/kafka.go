package handlers

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/mercury/cmd/worker/lib/managers"
	"github.com/mercury/pkg/clients/notifier"
	"github.com/mercury/pkg/clients/worker"
	"github.com/mercury/pkg/kmq"
	"github.com/segmentio/kafka-go"
)

type KafkaHandlers interface {
	SaveMessage(ctx context.Context, msg kafka.Message) (kmq.Result, error)
}

type kafkaHandlers struct {
	cassandraClient managers.CassandraClient
	notifierClient  notifier.Client
}

func NewKafkaHandlers(cassandraClient managers.CassandraClient, notifierClient notifier.Client) KafkaHandlers {
	return &kafkaHandlers{
		cassandraClient: cassandraClient,
		notifierClient:  notifierClient,
	}
}

type Message struct {
	Message string `json:"message"`
}

func (h *kafkaHandlers) SaveMessage(ctx context.Context, msg kafka.Message) (kmq.Result, error) {
	logger := kmq.LoggerFromContext(ctx)
	conversationID := string(msg.Key)
	var chatData = &worker.ChatMessage{}
	if err := json.Unmarshal(msg.Value, chatData); err != nil {
		logger.Infof("failed to unmarshal chatdata. skipping forever")
		return kmq.Success, nil
	}

	// Extract message_id from headers.
	messageID := ""
	for _, h := range msg.Headers {
		if h.Key == "message_id" {
			messageID = string(h.Value)
			break
		}
	}
	if messageID == "" {
		messageID = uuid.New().String()
	}

	if err := h.cassandraClient.SaveMessage(conversationID, messageID, chatData.User, chatData.Message, msg.Time); err != nil {
		logger.WithError(err).Error("cassandra: save message failed")
		return kmq.Retry, err
	}
	logger.Debug("sending notification")
	_, err := h.notifierClient.SendNotification(
		ctx,
		fmt.Sprintf("conversation:%s", conversationID),
		"Message",
		string(msg.Value),
	)
	if err != nil {
		return kmq.Retry, err
	}
	return kmq.Success, nil
}
