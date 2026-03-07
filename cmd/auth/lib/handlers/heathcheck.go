package handlers

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/mercury/pkg/clients/auth"
)

type HeathCheckHandlers interface {
	Ping(c echo.Context) error
}

type heathCheckHandlers struct {
}

func NewHealthCheckHandlers() HeathCheckHandlers {
	return &heathCheckHandlers{}
}

// Ping heathcheck
func (h *heathCheckHandlers) Ping(c echo.Context) error {
	return c.JSON(http.StatusOK, auth.PingResponse{
		Ping: "pong",
	})
}
