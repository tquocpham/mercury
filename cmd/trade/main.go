package main

import (
	"github.com/mercury/cmd/trade/lib/handlers"
	"github.com/mercury/cmd/trade/lib/managers"
	"github.com/mercury/pkg/config"
	"github.com/mercury/pkg/middleware"
	"github.com/mercury/pkg/rmq"
	"github.com/sirupsen/logrus"
)

func main() {
	cfg := config.NewConfig("yaml")
	err := cfg.LoadPaths(config.DefaultConfigPaths)
	if err != nil {
		panic(err.Error())
	}
	amqpURL := cfg.SetDefaultString("amqp_url", "amqp://guest:guest@rabbitmq:5672/", false)
	logLevel := cfg.SetDefaultString("log_level", "info", true)
	statsdAddr := cfg.SetDefaultString("statsd_addr", "telegraf:8125", false)
	mongoAddr := cfg.SetDefaultString("mongo_addr", "mongodb://root:root@mongo:27017", true)

	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})
	level, err := logrus.ParseLevel(logLevel)
	if err != nil {
		logrus.Fatal(err)
	}
	logger.SetLevel(level)

	statsdClient := middleware.NewStatsdClient(statsdAddr, "trade")

	outboxManager, err := managers.NewOutboxManager(mongoAddr, statsdClient)
	if err != nil {
		logrus.Fatal(err)
	}

	rmqHandlers := handlers.NewRMQHandlers(outboxManager)
	consumer, err := rmq.NewConsumer(amqpURL, logger)
	if err != nil {
		logrus.Fatal(err)
	}
	defer consumer.Close()
	consumer.Consume("trade.v1.trade", rmqHandlers.Trade,
		rmq.UseLogger(logger),
		rmq.UseStatsd(statsdClient),
	)
	consumer.Consume("trade.v1.status", rmqHandlers.TradeStatus,
		rmq.UseLogger(logger),
		rmq.UseStatsd(statsdClient),
	)
	consumer.Wait()
}
