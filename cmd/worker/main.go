package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/mercury/cmd/worker/lib/handlers"
	"github.com/mercury/cmd/worker/lib/managers"
	"github.com/mercury/pkg/config"
	"github.com/mercury/pkg/kmq"
	"github.com/mercury/pkg/middleware"
	"github.com/sirupsen/logrus"
)

func main() {
	cfg := config.NewConfig("yaml")
	err := cfg.LoadPaths(config.DefaultConfigPaths)
	if err != nil {
		panic(err.Error())
	}
	logLevel := cfg.SetDefaultString("log_level", "info", true)
	topic := cfg.SetDefaultString("kafka_topic", "messages", true)
	broker := cfg.SetDefaultString("kafka_broker", "kafka:9092", true)
	groupID := cfg.SetDefaultString("kafka_group_id", "messages-consumer-group", true)
	environment := cfg.SetDefaultString("environment", "local", true)
	statsdAddr := cfg.SetDefaultString("statsd_addr", "telegraf:8125", false)
	cassHost := cfg.SetDefaultString("cassandra_host", "localhost", false)

	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})
	level, err := logrus.ParseLevel(logLevel)
	if err != nil {
		logrus.Fatal(err)
	}
	logger.SetLevel(level)

	cass, err := managers.NewCassandraClient(cassHost)
	if err != nil {
		logger.WithError(err).Fatal("cassandra: failed to connect")
	}
	defer cass.Close()

	brokers := []string{broker}

	statsdClient := middleware.NewStatsdClient(statsdAddr, "worker")

	consumer := kmq.NewKafkaConsumer(brokers, groupID, topic, logger)
	defer consumer.Close()

	kh := handlers.NewKafkaHandlers(cass)

	consumer.Consume(
		kh.SaveMessage,
		kmq.UseLogger(logger, environment),
		kmq.UseStatsd(statsdClient),
	)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down")

}
