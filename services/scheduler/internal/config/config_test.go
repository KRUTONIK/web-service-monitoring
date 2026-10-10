package config

import (
	"testing"
	"time"
)

func TestLoadConfiguredValues(t *testing.T) {
	t.Setenv("RABBITMQ_URL", "amqp://rabbitmq.test")
	t.Setenv("API_GRPC_ADDRESS", "api.test:9091")
	t.Setenv("SCHEDULER_INTERVAL", "15s")

	cfg := Load()
	if cfg.RabbitMQURL != "amqp://rabbitmq.test" || cfg.APITarget != "api.test:9091" || cfg.Interval != 15*time.Second {
		t.Fatalf("unexpected configuration: %+v", cfg)
	}
}

func TestLoadUsesDefaultIntervalForInvalidValue(t *testing.T) {
	t.Setenv("SCHEDULER_INTERVAL", "invalid")
	if cfg := Load(); cfg.Interval != time.Minute {
		t.Fatalf("unexpected interval: %s", cfg.Interval)
	}
}
