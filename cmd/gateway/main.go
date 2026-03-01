package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/mercury/cmd/gateway/lib/handlers"
	"github.com/mercury/pkg/clients/query"
	"github.com/mercury/pkg/clients/worker"
	"github.com/mercury/pkg/config"
	"github.com/mercury/pkg/kmq"
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
	topic := cfg.SetDefaultString("kafka_topic", "messages", true)
	broker := cfg.SetDefaultString("kafka_broker", "kafka:9092", true)
	environment := cfg.SetDefaultString("environment", "local", true)
	queryHost := cfg.SetDefaultString("query_host", "http://query:9002", true)
	statsdAddr := cfg.SetDefaultString("statsd_addr", "telegraf:8125", true)

	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})
	level, err := logrus.ParseLevel(logLevel)
	if err != nil {
		logrus.Fatal(err)
	}
	logger.SetLevel(level)

	brokers := []string{broker}
	producer := kmq.NewProducer(brokers)
	defer producer.Close()

	workerClient := worker.NewClient(topic, producer)

	queryClient := query.NewClient(queryHost, &http.Client{
		Timeout: 10 * time.Second,
	})

	messagesHandler := handlers.NewMessageHandlers(workerClient, queryClient)
	statsdClient := middleware.NewStatsdClient(statsdAddr, "gateway")

	e := echo.New()
	messageRoutes := e.Group("api/v1",
		middleware.UseLogger(logger, environment),
		middleware.UseStatsd(statsdClient))
	messageRoutes.POST("/messages", messagesHandler.SendMessage,
		middleware.UseLogger(logger, environment))
	messageRoutes.GET("/messages", messagesHandler.GetMessages)
	messageRoutes.GET("/messages/refresh", messagesHandler.RefreshMessages)
	if err := server.Serve(e, fmt.Sprintf(":%s", port)); err != nil {
		logger.Fatal(err)
	}
}
