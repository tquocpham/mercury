package handlers

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/mercury/cmd/worker/lib/managers"
	"github.com/mercury/pkg/clients/worker"
	"github.com/mercury/pkg/kmq"
	"github.com/segmentio/kafka-go"
)

type KafkaHandlers interface {
	SaveMessage(ctx context.Context, msg kafka.Message) (kmq.Result, error)
}

type kafkaHandlers struct {
	cassandraClient managers.CassandraClient
}

func NewKafkaHandlers(cassandraClient managers.CassandraClient) KafkaHandlers {
	return &kafkaHandlers{
		cassandraClient: cassandraClient,
	}
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
	return kmq.Success, nil
}
