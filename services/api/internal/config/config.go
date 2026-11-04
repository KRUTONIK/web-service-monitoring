package config

import (
	"os"
	"strconv"
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
	defaultWebOrigin     = "http://localhost:5173"
)

type Config struct {
	ListenAddress  string
	GRPCAddress    string
	MetricsTarget  string
	RequestTimeout time.Duration
	PostgresDSN    string
	ServiceURL     string
	RabbitMQURL    string
	WebOrigin      string
	CookieSecure   bool
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
		WebOrigin:      valueOrDefault("WEB_ORIGIN", defaultWebOrigin),
		CookieSecure:   boolValue("API_COOKIE_SECURE", false),
	}
}

func boolValue(name string, fallback bool) bool {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}

	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}

	return parsed
}

func valueOrDefault(name string, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}

	return fallback
}
