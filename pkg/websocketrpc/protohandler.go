package websocketrpc

import (
	"context"

	"google.golang.org/protobuf/proto"
)

func ProtoHandler[Req proto.Message, Res proto.Message](
	reqFactory func() Req,
	logic func(context.Context, Req) (Res, error),
) Handler {
	return func(ctx context.Context, payload []byte) ([]byte, error) {
		req := reqFactory()
		if err := proto.Unmarshal(payload, req); err != nil {
			return nil, err
		}

		res, err := logic(ctx, req)
		if err != nil {
			return nil, err
		}

		return proto.Marshal(res)
	}
}
