package kmq

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"
	"github.com/sirupsen/logrus"
	"github.com/smira/go-statsd"
)

// Middleware receives the queue name (injected by Consume) and wraps a Handler.
type Middleware func(queue string, next Handler) Handler

type loggerCtxKey struct{}

func UseLogger(logger *logrus.Logger, environment string) Middleware {
	return func(queue string, next Handler) Handler {
		return func(ctx context.Context, msg kafka.Message) (Result, error) {
			log := logger.WithContext(ctx)
			correlationID := uuid.New().String()
			log = log.WithFields(logrus.Fields{
				"environment":    environment,
				"offset":         msg.Offset,
				"key":            string(msg.Key),
				"partition":      msg.Partition,
				"topic":          msg.Topic,
				"queue":          queue,
				"correlation_id": correlationID,
			})
			ctx = context.WithValue(ctx, loggerCtxKey{}, log)
			result, err := next(ctx, msg)
			if err != nil {
				log = log.WithFields(logrus.Fields{
					"err": err.Error(),
				})
			}
			log.Info("message")
			return result, err
		}
	}
}

func LoggerFromContext(ctx context.Context) *logrus.Entry {
	l, _ := ctx.Value(loggerCtxKey{}).(*logrus.Entry)
	if l == nil {
		return logrus.NewEntry(logrus.StandardLogger())
	}
	return l
}

type statsdCtxKey struct{}

func UseStatsd(client *statsd.Client) Middleware {
	return func(queue string, next Handler) Handler {
		return func(ctx context.Context, msg kafka.Message) (_ Result, err error) {
			ctx = context.WithValue(ctx, statsdCtxKey{}, client)
			start := time.Now()

			defer func(err error) {
				duration := time.Since(start)
				key := string(msg.Key)
				topic := msg.Topic

				tags := []statsd.Tag{
					statsd.StringTag("key", key),
					statsd.StringTag("topic", topic),
					statsd.StringTag("queue", queue),
				}
				if err != nil {
					tags = append(tags, statsd.StringTag("err", err.Error()))
				}
				client.Timing("kmq.duration", int64(duration/time.Millisecond), tags...)

			}(err)
			return next(ctx, msg)
		}
	}
}

func StatsdFromContext(ctx context.Context) *statsd.Client {
	l, _ := ctx.Value(statsdCtxKey{}).(*statsd.Client)
	if l == nil {
		return statsd.NewClient("localhost:0")
	}
	return l
}
