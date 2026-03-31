package matchmaking

import (
	"context"
	"time"

	"github.com/mercury/pkg/rmq"
)

type RMQClient interface {
	Close()
	MatchmakingQueue(
		ctx context.Context, partyID string, playerIDs []string) (*MatchmakingQueueResponse, error)
	GetQueue(ctx context.Context, partyID string) (*GetQueueResponse, error)
	GameserverRegister(
		ctx context.Context, serverID string, ipAddress string, port int,
		capacity int) (*GSRegisterResponse, error)
	GameserverUnregister(
		ctx context.Context, serverID string, version int) (*GSUnregisterResponse, error)
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

type MatchmakingQueueRequest struct {
	PartyID   string   `json:"party_id"`
	PlayerIDs []string `json:"player_ids"`
}

type MatchmakingQueueResponse struct {
	PartyID string `json:"party_id"`
}

func (c *rmqClient) MatchmakingQueue(
	ctx context.Context, partyID string, playerIDs []string) (*MatchmakingQueueResponse, error) {
	return rmq.Request[MatchmakingQueueRequest, MatchmakingQueueResponse](ctx, c.Publisher, "mm.v1.clientregister", MatchmakingQueueRequest{
		PartyID:   partyID,
		PlayerIDs: playerIDs,
	})
}

type GetQueueRequest struct {
	PartyID string `json:"party_id"`
}

type GetQueueResponse struct {
	PartyID          string    `json:"party_id"`
	PlayerIDs        []string  `json:"player_ids"`
	AssignedServerID string    `json:"server_id"`
	RegisterTime     time.Time `json:"register_time"`
	Status           string    `json:"status"`
	Version          int       `json:"version"`
}

func (c *rmqClient) GetQueue(ctx context.Context, partyID string) (*GetQueueResponse, error) {
	return rmq.Request[GetQueueRequest, GetQueueResponse](ctx, c.Publisher, "mm.v1.getqueue", GetQueueRequest{
		PartyID: partyID,
	})
}

type GSRegisterRequest struct {
	ServerID  string `json:"server_id"`
	IPAddress string `json:"ip_address"`
	Port      int    `json:"port"`
	Capacity  int    `json:"capacity"`
}

type GSRegisterResponse struct {
}

func (c *rmqClient) GameserverRegister(
	ctx context.Context, serverID string, ipAddress string, port int,
	capacity int) (*GSRegisterResponse, error) {
	return rmq.Request[GSRegisterRequest, GSRegisterResponse](ctx, c.Publisher, "mm.v1.gsregister", GSRegisterRequest{
		ServerID:  serverID,
		IPAddress: ipAddress,
		Port:      port,
		Capacity:  capacity,
	})
}

type GSUnregisterRequest struct {
	ServerID string `json:"server_id"`
	Version  int    `json:"version"`
}

type GSUnregisterResponse struct {
}

func (c *rmqClient) GameserverUnregister(
	ctx context.Context, serverID string, version int) (*GSUnregisterResponse, error) {
	return rmq.Request[GSUnregisterRequest, GSUnregisterResponse](ctx, c.Publisher, "mm.v1.gsunregister", GSUnregisterRequest{
		ServerID: serverID,
		Version:  version,
	})
}
