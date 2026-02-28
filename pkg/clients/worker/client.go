package worker

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/mercury/pkg/kmq"
	"github.com/segmentio/kafka-go"
)

// ChatData is the struct that is sent through the kafka queue
type ChatMessage struct {
	User    string `json:"user"`
	Message string `json:"message"`
}

type WorkerClient interface {
	SendChatMessage(ctx context.Context, conversationID, user, message string) (string, error)
}

type workerClient struct {
	topic    string
	producer *kmq.Producer
}

func NewClient(topic string, producer *kmq.Producer) WorkerClient {
	return &workerClient{
		topic:    topic,
		producer: producer,
	}
}

func (c *workerClient) SendChatMessage(ctx context.Context, conversationID, user, message string) (string, error) {
	msgID := uuid.New().String()
	var chatData = &ChatMessage{
		User:    user,
		Message: message,
	}
	chatDataBytes, err := json.Marshal(chatData)
	if err != nil {
		return "", err
	}
	kmsg := kafka.Message{
		Topic: c.topic,
		Key:   []byte(conversationID), // critical for ordering
		Value: chatDataBytes,
		Headers: []kafka.Header{
			{Key: "message_id", Value: []byte(msgID)},
		},
		Time: time.Now(),
	}
	if err := c.producer.Produce(ctx, c.topic, kmsg); err != nil {
		return "", err
	}
	return msgID, nil
}
