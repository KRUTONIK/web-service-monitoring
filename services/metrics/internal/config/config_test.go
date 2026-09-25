package config

import "testing"

func TestLoadConfiguredValues(t *testing.T) {
	t.Setenv("METRICS_GRPC_LISTEN_ADDRESS", ":9192")
	t.Setenv("INFLUXDB_URL", "http://influxdb.test")
	t.Setenv("INFLUXDB_ORG", "test-org")
	t.Setenv("INFLUXDB_BUCKET", "test-bucket")
	t.Setenv("INFLUXDB_TOKEN", "test-token")

	cfg := Load()
	if cfg.ListenAddress != ":9192" || cfg.InfluxDBURL != "http://influxdb.test" {
		t.Fatalf("unexpected configuration: %+v", cfg)
	}
}
