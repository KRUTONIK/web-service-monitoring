package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/KRUTONIK/web-service-monitoring/services/checker/internal/check"
	"github.com/KRUTONIK/web-service-monitoring/services/checker/internal/config"
	"github.com/KRUTONIK/web-service-monitoring/services/checker/internal/storage"
)

func main() {
	cfg := config.Load()
	client := &http.Client{Timeout: cfg.RequestTimeout}
	ctx := context.Background()
	result := check.New(client).Run(ctx, cfg.MonitorURL)

	checkStorage := storage.NewInfluxDB(
		client,
		cfg.InfluxDBURL,
		cfg.InfluxDBOrg,
		cfg.InfluxDBBucket,
		cfg.InfluxDBToken,
	)
	if err := checkStorage.Write(ctx, result); err != nil {
		fmt.Fprintf(os.Stderr, "store check result: %v\n", err)
		os.Exit(1)
	}

	if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
		fmt.Fprintf(os.Stderr, "encode check result: %v\n", err)
		os.Exit(1)
	}
}
