package main

import (
	"context"
	"time"

	"github.com/mercury/cmd/auth/lib/handlers"
	"github.com/mercury/cmd/auth/lib/managers"
	"github.com/mercury/pkg/config"
	"github.com/mercury/pkg/middleware"
	"github.com/mercury/pkg/rmq"
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

	amqpURL := cfg.SetDefaultString("amqp_url", "amqp://guest:guest@rabbitmq:5672/", false)
	pubKeySSMParam := cfg.SetDefaultString("pub_key_ssm_param", "/mercury/jwt-public-key", false)
	privKeySSMParam := cfg.SetDefaultString("priv_key_ssm_param", "/mercury/jwt-private-key", false)
	logLevel := cfg.SetDefaultString("log_level", "info", true)
	statsdAddr := cfg.SetDefaultString("statsd_addr", "telegraf:8125", false)
	mongoAddr := cfg.SetDefaultString("mongo_addr", "mongodb://root:root@mongo:27017", true)
	redisAddr := cfg.SetDefaultString("redis_addr", "redis:6379", false)
	redisPassword := cfg.SetDefaultString("redis_pw", "", true)
	awsAccessKey := cfg.SetDefaultString("aws_access_key", "test", true)
	awsSecretKey := cfg.SetDefaultString("aws_secret_key", "test", true)
	awsRegion := cfg.SetDefaultString("aws_region", "us-west-1", true)
	awsEndpoint := cfg.SetDefaultString("aws_endpoint", "", true)

	ssmClient := config.NewSSMClient(context.Background(), config.AWSConfig{
		AccessKey: awsAccessKey,
		SecretKey: awsSecretKey,
		Region:    awsRegion,
		Endpoint:  awsEndpoint,
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

	// hch := handlers.NewHealthCheckHandlers()

	statsdClient := middleware.NewStatsdClient(statsdAddr, "auth")

	rmqHandlers := handlers.NewRMQHandlers(
		accountsManager, sessionsManager, time.Hour, k)

	consumer, err := rmq.NewConsumer(amqpURL, logger)
	if err != nil {
		logrus.Fatal(err)
	}
	defer consumer.Close()
	consumer.Consume("auth.v1.login", rmqHandlers.Login,
		rmq.UseLogger(logger),
		rmq.UseStatsd(statsdClient),
	)
	consumer.Consume("auth.v1.refresh", rmqHandlers.Refresh,
		rmq.UseLogger(logger),
		rmq.UseStatsd(statsdClient),
	)
	consumer.Consume("auth.v1.revoke", rmqHandlers.Revoke,
		rmq.UseLogger(logger),
		rmq.UseStatsd(statsdClient),
	)
	consumer.Consume("auth.v1.createaccount", rmqHandlers.CreateAccount,
		rmq.UseLogger(logger),
		rmq.UseStatsd(statsdClient),
	)
	consumer.Consume("auth.v1.activateaccount", rmqHandlers.ActivateAccount,
		rmq.UseLogger(logger),
		rmq.UseStatsd(statsdClient),
	)
	// only expected to be exposed private
	consumer.Consume("auth.v1.getsession", rmqHandlers.GetSession,
		rmq.UseLogger(logger),
		rmq.UseStatsd(statsdClient),
	)
	consumer.Consume("auth.v1.refreshsession", rmqHandlers.RefreshSession,
		rmq.UseLogger(logger),
		rmq.UseStatsd(statsdClient),
	)
	consumer.Consume("auth.v1.deletesession", rmqHandlers.DeleteSession,
		rmq.UseLogger(logger),
		rmq.UseStatsd(statsdClient),
	)
	consumer.Wait()
}
