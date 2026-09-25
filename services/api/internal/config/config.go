package config

import (
	"os"
	"time"
)

const (
	defaultListenAddress = ":8080"
	defaultTimeout       = 5 * time.Second
	defaultInfluxDBURL   = "http://localhost:8086"
	defaultInfluxDBOrg   = "monitoring"
	defaultInfluxBucket  = "monitoring"
	defaultInfluxToken   = "monitoring-test-token"
)

type Config struct {
	ListenAddress  string
	RequestTimeout time.Duration
	InfluxDBURL    string
	InfluxDBOrg    string
	InfluxDBBucket string
	InfluxDBToken  string
}

func Load() Config {
	return Config{
		ListenAddress:  valueOrDefault("API_LISTEN_ADDRESS", defaultListenAddress),
		RequestTimeout: defaultTimeout,
		InfluxDBURL:    valueOrDefault("INFLUXDB_URL", defaultInfluxDBURL),
		InfluxDBOrg:    valueOrDefault("INFLUXDB_ORG", defaultInfluxDBOrg),
		InfluxDBBucket: valueOrDefault("INFLUXDB_BUCKET", defaultInfluxBucket),
		InfluxDBToken:  valueOrDefault("INFLUXDB_TOKEN", defaultInfluxToken),
	}
}

func valueOrDefault(name string, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}

	return fallback
}
