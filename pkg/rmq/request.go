package rmq

import (
	"context"
	"encoding/json"

	"github.com/mercury/pkg/instrumentation"
	"github.com/smira/go-statsd"
)

// CodedError is a sentinel error with a numeric code so errors.Is works across the RMQ boundary.
// It doubles as the wire format for error replies (JSON tags included).
type CodedError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// NewError creates a new CodedError sentinel. Use this to define package-level error variables.
func NewError(code int, message string) *CodedError {
	return &CodedError{
		Code:    code,
		Message: message,
	}
}

func (e *CodedError) Error() string { return e.Message }

// Is matches by code so errors.Is(remoteErr, SentinelErr) works even when the
// remote error was reconstructed from a wire code rather than the original pointer.
func (e *CodedError) Is(target error) bool {
	t, ok := target.(*CodedError)
	if !ok {
		return false
	}
	return e.Code == t.Code
}

func Request[Req any, Resp any](ctx context.Context, p *Publisher, route string, req Req) (_ *Resp, err error) {
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
	// Check if the response is an error reply.
	var errResp CodedError
	if json.Unmarshal(response, &errResp) == nil && errResp.Code != 0 {
		return nil, &errResp
	}
	var resp Resp
	if err := json.Unmarshal(response, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
