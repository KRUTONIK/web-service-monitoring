package config

import (
	"testing"
	"time"
)

func TestLoadConfiguredMonitorURL(t *testing.T) {
	t.Setenv("MONITOR_URL", "https://service.test")

	cfg := Load()

	if cfg.MonitorURL != "https://service.test" {
		t.Fatalf("unexpected monitor URL: %s", cfg.MonitorURL)
	}
	if cfg.RequestTimeout != 5*time.Second {
		t.Fatalf("unexpected request timeout: %s", cfg.RequestTimeout)
	}
}

func TestLoadDefaultMonitorURL(t *testing.T) {
	t.Setenv("MONITOR_URL", "")

	cfg := Load()

	if cfg.MonitorURL != "https://example.com" {
		t.Fatalf("unexpected default monitor URL: %s", cfg.MonitorURL)
	}
}
