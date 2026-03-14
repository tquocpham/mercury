package rmq

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/smira/go-statsd"
)

// Middleware receives the queue name (injected by Consume) and wraps a Handler.
type Middleware func(queue string, next Handler) Handler

type contextKey string

const requestIDKey contextKey = "request_id"
const loggerKey contextKey = "logger"
const statsdKey contextKey = "statsd"

func NewRequestID(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey).(string)
	return id
}

// GetLogger returns the logger stored in context, falling back to the standard logger.
func GetLogger(ctx context.Context) *logrus.Entry {
	entry, ok := ctx.Value(loggerKey).(*logrus.Entry)
	if !ok || entry == nil {
		return logrus.NewEntry(logrus.StandardLogger())
	}
	return entry
}

func GetMetrics(ctx context.Context) *statsd.Client {
	entry, ok := ctx.Value(statsdKey).(*statsd.Client)
	if !ok || entry == nil {
		// UDP just drops silently, nothing listens, nothing errors
		return statsd.NewClient("localhost:9999")
	}
	return entry
}

func UseStatsd(client *statsd.Client) Middleware {
	return func(queue string, next Handler) Handler {
		return func(ctx context.Context, body []byte) ([]byte, error) {
			start := time.Now()
			ctx = context.WithValue(ctx, statsdKey, client)

			resp, err := next(ctx, body)
			duration := time.Since(start)

			tags := []statsd.Tag{
				statsd.StringTag("queue", queue),
			}

			client.Incr("mq.msg.count", 1, tags...)
			client.Timing("mq.msg.duration", int64(duration/time.Millisecond), tags...)
			if err != nil {
				client.Incr("mq.msg.error", 1, tags...)
			}

			return resp, err
		}
	}
}

func UseLogger(logger *logrus.Logger) Middleware {
	return func(queue string, next Handler) Handler {
		return func(ctx context.Context, body []byte) ([]byte, error) {
			entry := logger.WithFields(logrus.Fields{
				"queue":      queue,
				"request_id": NewRequestID(ctx),
			})
			ctx = context.WithValue(ctx, loggerKey, entry)

			start := time.Now()
			resp, err := next(ctx, body)
			entry = entry.WithField("duration", time.Since(start).Milliseconds())
			if err != nil {
				entry.WithError(err).Error("mq: handler failed")
			} else {
				entry.Info("mq: handler ok")
			}
			return resp, err
		}
	}
}
