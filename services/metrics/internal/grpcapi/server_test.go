package grpcapi

import (
	"context"
	"errors"
	"testing"
	"time"

	metricsv1 "github.com/KRUTONIK/web-service-monitoring/contracts/gen/go/monitoring/metrics/v1"
	"github.com/KRUTONIK/web-service-monitoring/services/metrics/internal/metric"
	"github.com/KRUTONIK/web-service-monitoring/services/metrics/internal/storage"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type stubReader struct {
	result           metric.Result
	requestedService string
	err              error
}

func (stub *stubReader) Latest(_ context.Context, serviceID string) (metric.Result, error) {
	stub.requestedService = serviceID
	return stub.result, stub.err
}

func TestGetLatestCheckReturnsNotFound(t *testing.T) {
	server := New(&stubReader{err: storage.ErrNotFound})
	_, err := server.GetLatestCheck(context.Background(), &metricsv1.GetLatestCheckRequest{})
	if status.Code(err) != codes.NotFound {
		t.Fatalf("unexpected status code: %s", status.Code(err))
	}
}

func TestGetLatestCheckReturnsInternalError(t *testing.T) {
	server := New(&stubReader{err: errors.New("InfluxDB unavailable")})
	_, err := server.GetLatestCheck(context.Background(), &metricsv1.GetLatestCheckRequest{})
	if status.Code(err) != codes.Internal {
		t.Fatalf("unexpected status code: %s", status.Code(err))
	}
}

func TestGetLatestCheck(t *testing.T) {
	checkedAt := time.Date(2026, 9, 25, 10, 30, 0, 0, time.UTC)
	reader := &stubReader{result: metric.Result{ServiceID: "service-1", ServiceURL: "https://service.test", CheckedAt: checkedAt, Available: true, StatusCode: 200}}
	server := New(reader)

	response, err := server.GetLatestCheck(context.Background(), &metricsv1.GetLatestCheckRequest{ServiceId: "service-1"})
	if err != nil {
		t.Fatalf("get latest check: %v", err)
	}
	if response.ServiceId != "service-1" || response.ServiceUrl != "https://service.test" || !response.Available || response.CheckedAt.AsTime() != checkedAt {
		t.Fatalf("unexpected response: %+v", response)
	}
	if reader.requestedService != "service-1" {
		t.Fatalf("unexpected requested service: %q", reader.requestedService)
	}
}
