package query

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/mercury/pkg/instrumentation"
	"github.com/smira/go-statsd"
)

// Client is an interface for a golang client that allows other services to use query service.
type Client interface {
	Ping(ctx context.Context) (*PingResponse, error)
	GetMessages(ctx context.Context, conversationID string, props GetMessagesProps) (*GetMessagesResponse, error)
	RefreshMessages(ctx context.Context, conversationID string, messageID string) (*RefreshMessagesResponse, error)
	SendMessage(ctx context.Context, req SendMessageRequest) (*SendMessageResponse, error)
}

type queryClient struct {
	host       string
	httpClient *http.Client
}

// NewClient creates a new query client
func NewClient(host string, httpClient *http.Client) Client {
	return &queryClient{
		host:       host,
		httpClient: httpClient,
	}
}

type PingResponse struct {
	Ping string `json:"ping"`
}

func (c *queryClient) Ping(ctx context.Context) (_ *PingResponse, err error) {
	t := instrumentation.NewMetricsTimer(ctx, "query.dur", statsd.StringTag("op", "ping"))
	defer func() { t.Done(err) }()

	u := fmt.Sprintf("%s/api/v1/ping", c.host)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	response, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("query ping: unexpected status %d", response.StatusCode)
	}

	r := &PingResponse{}

	if err := json.NewDecoder(response.Body).Decode(r); err != nil {
		return nil, err
	}
	return r, nil
}

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

type GetMessagesProps struct {
	PageSize  int
	NextToken string
}

func (c *queryClient) GetMessages(
	ctx context.Context,
	conversationID string,
	props GetMessagesProps,
) (_ *GetMessagesResponse, err error) {

	t := instrumentation.NewMetricsTimer(ctx, "query.dur", statsd.StringTag("op", "get_messages"))
	defer func() { t.Done(err) }()

	baseURL := fmt.Sprintf("%s/api/v1/messages", c.host)
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("conversation_id", conversationID)
	if props.PageSize != 0 {
		q.Add("page_size", strconv.Itoa(props.PageSize))
	}
	if props.NextToken != "" {
		q.Add("next_token", props.NextToken)
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	response, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("query get_messages: unexpected status %d", response.StatusCode)
	}

	r := &GetMessagesResponse{}

	if err := json.NewDecoder(response.Body).Decode(r); err != nil {
		return nil, err
	}
	return r, nil
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

type RefreshMessagesResponse struct {
	Messages []MessageResponse `json:"Messages"`
}

func (c *queryClient) SendMessage(ctx context.Context, req SendMessageRequest) (_ *SendMessageResponse, err error) {
	t := instrumentation.NewMetricsTimer(ctx, "query.dur", statsd.StringTag("op", "send_message"))
	defer func() { t.Done(err) }()

	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/api/v1/messages", c.host), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	response, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("query send_message: unexpected status %d", response.StatusCode)
	}
	r := &SendMessageResponse{}
	if err := json.NewDecoder(response.Body).Decode(r); err != nil {
		return nil, err
	}
	return r, nil
}

func (c *queryClient) RefreshMessages(
	ctx context.Context,
	conversationID string,
	messageID string,
) (_ *RefreshMessagesResponse, err error) {

	t := instrumentation.NewMetricsTimer(ctx, "query.dur", statsd.StringTag("op", "refresh_message"))
	defer func() { t.Done(err) }()

	baseURL := fmt.Sprintf("%s/api/v1/messages/refresh", c.host)
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("conversation_id", conversationID)
	q.Set("message_id", messageID)
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	response, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("query refresh_messages: unexpected status %d", response.StatusCode)
	}

	r := &RefreshMessagesResponse{}

	if err := json.NewDecoder(response.Body).Decode(r); err != nil {
		return nil, err
	}
	return r, nil
}
