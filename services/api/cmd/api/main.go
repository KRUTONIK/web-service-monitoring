package main

import (
	"context"
	"log"
	"net"
	"net/http"

	configurationv1 "github.com/KRUTONIK/web-service-monitoring/contracts/gen/go/configuration/v1"
	"github.com/KRUTONIK/web-service-monitoring/services/api/internal/config"
	"github.com/KRUTONIK/web-service-monitoring/services/api/internal/grpcapi"
	"github.com/KRUTONIK/web-service-monitoring/services/api/internal/httpapi"
	"github.com/KRUTONIK/web-service-monitoring/services/api/internal/messaging"
	"github.com/KRUTONIK/web-service-monitoring/services/api/internal/serviceconfig"
	"google.golang.org/grpc"
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
	if err := configBroker.PublishCurrentConfiguration(ctx); err != nil {
		log.Fatalf("publish prototype configuration: %v", err)
	}
	metricsClient, err := grpcapi.OpenMetricsClient(cfg.MetricsTarget, cfg.RequestTimeout)
	if err != nil {
		log.Fatalf("initialize Metrics Service client: %v", err)
	}
	defer metricsClient.Close()

	grpcListener, err := net.Listen("tcp", cfg.GRPCAddress)
	if err != nil {
		log.Fatalf("listen for gRPC on %s: %v", cfg.GRPCAddress, err)
	}
	grpcServer := grpc.NewServer()
	configurationv1.RegisterConfigurationServiceServer(grpcServer, grpcapi.NewConfigurationServer(configRepository))
	go func() {
		log.Printf("API Service gRPC listening on %s", cfg.GRPCAddress)
		if err := grpcServer.Serve(grpcListener); err != nil {
			log.Printf("run API Service gRPC server: %v", err)
		}
	}()

	server := &http.Server{
		Addr:              cfg.ListenAddress,
		Handler:           httpapi.New(metricsClient),
		ReadHeaderTimeout: cfg.RequestTimeout,
	}

	log.Printf("API Service listening on %s", cfg.ListenAddress)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("run API Service: %v", err)
	}
}
