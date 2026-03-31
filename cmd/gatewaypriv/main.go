package main

import (
	"context"
	"fmt"

	"github.com/labstack/echo/v4"
	"github.com/mercury/cmd/gatewaypriv/lib/handlers"
	"github.com/mercury/pkg/clients/matchmaking"
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
	logLevel := cfg.SetDefaultString("log_level", "info", false)
	environment := cfg.SetDefaultString("environment", "local", false)
	statsdAddr := cfg.SetDefaultString("statsd_addr", "telegraf:8125", false)
	amqpURL := cfg.SetDefaultString("amqp_url", "amqp://guest:guest@rabbitmq:5672/", false)
	awsAccessKey := cfg.SetDefaultString("aws_access_key", "test", true)
	awsSecretKey := cfg.SetDefaultString("aws_secret_key", "test", true)
	awsRegion := cfg.SetDefaultString("aws_region", "us-west-1", true)
	awsEndpoint := cfg.SetDefaultString("aws_endpoint", "", false)

	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})
	level, err := logrus.ParseLevel(logLevel)
	if err != nil {
		logrus.Fatal(err)
	}
	logger.SetLevel(level)

	for k, v := range cfg.AllSettings() {
		logger.WithFields(logrus.Fields{
			"k": k,
			"v": v,
		}).Info("config")
	}

	// Load public key for JWT validation (game servers authenticate with the same JWT infra)
	ssmClient := config.NewSSMClient(context.Background(), config.AWSConfig{
		AccessKey: awsAccessKey,
		SecretKey: awsSecretKey,
		Region:    awsRegion,
		Endpoint:  awsEndpoint,
	})
	pubKeySSMParam := cfg.SetDefaultString("pub_key_ssm_param", "/mercury/jwt-public-key", false)
	k := config.NewKeys()
	if err := k.LoadPublicFromSSM(ssmClient, pubKeySSMParam); err != nil {
		panic(err)
	}

	mmClient, err := matchmaking.NewRMQClient(amqpURL)
	if err != nil {
		logrus.Fatal(err)
	}
	defer mmClient.Close()

	statsdClient := middleware.NewStatsdClient(statsdAddr, "gatewaypriv")
	gsHandlers := handlers.NewGameserverHandlers(mmClient)

	e := echo.New()
	v1 := e.Group("api/v1",
		middleware.UseLogger(logger, environment),
		middleware.UseStatsd(statsdClient))
	gsv1 := v1.Group("/gs")
	gsv1.POST("/register", gsHandlers.Register)
	gsv1.POST("/unregister", gsHandlers.Unregister)

	if err := server.Serve(e, fmt.Sprintf(":%s", port)); err != nil {
		logger.Fatal(err)
	}
}
