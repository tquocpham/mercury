package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/mercury/cmd/query/lib/handlers"
	"github.com/mercury/cmd/query/lib/managers"
	"github.com/mercury/pkg/clients/publisher"
	"github.com/mercury/pkg/clients/worker"
	"github.com/mercury/pkg/config"
	"github.com/mercury/pkg/kmq"
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
	logLevel := cfg.SetDefaultString("log_level", "info", true)
	environment := cfg.SetDefaultString("environment", "local", true)
	statsdAddr := cfg.SetDefaultString("statsd_addr", "telegraf:8125", false)
	cassHost := cfg.SetDefaultString("cassandra_host", "cassandra", false)
	publisherHost := cfg.SetDefaultString("publisher_addr", "http://publisher:9003", true)
	broker := cfg.SetDefaultString("kafka_broker", "kafka:9092", true)
	topic := cfg.SetDefaultString("kafka_topic", "messages", true)
	redisAddr := cfg.SetDefaultString("redis_addr", "redis:6379", false)
	redisPassword := cfg.SetDefaultString("redis_pw", "", true)

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

	brokers := []string{broker}
	producer := kmq.NewProducer(brokers)
	defer producer.Close()
	workerClient := worker.NewClient(topic, producer)

	publisherClient := publisher.NewClient(publisherHost, &http.Client{
		Timeout: 5 * time.Second,
	})

	redisClient := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: redisPassword,
	})

	messagesHandler := handlers.NewMessageHandlers(cassClient, publisherClient, workerClient, redisClient)

	e := echo.New()
	messageRoutes := e.Group("api/v1",
		middleware.UseLogger(logger, environment),
		middleware.UseStatsd(statsdClient))
	messageRoutes.POST("/messages", messagesHandler.SendMessage)
	messageRoutes.GET("/messages", messagesHandler.GetMessages)
	messageRoutes.GET("/messages/refresh", messagesHandler.RefreshMessages)
	if err := server.Serve(e, fmt.Sprintf(":%s", port)); err != nil {
		logger.Fatal(err)
	}
}
