package managers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/mercury/pkg/instrumentation"
	"github.com/redis/go-redis/v9"
	"github.com/smira/go-statsd"
)

var ErrSessionNotFound = errors.New("session not found or expired")

type Session struct {
	SessionID string
	UserID    string
	Username  string
	Roles     []string
}

type SessionsManager interface {
	Create(ctx context.Context, userID, username string, roles []string, ttl time.Duration) (_ *Session, err error)
	Get(ctx context.Context, sessionID string) (_ *Session, err error)
	Refresh(ctx context.Context, sessionID string, ttl time.Duration) (err error)
	Delete(ctx context.Context, sessionID string) (err error)
}

type sessionsManager struct {
	redis *redis.Client
}

func NewSessionsManager(redisClient *redis.Client) SessionsManager {
	return &sessionsManager{redis: redisClient}
}

type sessionDocument struct {
	UserID   string   `json:"user_id"`
	Username string   `json:"username"`
	Roles    []string `json:"roles"`
}

func sessionKey(sessionID string) string {
	return fmt.Sprintf("session:%s", sessionID)
}

func (m *sessionsManager) Create(ctx context.Context, userID, username string, roles []string, ttl time.Duration) (_ *Session, err error) {
	t := instrumentation.NewMetricsTimer(ctx, "sessionmgr.dur", statsd.StringTag("op", "create"))
	defer func() { t.Done(err) }()

	sessionID := uuid.New().String()
	doc := sessionDocument{
		UserID:   userID,
		Username: username,
		Roles:    roles,
	}
	data, err := json.Marshal(doc)
	if err != nil {
		return nil, err
	}

	if err := m.redis.Set(ctx, sessionKey(sessionID), data, ttl).Err(); err != nil {
		return nil, err
	}

	return &Session{
		SessionID: sessionID,
		UserID:    userID,
		Username:  username,
		Roles:     roles,
	}, nil
}

func (m *sessionsManager) Get(ctx context.Context, sessionID string) (_ *Session, err error) {
	t := instrumentation.NewMetricsTimer(ctx, "sessionmgr.dur", statsd.StringTag("op", "get"))
	defer func() { t.Done(err) }()

	data, err := m.redis.Get(ctx, sessionKey(sessionID)).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, ErrSessionNotFound
		}
		return nil, err
	}

	var doc sessionDocument
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, err
	}

	return &Session{
		SessionID: sessionID,
		UserID:    doc.UserID,
		Username:  doc.Username,
		Roles:     doc.Roles,
	}, nil
}

func (m *sessionsManager) Refresh(ctx context.Context, sessionID string, ttl time.Duration) (err error) {
	t := instrumentation.NewMetricsTimer(ctx, "sessionmgr.dur", statsd.StringTag("op", "refresh"))
	defer func() { t.Done(err) }()

	ok, err := m.redis.Expire(ctx, sessionKey(sessionID), ttl).Result()
	if err != nil {
		return err
	}
	if !ok {
		return ErrSessionNotFound
	}
	return nil
}

func (m *sessionsManager) Delete(ctx context.Context, sessionID string) (err error) {
	t := instrumentation.NewMetricsTimer(ctx, "sessionmgr.dur", statsd.StringTag("op", "delete"))
	defer func() { t.Done(err) }()

	if err := m.redis.Del(ctx, sessionKey(sessionID)).Err(); err != nil {
		return err
	}
	return nil
}
