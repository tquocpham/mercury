package main

import (
	"context"
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
	topic := cfg.SetDefaultString("kafka_topic", "messages", true)
	broker := cfg.SetDefaultString("kafka_broker", "kafka:9092", true)
	environment := cfg.SetDefaultString("environment", "local", true)
	queryHost := cfg.SetDefaultString("query_host", "http://query:9002", true)
	statsdAddr := cfg.SetDefaultString("statsd_addr", "telegraf:8125", true)
	redisAddr := cfg.SetDefaultString("redis_addr", "redis:6379", false)
	redisPassword := cfg.SetDefaultString("redis_pw", "", true)
	pubKeySSMParam := cfg.SetDefaultString("pub_key_ssm_param", "/mercury/jwt-public-key", false)

	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})
	level, err := logrus.ParseLevel(logLevel)
	if err != nil {
		logrus.Fatal(err)
	}
	logger.SetLevel(level)

	ssmClient := config.NewSSMClient(context.Background(), config.AWSConfig{
		AccessKey: cfg.SetDefaultString("aws_access_key", "test", true),
		SecretKey: cfg.SetDefaultString("aws_secret_key", "test", true),
		Region:    cfg.SetDefaultString("aws_region", "us-west-1", true),
		Endpoint:  cfg.SetDefaultString("aws_endpoint", "", false),
	})
	k := config.NewKeys()
	if err := k.LoadPublicFromSSM(ssmClient, pubKeySSMParam); err != nil {
		panic(err)
	}

	brokers := []string{broker}
	producer := kmq.NewProducer(brokers)
	defer producer.Close()

	workerClient := worker.NewClient(topic, producer)

	queryClient := query.NewClient(queryHost, &http.Client{
		Timeout: 10 * time.Second,
	})

	statsdClient := middleware.NewStatsdClient(statsdAddr, "gateway")

	redisClient := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: redisPassword,
	})

	messagesHandler := handlers.NewMessageHandlers(workerClient, queryClient, redisClient)
	hch := handlers.NewHealthCheckHandlers()

	e := echo.New()
	hc := e.Group("api/v1/hc",
		middleware.UseLogger(logger, environment),
		middleware.UseStatsd(statsdClient))
	hc.GET("/ping", hch.Ping)
	hc.GET("/auth", hch.Ping,
		middleware.UseAuth(k.Public, middleware.EnforceRoles("admin")))
	v1 := e.Group("api/v1",
		middleware.UseLogger(logger, environment),
		middleware.UseStatsd(statsdClient))
	v1.POST("/messages", messagesHandler.SendMessage)
	v1.GET("/messages", messagesHandler.GetMessages)
	v1.GET("/messages/refresh", messagesHandler.RefreshMessages)
	if err := server.Serve(e, fmt.Sprintf(":%s", port)); err != nil {
		logger.Fatal(err)
	}
}
