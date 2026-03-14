package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/mercury/pkg/instrumentation"
	"github.com/smira/go-statsd"
)

// Role defines user role type for the enumeration
type Role string

// Define the possible values as constants of the custom type
const (
	UserRole    Role = "user"
	AdminRole   Role = "admin"
	PremiumRole Role = "premium"
)

const TokenName = "Token"

// PingResponse is the response for a ping request
type PingResponse struct {
	Ping string `json:"ping"`
}

type SigninRequest struct {
	Credentials Credentials `json:"credentials"`
}

type AccountCreationRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}
type AccountCreationResponse struct {
	AccountID string `json:"account_id"`
}

// Create a struct to read the username and password from the request body
type Credentials struct {
	Password string `json:"password"`
	Username string `json:"username"`
}

type TokenResponse struct {
	Token string `json:"token"`
}

type SessionResponse struct {
	SessionID string   `json:"session_id"`
	UserID    string   `json:"user_id"`
	Username  string   `json:"username"`
	Roles     []string `json:"roles"`
}

type DeleteSessionResponse struct {
	SessionID string `json:"session_id"`
}

type Client interface {
	Signin(ctx context.Context, username, password string) (*TokenResponse, error)
	Refresh(ctx context.Context, cookie *http.Cookie) (*TokenResponse, error)
	Revoke(ctx context.Context, cookie *http.Cookie) error
	CreateAccount(ctx context.Context, username, email, password string) (*AccountCreationResponse, error)
	ActivateAccount(ctx context.Context, accountID string) error
	GetSession(ctx context.Context, cookie *http.Cookie, sessionID string) (*SessionResponse, error)
	ExtendSession(ctx context.Context, cookie *http.Cookie, sessionID string) (*SessionResponse, error)
	DeleteSession(ctx context.Context, cookie *http.Cookie, sessionID string) (*DeleteSessionResponse, error)
}

type client struct {
	host       string
	httpClient *http.Client
}

func NewClient(host string, httpClient *http.Client) Client {
	return &client{
		host:       host,
		httpClient: httpClient,
	}
}

func (c *client) Signin(ctx context.Context, username, password string) (_ *TokenResponse, err error) {
	t := instrumentation.NewMetricsTimer(ctx, "auth.dur", statsd.StringTag("op", "signin"))
	defer func() { t.Done(err) }()

	body, err := json.Marshal(SigninRequest{Credentials: Credentials{Username: username, Password: password}})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/api/v1/auth/login", c.host), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("auth signin: unexpected status %d", resp.StatusCode)
	}
	r := &TokenResponse{}
	if err := json.NewDecoder(resp.Body).Decode(r); err != nil {
		return nil, err
	}
	return r, nil
}

func (c *client) Refresh(ctx context.Context, cookie *http.Cookie) (_ *TokenResponse, err error) {
	t := instrumentation.NewMetricsTimer(ctx, "auth.dur", statsd.StringTag("op", "refresh"))
	defer func() { t.Done(err) }()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/api/v1/auth/refresh", c.host), nil)
	if err != nil {
		return nil, err
	}
	req.AddCookie(cookie)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("auth refresh: unexpected status %d", resp.StatusCode)
	}
	r := &TokenResponse{}
	if err := json.NewDecoder(resp.Body).Decode(r); err != nil {
		return nil, err
	}
	return r, nil
}

func (c *client) Revoke(ctx context.Context, cookie *http.Cookie) (err error) {
	t := instrumentation.NewMetricsTimer(ctx, "auth.dur", statsd.StringTag("op", "revoke"))
	defer func() { t.Done(err) }()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/api/v1/auth/revoke", c.host), nil)
	if err != nil {
		return err
	}
	req.AddCookie(cookie)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("auth revoke: unexpected status %d", resp.StatusCode)
	}
	return nil
}

func (c *client) CreateAccount(ctx context.Context, username, email, password string) (_ *AccountCreationResponse, err error) {
	t := instrumentation.NewMetricsTimer(ctx, "auth.dur", statsd.StringTag("op", "create_account"))
	defer func() { t.Done(err) }()

	body, err := json.Marshal(AccountCreationRequest{Username: username, Email: email, Password: password})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/api/v1/account", c.host), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("auth create_account: unexpected status %d", resp.StatusCode)
	}
	r := &AccountCreationResponse{}
	if err := json.NewDecoder(resp.Body).Decode(r); err != nil {
		return nil, err
	}
	return r, nil
}

func (c *client) ActivateAccount(ctx context.Context, accountID string) (err error) {
	t := instrumentation.NewMetricsTimer(ctx, "auth.dur", statsd.StringTag("op", "activate_account"))
	defer func() { t.Done(err) }()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/api/v1/account/activate/%s", c.host, accountID), nil)
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("auth activate_account: unexpected status %d", resp.StatusCode)
	}
	return nil
}

func (c *client) GetSession(ctx context.Context, cookie *http.Cookie, sessionID string) (_ *SessionResponse, err error) {
	t := instrumentation.NewMetricsTimer(ctx, "auth.dur", statsd.StringTag("op", "get_session"))
	defer func() { t.Done(err) }()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/api/v1/session/%s", c.host, sessionID), nil)
	if err != nil {
		return nil, err
	}
	req.AddCookie(cookie)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("auth get_session: unexpected status %d", resp.StatusCode)
	}
	r := &SessionResponse{}
	if err := json.NewDecoder(resp.Body).Decode(r); err != nil {
		return nil, err
	}
	return r, nil
}

func (c *client) ExtendSession(ctx context.Context, cookie *http.Cookie, sessionID string) (_ *SessionResponse, err error) {
	t := instrumentation.NewMetricsTimer(ctx, "auth.dur", statsd.StringTag("op", "extend_session"))
	defer func() { t.Done(err) }()

	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, fmt.Sprintf("%s/api/v1/session/%s", c.host, sessionID), nil)
	if err != nil {
		return nil, err
	}
	req.AddCookie(cookie)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("auth extend_session: unexpected status %d", resp.StatusCode)
	}
	r := &SessionResponse{}
	if err := json.NewDecoder(resp.Body).Decode(r); err != nil {
		return nil, err
	}
	return r, nil
}

func (c *client) DeleteSession(ctx context.Context, cookie *http.Cookie, sessionID string) (_ *DeleteSessionResponse, err error) {
	t := instrumentation.NewMetricsTimer(ctx, "auth.dur", statsd.StringTag("op", "delete_session"))
	defer func() { t.Done(err) }()

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, fmt.Sprintf("%s/api/v1/session/%s", c.host, sessionID), nil)
	if err != nil {
		return nil, err
	}
	req.AddCookie(cookie)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("auth delete_session: unexpected status %d", resp.StatusCode)
	}
	r := &DeleteSessionResponse{}
	if err := json.NewDecoder(resp.Body).Decode(r); err != nil {
		return nil, err
	}
	return r, nil
}
