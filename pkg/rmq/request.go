package rmq

import (
	"context"
	"encoding/json"

	"github.com/mercury/pkg/instrumentation"
	"github.com/smira/go-statsd"
)

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
	var resp Resp
	if err := json.Unmarshal(response, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
