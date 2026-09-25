package config

import (
	"testing"
	"time"
)

func TestLoadConfiguredValues(t *testing.T) {
	t.Setenv("API_LISTEN_ADDRESS", ":9090")
	t.Setenv("POSTGRES_DSN", "postgres://test")
	t.Setenv("PROTOTYPE_SERVICE_URL", "https://configured.test")
	t.Setenv("RABBITMQ_URL", "amqp://rabbitmq.test")

	cfg := Load()

	if cfg.ListenAddress != ":9090" {
		t.Fatalf("unexpected listen address: %s", cfg.ListenAddress)
	}
	if cfg.RequestTimeout != 5*time.Second {
		t.Fatalf("unexpected request timeout: %s", cfg.RequestTimeout)
	}
	if cfg.PostgresDSN != "postgres://test" {
		t.Fatalf("unexpected PostgreSQL DSN: %s", cfg.PostgresDSN)
	}
	if cfg.ServiceURL != "https://configured.test" {
		t.Fatalf("unexpected prototype service URL: %s", cfg.ServiceURL)
	}
	if cfg.RabbitMQURL != "amqp://rabbitmq.test" {
		t.Fatalf("unexpected RabbitMQ URL: %s", cfg.RabbitMQURL)
	}
}

func TestLoadDefaultValues(t *testing.T) {
	t.Setenv("API_LISTEN_ADDRESS", "")
	t.Setenv("POSTGRES_DSN", "")
	t.Setenv("PROTOTYPE_SERVICE_URL", "")
	t.Setenv("RABBITMQ_URL", "")

	cfg := Load()

	if cfg.ListenAddress != ":8080" {
		t.Fatalf("unexpected default listen address: %s", cfg.ListenAddress)
	}
	if cfg.PostgresDSN != "postgres://monitoring:monitoring@localhost:5432/monitoring?sslmode=disable" {
		t.Fatalf("unexpected default PostgreSQL DSN: %s", cfg.PostgresDSN)
	}
	if cfg.ServiceURL != "https://example.com" {
		t.Fatalf("unexpected default prototype service URL: %s", cfg.ServiceURL)
	}
	if cfg.RabbitMQURL != "amqp://monitoring:monitoring@localhost:5672/" {
		t.Fatalf("unexpected default RabbitMQ URL: %s", cfg.RabbitMQURL)
	}
}
