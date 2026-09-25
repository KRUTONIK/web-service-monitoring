package config

import (
	"os"
	"time"
)

const (
	defaultMonitorURL = "https://example.com"
	defaultTimeout    = 5 * time.Second
)

type Config struct {
	MonitorURL     string
	RequestTimeout time.Duration
}

func Load() Config {
	monitorURL := os.Getenv("MONITOR_URL")
	if monitorURL == "" {
		monitorURL = defaultMonitorURL
	}

	return Config{
		MonitorURL:     monitorURL,
		RequestTimeout: defaultTimeout,
	}
}
