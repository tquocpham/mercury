package middleware

import (
	"github.com/labstack/echo/v4"
	"github.com/sirupsen/logrus"
)

// UseTest prints an hello message
func UseTest(logger *logrus.Logger, environment string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			logger.Debug("hello from common/middleware\n")
			return next(c)
		}
	}
}
