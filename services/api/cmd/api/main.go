package main

import (
	"log"
	"net/http"

	"github.com/KRUTONIK/web-service-monitoring/services/api/internal/config"
	"github.com/KRUTONIK/web-service-monitoring/services/api/internal/httpapi"
	"github.com/KRUTONIK/web-service-monitoring/services/api/internal/storage"
)

func main() {
	cfg := config.Load()
	client := &http.Client{Timeout: cfg.RequestTimeout}
	checkStorage := storage.NewInfluxDB(
		client,
		cfg.InfluxDBURL,
		cfg.InfluxDBOrg,
		cfg.InfluxDBBucket,
		cfg.InfluxDBToken,
	)
	server := &http.Server{
		Addr:              cfg.ListenAddress,
		Handler:           httpapi.New(checkStorage),
		ReadHeaderTimeout: cfg.RequestTimeout,
	}

	log.Printf("API Service listening on %s", cfg.ListenAddress)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("run API Service: %v", err)
	}
}
