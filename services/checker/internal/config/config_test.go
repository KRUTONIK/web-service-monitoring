package config

import (
	"testing"
	"time"
)

func TestLoadConfiguredValues(t *testing.T) {
	t.Setenv("RABBITMQ_URL", "amqp://rabbitmq.test")
	t.Setenv("API_GRPC_ADDRESS", "api.test:9091")
	t.Setenv("INFLUXDB_URL", "http://influxdb.test")
	t.Setenv("INFLUXDB_ORG", "test-org")
	t.Setenv("INFLUXDB_BUCKET", "test-bucket")
	t.Setenv("INFLUXDB_TOKEN", "test-token")

	cfg := Load()

	if cfg.RequestTimeout != 5*time.Second {
		t.Fatalf("unexpected request timeout: %s", cfg.RequestTimeout)
	}
	if cfg.RabbitMQURL != "amqp://rabbitmq.test" {
		t.Fatalf("unexpected RabbitMQ URL: %s", cfg.RabbitMQURL)
	}
	if cfg.APITarget != "api.test:9091" {
		t.Fatalf("unexpected API gRPC address: %s", cfg.APITarget)
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
	t.Setenv("RABBITMQ_URL", "")
	t.Setenv("API_GRPC_ADDRESS", "")
	t.Setenv("INFLUXDB_URL", "")
	t.Setenv("INFLUXDB_ORG", "")
	t.Setenv("INFLUXDB_BUCKET", "")
	t.Setenv("INFLUXDB_TOKEN", "")

	cfg := Load()

	if cfg.RabbitMQURL != "amqp://monitoring:monitoring@localhost:5672/" {
		t.Fatalf("unexpected default RabbitMQ URL: %s", cfg.RabbitMQURL)
	}
	if cfg.APITarget != "localhost:9091" {
		t.Fatalf("unexpected default API gRPC address: %s", cfg.APITarget)
	}
	if cfg.InfluxDBURL != "http://localhost:8086" {
		t.Fatalf("unexpected default InfluxDB URL: %s", cfg.InfluxDBURL)
	}
	if cfg.InfluxDBOrg != "monitoring" {
		t.Fatalf("unexpected default InfluxDB organization: %s", cfg.InfluxDBOrg)
	}
	if cfg.InfluxDBBucket != "monitoring" {
		t.Fatalf("unexpected default InfluxDB bucket: %s", cfg.InfluxDBBucket)
	}
	if cfg.InfluxDBToken != "monitoring-test-token" {
		t.Fatalf("unexpected default InfluxDB token: %s", cfg.InfluxDBToken)
	}
}
