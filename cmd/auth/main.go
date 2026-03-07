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

	ssmClient := config.NewSSMClient(context.Background(), config.AWSConfig{
		AccessKey: cfg.SetDefaultString("aws_access_key", "test", true),
		SecretKey: cfg.SetDefaultString("aws_secret_key", "test", true),
		Region:    cfg.SetDefaultString("aws_region", "us-west-1", true),
		Endpoint:  cfg.SetDefaultString("ssm_endpoint", "", true),
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

	// TODO CREATE A USERS DATABASE
	userManager, err := managers.NewUsersManager(mongoAddr)
	if err != nil {
		logrus.Fatal(err)
	}

	hch := handlers.NewHealthCheckHandlers()

	statsdClient := middleware.NewStatsdClient(statsdAddr, "auth")

	e := echo.New()
	e.Use(middleware.UseLogger(logger, environment))
	e.Use(middleware.UseStatsd(statsdClient))

	hcRoutes := e.Group("api/v1")
	hcRoutes.GET("/ping", hch.Ping)

	authHandlers := handlers.NewAuthHandler(userManager, time.Hour, k.Private, k.Public)
	v1 := e.Group("api/v1")
	v1.POST("/auth", authHandlers.Signin)
	v1.GET("/auth/refresh", authHandlers.Refresh)

	if err := server.Serve(e, fmt.Sprintf(":%s", port)); err != nil {
		logger.Fatal(err)
	}
}
