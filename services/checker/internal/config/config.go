package config

import (
	"os"
	"time"
)

const (
	defaultTimeout        = 5 * time.Second
	defaultStorageTimeout = 15 * time.Second
	defaultRabbitMQURL    = "amqp://monitoring:monitoring@localhost:5672/"
	defaultAPITarget      = "localhost:9091"
	defaultInfluxDBURL    = "http://localhost:8086"
	defaultInfluxDBOrg    = "monitoring"
	defaultInfluxDBBucket = "monitoring"
	defaultInfluxDBToken  = "monitoring-test-token"
)

type Config struct {
	RequestTimeout time.Duration
	StorageTimeout time.Duration
	RabbitMQURL    string
	APITarget      string
	InfluxDBURL    string
	InfluxDBOrg    string
	InfluxDBBucket string
	InfluxDBToken  string
}

func Load() Config {
	return Config{
		RequestTimeout: defaultTimeout,
		StorageTimeout: defaultStorageTimeout,
		RabbitMQURL:    valueOrDefault("RABBITMQ_URL", defaultRabbitMQURL),
		APITarget:      valueOrDefault("API_GRPC_ADDRESS", defaultAPITarget),
		InfluxDBURL:    valueOrDefault("INFLUXDB_URL", defaultInfluxDBURL),
		InfluxDBOrg:    valueOrDefault("INFLUXDB_ORG", defaultInfluxDBOrg),
		InfluxDBBucket: valueOrDefault("INFLUXDB_BUCKET", defaultInfluxDBBucket),
		InfluxDBToken:  valueOrDefault("INFLUXDB_TOKEN", defaultInfluxDBToken),
	}
}

func valueOrDefault(name string, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}

	return fallback
}
