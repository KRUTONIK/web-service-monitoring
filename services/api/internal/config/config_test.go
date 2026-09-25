package config

import (
	"testing"
	"time"
)

func TestLoadConfiguredValues(t *testing.T) {
	t.Setenv("API_LISTEN_ADDRESS", ":9090")
	t.Setenv("INFLUXDB_URL", "http://influxdb.test")
	t.Setenv("INFLUXDB_ORG", "test-org")
	t.Setenv("INFLUXDB_BUCKET", "test-bucket")
	t.Setenv("INFLUXDB_TOKEN", "test-token")

	cfg := Load()

	if cfg.ListenAddress != ":9090" {
		t.Fatalf("unexpected listen address: %s", cfg.ListenAddress)
	}
	if cfg.RequestTimeout != 5*time.Second {
		t.Fatalf("unexpected request timeout: %s", cfg.RequestTimeout)
	}
	if cfg.InfluxDBURL != "http://influxdb.test" {
		t.Fatalf("unexpected InfluxDB URL: %s", cfg.InfluxDBURL)
	}
	if cfg.InfluxDBOrg != "test-org" {
		t.Fatalf("unexpected InfluxDB organization: %s", cfg.InfluxDBOrg)
	}
	if cfg.InfluxDBBucket != "test-bucket" {
		t.Fatalf("unexpected InfluxDB bucket: %s", cfg.InfluxDBBucket)
	}
	if cfg.InfluxDBToken != "test-token" {
		t.Fatalf("unexpected InfluxDB token: %s", cfg.InfluxDBToken)
	}
}

func TestLoadDefaultValues(t *testing.T) {
	t.Setenv("API_LISTEN_ADDRESS", "")
	t.Setenv("INFLUXDB_URL", "")
	t.Setenv("INFLUXDB_ORG", "")
	t.Setenv("INFLUXDB_BUCKET", "")
	t.Setenv("INFLUXDB_TOKEN", "")

	cfg := Load()

	if cfg.ListenAddress != ":8080" {
		t.Fatalf("unexpected default listen address: %s", cfg.ListenAddress)
	}
	if cfg.InfluxDBURL != "http://localhost:8086" {
		t.Fatalf("unexpected default InfluxDB URL: %s", cfg.InfluxDBURL)
	}
	if cfg.InfluxDBOrg != "monitoring" || cfg.InfluxDBBucket != "monitoring" {
		t.Fatalf("unexpected default InfluxDB destination: %s/%s", cfg.InfluxDBOrg, cfg.InfluxDBBucket)
	}
	if cfg.InfluxDBToken != "monitoring-test-token" {
		t.Fatalf("unexpected default InfluxDB token: %s", cfg.InfluxDBToken)
	}
}
