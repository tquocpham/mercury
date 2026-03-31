package handlers

import (
	"context"
	"encoding/json"

	"github.com/mercury/pkg/clients/matchmaking"
	"github.com/mercury/pkg/matchmaking/managers"
	"github.com/mercury/pkg/rmq"
)

type RMQHandlers interface {
	UserJoinQueue(ctx context.Context, body []byte) ([]byte, error)
	GetQueue(ctx context.Context, body []byte) ([]byte, error)
	UserJoinDequeue(ctx context.Context, body []byte) ([]byte, error)
	GameserverRegister(ctx context.Context, body []byte) ([]byte, error)
	GameserverUnregister(ctx context.Context, body []byte) ([]byte, error)
}

type rmqHanders struct {
	mmManager managers.MatchmakingManager
}

func NewRMQHandlers(mmManager managers.MatchmakingManager) RMQHandlers {
	return &rmqHanders{
		mmManager: mmManager,
	}
}

func (h *rmqHanders) UserJoinQueue(ctx context.Context, body []byte) ([]byte, error) {
	logger := rmq.GetLogger(ctx)
	request := &matchmaking.MatchmakingQueueRequest{}
	if err := json.Unmarshal(body, request); err != nil {
		logger.WithError(err).Error("Failed to parse matchmaking request")
		return nil, matchmaking.ErrInvalidRequest
	}
	queueID, err := h.mmManager.QueueParty(ctx, request.PartyID, request.PlayerIDs)
	if err != nil {
		logger.WithError(err).Error("Failed to queue matchmaking party")
		return nil, matchmaking.ErrFailedToQueueParty
	}
	return json.Marshal(matchmaking.MatchmakingQueueResponse{
		PartyID: queueID,
	})
}

func (h *rmqHanders) GetQueue(ctx context.Context, body []byte) ([]byte, error) {
	logger := rmq.GetLogger(ctx)
	request := &matchmaking.GetQueueRequest{}
	if err := json.Unmarshal(body, request); err != nil {
		logger.WithError(err).Error("Failed to parse matchmaking request")
		return nil, matchmaking.ErrInvalidRequest
	}
	queue, err := h.mmManager.GetQueue(ctx, request.PartyID)
	if err != nil {
		logger.WithError(err).Error("Failed to queue matchmaking party")
		return nil, matchmaking.ErrFailedToQueueParty
	}
	return json.Marshal(matchmaking.GetQueueResponse{
		PartyID:          queue.PartyID,
		PlayerIDs:        queue.PlayerIDs,
		AssignedServerID: queue.AssignedServerID,
		RegisterTime:     queue.RegisterTime,
		Status:           string(queue.Status),
		Version:          queue.Version,
	})
}

func (h *rmqHanders) UserJoinDequeue(ctx context.Context, body []byte) ([]byte, error) {
	return nil, nil
}

func (h *rmqHanders) GameserverRegister(ctx context.Context, body []byte) ([]byte, error) {
	logger := rmq.GetLogger(ctx)
	request := &matchmaking.GSRegisterRequest{}
	if err := json.Unmarshal(body, request); err != nil {
		logger.WithError(err).Error("Failed to parse gameserver register request")
		return nil, matchmaking.ErrInvalidRequest
	}
	err := h.mmManager.RegisterServer(ctx, request.ServerID, request.IPAddress, request.Port, request.Capacity)
	if err != nil {
		logger.WithError(err).Error("Failed to register game server")
		return nil, matchmaking.ErrFailedToRegisterGameserver
	}
	return json.Marshal(matchmaking.GSRegisterResponse{})
}

func (h *rmqHanders) GameserverUnregister(ctx context.Context, body []byte) ([]byte, error) {
	return nil, nil
}
