package main

import (
	"github.com/mercury/cmd/messages/lib/handlers"
	"github.com/mercury/cmd/messages/lib/managers"
	"github.com/mercury/pkg/clients/publisher"
	"github.com/mercury/pkg/clients/worker"
	"github.com/mercury/pkg/config"
	"github.com/mercury/pkg/kmq"
	"github.com/mercury/pkg/middleware"
	"github.com/mercury/pkg/rmq"
	"github.com/redis/go-redis/v9"
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
	cassHost := cfg.SetDefaultString("cassandra_host", "cassandra", false)
	broker := cfg.SetDefaultString("kafka_broker", "kafka:9092", true)
	topic := cfg.SetDefaultString("kafka_topic", "messages", true)
	redisAddr := cfg.SetDefaultString("redis_addr", "redis:6379", false)
	redisPassword := cfg.SetDefaultString("redis_pw", "", true)

	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})
	level, err := logrus.ParseLevel(logLevel)
	if err != nil {
		logrus.Fatal(err)
	}
	logger.SetLevel(level)

	cassClient, err := managers.NewCassandraClient(cassHost)
	if err != nil {
		logger.WithError(err).Fatal("cassandra: failed to connect")
	}
	defer cassClient.Close()

	statsdClient := middleware.NewStatsdClient(statsdAddr, "query")

	brokers := []string{broker}
	producer := kmq.NewProducer(brokers)
	defer producer.Close()
	workerClient := worker.NewClient(topic, producer)

	publisherClient, err := publisher.NewRMQClient(amqpURL)
	if err != nil {
		logrus.Fatal(err)
	}
	defer publisherClient.Close()

	redisClient := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: redisPassword,
	})
	rmqHandlers := handlers.NewRMQHandlers(cassClient, publisherClient, workerClient, redisClient)

	consumer, err := rmq.NewConsumer(amqpURL, logger)
	if err != nil {
		logrus.Fatal(err)
	}
	defer consumer.Close()
	consumer.Consume("msgs.v1.getmessages", rmqHandlers.GetMessages,
		rmq.UseLogger(logger),
		rmq.UseStatsd(statsdClient),
	)
	consumer.Consume("msgs.v1.refreshmessages", rmqHandlers.RefreshMessages,
		rmq.UseLogger(logger),
		rmq.UseStatsd(statsdClient),
	)
	consumer.Consume("msgs.v1.sendmessage", rmqHandlers.SendMessage,
		rmq.UseLogger(logger),
		rmq.UseStatsd(statsdClient),
	)
	consumer.Wait()
}
