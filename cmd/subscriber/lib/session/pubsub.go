package session

import (
	"context"
	"encoding/json"
	"time"

	"github.com/gorilla/websocket"
	"github.com/mercury/pkg/clients/publisher"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
	"github.com/smira/go-statsd"
)

// Handler processes a notification. Write to send to deliver messages to the
// WebSocket client. Use pubsub to subscribe or unsubscribe from channels.
// Close done to signal the session should disconnect.
type PubSubHandler func(
	ctx context.Context, logger *logrus.Entry, notification *publisher.SendNotificationRequest,
	pubsub *redis.PubSub, send chan<- []byte, done chan<- struct{}) error

type PubSubDispatcher interface {
	RegisterOnSub(notificationType publisher.NotificationName, h PubSubHandler)
	// Start spawns the dispatch and writer goroutines. Must be called before Listen.
	Start(
		ctx context.Context, logger *logrus.Entry, metrics *statsd.Client, conn *websocket.Conn,
		pubsub *redis.PubSub)
}

type pubSubDispatcher struct {
	handlers map[publisher.NotificationName]PubSubHandler
	subQueue chan *publisher.SendNotificationRequest
	send     chan []byte
}

func NewPubSubDispatcher() PubSubDispatcher {
	return &pubSubDispatcher{
		handlers: map[publisher.NotificationName]PubSubHandler{},
		subQueue: make(chan *publisher.SendNotificationRequest, 100),
		send:     make(chan []byte, 256),
	}
}

func (p *pubSubDispatcher) RegisterOnSub(notificationType publisher.NotificationName, h PubSubHandler) {
	p.handlers[notificationType] = h
}

// Start spawns two goroutines:
//  1. A writer that is the sole owner of websocket writes, draining p.send.
//  2. A dispatcher that routes items from subQueue to registered handlers.
//
// When subQueue is closed (by Listen's goroutine when the Redis channel closes),
// the dispatcher exits and closes send, which causes the writer to exit.
func (p *pubSubDispatcher) Start(
	ctx context.Context, logger *logrus.Entry, metrics *statsd.Client, conn *websocket.Conn,
	pubsub *redis.PubSub) {

	done := make(chan struct{})

	go func() {
		for {
			select {
			case msg, ok := <-p.send:
				if !ok {
					return
				}
				if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
					logger.WithError(err).Error("failed to write to websocket")
					return
				}
			case <-done:
				conn.WriteControl(
					websocket.CloseMessage,
					websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "session terminated"),
					time.Now().Add(time.Second),
				)
				conn.Close()
				return
			}
		}
	}()

	go func() {
		defer close(p.send)
		for item := range p.subQueue {
			handler, ok := p.handlers[item.Type]
			subLogger := logger.WithFields(logrus.Fields{
				"refid": item.ReferenceID,
				"type":  item.Type,
				"chan":  item.Channel,
				"cmd":   item.Command,
			})
			if !ok {
				subLogger.Error("no handler registered for notification type")
				continue
			}
			// TODO: add some statsd metrics here
			if err := handler(ctx, subLogger, item, pubsub, p.send, done); err != nil {
				subLogger.WithError(err).Error("handler error")
			}
		}
	}()

	go func() {
		defer close(p.subQueue)
		for msg := range pubsub.Channel() {
			notification := &publisher.SendNotificationRequest{}
			if err := json.Unmarshal([]byte(msg.Payload), notification); err != nil {
				logger.Error("failed to parse pubsub message to SendNotificationRequest")
				continue
			}
			p.subQueue <- notification
		}
	}()
}
