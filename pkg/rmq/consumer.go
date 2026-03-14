package rmq

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
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
	mu      sync.Mutex
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
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}
	conn, err := amqp.Dial(c.amqpURL)
	if err != nil {
		return err
	}
	c.conn = conn
	return nil
}

func (c *Consumer) newChannel() (*amqp.Channel, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn == nil || c.conn.IsClosed() {
		return nil, fmt.Errorf("connection is closed")
	}
	return c.conn.Channel()
}

func (c *Consumer) Consume(queue string, handler Handler, middlewares ...Middleware) {
	// Apply middleware right-to-left so the first one listed is the outermost wrapper.
	h := handler
	for i := len(middlewares) - 1; i >= 0; i-- {
		h = middlewares[i](queue, h)
	}

	go func() {
		for {
			ch, msgs, err := c.startConsuming(queue)
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
					ch.Publish(
						"",
						msg.ReplyTo,
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

			ch.Close()
			c.logger.Warnf("mq: consumer channel closed for %s, reconnecting...", queue)
			if err := c.connect(); err != nil {
				c.logger.WithError(err).Error("mq: reconnect failed, retrying in 5s")
				time.Sleep(5 * time.Second)
			}
		}
	}()
}

func (c *Consumer) startConsuming(queue string) (*amqp.Channel, <-chan amqp.Delivery, error) {
	ch, err := c.newChannel()
	if err != nil {
		return nil, nil, err
	}
	if _, err := ch.QueueDeclare(queue, true, false, false, false, nil); err != nil {
		ch.Close()
		return nil, nil, err
	}
	msgs, err := ch.Consume(queue, "", false, false, false, false, nil)
	if err != nil {
		ch.Close()
		return nil, nil, err
	}
	return ch, msgs, nil
}

func (c *Consumer) Healthy() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.conn != nil && !c.conn.IsClosed()
}

func (c *Consumer) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn != nil {
		c.conn.Close()
	}
}

// Wait blocks until SIGINT or SIGTERM is received, then logs shutdown.
func (c *Consumer) Wait() {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	c.logger.Info("shutting down")
}
