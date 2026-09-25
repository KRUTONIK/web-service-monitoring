package config

import (
	"fmt"
	"os"
	"strings"
	"time"
)

const (
	defaultListenAddress = ":9092"
	defaultTimeout       = 15 * time.Second
	defaultInfluxDBURL   = "http://localhost:8086"
	defaultInfluxDBOrg   = "monitoring"
	defaultInfluxBucket  = "monitoring"
	defaultInfluxToken   = "monitoring-test-token"
)

type Config struct {
	ListenAddress     string
	RequestTimeout    time.Duration
	InfluxDBURL       string
	InfluxDBOrg       string
	InfluxDBBucket    string
	InfluxDBToken     string
	InfluxDBTokenFile string
}

func Load() Config {
	return Config{
		ListenAddress:     valueOrDefault("METRICS_GRPC_LISTEN_ADDRESS", defaultListenAddress),
		RequestTimeout:    defaultTimeout,
		InfluxDBURL:       valueOrDefault("INFLUXDB_URL", defaultInfluxDBURL),
		InfluxDBOrg:       valueOrDefault("INFLUXDB_ORG", defaultInfluxDBOrg),
		InfluxDBBucket:    valueOrDefault("INFLUXDB_BUCKET", defaultInfluxBucket),
		InfluxDBToken:     valueOrDefault("INFLUXDB_TOKEN", defaultInfluxToken),
		InfluxDBTokenFile: os.Getenv("INFLUXDB_TOKEN_FILE"),
	}
}

func (config Config) ReadInfluxDBToken() (string, error) {
	if config.InfluxDBTokenFile == "" {
		return config.InfluxDBToken, nil
	}
	content, err := os.ReadFile(config.InfluxDBTokenFile)
	if err != nil {
		return "", fmt.Errorf("read InfluxDB token file: %w", err)
	}
	token := strings.TrimSpace(string(content))
	if token == "" {
		return "", fmt.Errorf("InfluxDB token file is empty")
	}
	return token, nil
}

func valueOrDefault(name string, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
