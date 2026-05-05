package handlers

import (
	"context"
	"encoding/json"

	"github.com/mercury/pkg/archiver"
	"github.com/mercury/pkg/rmq"
)

type ArchivePlayerRequest struct {
	PlayerID string `json:"player_id"`
	Source   string `json:"source"` // "wallet" or "inventory"
}

type ArchivePlayerResponse struct {
	PlayerID string `json:"player_id"`
	Source   string `json:"source"`
	Archived int    `json:"archived"`
}

var ErrInvalidRequest = rmq.NewError(9000, "failed to read request")
var ErrSourceNotFound = rmq.NewError(9001, "unknown archive source")
var ErrArchiveFailed  = rmq.NewError(9002, "archive failed")

type RMQHandlers interface {
	ArchivePlayer(ctx context.Context, body []byte) ([]byte, error)
}

type rmqHandlers struct {
	archivers map[string]*archiver.Archiver
}

func NewRMQHandlers(archivers map[string]*archiver.Archiver) RMQHandlers {
	return &rmqHandlers{archivers: archivers}
}

func (h *rmqHandlers) ArchivePlayer(ctx context.Context, body []byte) ([]byte, error) {
	logger := rmq.GetLogger(ctx)
	request := &ArchivePlayerRequest{}
	if err := json.Unmarshal(body, request); err != nil {
		logger.WithError(err).Error("failed to parse archive player request")
		return nil, ErrInvalidRequest
	}

	arc, ok := h.archivers[request.Source]
	if !ok {
		logger.WithField("source", request.Source).Error("unknown archive source")
		return nil, ErrSourceNotFound
	}

	count, err := arc.ArchivePlayer(ctx, request.PlayerID)
	if err != nil {
		logger.WithError(err).WithFields(map[string]any{
			"player_id": request.PlayerID,
			"source":    request.Source,
		}).Error("archive player failed")
		return nil, ErrArchiveFailed
	}

	bts, err := json.Marshal(ArchivePlayerResponse{
		PlayerID: request.PlayerID,
		Source:   request.Source,
		Archived: count,
	})
	if err != nil {
		return nil, ErrArchiveFailed
	}
	return bts, nil
}
