package rmq

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"
)

type Publisher struct {
	amqpURL string
	conn    *amqp.Connection
	channel *amqp.Channel
	pending map[string]chan []byte
	mu      sync.Mutex
}

func NewPublisher(amqpURL string) (*Publisher, error) {
	p := &Publisher{
		amqpURL: amqpURL,
		pending: make(map[string]chan []byte),
	}
	if err := p.connect(); err != nil {
		return nil, err
	}
	return p, nil
}

func (p *Publisher) connect() error {
	// Establish a new TCP connection to the broker.
	conn, err := amqp.Dial(p.amqpURL)
	if err != nil {
		return err
	}
	// Open a multiplexed channel over the connection.
	// Most AMQP operations (declare, publish, consume) happen on a channel, not the connection.
	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return err
	}
	// Subscribe to the pseudo-queue "amq.rabbitmq.reply-to".
	// RabbitMQ's Direct Reply-to feature routes reply messages back to this consumer
	// without needing to declare a temporary reply queue. auto-ack (4th arg) is required
	// by the broker for this queue; exclusive (5th arg) ensures only this channel can use it.
	replies, err := ch.Consume("amq.rabbitmq.reply-to", "", true, true, false, false, nil)
	if err != nil {
		ch.Close()
		conn.Close()
		return err
	}
	// Fan replies out to the callers waiting in Request().
	// Each in-flight request registers a channel in p.pending keyed by its correlation ID.
	// When the broker delivers a reply, we look up the waiting caller and unblock it.
	// The goroutine exits naturally when the replies channel is closed (connection drop).
	go func() {
		for msg := range replies {
			p.mu.Lock()
			pending, ok := p.pending[msg.CorrelationId]
			p.mu.Unlock()
			if ok {
				pending <- msg.Body
			}
		}
	}()
	p.conn = conn
	p.channel = ch
	return nil
}

func (p *Publisher) ensureConnected() error {
	if p.conn != nil && !p.conn.IsClosed() {
		return nil
	}
	return p.connect()
}

// publishLocked declares the queue and publishes a message. Must be called with p.mu held.
func (p *Publisher) publishLocked(queue string, msg amqp.Publishing) error {
	if err := p.ensureConnected(); err != nil {
		return err
	}
	if _, err := p.channel.QueueDeclare(queue, true, false, false, false, nil); err != nil {
		return err
	}
	return p.channel.Publish(
		"",    // default exchange
		queue, // routing key = queue name
		false, // mandatory
		false, // immediate
		msg,
	)
}

func (p *Publisher) Publish(queue string, body []byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.publishLocked(queue, amqp.Publishing{
		ContentType:  "application/json",
		Body:         body,
		DeliveryMode: amqp.Persistent, // survives broker restart
	})
}

func (p *Publisher) Request(queue string, body []byte) ([]byte, error) {
	corrID := uuid.New().String()
	ch := make(chan []byte, 1)

	// Hold the lock for connection check, QueueDeclare, and Publish.
	// Release before blocking on the reply so the reply goroutine can acquire it.
	p.mu.Lock()
	p.pending[corrID] = ch
	err := p.publishLocked(queue, amqp.Publishing{
		ContentType:   "application/json",
		CorrelationId: corrID,
		ReplyTo:       "amq.rabbitmq.reply-to",
		Body:          body,
	})
	if err != nil {
		delete(p.pending, corrID)
		p.mu.Unlock()
		return nil, err
	}
	p.mu.Unlock()

	defer func() {
		p.mu.Lock()
		delete(p.pending, corrID)
		p.mu.Unlock()
	}()

	select {
	case resp := <-ch:
		return resp, nil
	case <-time.After(30 * time.Second):
		return nil, fmt.Errorf("mq: request timed out")
	}
}

func (p *Publisher) Close() {
	p.channel.Close()
	p.conn.Close()
}
