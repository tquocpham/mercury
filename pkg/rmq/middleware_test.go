package rmq

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/sirupsen/logrus"
)

func TestNewRequestID_missingKeyReturnsEmpty(t *testing.T) {
	if id := NewRequestID(context.Background()); id != "" {
		t.Fatalf("expected empty string, got %q", id)
	}
}

func TestNewRequestID_returnsStoredID(t *testing.T) {
	ctx := context.WithValue(context.Background(), requestIDKey, "abc-123")
	if id := NewRequestID(ctx); id != "abc-123" {
		t.Fatalf("expected %q, got %q", "abc-123", id)
	}
}

func TestGetLogger_missingKeyReturnsFallback(t *testing.T) {
	entry := GetLogger(context.Background())
	if entry == nil {
		t.Fatal("expected non-nil logger entry")
	}
}

func TestGetLogger_returnsStoredEntry(t *testing.T) {
	logger := logrus.New()
	stored := logrus.NewEntry(logger)
	ctx := context.WithValue(context.Background(), loggerKey, stored)
	if got := GetLogger(ctx); got != stored {
		t.Fatal("expected the stored logger entry")
	}
}

func TestGetMetrics_missingKeyReturnsNoop(t *testing.T) {
	client := GetMetrics(context.Background())
	if client == nil {
		t.Fatal("expected non-nil statsd client")
	}
	// Verify noop client doesn't panic on use.
	client.Incr("test", 1)
}

func newBufLogger() (*logrus.Logger, *bytes.Buffer) {
	buf := &bytes.Buffer{}
	logger := logrus.New()
	logger.SetOutput(buf)
	logger.SetFormatter(&logrus.TextFormatter{DisableTimestamp: true})
	return logger, buf
}

func TestUseLogger_callsNextAndReturnsResult(t *testing.T) {
	logger, _ := newBufLogger()
	called := false
	handler := func(ctx context.Context, body []byte) ([]byte, error) {
		called = true
		return []byte("response"), nil
	}

	wrapped := UseLogger(logger)("my.queue", handler)
	resp, err := wrapped(context.Background(), []byte("input"))

	if !called {
		t.Error("expected handler to be called")
	}
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if string(resp) != "response" {
		t.Fatalf("expected %q, got %q", "response", resp)
	}
}

func TestUseLogger_propagatesError(t *testing.T) {
	logger, _ := newBufLogger()
	handlerErr := errors.New("handler failed")
	handler := func(ctx context.Context, body []byte) ([]byte, error) {
		return nil, handlerErr
	}

	wrapped := UseLogger(logger)("my.queue", handler)
	_, err := wrapped(context.Background(), nil)
	if !errors.Is(err, handlerErr) {
		t.Fatalf("expected handler error to propagate, got %v", err)
	}
}

func TestUseLogger_injectsLoggerIntoContext(t *testing.T) {
	logger, _ := newBufLogger()
	var ctxLogger interface{}
	handler := func(ctx context.Context, body []byte) ([]byte, error) {
		ctxLogger = ctx.Value(loggerKey)
		return nil, nil
	}

	wrapped := UseLogger(logger)("my.queue", handler)
	_, _ = wrapped(context.Background(), nil)

	if ctxLogger == nil {
		t.Error("expected logger to be injected into context")
	}
}

func TestUseLogger_logsQueueName(t *testing.T) {
	logger, buf := newBufLogger()
	handler := func(ctx context.Context, body []byte) ([]byte, error) { return nil, nil }

	wrapped := UseLogger(logger)("orders.queue", handler)
	_, _ = wrapped(context.Background(), nil)

	if !bytes.Contains(buf.Bytes(), []byte("orders.queue")) {
		t.Fatalf("expected queue name in log output, got: %s", buf.String())
	}
}

func TestUseStatsd_callsNextAndReturnsResult(t *testing.T) {
	// Use the noop client so no real UDP socket is needed.
	client := GetMetrics(context.Background())
	called := false
	handler := func(ctx context.Context, body []byte) ([]byte, error) {
		called = true
		return []byte("pong"), nil
	}

	wrapped := UseStatsd(client)("test.queue", handler)
	resp, err := wrapped(context.Background(), []byte("ping"))

	if !called {
		t.Error("expected handler to be called")
	}
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if string(resp) != "pong" {
		t.Fatalf("expected %q, got %q", "pong", resp)
	}
}

func TestUseStatsd_propagatesError(t *testing.T) {
	client := GetMetrics(context.Background())
	handlerErr := errors.New("broken")
	handler := func(ctx context.Context, body []byte) ([]byte, error) {
		return nil, handlerErr
	}

	wrapped := UseStatsd(client)("test.queue", handler)
	_, err := wrapped(context.Background(), nil)
	if !errors.Is(err, handlerErr) {
		t.Fatalf("expected error to propagate, got %v", err)
	}
}

func TestUseStatsd_injectsClientIntoContext(t *testing.T) {
	client := GetMetrics(context.Background())
	var ctxClient interface{}
	handler := func(ctx context.Context, body []byte) ([]byte, error) {
		ctxClient = ctx.Value(statsdKey)
		return nil, nil
	}

	wrapped := UseStatsd(client)("test.queue", handler)
	_, _ = wrapped(context.Background(), nil)

	if ctxClient == nil {
		t.Error("expected statsd client to be injected into context")
	}
}

func TestMiddlewareChaining_outerToInner(t *testing.T) {
	// Verify the order matches what Consumer.Consume does:
	// middlewares applied right-to-left so first listed is outermost.
	order := []string{}
	makeMiddleware := func(name string) Middleware {
		return func(queue string, next Handler) Handler {
			return func(ctx context.Context, body []byte) ([]byte, error) {
				order = append(order, name+":before")
				resp, err := next(ctx, body)
				order = append(order, name+":after")
				return resp, err
			}
		}
	}

	handler := func(ctx context.Context, body []byte) ([]byte, error) {
		order = append(order, "handler")
		return nil, nil
	}

	middlewares := []Middleware{makeMiddleware("A"), makeMiddleware("B")}
	h := handler
	for i := len(middlewares) - 1; i >= 0; i-- {
		h = middlewares[i]("q", h)
	}
	_, _ = h(context.Background(), nil)

	expected := []string{"A:before", "B:before", "handler", "B:after", "A:after"}
	if len(order) != len(expected) {
		t.Fatalf("expected %v, got %v", expected, order)
	}
	for i, v := range expected {
		if order[i] != v {
			t.Fatalf("at index %d: expected %q, got %q", i, v, order[i])
		}
	}
}
