package rmq

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/sirupsen/logrus"
)

type mockAck struct {
	mu     sync.Mutex
	acked  bool
	nacked bool
}

func (m *mockAck) Ack(_ uint64, _ bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.acked = true
	return nil
}

func (m *mockAck) Nack(_ uint64, _, _ bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.nacked = true
	return nil
}

func (m *mockAck) Reject(_ uint64, _ bool) error { return nil }

func (m *mockAck) wasAcked() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.acked
}

func (m *mockAck) wasNacked() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.nacked
}

type mockChannel struct {
	mu         sync.Mutex
	msgs       chan amqp.Delivery
	published  []amqp.Publishing
	publishErr error
	queueErr   error
	consumeErr error
}

func (m *mockChannel) QueueDeclare(name string, _, _, _, _ bool, _ amqp.Table) (amqp.Queue, error) {
	return amqp.Queue{Name: name}, m.queueErr
}

func (m *mockChannel) Consume(_, _ string, _, _, _, _ bool, _ amqp.Table) (<-chan amqp.Delivery, error) {
	if m.consumeErr != nil {
		return nil, m.consumeErr
	}
	return m.msgs, nil
}

func (m *mockChannel) Publish(_, _ string, _, _ bool, msg amqp.Publishing) error {
	m.mu.Lock()
	m.published = append(m.published, msg)
	m.mu.Unlock()
	return m.publishErr
}

func (m *mockChannel) Close() error { return nil }

func (m *mockChannel) lastPublished() *amqp.Publishing {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.published) == 0 {
		return nil
	}
	p := m.published[len(m.published)-1]
	return &p
}

type mockConn struct {
	mu          sync.Mutex
	closed      bool
	closeCalled bool
	ch          amqpChannel
	channelErr  error
}

func (m *mockConn) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closeCalled = true
	return nil
}

func (m *mockConn) IsClosed() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.closed
}

func (m *mockConn) Channel() (amqpChannel, error) {
	return m.ch, m.channelErr
}

func (m *mockConn) wasCloseCalled() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.closeCalled
}

func newTestConsumer(conn amqpConnection) *Consumer {
	logger := logrus.New()
	logger.SetOutput(io.Discard)
	return &Consumer{conn: conn, logger: logger}
}

func newMockSetup() (*mockConn, *mockChannel) {
	ch := &mockChannel{msgs: make(chan amqp.Delivery, 1)}
	conn := &mockConn{ch: ch}
	return conn, ch
}

func delivery(ack *mockAck, body []byte, replyTo string) amqp.Delivery {
	return amqp.Delivery{
		Acknowledger: ack,
		Body:         body,
		ReplyTo:      replyTo,
	}
}

// waitFor polls condition until it's true or 1s elapses.
func waitFor(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("condition not met within timeout")
}

func TestHealthy_nilConn(t *testing.T) {
	c := newTestConsumer(nil)
	c.conn = nil
	if c.Healthy() {
		t.Fatal("expected false for nil conn")
	}
}

func TestHealthy_closedConn(t *testing.T) {
	c := newTestConsumer(&mockConn{closed: true})
	if c.Healthy() {
		t.Fatal("expected false for closed conn")
	}
}

func TestHealthy_openConn(t *testing.T) {
	c := newTestConsumer(&mockConn{closed: false})
	if !c.Healthy() {
		t.Fatal("expected true for open conn")
	}
}

func TestClose_nilConn_noPanic(t *testing.T) {
	c := newTestConsumer(nil)
	c.conn = nil
	c.Close() // should not panic
}

func TestClose_callsConnClose(t *testing.T) {
	conn := &mockConn{}
	c := newTestConsumer(conn)
	c.Close()
	if !conn.wasCloseCalled() {
		t.Fatal("expected conn.Close() to be called")
	}
}

func TestNewChannel_nilConn_returnsError(t *testing.T) {
	c := newTestConsumer(nil)
	c.conn = nil
	if _, err := c.newChannel(); err == nil {
		t.Fatal("expected error for nil conn")
	}
}

func TestNewChannel_closedConn_returnsError(t *testing.T) {
	c := newTestConsumer(&mockConn{closed: true})
	if _, err := c.newChannel(); err == nil {
		t.Fatal("expected error for closed conn")
	}
}

func TestNewChannel_connChannelError_propagates(t *testing.T) {
	channelErr := errors.New("channel failed")
	c := newTestConsumer(&mockConn{channelErr: channelErr})
	_, err := c.newChannel()
	if !errors.Is(err, channelErr) {
		t.Fatalf("expected channel error, got %v", err)
	}
}

func TestNewChannel_success_returnsChannel(t *testing.T) {
	conn, _ := newMockSetup()
	c := newTestConsumer(conn)
	ch, err := c.newChannel()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ch == nil {
		t.Fatal("expected non-nil channel")
	}
}

func TestConsume_successNoReplyTo_acks(t *testing.T) {
	conn, ch := newMockSetup()
	c := newTestConsumer(conn)
	handler := func(_ context.Context, _ []byte) ([]byte, error) {
		return []byte("ok"), nil
	}
	c.Consume("q", handler)

	ack := &mockAck{}
	ch.msgs <- delivery(ack, []byte("body"), "")

	waitFor(t, ack.wasAcked)
	if ack.wasNacked() {
		t.Fatal("should not have nacked")
	}
}

func TestConsume_successWithReplyTo_publishesEnvelopeAndAcks(t *testing.T) {
	conn, ch := newMockSetup()
	c := newTestConsumer(conn)
	handler := func(_ context.Context, body []byte) ([]byte, error) {
		return []byte(`"pong"`), nil
	}
	c.Consume("q", handler)

	ack := &mockAck{}
	ch.msgs <- delivery(ack, []byte("ping"), "reply-queue")

	waitFor(t, ack.wasAcked)

	pub := ch.lastPublished()
	if pub == nil {
		t.Fatal("expected a published reply")
	}
	var env envelope
	if err := json.Unmarshal(pub.Body, &env); err != nil {
		t.Fatalf("reply is not a valid envelope: %v", err)
	}
	if env.Type != responseTypeSuccess {
		t.Fatalf("expected success envelope, got %q", env.Type)
	}
}

func TestConsume_successWithReplyTo_nilResponse_doesNotPublish(t *testing.T) {
	conn, ch := newMockSetup()
	c := newTestConsumer(conn)
	handler := func(_ context.Context, _ []byte) ([]byte, error) {
		return nil, nil
	}
	c.Consume("q", handler)

	ack := &mockAck{}
	ch.msgs <- delivery(ack, []byte("body"), "reply-queue")

	waitFor(t, ack.wasAcked)
	if ch.lastPublished() != nil {
		t.Fatal("expected no publish for nil response")
	}
}

func TestConsume_successPublishFails_nacks(t *testing.T) {
	conn, ch := newMockSetup()
	ch.publishErr = errors.New("broker down")
	c := newTestConsumer(conn)
	handler := func(_ context.Context, _ []byte) ([]byte, error) {
		return []byte("data"), nil
	}
	c.Consume("q", handler)

	ack := &mockAck{}
	ch.msgs <- delivery(ack, []byte("body"), "reply-queue")

	waitFor(t, ack.wasNacked)
	if ack.wasAcked() {
		t.Fatal("should not have acked when publish fails")
	}
}

func TestConsume_handlerErrorNoReplyTo_nacks(t *testing.T) {
	conn, ch := newMockSetup()
	c := newTestConsumer(conn)
	handler := func(_ context.Context, _ []byte) ([]byte, error) {
		return nil, errors.New("handler failed")
	}
	c.Consume("q", handler)

	ack := &mockAck{}
	ch.msgs <- delivery(ack, []byte("body"), "")

	waitFor(t, ack.wasNacked)
	if ack.wasAcked() {
		t.Fatal("should not have acked on handler error")
	}
	if ch.lastPublished() != nil {
		t.Fatal("should not publish when no ReplyTo")
	}
}

func TestConsume_handlerRMQErrorWithReplyTo_publishesErrorEnvelopeAndAcks(t *testing.T) {
	conn, ch := newMockSetup()
	c := newTestConsumer(conn)
	rmqErr := NewError(1042, "item not found")
	handler := func(_ context.Context, _ []byte) ([]byte, error) {
		return nil, rmqErr
	}
	c.Consume("q", handler)

	ack := &mockAck{}
	ch.msgs <- delivery(ack, []byte("body"), "reply-queue")

	waitFor(t, ack.wasAcked)

	pub := ch.lastPublished()
	if pub == nil {
		t.Fatal("expected a published error reply")
	}
	var env envelope
	if err := json.Unmarshal(pub.Body, &env); err != nil {
		t.Fatalf("reply is not a valid envelope: %v", err)
	}
	if env.Type != responseTypeError {
		t.Fatalf("expected error envelope, got %q", env.Type)
	}
	var got Error
	if err := json.Unmarshal(env.Response, &got); err != nil {
		t.Fatal(err)
	}
	if got.Code != rmqErr.Code {
		t.Fatalf("expected code %d, got %d", rmqErr.Code, got.Code)
	}
}

func TestConsume_handlerPlainErrorWithReplyTo_wrapsAs500(t *testing.T) {
	conn, ch := newMockSetup()
	c := newTestConsumer(conn)
	handler := func(_ context.Context, _ []byte) ([]byte, error) {
		return nil, errors.New("unexpected db error")
	}
	c.Consume("q", handler)

	ack := &mockAck{}
	ch.msgs <- delivery(ack, []byte("body"), "reply-queue")

	waitFor(t, ack.wasAcked)

	pub := ch.lastPublished()
	if pub == nil {
		t.Fatal("expected a published error reply")
	}
	var env envelope
	if err := json.Unmarshal(pub.Body, &env); err != nil {
		t.Fatalf("reply is not a valid envelope: %v", err)
	}
	var got Error
	if err := json.Unmarshal(env.Response, &got); err != nil {
		t.Fatal(err)
	}
	if got.Code != 500 {
		t.Fatalf("expected plain errors wrapped as 500, got %d", got.Code)
	}
}

func TestConsume_handlerErrorPublishFails_nacks(t *testing.T) {
	conn, ch := newMockSetup()
	ch.publishErr = errors.New("broker down")
	c := newTestConsumer(conn)
	handler := func(_ context.Context, _ []byte) ([]byte, error) {
		return nil, errors.New("handler failed")
	}
	c.Consume("q", handler)

	ack := &mockAck{}
	ch.msgs <- delivery(ack, []byte("body"), "reply-queue")

	waitFor(t, ack.wasNacked)
	if ack.wasAcked() {
		t.Fatal("should not have acked when error publish fails")
	}
}

func TestConsume_correlationIDCopiedToReply(t *testing.T) {
	conn, ch := newMockSetup()
	c := newTestConsumer(conn)
	handler := func(_ context.Context, _ []byte) ([]byte, error) {
		return []byte(`"data"`), nil
	}
	c.Consume("q", handler)

	ack := &mockAck{}
	d := delivery(ack, []byte("body"), "reply-queue")
	d.CorrelationId = "corr-123"
	ch.msgs <- d

	waitFor(t, func() bool { return ch.lastPublished() != nil })

	pub := ch.lastPublished()
	if pub.CorrelationId != "corr-123" {
		t.Fatalf("expected correlation ID %q, got %q", "corr-123", pub.CorrelationId)
	}
}
