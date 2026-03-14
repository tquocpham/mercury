package rmq

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/sirupsen/logrus"
)

type Handler func(ctx context.Context, body []byte) ([]byte, error)

type Consumer struct {
	amqpURL string
	conn    *amqp.Connection
	channel *amqp.Channel
	logger  *logrus.Logger
}

func NewConsumer(amqpURL string, logger *logrus.Logger) (*Consumer, error) {
	c := &Consumer{amqpURL: amqpURL, logger: logger}
	if err := c.connect(); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Consumer) connect() error {
	// When the msgs channel closes (broker dropped it),
	// c.connect() is called which creates a new connection,
	// but the old c.conn is never closed first.
	// This will leak the old connections.
	if c.channel != nil {
		c.channel.Close()
		c.channel = nil
	}
	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}
	conn, err := amqp.Dial(c.amqpURL)
	if err != nil {
		return err
	}
	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return err
	}
	c.conn = conn
	c.channel = ch
	return nil
}

func (c *Consumer) Consume(queue string, handler Handler, middlewares ...Middleware) {
	// Apply middleware right-to-left so the first one listed is the outermost wrapper.
	h := handler
	for i := len(middlewares) - 1; i >= 0; i-- {
		h = middlewares[i](queue, h)
	}

	go func() {
		for {
			msgs, err := c.startConsuming(queue)
			if err != nil {
				c.logger.WithError(err).Errorf("mq: failed to start consuming %s, reconnecting in 5s", queue)
				time.Sleep(5 * time.Second)
				if err := c.connect(); err != nil {
					c.logger.WithError(err).Error("mq: reconnect failed")
				}
				continue
			}

			c.logger.Infof("mq consumer listening on %s", queue)

			for msg := range msgs {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
				ctx = context.WithValue(ctx, requestIDKey, uuid.New().String())
				response, err := h(ctx, msg.Body)
				cancel()
				if err != nil {
					c.logger.WithError(err).Error("mq: handler error, nacking")
					msg.Nack(false, !msg.Redelivered)
					continue
				}

				msg.Ack(false)
				if msg.ReplyTo != "" && response != nil {
					c.channel.Publish(
						"",          // default exchange
						msg.ReplyTo, // reply queue
						false,
						false,
						amqp.Publishing{
							ContentType:   "application/json",
							CorrelationId: msg.CorrelationId,
							Body:          response,
						},
					)
				}
			}

			c.logger.Warnf("mq: consumer channel closed for %s, reconnecting...", queue)
			if err := c.connect(); err != nil {
				c.logger.WithError(err).Error("mq: reconnect failed, retrying in 5s")
				time.Sleep(5 * time.Second)
			}
		}
	}()
}

func (c *Consumer) startConsuming(queue string) (<-chan amqp.Delivery, error) {
	_, err := c.channel.QueueDeclare(queue, true, false, false, false, nil)
	if err != nil {
		return nil, err
	}
	return c.channel.Consume(queue, "", false, false, false, false, nil)
}

func (c *Consumer) Healthy() bool {
	return c.conn != nil && !c.conn.IsClosed() && c.channel != nil && !c.channel.IsClosed()
}

func (c *Consumer) Close() {
	c.channel.Close()
	c.conn.Close()
}

// Wait blocks until SIGINT or SIGTERM is received, then logs shutdown.
func (c *Consumer) Wait() {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	c.logger.Info("shutting down")
}
