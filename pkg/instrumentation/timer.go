package instrumentation

import (
	"context"
	"time"

	"github.com/smira/go-statsd"
)

type Timing interface {
	Start() Timer
}

type Timer interface {
	Done(err error)
}

type timing struct {
}

func (t *timing) Start() Timer {
	return &timer{}
}

type timer struct {
	start time.Time
	d     time.Duration
}

func (t *timer) Done(_ error) {
	t.d = time.Since(t.start)
}

func NewTimer() Timing {
	return &timing{}
}

type metrcstimer struct {
	start   time.Time
	metrics *statsd.Client
	name    string
	tags    []statsd.Tag
}

func (t *metrcstimer) Done(err error) {
	dur := time.Since(t.start)
	errTag := "false"
	if err != nil {
		errTag = "true"
	}

	tags := append(t.tags, statsd.StringTag("error", errTag))
	t.metrics.Timing(t.name, int64(dur/time.Millisecond), tags...)
}

func NewMetricsTimer(ctx context.Context, name string, tags ...statsd.Tag) Timer {
	metrics := StatsdFromContext(ctx)
	logger := LoggerFromContext(ctx)

	// if no client, dummy statsd client
	if metrics == nil {
		logger.Warn("failed to get statsd client from context - defaulting to dummy statsd client")
		metrics = statsd.NewClient("localhost:0")
	}
	start := time.Now()

	return &metrcstimer{
		name:    name,
		start:   start,
		metrics: metrics,
		tags:    tags,
	}
}
