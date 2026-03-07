package auth

import (
	"encoding/json"
	"fmt"
	"net/http"
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

type Client interface {
	Refresh(cookie *http.Cookie) (*TokenResponse, error)
}

type client struct {
	host string
}

func NewClient(host string) Client {
	return &client{
		host: host,
	}
}

func (c *client) Refresh(cookie *http.Cookie) (*TokenResponse, error) {
	req, _ := http.NewRequest("GET", fmt.Sprintf("%s/api/v1/auth/refresh", c.host), nil)
	req.AddCookie(cookie)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("helium find: unexpected status %d", resp.StatusCode)
	}

	body := &TokenResponse{}
	if err := json.NewDecoder(resp.Body).Decode(body); err != nil {
		return nil, err
	}
	return body, nil
}
