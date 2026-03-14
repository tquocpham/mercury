package messages

import (
	"context"
	"time"

	"github.com/mercury/pkg/rmq"
)

type RMQClient interface {
	Close()
	GetMessages(ctx context.Context, conversationID string, limit int, nextToken string) (*GetMessagesResponse, error)
	RefreshMessages(ctx context.Context, conversationID string, messageID string) (*RefreshMessagesResponse, error)
	SendMessage(ctx context.Context, conversationID string, body string, user string, userID string, to []string) (*SendMessageResponse, error)
}

type rmqClient struct {
	Publisher *rmq.Publisher
}

// NewClient creates a new query client
func NewRMQClient(amqpURL string) (RMQClient, error) {
	publisher, err := rmq.NewPublisher(amqpURL)
	if err != nil {
		return nil, err
	}
	return &rmqClient{
		Publisher: publisher,
	}, nil
}

func (c *rmqClient) Close() {
	c.Publisher.Close()
}

type MessageResponse struct {
	ConversationID string    `json:"conversation_id"`
	CreatedAt      time.Time `json:"created_at"`
	MessageID      string    `json:"message_id"`
	User           string    `json:"user"`
	Body           string    `json:"body"`
}

type GetMessagesRequest struct {
	ConversationID string `json:"conversation_id"`
	Limit          int    `json:"limit"`
	NextToken      string `json:"next_token"`
}

type GetMessagesResponse struct {
	Messages  []MessageResponse `json:"Messages"`
	NextToken string            `json:"NextToken"`
}

func (c *rmqClient) GetMessages(
	ctx context.Context, conversationID string, limit int, nextToken string) (*GetMessagesResponse, error) {
	return rmq.Request[GetMessagesRequest, GetMessagesResponse](ctx, c.Publisher, "msgs.v1.getmessages", GetMessagesRequest{
		ConversationID: conversationID,
		Limit:          limit,
		NextToken:      nextToken,
	})
}

type RefreshMessagesRequest struct {
	ConversationID string `json:"conversation_id"`
	MessageID      string `json:"message_id"`
}

type RefreshMessagesResponse struct {
	Messages []MessageResponse `json:"Messages"`
}

func (c *rmqClient) RefreshMessages(
	ctx context.Context,
	conversationID string, messageID string) (*RefreshMessagesResponse, error) {
	return rmq.Request[RefreshMessagesRequest, RefreshMessagesResponse](ctx, c.Publisher, "msgs.v1.refreshmessages", RefreshMessagesRequest{
		ConversationID: conversationID,
		MessageID:      messageID,
	})
}

type SendMessageRequest struct {
	ConversationID string   `json:"conversation_id"`
	Body           string   `json:"body"`
	User           string   `json:"user"`
	UserID         string   `json:"user_id"`
	To             []string `json:"to"`
}

type SendMessageResponse struct {
	Status    string `json:"status"`
	MessageID string `json:"message_id"`
}

func (c *rmqClient) SendMessage(
	ctx context.Context,
	conversationID string,
	body string,
	user string,
	userID string,
	to []string,
) (*SendMessageResponse, error) {
	return rmq.Request[SendMessageRequest, SendMessageResponse](ctx, c.Publisher, "msgs.v1.sendmessage", SendMessageRequest{
		ConversationID: conversationID,
		Body:           body,
		User:           user,
		UserID:         userID,
		To:             to,
	})
}
