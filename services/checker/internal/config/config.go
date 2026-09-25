package config

import (
	"os"
	"time"
)

const (
	defaultTimeout        = 5 * time.Second
	defaultRabbitMQURL    = "amqp://monitoring:monitoring@localhost:5672/"
	defaultInfluxDBURL    = "http://localhost:8086"
	defaultInfluxDBOrg    = "monitoring"
	defaultInfluxDBBucket = "monitoring"
	defaultInfluxDBToken  = "monitoring-test-token"
)

type Config struct {
	RequestTimeout time.Duration
	RabbitMQURL    string
	InfluxDBURL    string
	InfluxDBOrg    string
	InfluxDBBucket string
	InfluxDBToken  string
}

func Load() Config {
	return Config{
		RequestTimeout: defaultTimeout,
		RabbitMQURL:    valueOrDefault("RABBITMQ_URL", defaultRabbitMQURL),
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
