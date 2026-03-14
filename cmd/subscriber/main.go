package main

import (
	"context"
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/mercury/cmd/subscriber/lib/handlers"
	"github.com/mercury/pkg/clients/auth"
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
	// JWT Configs
	pubKeySSMParam := cfg.SetDefaultString("pub_key_ssm_param", "/mercury/jwt-public-key", false)
	awsAccessKey := cfg.SetDefaultString("aws_access_key", "test", true)
	awsSecretKey := cfg.SetDefaultString("aws_secret_key", "test", true)
	awsRegion := cfg.SetDefaultString("aws_region", "us-west-1", true)
	awsEndpoint := cfg.SetDefaultString("aws_endpoint", "", false)
	authHost := cfg.SetDefaultString("auth_host", "http://auth:9005", false)

	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})
	level, err := logrus.ParseLevel(logLevel)
	if err != nil {
		logrus.Fatal(err)
	}
	logger.SetLevel(level)

	ssmClient := config.NewSSMClient(context.Background(), config.AWSConfig{
		AccessKey: awsAccessKey,
		SecretKey: awsSecretKey,
		Region:    awsRegion,
		Endpoint:  awsEndpoint,
	})
	k := config.NewKeys()
	if err := k.LoadPublicFromSSM(ssmClient, pubKeySSMParam); err != nil {
		logger.Fatal(err)
	}

	redisClient := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: redisPassword,
	})

	authClient := auth.NewClient(authHost, http.DefaultClient)
	handler := handlers.NewNotifierHandlers(authClient, redisClient)
	e := echo.New()
	// TODO: intergrate this with statsd. The statsd middleware currently times connection latency
	// assuming brief http connections Websockets are long lived so we need to measure it differently.
	// statsdAddr := cfg.SetDefaultString("statsd_addr", "telegraf:8125", false)
	// statsdClient := middleware.NewStatsdClient(statsdAddr, "websocket")
	v1 := e.Group("api/v1",
		middleware.UseLogger(logger, environment))
	v1.GET("/ws", handler.NotifyClient,
		middleware.UseAuth(k.Public))

	if err := server.Serve(e, fmt.Sprintf(":%s", port)); err != nil {
		logger.Fatal(err)
	}
}
