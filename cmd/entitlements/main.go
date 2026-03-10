package main

import (
	"context"
	"fmt"

	"github.com/labstack/echo/v4"
	"github.com/mercury/cmd/entitlements/lib/handlers"
	"github.com/mercury/cmd/entitlements/lib/managers"
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

	// basic service configs
	port := cfg.SetDefaultString("web_port", "80", false)
	logLevel := cfg.SetDefaultString("log_level", "info", true)
	environment := cfg.SetDefaultString("environment", "container", true)
	statsdAddr := cfg.SetDefaultString("statsd_addr", "telegraf:8125", false)
	// jwt configs
	pubKeySSMParam := cfg.SetDefaultString("pub_key_ssm_param", "/mercury/jwt-public-key", false)
	awsAccessKey := cfg.SetDefaultString("aws_access_key", "test", true)
	awsSecretKey := cfg.SetDefaultString("aws_secret_key", "test", true)
	awsRegion := cfg.SetDefaultString("aws_region", "us-west-1", true)
	awsEndpoint := cfg.SetDefaultString("aws_endpoint", "", false)
	// database configs
	mongoAddr := cfg.SetDefaultString("mongo_addr", "mongodb://root:root@mongo:27017", true)

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
	catalogManager, err := managers.NewentitlementsManager(mongoAddr)
	if err != nil {
		logrus.Fatal(err)
	}
	grantsManager, err := managers.NewGrantsManager(mongoAddr)
	if err != nil {
		logrus.Fatal(err)
	}

	statsdClient := middleware.NewStatsdClient(statsdAddr, "auth")

	e := echo.New()
	e.Use(middleware.UseLogger(logger, environment))
	e.Use(middleware.UseStatsd(statsdClient))
	h := handlers.NewEntitlementHandlers(grantsManager, catalogManager)

	// hcRoutes := e.Group("api/v1")
	// hcRoutes.GET("/ping", hch.Ping)
	v1 := e.Group("api/v1",
		// only admins can modify entitlements.
		// This api is only supposed to be used by admin and other services.
		middleware.UseAuth(k.Public, middleware.EnforceRoles("admin")))
	v1.GET("/check", h.Check)
	v1.GET("/grant", h.Grant)
	v1.GET("/revoke", h.Revoke)

	if err := server.Serve(e, fmt.Sprintf(":%s", port)); err != nil {
		logger.Fatal(err)
	}
}
