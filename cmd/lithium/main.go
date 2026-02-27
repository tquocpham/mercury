package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/segmentio/kafka-go"
	"github.com/sirupsen/logrus"
	"github.com/venus/lithium/lib/clients/lithium"
	"github.com/venus/lithium/lib/kmq"
	"github.com/venus/lithium/lib/managers"
)

type MessageResponse struct {
	ConversationID string    `json:"conversation_id"`
	CreatedAt      time.Time `json:"created_at"`
	MessageID      string    `json:"message_id"`
	User           string    `json:"user"`
	Body           string    `json:"body"`
}

type GetMessagesResponse struct {
	Messages  []MessageResponse `json:"Messages"`
	NextToken string            `json:"NextToken"`
}

type MessageRequest struct {
	ConversationID string `json:"conversation_id"`
	Body           string `json:"body"`
	User           string `json:"user"`
}

func main() {
	logger := logrus.New()
	logger.SetFormatter(&logrus.TextFormatter{FullTimestamp: true})

	brokers := []string{"localhost:9092"}
	topic := "messages"
	groupID := "messages-consumer-group"

	cass, err := managers.NewCassandraClient("localhost")
	if err != nil {
		logger.WithError(err).Fatal("cassandra: failed to connect")
	}
	defer cass.Close()

	producer := kmq.NewProducer(brokers)
	defer producer.Close()

	consumer := kmq.NewKafkaConsumer(brokers, groupID, topic, logger)
	defer consumer.Close()

	consumer.Consume(func(ctx context.Context, msg kafka.Message) (kmq.Result, error) {
		conversationID := string(msg.Key)
		var chatData = &lithium.ChatMessage{}
		logger.Infof(string(msg.Value))
		if err := json.Unmarshal(msg.Value, chatData); err != nil {
			logger.Infof("failed to unmarshal chatdata. skipping forever")
			return kmq.Success, nil
		}

		// Extract message_id from headers.
		messageID := ""
		for _, h := range msg.Headers {
			if h.Key == "message_id" {
				messageID = string(h.Value)
				break
			}
		}
		if messageID == "" {
			messageID = uuid.New().String()
		}

		if err := cass.SaveMessage(conversationID, messageID, chatData.User, chatData.Message, msg.Time); err != nil {
			logger.WithError(err).Error("cassandra: save message failed")
			return kmq.Retry, err
		}

		logger.Infof("saved message | conversation=%s message_id=%s", conversationID, messageID)
		return kmq.Success, nil
	})

	e := echo.New()
	e.GET("/messages", func(c echo.Context) error {
		conversationID := c.QueryParam("conversation_id")
		if conversationID == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"error": "conversation_id query param required",
			})
		}
		pageSizeStr := c.QueryParam("page_size")
		pageSize := 10
		if pageSizeStr != "" {
			parsed, parseErr := strconv.Atoi(pageSizeStr)
			if parseErr != nil {
				return c.JSON(http.StatusBadRequest, map[string]string{
					"error": "Invalid page_size",
				})
			}
			pageSize = parsed
		}

		// Assume 'tokenStr' comes from your API request
		nextToken := c.QueryParam("next_token")
		pagingState := []byte(nil)
		if nextToken != "" {
			parsed, parsedErr := base64.StdEncoding.DecodeString(nextToken)
			if parsedErr != nil {
				return c.JSON(http.StatusBadRequest, map[string]string{
					"error": "Invalid next_token",
				})
			}
			pagingState = parsed
		}

		messages, err := cass.GetMessages(conversationID, pageSize, pagingState)
		if err != nil {
			logger.WithError(err).Error("cassandra: get messages failed")
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": "failed to fetch messages",
			})
		}
		msgs := make([]MessageResponse, len(messages.Messages))
		for i, msg := range messages.Messages {
			msgs[i] = MessageResponse{
				ConversationID: msg.ConversationID,
				MessageID:      msg.MessageID,
				Body:           msg.Body,
				User:           msg.User,
				CreatedAt:      msg.CreatedAt,
			}
		}

		respNextToken := ""
		if len(messages.Next) > 0 {
			respNextToken = base64.StdEncoding.EncodeToString(messages.Next)
		}

		return c.JSON(http.StatusOK, &GetMessagesResponse{
			Messages:  msgs,
			NextToken: respNextToken,
		})
	})
	e.GET("/messages/refresh", func(c echo.Context) error {
		conversationID := c.QueryParam("conversation_id")
		if conversationID == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"error": "conversation_id query param required",
			})
		}
		messageID := c.QueryParam("message_id")

		messages, err := cass.RefreshMessages(conversationID, messageID)
		if err != nil {
			logger.WithError(err).Error("cassandra: get messages failed")
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": "failed to fetch messages",
			})
		}
		msgs := make([]MessageResponse, len(messages.Messages))
		for i, msg := range messages.Messages {
			msgs[i] = MessageResponse{
				ConversationID: msg.ConversationID,
				MessageID:      msg.MessageID,
				Body:           msg.Body,
				User:           msg.User,
				CreatedAt:      msg.CreatedAt,
			}
		}

		respNextToken := ""
		if len(messages.Next) > 0 {
			respNextToken = base64.StdEncoding.EncodeToString(messages.Next)
		}

		return c.JSON(http.StatusOK, &GetMessagesResponse{
			Messages:  msgs,
			NextToken: respNextToken,
		})
	})
	e.POST("/messages", func(c echo.Context) error {
		var req MessageRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"error": "invalid request",
			})
		}

		if req.ConversationID == "" || req.Body == "" || req.User == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"error": "conversation_id, user and body required",
			})
		}

		msgID := uuid.New().String()

		var chatData = &lithium.ChatMessage{
			User:    req.User,
			Message: req.Body,
		}
		chatDataBytes, err := json.Marshal(chatData)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{
				"error": "failed to encode chatdata",
			})
		}

		kmsg := kafka.Message{
			Topic: topic,
			Key:   []byte(req.ConversationID), // critical for ordering
			Value: chatDataBytes,
			Headers: []kafka.Header{
				{Key: "message_id", Value: []byte(msgID)},
			},
			Time: time.Now(),
		}

		if err := producer.Produce(c.Request().Context(), topic, kmsg); err != nil {
			logger.WithError(err).Error("produce failed")
			return c.JSON(http.StatusInternalServerError, map[string]string{
				"error": "failed to enqueue message",
			})
		}

		return c.JSON(http.StatusOK, map[string]string{
			"status":     "queued",
			"message_id": msgID,
		})
	})

	go func() {
		if err := e.Start(":8080"); err != nil {
			logger.Fatal(err)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	e.Shutdown(ctx)
}
