package main

import (
	"github.com/mercury/cmd/mmsolver/lib/solver"
	"github.com/mercury/pkg/clients/publisher"
	"github.com/mercury/pkg/config"
	"github.com/mercury/pkg/matchmaking/managers"
	"github.com/mercury/pkg/middleware"
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

	statsdClient := middleware.NewStatsdClient(statsdAddr, "query")

	publisherClient, err := publisher.NewRMQClient(amqpURL)
	if err != nil {
		logrus.Fatal(err)
	}
	defer publisherClient.Close()

	mmManager, err := managers.NewAMatchmakingManager(mongoAddr)
	if err != nil {
		logrus.Fatal(err)
	}

	solver := solver.NewMMSolver(publisherClient, mmManager, statsdClient)
	solver.Solve(logger)
}
