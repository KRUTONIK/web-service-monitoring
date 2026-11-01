package main

import (
	"context"
	"log"
	"time"

	"github.com/KRUTONIK/web-service-monitoring/services/scheduler/internal/config"
	"github.com/KRUTONIK/web-service-monitoring/services/scheduler/internal/configsync"
	"github.com/KRUTONIK/web-service-monitoring/services/scheduler/internal/grpcapi"
	"github.com/KRUTONIK/web-service-monitoring/services/scheduler/internal/messaging"
	"github.com/KRUTONIK/web-service-monitoring/services/scheduler/internal/schedule"
	"github.com/KRUTONIK/web-service-monitoring/services/scheduler/internal/serviceconfig"
)

func main() {
	cfg := config.Load()
	ctx := context.Background()
	configurationClient, err := grpcapi.OpenConfigurationClient(cfg.APITarget)
	if err != nil {
		log.Fatalf("open API Service gRPC connection: %v", err)
	}
	defer func() {
		if err := configurationClient.Close(); err != nil {
			log.Printf("close API Service gRPC connection: %v", err)
		}
	}()

	snapshotContext, cancel := context.WithTimeout(ctx, 10*time.Second)
	snapshot, err := configurationClient.Snapshot(snapshotContext)
	cancel()
	if err != nil {
		log.Fatalf("request configuration snapshot: %v", err)
	}

	broker, err := messaging.Open(cfg.RabbitMQURL, cfg.JobTTL, cfg.RetryDelay)
	if err != nil {
		log.Fatalf("open RabbitMQ connection: %v", err)
	}
	defer func() {
		if err := broker.Close(); err != nil {
			log.Printf("close RabbitMQ connection: %v", err)
		}
	}()

	state := serviceconfig.NewState()
	state.Replace(snapshot)
	synchronizer := configsync.New(configurationClient, state)
	scheduler := schedule.New(state, broker)
	if err := scheduler.Schedule(ctx); err != nil {
		log.Printf("schedule initial checks: %v", err)
	}

	go func() {
		if err := broker.ConsumeUpdates(ctx, func(update serviceconfig.Update) error {
			state.Apply(update.Service)
			return nil
		}); err != nil {
			log.Printf("consume configuration updates: %v", err)
		}
	}()

	go func() {
		ticker := time.NewTicker(cfg.ConfigSyncInterval)
		defer ticker.Stop()
		for range ticker.C {
			syncContext, cancel := context.WithTimeout(ctx, 10*time.Second)
			updated, err := synchronizer.Sync(syncContext)
			cancel()
			if err != nil {
				log.Printf("synchronize configuration snapshot: %v", err)
				continue
			}
			if updated > 0 {
				log.Printf("reconciled configuration snapshot: %d updates", updated)
			}
		}
	}()

	ticker := time.NewTicker(cfg.Interval)
	defer ticker.Stop()
	for range ticker.C {
		if err := scheduler.Schedule(ctx); err != nil {
			log.Printf("schedule checks: %v", err)
		}
	}
}
