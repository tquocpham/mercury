package middleware

import (
	"time"

	"github.com/labstack/echo/v4"
	"github.com/smira/go-statsd"
)

const ContextKeyStatsd = "Statsd"

func NewStatsdClient(addr string, service string) *statsd.Client {
	return statsd.NewClient(
		addr,
		statsd.TagStyle(statsd.TagFormatDatadog),
		statsd.DefaultTags(statsd.StringTag("service", service)),
	)
}

func UseStatsd(client *statsd.Client) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set(ContextKeyStatsd, client)
			start := time.Now()

			defer func() {
				duration := time.Since(start)
				method := c.Request().Method
				path := c.Path()
				status := c.Response().Status

				tags := []statsd.Tag{
					statsd.StringTag("method", method),
					statsd.StringTag("path", path),
				}

				client.Incr("http.request.count", 1, tags...)
				client.Incr("http.response.status", 1, append(tags, statsd.IntTag("status", status))...)
				client.Timing("http.request.duration", int64(duration/time.Millisecond), tags...)
			}()

			return next(c)
		}
	}
}

func GetStatsd(c echo.Context) *statsd.Client {
	s, ok := c.Get(ContextKeyStatsd).(*statsd.Client)
	if !ok || s == nil {
		return statsd.NewClient("")
	}
	return s
}
