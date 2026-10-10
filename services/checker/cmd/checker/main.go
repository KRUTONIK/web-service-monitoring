package main

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/KRUTONIK/web-service-monitoring/contracts/jobs"
	"github.com/KRUTONIK/web-service-monitoring/services/checker/internal/check"
	"github.com/KRUTONIK/web-service-monitoring/services/checker/internal/config"
	"github.com/KRUTONIK/web-service-monitoring/services/checker/internal/messaging"
	"github.com/KRUTONIK/web-service-monitoring/services/checker/internal/storage"
)

func main() {
	cfg := config.Load()
	ctx := context.Background()
	checker := check.New(&http.Client{Timeout: cfg.RequestTimeout})
	checkStorage := storage.NewInfluxDB(&http.Client{Timeout: cfg.StorageTimeout}, cfg.InfluxDBURL, cfg.InfluxDBOrg, cfg.InfluxDBBucket, cfg.InfluxDBToken)

	broker, err := messaging.Open(cfg.RabbitMQURL)
	if err != nil {
		log.Fatalf("open RabbitMQ connection: %v", err)
	}
	defer func() {
		if err := broker.Close(); err != nil {
			log.Printf("close RabbitMQ connection: %v", err)
		}
	}()

	if err := broker.ConsumeJobs(ctx, func(job jobs.CheckJob) error {
		return runJob(ctx, checker, checkStorage, job)
	}); err != nil {
		log.Fatalf("consume check jobs: %v", err)
	}
}

type resultWriter interface {
	Write(context.Context, check.Result) error
}

func runJob(ctx context.Context, checker *check.Checker, checkStorage resultWriter, job jobs.CheckJob) error {
	result := checker.Run(ctx, job.ServiceID, job.ServiceURL)
	if err := checkStorage.Write(ctx, result); err != nil {
		return fmt.Errorf("store result: %w", err)
	}
	log.Printf("checked service=%s url=%s available=%t status=%d response_time_ms=%d", job.ServiceID, job.ServiceURL, result.Available, result.StatusCode, result.ResponseTimeMS)
	return nil
}
