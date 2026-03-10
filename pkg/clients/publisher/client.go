package publisher

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

type Client interface {
	SendNotification(
		ctx context.Context,
		channel string,
		Type string,
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
	Type    string         `json:"type"`
	Payload map[string]any `json:"payload"`
}

func (c *publisherClient) SendNotification(
	ctx context.Context,
	channel string,
	Type string,
	payload map[string]any,
) (*SendNotificationResponse, error) {

	bts, err := json.Marshal(&SendNotificationRequest{
		Channel: channel,
		Notification: Notification{
			Type:    Type,
			Payload: payload,
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
