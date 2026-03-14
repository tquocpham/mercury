package main

import (
	"context"
	"fmt"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/mercury/cmd/auth/lib/handlers"
	"github.com/mercury/cmd/auth/lib/managers"
	"github.com/mercury/pkg/config"
	"github.com/mercury/pkg/middleware"
	"github.com/mercury/pkg/server"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
)

func main() {
	// configurations
	cfg := config.NewConfig("yaml")
	err := cfg.LoadPaths(config.DefaultConfigPaths)
	if err != nil {
		panic(err.Error())
	}

	port := cfg.SetDefaultString("web_port", "80", false)
	pubKeySSMParam := cfg.SetDefaultString("pub_key_ssm_param", "/mercury/jwt-public-key", false)
	privKeySSMParam := cfg.SetDefaultString("priv_key_ssm_param", "/mercury/jwt-private-key", false)
	logLevel := cfg.SetDefaultString("log_level", "info", true)
	environment := cfg.SetDefaultString("environment", "container", true)
	statsdAddr := cfg.SetDefaultString("statsd_addr", "telegraf:8125", false)
	mongoAddr := cfg.SetDefaultString("mongo_addr", "mongodb://root:root@mongo:27017", true)
	redisAddr := cfg.SetDefaultString("redis_addr", "redis:6379", false)
	redisPassword := cfg.SetDefaultString("redis_pw", "", true)

	ssmClient := config.NewSSMClient(context.Background(), config.AWSConfig{
		AccessKey: cfg.SetDefaultString("aws_access_key", "test", true),
		SecretKey: cfg.SetDefaultString("aws_secret_key", "test", true),
		Region:    cfg.SetDefaultString("aws_region", "us-west-1", true),
		Endpoint:  cfg.SetDefaultString("aws_endpoint", "", true),
	})

	k := config.NewKeys()
	if err := k.LoadPublicFromSSM(ssmClient, pubKeySSMParam); err != nil {
		panic(err)
	}
	if err := k.LoadPrivateFromSSM(ssmClient, privKeySSMParam); err != nil {
		panic(err)
	}

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

	accountsManager, err := managers.NewAccountsManager(mongoAddr)
	if err != nil {
		logrus.Fatal(err)
	}

	sessionsManager := managers.NewSessionsManager(redisClient)

	hch := handlers.NewHealthCheckHandlers()

	statsdClient := middleware.NewStatsdClient(statsdAddr, "auth")

	e := echo.New()
	e.Use(middleware.UseLogger(logger, environment))
	e.Use(middleware.UseStatsd(statsdClient))

	hcRoutes := e.Group("api/v1")
	hcRoutes.GET("/ping", hch.Ping)

	authHandlers := handlers.NewAuthHandler(accountsManager, sessionsManager, time.Hour, k)
	v1 := e.Group("api/v1")
	v1.POST("/auth/login", authHandlers.Signin)
	v1.POST("/auth/refresh", authHandlers.Refresh,
		middleware.UseAuth(k.Public))
	v1.POST("/auth/revoke", authHandlers.Revoke,
		middleware.UseAuth(k.Public, middleware.EnforceRoles("admin")))
	v1.POST("/account", authHandlers.CreateAccount)
	// TODO: This link will get emailed out to the user when the email
	// service is setup. For now it can just be chained from /account
	v1.POST("/account/activate/:accountid", authHandlers.ActivateAccount)

	// Private apis
	// TODO Make this rmq and make these only accessible from within the platform
	v1.GET("/session/:sessionid", authHandlers.GetSession)
	v1.PATCH("/session/:sessionid", authHandlers.ExtendSession)
	v1.DELETE("/session/:sessionid", authHandlers.DeleteSession)

	if err := server.Serve(e, fmt.Sprintf(":%s", port)); err != nil {
		logger.Fatal(err)
	}
}
