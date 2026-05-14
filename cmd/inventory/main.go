package main

import (
	"github.com/mercury/cmd/inventory/lib/handlers"
	"github.com/mercury/cmd/inventory/lib/managers"
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
	logLevel := cfg.SetDefaultString("log_level", "info", false)
	statsdAddr := cfg.SetDefaultString("statsd_addr", "telegraf:8125", false)
	mongoAddr := cfg.SetDefaultString("mongo_addr", "mongodb://root:root@mongo:27017", false)

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

	statsdClient := middleware.NewStatsdClient(statsdAddr, "inventory")

	inventoryManager, err := managers.NewInventoryManager(mongoAddr, statsdClient)
	if err != nil {
		logrus.Fatal(err)
	}

	rmqHandlers := handlers.NewRMQHandlers(inventoryManager)
	consumer, err := rmq.NewConsumer(amqpURL, logger)
	if err != nil {
		logrus.Fatal(err)
	}
	defer consumer.Close()
	consumer.Consume("inventory.v1.createinventory", rmqHandlers.CreateInventory,
		rmq.UseLogger(logger),
		rmq.UseStatsd(statsdClient),
	)
	consumer.Consume("inventory.v1.getinventory", rmqHandlers.GetInventory,
		rmq.UseLogger(logger),
		rmq.UseStatsd(statsdClient),
	)
	consumer.Consume("inventory.v1.additem", rmqHandlers.AddItem,
		rmq.UseLogger(logger),
		rmq.UseStatsd(statsdClient),
	)
	consumer.Consume("inventory.v1.additemtoslot", rmqHandlers.AddItemToSlot,
		rmq.UseLogger(logger),
		rmq.UseStatsd(statsdClient),
	)
	consumer.Wait()
}
