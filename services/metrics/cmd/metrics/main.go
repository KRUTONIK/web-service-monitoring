package main

import (
	"log"
	"net"
	"net/http"

	metricsv1 "github.com/KRUTONIK/web-service-monitoring/contracts/gen/go/monitoring/metrics/v1"
	"github.com/KRUTONIK/web-service-monitoring/services/metrics/internal/config"
	"github.com/KRUTONIK/web-service-monitoring/services/metrics/internal/grpcapi"
	"github.com/KRUTONIK/web-service-monitoring/services/metrics/internal/storage"
	"google.golang.org/grpc"
)

func main() {
	cfg := config.Load()
	influxToken, err := cfg.ReadInfluxDBToken()
	if err != nil {
		log.Fatalf("load InfluxDB token: %v", err)
	}
	listener, err := net.Listen("tcp", cfg.ListenAddress)
	if err != nil {
		log.Fatalf("listen on %s: %v", cfg.ListenAddress, err)
	}

	client := &http.Client{Timeout: cfg.RequestTimeout}
	reader := storage.NewInfluxDB(client, cfg.InfluxDBURL, cfg.InfluxDBOrg, cfg.InfluxDBBucket, influxToken)
	server := grpc.NewServer()
	metricsv1.RegisterMetricsServiceServer(server, grpcapi.New(reader))

	log.Printf("Metrics Service gRPC listening on %s", cfg.ListenAddress)
	if err := server.Serve(listener); err != nil {
		log.Fatalf("run Metrics Service: %v", err)
	}
}
