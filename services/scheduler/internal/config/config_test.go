package config

import (
	"testing"
	"time"
)

func TestLoadConfiguredValues(t *testing.T) {
	t.Setenv("RABBITMQ_URL", "amqp://rabbitmq.test")
	t.Setenv("API_GRPC_ADDRESS", "api.test:9091")
	t.Setenv("SCHEDULER_INTERVAL", "15s")
	t.Setenv("CHECKER_JOB_RETRY_DELAY", "2s")
	t.Setenv("CHECKER_JOB_TTL", "30s")

	cfg := Load()
	if cfg.RabbitMQURL != "amqp://rabbitmq.test" || cfg.APITarget != "api.test:9091" || cfg.Interval != 15*time.Second || cfg.RetryDelay != 2*time.Second || cfg.JobTTL != 30*time.Second {
		t.Fatalf("unexpected configuration: %+v", cfg)
	}
}

func TestLoadUsesDefaultsForInvalidValues(t *testing.T) {
	t.Setenv("SCHEDULER_INTERVAL", "invalid")
	t.Setenv("CHECKER_JOB_RETRY_DELAY", "invalid")
	t.Setenv("CHECKER_JOB_TTL", "invalid")
	if cfg := Load(); cfg.Interval != time.Minute || cfg.RetryDelay != defaultRetryDelay || cfg.JobTTL != defaultJobTTL {
		t.Fatalf("unexpected configuration: %+v", cfg)
	}
}
