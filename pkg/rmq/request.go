package rmq

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mercury/pkg/instrumentation"
	"github.com/smira/go-statsd"
)

func Request[Req any, Resp any](ctx context.Context, p *Publisher, route string, req Req) (_ *Resp, err error) {
	metricsname := fmt.Sprintf("rmq.%s", route)
	t := instrumentation.NewMetricsTimer(ctx, metricsname, statsd.StringTag("r", route))
	defer func() { t.Done(err) }()

	b, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	response, err := p.Request(route, b)
	if err != nil {
		return nil, err
	}
	var env envelope
	if err := json.Unmarshal(response, &env); err != nil {
		return nil, err
	}
	if env.Version != envelopeVersion {
		return nil, NewError(503, "unsupported envelope version")
	}
	switch env.Type {
	case responseTypeError:
		var rmqErr Error
		if err := json.Unmarshal(env.Response, &rmqErr); err != nil {
			return nil, err
		}
		return nil, &rmqErr
	case responseTypeSuccess:
		var resp Resp
		if err := json.Unmarshal(env.Response, &resp); err != nil {
			return nil, err
		}
		return &resp, nil
	default:
		return nil, NewError(500, "unknown response type")
	}
}
