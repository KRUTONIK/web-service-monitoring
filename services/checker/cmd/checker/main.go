package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/KRUTONIK/web-service-monitoring/services/checker/internal/check"
	"github.com/KRUTONIK/web-service-monitoring/services/checker/internal/config"
	"github.com/KRUTONIK/web-service-monitoring/services/checker/internal/grpcapi"
	"github.com/KRUTONIK/web-service-monitoring/services/checker/internal/messaging"
	"github.com/KRUTONIK/web-service-monitoring/services/checker/internal/serviceconfig"
	"github.com/KRUTONIK/web-service-monitoring/services/checker/internal/storage"
)

func main() {
	cfg := config.Load()
	ctx := context.Background()
	checker := check.New(&http.Client{Timeout: cfg.RequestTimeout})

	checkStorage := storage.NewInfluxDB(
		&http.Client{Timeout: cfg.StorageTimeout},
		cfg.InfluxDBURL,
		cfg.InfluxDBOrg,
		cfg.InfluxDBBucket,
		cfg.InfluxDBToken,
	)
	broker, err := messaging.Open(cfg.RabbitMQURL)
	if err != nil {
		log.Fatalf("open RabbitMQ connection: %v", err)
	}
	defer broker.Close()
	configurationClient, err := grpcapi.OpenConfigurationClient(cfg.APITarget)
	if err != nil {
		log.Fatalf("open API Service gRPC connection: %v", err)
	}
	defer configurationClient.Close()

	snapshotContext, cancel := context.WithTimeout(ctx, 10*time.Second)
	snapshot, err := configurationClient.Snapshot(snapshotContext)
	cancel()
	if err != nil {
		log.Fatalf("request configuration snapshot: %v", err)
	}

	state := serviceconfig.NewState()
	state.Replace(snapshot)
	for _, service := range state.Services() {
		if service.Enabled {
			if err := runCheck(ctx, checker, checkStorage, service); err != nil {
				log.Printf("check service %s: %v", service.ID, err)
			}
		}
	}

	err = broker.ConsumeUpdates(ctx, func(update serviceconfig.Update) error {
		if !state.Apply(update.Service) || !update.Service.Enabled {
			return nil
		}
		return runCheck(ctx, checker, checkStorage, update.Service)
	})
	if err != nil {
		log.Fatalf("consume configuration updates: %v", err)
	}
}

type resultWriter interface {
	Write(context.Context, check.Result) error
}

func runCheck(
	ctx context.Context,
	checker *check.Checker,
	checkStorage resultWriter,
	service serviceconfig.Service,
) error {
	result := checker.Run(ctx, service.URL)
	if err := checkStorage.Write(ctx, result); err != nil {
		return fmt.Errorf("store result: %w", err)
	}

	log.Printf(
		"checked service=%s url=%s available=%t status=%d response_time_ms=%d",
		service.ID,
		service.URL,
		result.Available,
		result.StatusCode,
		result.ResponseTimeMS,
	)
	return nil
}
