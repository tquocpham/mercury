package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	courier "github.com/mercury/cmd/tradecourier/lib"
	"github.com/mercury/pkg/clients/inventory"
	"github.com/mercury/pkg/clients/wallet"
	"github.com/mercury/pkg/config"
	"github.com/mercury/pkg/middleware"
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
	logLevel := cfg.SetDefaultString("log_level", "info", true)
	statsdAddr := cfg.SetDefaultString("statsd_addr", "telegraf:8125", false)
	amqpURL := cfg.SetDefaultString("amqp_url", "amqp://guest:guest@rabbitmq:5672/", false)
	// // database configs
	mongoAddr := cfg.SetDefaultString("mongo_addr", "mongodb://root:root@mongo:27017", true)

	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})
	level, err := logrus.ParseLevel(logLevel)
	if err != nil {
		logrus.Fatal(err)
	}
	logger.SetLevel(level)

	// 1. Setup Context with Cancellation for Graceful Shutdown
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	walletClient, err := wallet.NewClient(amqpURL)
	if err != nil {
		logrus.Fatal(err)
	}
	inventoryClient, err := inventory.NewClient(amqpURL)
	if err != nil {
		logrus.Fatal(err)
	}
	statsdClient := middleware.NewStatsdClient(statsdAddr, "query")
	interval := 1 * time.Second
	courier, err := courier.NewCourier(interval, mongoAddr, inventoryClient, walletClient, statsdClient)
	if err != nil {
		logrus.Fatal(err)
	}

	log.Println("Courier Relay started. Churning outbox...")

	// 5. Start the courier loop
	// We run this in its own goroutine if we want to add more workers
	go courier.Run(ctx, logger)

	// 6. Wait for Shutdown Signal
	<-ctx.Done()
	log.Println("Courier shutting down")
}
