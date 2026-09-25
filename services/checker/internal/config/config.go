package config

import (
	"os"
	"time"
)

const (
	defaultMonitorURL     = "https://example.com"
	defaultTimeout        = 5 * time.Second
	defaultInfluxDBURL    = "http://localhost:8086"
	defaultInfluxDBOrg    = "monitoring"
	defaultInfluxDBBucket = "monitoring"
	defaultInfluxDBToken  = "monitoring-test-token"
)

type Config struct {
	MonitorURL     string
	RequestTimeout time.Duration
	InfluxDBURL    string
	InfluxDBOrg    string
	InfluxDBBucket string
	InfluxDBToken  string
}

func Load() Config {
	return Config{
		MonitorURL:     valueOrDefault("MONITOR_URL", defaultMonitorURL),
		RequestTimeout: defaultTimeout,
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
