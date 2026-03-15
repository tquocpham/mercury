package publisher

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/mercury/pkg/rmq"
)

type NotificationName string

const (
	MESSAGE     NotificationName = "Message"
	TOAST       NotificationName = "Toast"
	SUBSCRIBE   NotificationName = "Subscribe"
	UNSUBSCRIBE NotificationName = "Unsubscribe"
	DISCONNECT  NotificationName = "Disconnect"
)

type RMQClient interface {
	Close()
	SendNotification(
		ctx context.Context, channel string, typ NotificationName, payload []byte) (*SendNotificationResponse, error)
	SendSubscribeNotification(
		ctx context.Context, userID string, channels []string) (*SendNotificationResponse, error)
	SendUnsubscribeNotification(
		ctx context.Context, userID string, channels []string) (*SendNotificationResponse, error)
	SendMessageNotification(
		ctx context.Context, messageID, conversationID, user, message string) (*SendNotificationResponse, error)
	Subscribe(
		ctx context.Context, userID string, channels []string) (*SubscribeResponse, error)
}

type rmqClient struct {
	Publisher *rmq.Publisher
}

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

type SendNotificationRequest struct {
	Channel     string           `json:"channel"`
	Type        NotificationName `json:"type"`
	Payload     []byte           `json:"payload"`
	Command     string           `json:"cmd"`
	ReferenceID string           `json:"ref"`
}

type SendNotificationResponse struct {
	Notified int64 `json:"notified"`
}

func (c *rmqClient) SendNotification(
	ctx context.Context, channel string, typ NotificationName, payload []byte) (*SendNotificationResponse, error) {
	referenceID := uuid.New().String()
	return rmq.Request[SendNotificationRequest, SendNotificationResponse](ctx, c.Publisher, "pbs.v1.sendnotification", SendNotificationRequest{
		Channel:     channel,
		Type:        typ,
		Payload:     payload,
		ReferenceID: referenceID,
	})
}

type SubscribePayload struct {
	Channels []string `json:"channels"`
}

func (c *rmqClient) SendSubscribeNotification(
	ctx context.Context, userID string, channels []string) (*SendNotificationResponse, error) {

	userChannel := UserChannel(userID)
	bytes, err := json.Marshal(SubscribePayload{
		Channels: channels,
	})
	if err != nil {
		return nil, err
	}
	return c.SendNotification(ctx, userChannel, SUBSCRIBE, bytes)
}

type UnsubscribePayload struct {
	Channels []string `json:"channels"`
}

func (c *rmqClient) SendUnsubscribeNotification(
	ctx context.Context, userID string, channels []string) (*SendNotificationResponse, error) {
	userChannel := UserChannel(userID)
	bytes, err := json.Marshal(UnsubscribePayload{
		Channels: channels,
	})
	if err != nil {
		return nil, err
	}
	return c.SendNotification(ctx, userChannel, UNSUBSCRIBE, bytes)
}

type DisconnectPayload struct {
}

func (c *rmqClient) SendDisconnectNotification(
	ctx context.Context, userID string, channels []string) (*SendNotificationResponse, error) {
	userChannel := UserChannel(userID)
	bytes, err := json.Marshal(DisconnectPayload{})
	if err != nil {
		return nil, err
	}
	return c.SendNotification(ctx, userChannel, DISCONNECT, bytes)
}

type MessagePayload struct {
	MessageID      string `json:"message_id"`
	MessageType    string `json:"message_type"`
	ConversationID string `json:"conversation_id"`
	User           string `json:"user"`
	Message        string `json:"message"`
}

func (c *rmqClient) SendMessageNotification(
	ctx context.Context, messageID, conversationID string,
	user, message string,
) (*SendNotificationResponse, error) {

	bytes, err := json.Marshal(MessagePayload{
		MessageID:      messageID,
		ConversationID: conversationID,
		User:           user,
		Message:        message,
	})
	if err != nil {
		return nil, err
	}
	return c.SendNotification(ctx, MessageChannel(conversationID), MESSAGE, bytes)
}

type SubscribeRequest struct {
	UserID   string   `json:"user_id"`
	Channels []string `json:"channels"`
}
type SubscribeResponse struct {
	Channels []string `json:"channels"`
}

func (c *rmqClient) Subscribe(
	ctx context.Context, userID string, channels []string) (*SubscribeResponse, error) {
	return rmq.Request[SubscribeRequest, SubscribeResponse](ctx, c.Publisher, "pbs.v1.sendnotification", SubscribeRequest{
		UserID:   userID,
		Channels: channels,
	})

}

// type Client interface {
// 	SendNotification(
// 		ctx context.Context, channel string, typ NotificationName, payload []byte) (*SendNotificationResponse, error)
// 	SendSubscribeNotification(
// 		ctx context.Context, userID string, channels []string) (*SendNotificationResponse, error)
// 	SendUnsubscribeNotification(
// 		ctx context.Context, userID string, channels []string) (*SendNotificationResponse, error)
// 	SendMessageNotification(
// 		ctx context.Context, messageID, conversationID, user, message string) (*SendNotificationResponse, error)
// 	Subscribe(
// 		ctx context.Context, userID string, channels []string) (*SubscribeResponse, error)
// }

// type publisherClient struct {
// 	host       string
// 	httpClient *http.Client
// }

// // NewClient creates a new publisher client
// func NewClient(host string, httpClient *http.Client) Client {
// 	return &publisherClient{
// 		host:       host,
// 		httpClient: httpClient,
// 	}
// }

// func (c *publisherClient) SendNotification(
// 	ctx context.Context,
// 	channel string,
// 	typ NotificationName,
// 	payload []byte,
// ) (*SendNotificationResponse, error) {

// 	referenceID := uuid.New().String()

// 	bts, err := json.Marshal(&SendNotificationRequest{
// 		Channel:     channel,
// 		Type:        typ,
// 		Payload:     payload,
// 		ReferenceID: referenceID,
// 	})
// 	if err != nil {
// 		return nil, err
// 	}

// 	u := fmt.Sprintf("%s/api/v1/send", c.host)
// 	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewBuffer(bts))
// 	if err != nil {
// 		return nil, err
// 	}
// 	req.Header.Set("Content-Type", "application/json")

// 	response, err := c.httpClient.Do(req)
// 	if err != nil {
// 		return nil, err
// 	}
// 	defer response.Body.Close()
// 	if response.StatusCode != http.StatusOK {
// 		return nil, fmt.Errorf("publisher sendnotification: unexpected status %d", response.StatusCode)
// 	}

// 	r := &SendNotificationResponse{}
// 	if err := json.NewDecoder(response.Body).Decode(r); err != nil {
// 		return nil, err
// 	}
// 	return r, nil
// }

// func (c *publisherClient) SendSubscribeNotification(
// 	ctx context.Context, userID string, channels []string) (*SendNotificationResponse, error) {
// 	userChannel := UserChannel(userID)
// 	bytes, err := json.Marshal(SubscribePayload{
// 		Channels: channels,
// 	})
// 	if err != nil {
// 		return nil, err
// 	}
// 	return c.SendNotification(ctx, userChannel, SUBSCRIBE, bytes)
// }

// func (c *publisherClient) SendUnsubscribeNotification(
// 	ctx context.Context, userID string, channels []string) (*SendNotificationResponse, error) {
// 	userChannel := UserChannel(userID)
// 	bytes, err := json.Marshal(UnsubscribePayload{
// 		Channels: channels,
// 	})
// 	if err != nil {
// 		return nil, err
// 	}
// 	return c.SendNotification(ctx, userChannel, UNSUBSCRIBE, bytes)
// }

// func (c *publisherClient) SendDisconnectNotification(
// 	ctx context.Context, userID string, channels []string) (*SendNotificationResponse, error) {
// 	userChannel := UserChannel(userID)
// 	bytes, err := json.Marshal(DisconnectPayload{})
// 	if err != nil {
// 		return nil, err
// 	}
// 	return c.SendNotification(ctx, userChannel, DISCONNECT, bytes)
// }

// func (c *publisherClient) SendMessageNotification(
// 	ctx context.Context, messageID, conversationID string,
// 	user, message string,
// ) (*SendNotificationResponse, error) {

// 	bytes, err := json.Marshal(MessagePayload{
// 		MessageID:      messageID,
// 		ConversationID: conversationID,
// 		User:           user,
// 		Message:        message,
// 	})
// 	if err != nil {
// 		return nil, err
// 	}
// 	return c.SendNotification(ctx, MessageChannel(conversationID), MESSAGE, bytes)
// }

// func (c *publisherClient) Subscribe(
// 	ctx context.Context, userID string, channels []string) (*SubscribeResponse, error) {
// 	bts, err := json.Marshal(&SubscribeRequest{
// 		UserID:   userID,
// 		Channels: channels,
// 	})
// 	if err != nil {
// 		return nil, err
// 	}
// 	u := fmt.Sprintf("%s/api/v1/subscribe", c.host)
// 	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewBuffer(bts))
// 	if err != nil {
// 		return nil, err
// 	}
// 	req.Header.Set("Content-Type", "application/json")

// 	response, err := c.httpClient.Do(req)
// 	if err != nil {
// 		return nil, err
// 	}
// 	defer response.Body.Close()
// 	if response.StatusCode != http.StatusOK {
// 		return nil, fmt.Errorf("publisher subscribe unexpected status %d", response.StatusCode)
// 	}

// 	r := &SubscribeResponse{}
// 	if err := json.NewDecoder(response.Body).Decode(r); err != nil {
// 		return nil, err
// 	}
// 	return r, nil

// }
