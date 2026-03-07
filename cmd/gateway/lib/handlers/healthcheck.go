package handlers

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

type HeathCheckHandlers interface {
	Ping(c echo.Context) error
}

type heathCheckHandlers struct {
}

func NewHealthCheckHandlers() HeathCheckHandlers {
	return &heathCheckHandlers{}
}

type PingResponse struct {
	Ping string `json:"ping"`
}

// Ping heathcheck
func (h *heathCheckHandlers) Ping(c echo.Context) error {
	return c.JSON(http.StatusOK, PingResponse{
		Ping: "pong",
	})
}
