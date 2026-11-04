package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	contracts "github.com/KRUTONIK/web-service-monitoring/contracts/incidents"
	"github.com/KRUTONIK/web-service-monitoring/contracts/jobs"
	"github.com/KRUTONIK/web-service-monitoring/services/checker/internal/check"
	"github.com/KRUTONIK/web-service-monitoring/services/checker/internal/config"
	"github.com/KRUTONIK/web-service-monitoring/services/checker/internal/incident"
	"github.com/KRUTONIK/web-service-monitoring/services/checker/internal/messaging"
	"github.com/KRUTONIK/web-service-monitoring/services/checker/internal/storage"
)

func main() {
	cfg := config.Load()
	ctx := context.Background()
	checker := check.New(&http.Client{})
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
		return runJob(ctx, checker, checkStorage, broker, job)
	}); err != nil {
		log.Fatalf("consume check jobs: %v", err)
	}
}

type resultWriter interface {
	Write(context.Context, check.Result) error
	LatestBefore(context.Context, string, time.Time) (check.Result, error)
}

type incidentPublisher interface {
	PublishIncident(context.Context, contracts.Event) error
}

func runJob(ctx context.Context, checker *check.Checker, checkStorage resultWriter, publisher incidentPublisher, job jobs.CheckJob) error {
	previous, previousErr := checkStorage.LatestBefore(ctx, job.ServiceID, job.ScheduledAt)
	if previousErr != nil && !errors.Is(previousErr, storage.ErrNotFound) {
		return fmt.Errorf("read previous result: %w", previousErr)
	}
	result := checker.Run(ctx, job.ServiceID, job.ServiceURL, time.Duration(job.TimeoutSeconds)*time.Second)
	result.JobID = job.JobID
	result.CheckedAt = job.ScheduledAt
	if err := checkStorage.Write(ctx, result); err != nil {
		return fmt.Errorf("store result: %w", err)
	}
	if event := incident.Transition(previous, previousErr == nil, result); event != nil {
		if err := publisher.PublishIncident(ctx, *event); err != nil {
			return fmt.Errorf("publish incident event: %w", err)
		}
	}
	log.Printf("checked service=%s url=%s available=%t status=%d response_time_ms=%d", job.ServiceID, job.ServiceURL, result.Available, result.StatusCode, result.ResponseTimeMS)
	return nil
}
