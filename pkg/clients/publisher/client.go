package publisher

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/google/uuid"
)

// Define a custom type for the enum
type NotificationName string

// Define the possible values as constants of the custom type
const (
	MESSAGE NotificationName = "Message"
	// Command is used by the system to perform an action but not notify the client.
	COMMAND NotificationName = "Command"
)

type Client interface {
	SendNotification(
		ctx context.Context,
		channel string,
		Type NotificationName,
		payload map[string]any,
	) (*SendNotificationResponse, error)
}

type publisherClient struct {
	host       string
	httpClient *http.Client
}

// NewClient creates a new publisher client
func NewClient(host string, httpClient *http.Client) Client {
	return &publisherClient{
		host:       host,
		httpClient: httpClient,
	}
}

type SendNotificationRequest struct {
	Channel      string       `json:"channel"`
	Notification Notification `json:"notification"`
}

type SendNotificationResponse struct {
	Notified int64 `json:"notified"`
}

type Notification struct {
	Type        NotificationName `json:"type"`
	Payload     map[string]any   `json:"payload"`
	Version     string           `json:"ver"`
	Command     string           `json:"cmd"`
	ReferenceID string           `json:"ref"`
}

func (c *publisherClient) SendNotification(
	ctx context.Context,
	channel string,
	Type NotificationName,
	payload map[string]any,
) (*SendNotificationResponse, error) {

	referenceID := uuid.New().String()

	bts, err := json.Marshal(&SendNotificationRequest{
		Channel: channel,
		Notification: Notification{
			Type:        Type,
			Payload:     payload,
			Version:     "1",
			Command:     channel,
			ReferenceID: referenceID,
		},
	})
	if err != nil {
		return nil, err
	}

	u := fmt.Sprintf("%s/api/v1/send", c.host)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewBuffer(bts))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	response, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("publisher sendnotification: unexpected status %d", response.StatusCode)
	}

	r := &SendNotificationResponse{}
	if err := json.NewDecoder(response.Body).Decode(r); err != nil {
		return nil, err
	}
	return r, nil
}
