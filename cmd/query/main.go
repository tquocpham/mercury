package main

import (
	"fmt"

	"github.com/labstack/echo/v4"
	"github.com/mercury/cmd/query/lib/handlers"
	"github.com/mercury/cmd/query/lib/managers"
	"github.com/mercury/pkg/config"
	"github.com/mercury/pkg/middleware"
	"github.com/mercury/pkg/server"
	"github.com/sirupsen/logrus"
)

func main() {
	cfg := config.NewConfig("yaml")
	err := cfg.LoadPaths(config.DefaultConfigPaths)
	if err != nil {
		panic(err.Error())
	}
	port := cfg.SetDefaultString("web_port", "80", false)
	logLevel := cfg.SetDefaultString("log_level", "info", true)
	environment := cfg.SetDefaultString("environment", "local", true)
	statsdAddr := cfg.SetDefaultString("statsd_addr", "telegraf:8125", false)
	cassHost := cfg.SetDefaultString("cassandra_host", "localhost", false)

	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})
	level, err := logrus.ParseLevel(logLevel)
	if err != nil {
		logrus.Fatal(err)
	}
	logger.SetLevel(level)

	cassClient, err := managers.NewCassandraClient(cassHost)
	if err != nil {
		logger.WithError(err).Fatal("cassandra: failed to connect")
	}
	defer cassClient.Close()

	statsdClient := middleware.NewStatsdClient(statsdAddr, "query")

	messagesHandler := handlers.NewMessageHandlers(cassClient)

	e := echo.New()
	messageRoutes := e.Group("api/v1",
		middleware.UseLogger(logger, environment),
		middleware.UseStatsd(statsdClient))
	messageRoutes.GET("/messages", messagesHandler.GetMessages)
	messageRoutes.GET("/messages/refresh", messagesHandler.RefreshMessages)
	if err := server.Serve(e, fmt.Sprintf(":%s", port)); err != nil {
		logger.Fatal(err)
	}
}
