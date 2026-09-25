package grpcapi

import (
	"context"
	"fmt"
	"time"

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
	timeout    time.Duration
}

func OpenMetricsClient(address string, timeout time.Duration) (*MetricsClient, error) {
	connection, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("create Metrics Service connection: %w", err)
	}
	return &MetricsClient{connection: connection, client: metricsv1.NewMetricsServiceClient(connection), timeout: timeout}, nil
}

func (client *MetricsClient) Close() error {
	return client.connection.Close()
}

func (client *MetricsClient) Latest(ctx context.Context) (monitoring.Result, error) {
	requestContext, cancel := context.WithTimeout(ctx, client.timeout)
	defer cancel()
	response, err := client.client.GetLatestCheck(requestContext, &metricsv1.GetLatestCheckRequest{})
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
