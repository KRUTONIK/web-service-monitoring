package config

import (
	"os"
	"time"
)

const (
	defaultRabbitMQURL = "amqp://monitoring:monitoring@localhost:5672/"
	defaultAPITarget   = "localhost:9091"
	defaultInterval    = time.Minute
)

type Config struct {
	RabbitMQURL string
	APITarget   string
	Interval    time.Duration
}

func Load() Config {
	return Config{
		RabbitMQURL: valueOrDefault("RABBITMQ_URL", defaultRabbitMQURL),
		APITarget:   valueOrDefault("API_GRPC_ADDRESS", defaultAPITarget),
		Interval:    durationOrDefault("SCHEDULER_INTERVAL", defaultInterval),
	}
}

func valueOrDefault(name string, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func durationOrDefault(name string, fallback time.Duration) time.Duration {
	value, err := time.ParseDuration(os.Getenv(name))
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}
