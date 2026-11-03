package main

import (
	"context"
	"log"
	"net"
	"net/http"

	configurationv1 "github.com/KRUTONIK/web-service-monitoring/contracts/gen/go/monitoring/configuration/v1"
	"github.com/KRUTONIK/web-service-monitoring/services/api/internal/auth"
	"github.com/KRUTONIK/web-service-monitoring/services/api/internal/config"
	"github.com/KRUTONIK/web-service-monitoring/services/api/internal/database"
	"github.com/KRUTONIK/web-service-monitoring/services/api/internal/grpcapi"
	"github.com/KRUTONIK/web-service-monitoring/services/api/internal/httpapi"
	"github.com/KRUTONIK/web-service-monitoring/services/api/internal/incident"
	"github.com/KRUTONIK/web-service-monitoring/services/api/internal/messaging"
	"github.com/KRUTONIK/web-service-monitoring/services/api/internal/serviceconfig"
	"github.com/KRUTONIK/web-service-monitoring/services/api/internal/session"
	"github.com/KRUTONIK/web-service-monitoring/services/api/internal/user"
	"google.golang.org/grpc"
)

func main() {
	cfg := config.Load()
	ctx := context.Background()
	postgres, err := database.Open(ctx, cfg.PostgresDSN)
	if err != nil {
		log.Fatalf("initialize PostgreSQL: %v", err)
	}
	defer func() {
		if err := database.Close(postgres); err != nil {
			log.Printf("close PostgreSQL: %v", err)
		}
	}()
	configRepository := serviceconfig.New(postgres)
	incidentRepository := incident.New(postgres)
	sessions, err := session.New(postgres, cfg.CookieSecure)
	if err != nil {
		log.Fatalf("initialize sessions: %v", err)
	}
	authService := auth.New(user.NewRepository(postgres))
	if err := configRepository.Initialize(ctx, cfg.ServiceURL); err != nil {
		log.Fatalf("initialize prototype configuration: %v", err)
	}
	configBroker, err := messaging.OpenConfigurationBroker(cfg.RabbitMQURL, configRepository)
	if err != nil {
		log.Fatalf("initialize configuration messaging: %v", err)
	}
	defer func() {
		if err := configBroker.Close(); err != nil {
			log.Printf("close RabbitMQ connection: %v", err)
		}
	}()
	if err := configBroker.PublishCurrentConfiguration(ctx); err != nil {
		log.Fatalf("publish prototype configuration: %v", err)
	}
	incidentConsumer, err := messaging.OpenIncidentConsumer(cfg.RabbitMQURL, incidentRepository)
	if err != nil { log.Fatalf("initialize incident messaging: %v", err) }
	defer func() { _ = incidentConsumer.Close() }()
	go func() { if err := incidentConsumer.Consume(ctx); err != nil { log.Printf("consume incident events: %v", err) } }()
	metricsClient, err := grpcapi.OpenMetricsClient(cfg.MetricsTarget, cfg.RequestTimeout)
	if err != nil {
		log.Fatalf("initialize Metrics Service client: %v", err)
	}
	defer func() {
		if err := metricsClient.Close(); err != nil {
			log.Printf("close Metrics Service gRPC connection: %v", err)
		}
	}()

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
		Handler:           sessions.LoadAndSave(httpapi.New(metricsClient, authService, sessions, configRepository, incidentRepository, configBroker, cfg.WebOrigin)),
		ReadHeaderTimeout: cfg.RequestTimeout,
	}

	log.Printf("API Service listening on %s", cfg.ListenAddress)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("run API Service: %v", err)
	}
}
