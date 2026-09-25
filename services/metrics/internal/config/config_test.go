package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadInfluxDBTokenFromFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(path, []byte("read-token\n"), 0o600); err != nil {
		t.Fatalf("write token: %v", err)
	}
	token, err := (Config{InfluxDBTokenFile: path}).ReadInfluxDBToken()
	if err != nil || token != "read-token" {
		t.Fatalf("unexpected token result: token=%q err=%v", token, err)
	}
}

func TestLoadConfiguredValues(t *testing.T) {
	t.Setenv("METRICS_GRPC_LISTEN_ADDRESS", ":9192")
	t.Setenv("INFLUXDB_URL", "http://influxdb.test")
	t.Setenv("INFLUXDB_ORG", "test-org")
	t.Setenv("INFLUXDB_BUCKET", "test-bucket")
	t.Setenv("INFLUXDB_TOKEN", "test-token")
	t.Setenv("INFLUXDB_TOKEN_FILE", "")

	cfg := Load()
	if cfg.ListenAddress != ":9192" || cfg.InfluxDBURL != "http://influxdb.test" {
		t.Fatalf("unexpected configuration: %+v", cfg)
	}
}
