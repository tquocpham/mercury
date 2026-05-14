package websocketrpc

import (
	"context"
	"encoding/binary"
	"fmt"
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
	"github.com/mercury/pkg/instrumentation"
	"github.com/mercury/pkg/middleware"
	"github.com/sirupsen/logrus"
	"github.com/smira/go-statsd"
)

// Handler takes the request payload and returns the response payload + error
type Handler func(ctx context.Context, payload []byte) (res []byte, err error)

// Packet header: [Type(2) | SeqID(4) | Status(1) | Payload(N)]
// Status 0 = OK, 1 = Error. On error, Payload is a UTF-8 error string.
const statusOK byte = 0
const statusError byte = 1

func buildPacket(msgType uint16, seqID uint32, status byte, payload []byte) []byte {
	pkt := make([]byte, 7+len(payload))
	binary.BigEndian.PutUint16(pkt[:2], msgType)
	binary.BigEndian.PutUint32(pkt[2:6], seqID)
	pkt[6] = status
	copy(pkt[7:], payload)
	return pkt
}

type WsRpcHandler interface {
	Handle(c echo.Context) error
	Register(msgType uint16, handler Handler)
}

type wsRpcHandler struct {
	writeChanSize int
	name          string
	mname         string
	handlers      map[uint16]Handler
}

type WsRpcOpt struct {
	WriteChanSize int
	Name          string
}

func NewWsRpcHandler(opt WsRpcOpt) WsRpcHandler {
	writeChanSize := 256
	if opt.WriteChanSize != 0 {
		writeChanSize = opt.WriteChanSize
	}

	if opt.Name == "" {
		panic("opt.Name is required")
	}

	return &wsRpcHandler{
		writeChanSize: writeChanSize,
		name:          opt.Name,
		mname:         fmt.Sprintf("%s.wsrpc.dur", opt.Name),
		handlers:      make(map[uint16]Handler),
	}
}

// Register maps a message type to a handler. Must be called before Handle.
func (h *wsRpcHandler) Register(msgType uint16, handler Handler) {
	h.handlers[msgType] = handler
}

// startWriter starts a dedicated goroutine for websocket writer flow.
// This is the ONLY place where ws.WriteMessage is called.
func (h *wsRpcHandler) startWriter(logger *logrus.Entry, metrics *statsd.Client, ws *websocket.Conn, writeChan <-chan []byte) {
	go func() {
		for msg := range writeChan {
			if err := h.onWrite(logger, metrics, ws, msg); err != nil {
				logger.WithError(err).Error("websocket writer stopping")
				return
			}
		}
	}()
}

func (h *wsRpcHandler) onWrite(
	logger *logrus.Entry, metrics *statsd.Client, ws *websocket.Conn, msg []byte) (err error) {
	t := instrumentation.NewStatsdTimer(metrics, h.mname, statsd.StringTag("op", "write"), statsd.StringTag("h", h.name))
	defer func() { t.Done(err) }()

	err = ws.WriteMessage(websocket.BinaryMessage, msg)
	if err != nil {
		logger.WithError(err).Error("websocket write failed")
		return err
	}
	return nil
}

func (h *wsRpcHandler) startReader(ctx context.Context, logger *logrus.Entry, metrics *statsd.Client, ws *websocket.Conn, writeChan chan<- []byte) error {
	defer close(writeChan)
	for {
		if err := h.onRead(ctx, logger, metrics, ws, writeChan); err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				logger.WithError(err).Error("Failed to read websocket message")
			}
			return err
		}
	}
}

func (h *wsRpcHandler) onRead(
	ctx context.Context, logger *logrus.Entry, metrics *statsd.Client, ws *websocket.Conn, writeChan chan<- []byte) (err error) {
	t := instrumentation.NewStatsdTimer(metrics, h.mname, statsd.StringTag("op", "read"), statsd.StringTag("h", h.name))
	defer func() { t.Done(err) }()

	_, data, err := ws.ReadMessage()
	if err != nil {
		return err
	}
	// safety guard to prevent server from crashing or panicking
	// A Header is defined to be 6 bytes long:
	// 2 bytes for the Message Type.
	// 4 bytes for the Sequence ID.
	if len(data) < 6 {
		logger.Error("Failed to read data header")
		return nil
	}
	msgType := binary.BigEndian.Uint16(data[:2])
	seqID := binary.BigEndian.Uint32(data[2:6])
	payload := data[6:]

	handler, ok := h.handlers[msgType]
	if !ok {
		logger.WithField("type", msgType).Warn("No handler registered for message type")
		return nil // Ignore unknown types to keep connection alive
	}

	go func(t uint16, s uint32, p []byte, fn Handler) {
		resPayload, err := fn(ctx, p)
		var pkt []byte
		if err != nil {
			logger.WithError(err).Error("handler failed")
			pkt = buildPacket(t, s, statusError, []byte(err.Error()))
		} else {
			pkt = buildPacket(t, s, statusOK, resPayload)
		}

		select {
		case writeChan <- pkt:
		case <-ctx.Done():
			logger.Warn("connection closed before response could be sent, dropping")
		}
	}(msgType, seqID, payload, handler)

	return nil
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Adjust for production security
	},
}

func (h *wsRpcHandler) Handle(c echo.Context) error {
	logger := middleware.GetLogger(c)
	metrics := middleware.GetStatsd(c)

	ws, err := upgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		logger.WithError(err).Error("Error upgrading connection")
		return err
	}
	defer ws.Close()

	writeChan := make(chan []byte, h.writeChanSize)
	h.startWriter(logger, metrics, ws, writeChan)

	if err := h.startReader(c.Request().Context(), logger, metrics, ws, writeChan); err != nil {
		if websocket.IsUnexpectedCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
			logger.WithError(err).Error("WebSocket connection terminated with error")
			return err
		}
	}
	return nil
}
