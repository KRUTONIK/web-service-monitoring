package config

import (
	"os"
	"time"
)

const (
	defaultListenAddress = ":8080"
	defaultGRPCAddress   = ":9091"
	defaultMetricsTarget = "localhost:9092"
	defaultTimeout       = 5 * time.Second
	defaultPostgresDSN   = "postgres://monitoring:monitoring@localhost:5432/monitoring?sslmode=disable"
	defaultServiceURL    = "https://example.com"
	defaultRabbitMQURL   = "amqp://monitoring:monitoring@localhost:5672/"
)

type Config struct {
	ListenAddress  string
	GRPCAddress    string
	MetricsTarget  string
	RequestTimeout time.Duration
	PostgresDSN    string
	ServiceURL     string
	RabbitMQURL    string
}

func Load() Config {
	return Config{
		ListenAddress:  valueOrDefault("API_LISTEN_ADDRESS", defaultListenAddress),
		GRPCAddress:    valueOrDefault("API_GRPC_LISTEN_ADDRESS", defaultGRPCAddress),
		MetricsTarget:  valueOrDefault("METRICS_GRPC_ADDRESS", defaultMetricsTarget),
		RequestTimeout: defaultTimeout,
		PostgresDSN:    valueOrDefault("POSTGRES_DSN", defaultPostgresDSN),
		ServiceURL:     valueOrDefault("PROTOTYPE_SERVICE_URL", defaultServiceURL),
		RabbitMQURL:    valueOrDefault("RABBITMQ_URL", defaultRabbitMQURL),
	}
}

func valueOrDefault(name string, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}

	return fallback
}
