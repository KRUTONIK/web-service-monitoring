package grpcapi

import (
	"context"
	"fmt"

	metricsv1 "github.com/KRUTONIK/web-service-monitoring/contracts/gen/go/metrics/v1"
	"github.com/KRUTONIK/web-service-monitoring/services/api/internal/monitoring"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

type MetricsClient struct {
	connection *grpc.ClientConn
	client     metricsv1.MetricsServiceClient
}

func OpenMetricsClient(address string) (*MetricsClient, error) {
	connection, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("create Metrics Service connection: %w", err)
	}
	return &MetricsClient{connection: connection, client: metricsv1.NewMetricsServiceClient(connection)}, nil
}

func (client *MetricsClient) Close() error {
	return client.connection.Close()
}

func (client *MetricsClient) Latest(ctx context.Context) (monitoring.Result, error) {
	response, err := client.client.GetLatestCheck(ctx, &metricsv1.GetLatestCheckRequest{})
	if status.Code(err) == codes.NotFound {
		return monitoring.Result{}, monitoring.ErrNotFound
	}
	if err != nil {
		return monitoring.Result{}, fmt.Errorf("request latest check: %w", err)
	}
	return monitoring.Result{
		ServiceURL: response.ServiceUrl, CheckedAt: response.CheckedAt.AsTime(), Available: response.Available,
		StatusCode: int(response.StatusCode), ResponseTimeMS: response.ResponseTimeMs, Error: response.Error,
	}, nil
}
