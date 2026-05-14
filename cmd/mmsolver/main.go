package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mercury/cmd/mmsolver/lib/managers"
	"github.com/mercury/cmd/mmsolver/lib/solver"
	"github.com/mercury/pkg/clients/publisher"
	"github.com/mercury/pkg/config"
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
	solverWorkInterval := cfg.SetDefaultDuration("solver_work_check_interval", 500*time.Millisecond, false)
	solverCheckInterval := cfg.SetDefaultDuration("solver_check_interval", 5*time.Second, false)
	maxSolveTime := cfg.SetDefaultDuration("max_solve_time", 5*time.Minute, false)

	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})
	level, err := logrus.ParseLevel(logLevel)
	if err != nil {
		logrus.Fatal(err)
	}
	logger.SetLevel(level)

	statsdClient := middleware.NewStatsdClient(statsdAddr, "mmsolver")

	publisherClient, err := publisher.NewRMQClient(amqpURL)
	if err != nil {
		logrus.Fatal(err)
	}
	defer publisherClient.Close()

	mmManager, err := managers.NewAMatchmakingManager(mongoAddr)
	if err != nil {
		logrus.Fatal(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	solver := solver.NewMMSolver(
		solverCheckInterval, solverWorkInterval, maxSolveTime, publisherClient,
		mmManager, statsdClient)
	solver.Solve(ctx, logger)
}
