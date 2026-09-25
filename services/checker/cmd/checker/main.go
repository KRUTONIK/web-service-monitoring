package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/KRUTONIK/web-service-monitoring/services/checker/internal/check"
	"github.com/KRUTONIK/web-service-monitoring/services/checker/internal/config"
)

func main() {
	cfg := config.Load()
	client := &http.Client{Timeout: cfg.RequestTimeout}
	result := check.New(client).Run(context.Background(), cfg.MonitorURL)

	if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
		fmt.Fprintf(os.Stderr, "encode check result: %v\n", err)
		os.Exit(1)
	}
}
