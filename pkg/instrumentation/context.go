package instrumentation

import (
	"context"

	"github.com/labstack/echo/v4"
	"github.com/mercury/pkg/middleware"
	"github.com/sirupsen/logrus"
	"github.com/smira/go-statsd"
)

type loggerCtxKey struct{}

func LoggerFromContext(ctx context.Context) *logrus.Entry {
	l, _ := ctx.Value(loggerCtxKey{}).(*logrus.Entry)
	if l == nil {
		return logrus.NewEntry(logrus.StandardLogger())
	}
	return l
}

type statsdCtxKey struct{}

func StatsdFromContext(ctx context.Context) *statsd.Client {
	s, _ := ctx.Value(statsdCtxKey{}).(*statsd.Client)
	return s
}

// ToContext converts an echo.Context to a plain context.Context with the
// logger and statsd client embedded so they can be passed to non-echo code.
func ToContext(c echo.Context) context.Context {
	ctx := c.Request().Context()
	ctx = context.WithValue(ctx, loggerCtxKey{}, middleware.GetLogger(c))
	ctx = context.WithValue(ctx, statsdCtxKey{}, middleware.GetStatsd(c))
	return ctx
}
