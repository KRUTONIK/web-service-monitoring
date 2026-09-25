package main

import (
	"context"
	"log"
	"net/http"

	"github.com/KRUTONIK/web-service-monitoring/services/api/internal/config"
	"github.com/KRUTONIK/web-service-monitoring/services/api/internal/httpapi"
	"github.com/KRUTONIK/web-service-monitoring/services/api/internal/messaging"
	"github.com/KRUTONIK/web-service-monitoring/services/api/internal/serviceconfig"
	"github.com/KRUTONIK/web-service-monitoring/services/api/internal/storage"
)

func main() {
	cfg := config.Load()
	ctx := context.Background()
	configRepository, err := serviceconfig.Open(ctx, cfg.PostgresDSN)
	if err != nil {
		log.Fatalf("initialize configuration repository: %v", err)
	}
	defer configRepository.Close()
	if err := configRepository.Initialize(ctx, cfg.ServiceURL); err != nil {
		log.Fatalf("initialize prototype configuration: %v", err)
	}
	configBroker, err := messaging.OpenConfigurationBroker(cfg.RabbitMQURL, configRepository)
	if err != nil {
		log.Fatalf("initialize configuration messaging: %v", err)
	}
	defer configBroker.Close()
	if err := configBroker.StartSnapshotResponder(ctx); err != nil {
		log.Fatalf("start configuration snapshot responder: %v", err)
	}
	if err := configBroker.PublishCurrentConfiguration(ctx); err != nil {
		log.Fatalf("publish prototype configuration: %v", err)
	}

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
