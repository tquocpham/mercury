package kmq

import (
	"context"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"
	"github.com/sirupsen/logrus"
)

type Result int

const (
	Success Result = iota
	Retry
	DeadLetter
)

type Handler func(ctx context.Context, msg kafka.Message) (Result, error)

type requestIDCtxKey struct{}

type KMQConsumer interface {
	Close()
	Consume(handler Handler, middlewares ...Middleware)
}

type kmqConsumer struct {
	reader   *kafka.Reader
	logger   *logrus.Logger
	producer *Producer
	ctx      context.Context // loop lifetime — cancelled by Close()
	cancel   context.CancelFunc
}

func NewKafkaConsumer(
	brokers []string,
	groupID string,
	topic string,
	logger *logrus.Logger,
) KMQConsumer {
	ctx, cancel := context.WithCancel(context.Background())

	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        brokers,
		GroupID:        groupID,
		Topic:          topic,
		MinBytes:       1,
		MaxBytes:       10e6,
		CommitInterval: 0, // disable auto-commit (manual control)
	})

	producer := NewProducer(brokers)

	return &kmqConsumer{
		reader:   reader,
		logger:   logger,
		producer: producer,
		ctx:      ctx,
		cancel:   cancel,
	}
}

func (mq *kmqConsumer) Close() {
	mq.cancel()
	mq.reader.Close()
	mq.producer.Close()
}

func (c *kmqConsumer) Consume(handler Handler, middlewares ...Middleware) {
	// Apply middleware (right-to-left)
	h := handler
	for i := len(middlewares) - 1; i >= 0; i-- {
		h = middlewares[i](c.reader.Config().Topic, h)
	}

	c.logger.Infof("kafka consumer listening on (%s)", c.reader.Config().Topic)

	go func() {
		for {
			// FetchMessage blocks until a message arrives or the loop ctx is cancelled.
			msg, err := c.reader.FetchMessage(c.ctx)
			if err != nil {
				if c.ctx.Err() != nil {
					log.Println("consumer shutting down")
					return
				}
				log.Printf("fetch error: %v\n", err)
				continue
			}

			// Per-message context: timeout for processing this one message.
			// Derived from the loop ctx so shutdown also cancels in-flight work.
			msgCtx, cancel := context.WithTimeout(c.ctx, 5*time.Minute)
			msgCtx = context.WithValue(msgCtx, requestIDCtxKey{}, uuid.New().String())

			res, err := h(msgCtx, msg)
			if err != nil {
				cancel()
				c.logger.WithError(err).Error("handler execution failed")
				// Do NOT commit — let it retry after restart/rebalance.
				continue
			}

			switch res {
			case Success:
				// nothing extra
			case Retry:
				if err = c.producer.Produce(msgCtx, "messages.retry", msg); err != nil {
					cancel()
					c.logger.WithError(err).Error("retry produce failed")
					continue // do NOT commit
				}
			case DeadLetter:
				if err = c.producer.Produce(msgCtx, "messages.dlq", msg); err != nil {
					cancel()
					c.logger.WithError(err).Error("dlq produce failed")
					continue // do NOT commit
				}
			}
			cancel()

			if err := c.reader.CommitMessages(context.Background(), msg); err != nil {
				c.logger.WithError(err).Error("kafka: commit failed")
			}
		}
	}()
}
