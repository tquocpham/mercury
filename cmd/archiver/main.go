package main

import (
	"context"
	"time"

	archandlers "github.com/mercury/cmd/archiver/lib/handlers"
	"github.com/mercury/pkg/archiver"
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
	logLevel := cfg.SetDefaultString("log_level", "info", false)
	mongoAddr := cfg.SetDefaultString("mongo_addr", "mongodb://root:root@mongo:27017", false)
	amqpURL := cfg.SetDefaultString("amqp_url", "amqp://guest:guest@rabbitmq:5672/", false)
	statsdAddr := cfg.SetDefaultString("statsd_addr", "telegraf:8125", false)
	retentionDays := cfg.SetDefaultInt("archive_retention_days", 30, false)
	archiveInterval := cfg.SetDefaultDuration("archive_interval", 24*time.Hour, false)
	archiveBatchSize := cfg.SetDefaultInt("archive_batch_size", 500, false)

	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})
	level, err := logrus.ParseLevel(logLevel)
	if err != nil {
		logrus.Fatal(err)
	}
	logger.SetLevel(level)

	retention := time.Duration(retentionDays) * 24 * time.Hour
	statsdClient := middleware.NewStatsdClient(statsdAddr, "archiver")

	collections := []struct {
		sourceDB   string
		sourceColl string
		sourceName string
	}{
		{"wallet", "wallets", "wallet"},
		{"inventory", "inventory", "inventory"},
	}

	ctx := context.Background()
	archivers := make(map[string]*archiver.Archiver, len(collections))

	for _, c := range collections {
		arc, err := archiver.New(
			mongoAddr,
			c.sourceDB, c.sourceColl,
			"archive", "archived_orders",
			"processed_orders", c.sourceName,
			retention,
			archiveInterval,
			archiveBatchSize,
		)
		if err != nil {
			logrus.WithField("source", c.sourceName).Fatal(err)
		}
		archivers[c.sourceName] = arc
		go arc.Run(ctx, logger)
		logger.WithField("source", c.sourceName).Info("archiver started")
	}

	rmqHandlers := archandlers.NewRMQHandlers(archivers)
	consumer, err := rmq.NewConsumer(amqpURL, logger)
	if err != nil {
		logrus.Fatal(err)
	}
	defer consumer.Close()
	consumer.Consume("archiver.v1.archive_player", rmqHandlers.ArchivePlayer,
		rmq.UseLogger(logger),
		rmq.UseStatsd(statsdClient),
	)
	consumer.Wait()
}
