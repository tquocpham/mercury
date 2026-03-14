package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/mercury/cmd/gateway/lib/handlers"
	"github.com/mercury/pkg/clients/auth"
	"github.com/mercury/pkg/clients/query"
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
	logLevel := cfg.SetDefaultString("log_level", "info", true)
	environment := cfg.SetDefaultString("environment", "local", true)
	queryHost := cfg.SetDefaultString("query_host", "http://query:9002", true)
	statsdAddr := cfg.SetDefaultString("statsd_addr", "telegraf:8125", true)
	redisAddr := cfg.SetDefaultString("redis_addr", "redis:6379", false)
	redisPassword := cfg.SetDefaultString("redis_pw", "", true)
	pubKeySSMParam := cfg.SetDefaultString("pub_key_ssm_param", "/mercury/jwt-public-key", false)
	amqpURL := cfg.SetDefaultString("amqp_url", "amqp://guest:guest@rabbitmq:5672/", false)

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

	queryClient := query.NewClient(queryHost, &http.Client{
		Timeout: 10 * time.Second,
	})
	authClient, err := auth.NewRMQClient(amqpURL)
	if err != nil {
		logrus.Fatal(err)
	}
	defer authClient.Close()

	statsdClient := middleware.NewStatsdClient(statsdAddr, "gateway")

	redisClient := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: redisPassword,
	})

	messagesHandler := handlers.NewMessageHandlers(queryClient, redisClient)
	authHandlers := handlers.NewAuthHandlers(authClient)
	hch := handlers.NewHealthCheckHandlers()

	// TODO: implement ratelimiter
	// https://pkg.go.dev/github.com/webx-top/echo/middleware/ratelimiter#RateLimiterWithConfig
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
	v1.POST("/messages", messagesHandler.SendMessage,
		middleware.UseAuth(k.Public))
	v1.GET("/messages", messagesHandler.GetMessages,
		middleware.UseAuth(k.Public))
	v1.GET("/messages/refresh", messagesHandler.RefreshMessages,
		middleware.UseAuth(k.Public))

	v1.POST("/auth/login", authHandlers.Login)
	v1.POST("/auth/refresh", authHandlers.Refresh,
		middleware.UseAuth(k.Public))
	v1.POST("/auth/revoke", authHandlers.Revoke,
		middleware.UseAuth(k.Public, middleware.EnforceRoles("admin")))
	v1.POST("/account", authHandlers.CreateAccount)
	// TODO: This link will get emailed out to the user when the email
	// service is setup. For now it can just be chained from /account
	v1.POST("/account/activate/:accountid", authHandlers.ActivateAccount)

	if err := server.Serve(e, fmt.Sprintf(":%s", port)); err != nil {
		logger.Fatal(err)
	}

}
