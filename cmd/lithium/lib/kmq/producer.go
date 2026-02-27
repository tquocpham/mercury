package kmq

import (
	"context"
	"time"

	"github.com/segmentio/kafka-go"
)

type Producer struct {
	writer *kafka.Writer
}

func NewProducer(brokers []string) *Producer {
	return &Producer{
		writer: &kafka.Writer{
			Addr:         kafka.TCP(brokers...),
			Balancer:     &kafka.Hash{}, // required for keyed ordering
			RequiredAcks: kafka.RequireAll,
			Async:        false,
			BatchTimeout: 10 * time.Millisecond,
		},
	}
}

func (p *Producer) Close() error {
	return p.writer.Close()
}

func (p *Producer) Produce(
	ctx context.Context,
	topic string,
	original kafka.Message,
) error {

	newMsg := kafka.Message{
		Topic: topic,
		Key:   original.Key, // preserve partitioning
		Value: original.Value,
		Headers: append(original.Headers, kafka.Header{
			Key:   "x-retry-at",
			Value: []byte(time.Now().UTC().Format(time.RFC3339)),
		}),
		Time: time.Now(),
	}

	return p.writer.WriteMessages(ctx, newMsg)
}
