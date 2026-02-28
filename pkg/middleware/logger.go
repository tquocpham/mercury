package middleware

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/sirupsen/logrus"
)

const ContextKeyLogger = "Logger"

// UseLogger adds a structured logger to the request context.
func UseLogger(logger *logrus.Logger, environment string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			req := c.Request()
			log := logger.WithContext(req.Context())
			correlationID := uuid.New().String()
			log = log.WithFields(logrus.Fields{
				"content_length": req.ContentLength,
				"method":         req.Method,
				"rpath":          c.Path(),
				"path":           req.URL.Path,
				"raw_query":      req.URL.RawQuery,
				"correlation_id": correlationID,
				"environment":    environment,
			})
			c.Set(ContextKeyLogger, log)
			err := next(c)
			status := c.Response().Status
			if err != nil {
				if he, ok := err.(*echo.HTTPError); ok {
					status = he.Code
				} else {
					status = http.StatusInternalServerError
				}
			}
			log.WithFields(logrus.Fields{
				"status": status,
			}).Info("request")
			return err
		}
	}
}

func GetLogger(c echo.Context) *logrus.Entry {
	l, ok := c.Get(ContextKeyLogger).(*logrus.Entry)
	if !ok || l == nil {
		return logrus.NewEntry(logrus.StandardLogger())
	}
	return l
}
