package main

import (
	"fmt"

	"github.com/labstack/echo/v4"
	"github.com/mercury/cmd/subscriber/lib/handlers"
	"github.com/mercury/pkg/config"
	"github.com/mercury/pkg/middleware"
	"github.com/mercury/pkg/server"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
)

func main() {
	cfg := config.NewConfig("yaml")
	err := cfg.LoadPaths(config.DefaultConfigPaths)
	if err != nil {
		panic(err.Error())
	}
	port := cfg.SetDefaultString("web_port", "80", false)
	logLevel := cfg.SetDefaultString("log_level", "info", false)
	redisAddr := cfg.SetDefaultString("redis_addr", "redis:6379", false)
	redisPassword := cfg.SetDefaultString("redis_pw", "", true)
	environment := cfg.SetDefaultString("environment", "local", false)

	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})
	level, err := logrus.ParseLevel(logLevel)
	if err != nil {
		logrus.Fatal(err)
	}
	logger.SetLevel(level)

	redisClient := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: redisPassword,
	})

	handler := handlers.NewNotifierHandlers(redisClient)
	e := echo.New()
	// TODO: intergrate this with statsd. The statsd middleware currently times connection latency
	// assuming brief http connections Websockets are long lived so we need to measure it differently.
	// statsdAddr := cfg.SetDefaultString("statsd_addr", "telegraf:8125", false)
	// statsdClient := middleware.NewStatsdClient(statsdAddr, "websocket")
	v1 := e.Group("api/v1",
		middleware.UseLogger(logger, environment))
	v1.GET("/ws", handler.NotifyClient)

	if err := server.Serve(e, fmt.Sprintf(":%s", port)); err != nil {
		logger.Fatal(err)
	}
}
