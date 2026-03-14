package auth

import (
	"context"
	"encoding/json"

	"github.com/mercury/pkg/instrumentation"
	"github.com/mercury/pkg/rmq"
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

type RMQClient interface {
	Close()
	Login(ctx context.Context, username, password string) (_ *TokenResponse, err error)
	Refresh(ctx context.Context, token string) (_ *RefreshResponse, err error)
	Revoke(ctx context.Context) error
	CreateAccount(ctx context.Context,
		username string, email string, password string) (_ *AccountCreationResponse, err error)
	ActivateAccount(ctx context.Context, accountID string) (_ *ActivateAccountResponse, err error)
	GetSession(ctx context.Context, sessionID string) (_ *SessionResponse, err error)
	RefreshSession(ctx context.Context, sessionID string) (_ *SessionResponse, err error)
	DeleteSession(ctx context.Context, sessionID string) (_ *DeleteSessionResponse, err error)
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

func request[Req any, Resp any](ctx context.Context, p *rmq.Publisher, route string, req Req) (_ *Resp, err error) {
	t := instrumentation.NewMetricsTimer(ctx, "auth.dur", statsd.StringTag("r", route))
	defer func() { t.Done(err) }()
	b, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	response, err := p.Request(route, b)
	if err != nil {
		return nil, err
	}
	var resp Resp
	if err := json.Unmarshal(response, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// PingResponse is the response for a ping request
type PingResponse struct {
	Ping string `json:"ping"`
}

type LoginRequest struct {
	Credentials Credentials `json:"credentials"`
}

type Credentials struct {
	Password string `json:"password"`
	Username string `json:"username"`
}

type TokenResponse struct {
	Token string `json:"token"`
}

func (c *rmqClient) Login(ctx context.Context, username, password string) (_ *TokenResponse, err error) {
	return request[LoginRequest, TokenResponse](ctx, c.Publisher, "auth.v1.login", LoginRequest{
		Credentials: Credentials{
			Username: username,
			Password: password,
		},
	})
}

type RefreshRequest struct {
	Token string `json:"token"`
}

type RefreshResponse struct {
	Token string `json:"token"`
}

func (c *rmqClient) Refresh(ctx context.Context, token string) (_ *RefreshResponse, err error) {
	return request[RefreshRequest, RefreshResponse](ctx, c.Publisher, "auth.v1.refresh", RefreshRequest{
		Token: token,
	})
}

func (c *rmqClient) Revoke(ctx context.Context) error {
	// TODO: implement
	return nil
}

type AccountCreationRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}
type AccountCreationResponse struct {
	AccountID string `json:"account_id"`
}

func (c *rmqClient) CreateAccount(ctx context.Context,
	username string, email string, password string) (_ *AccountCreationResponse, err error) {
	return request[AccountCreationRequest, AccountCreationResponse](ctx, c.Publisher, "auth.v1.createaccount", AccountCreationRequest{
		Username: username,
		Email:    email,
		Password: password,
	})
}

type ActivateAccountRequest struct {
	AccountID string `json:"account_id"`
}

type ActivateAccountResponse struct {
	AccountID string `json:"account_id"`
}

func (c *rmqClient) ActivateAccount(ctx context.Context, accountID string) (_ *ActivateAccountResponse, err error) {
	return request[ActivateAccountRequest, ActivateAccountResponse](ctx, c.Publisher, "auth.v1.activateaccount", ActivateAccountRequest{
		AccountID: accountID,
	})
}

type SessionResponse struct {
	SessionID string   `json:"session_id"`
	UserID    string   `json:"user_id"`
	Username  string   `json:"username"`
	Roles     []string `json:"roles"`
}

type GetSessionRequest struct {
	SessionID string `json:"session_id"`
}

func (c *rmqClient) GetSession(ctx context.Context, sessionID string) (_ *SessionResponse, err error) {
	return request[GetSessionRequest, SessionResponse](ctx, c.Publisher, "auth.v1.getsession", GetSessionRequest{
		SessionID: sessionID,
	})
}

type RefreshSessionRequest struct {
	SessionID string `json:"session_id"`
}

func (c *rmqClient) RefreshSession(ctx context.Context, sessionID string) (_ *SessionResponse, err error) {
	return request[RefreshSessionRequest, SessionResponse](ctx, c.Publisher, "auth.v1.refreshsession", RefreshSessionRequest{
		SessionID: sessionID,
	})
}

type DeleteSessionRequest struct {
	SessionID string `json:"session_id"`
}

type DeleteSessionResponse struct {
	SessionID string `json:"session_id"`
}

func (c *rmqClient) DeleteSession(ctx context.Context, sessionID string) (_ *DeleteSessionResponse, err error) {
	return request[DeleteSessionRequest, DeleteSessionResponse](ctx, c.Publisher, "auth.v1.deletesession", DeleteSessionRequest{
		SessionID: sessionID,
	})
}
