package grpcapi

import (
	"context"
	"testing"
	"time"

	metricsv1 "github.com/KRUTONIK/web-service-monitoring/contracts/gen/go/metrics/v1"
	"github.com/KRUTONIK/web-service-monitoring/services/metrics/internal/metric"
)

type stubReader struct{ result metric.Result }

func (stub stubReader) Latest(context.Context) (metric.Result, error) { return stub.result, nil }

func TestGetLatestCheck(t *testing.T) {
	checkedAt := time.Date(2026, 9, 25, 10, 30, 0, 0, time.UTC)
	server := New(stubReader{result: metric.Result{ServiceURL: "https://service.test", CheckedAt: checkedAt, Available: true, StatusCode: 200}})

	response, err := server.GetLatestCheck(context.Background(), &metricsv1.GetLatestCheckRequest{})
	if err != nil {
		t.Fatalf("get latest check: %v", err)
	}
	if response.ServiceUrl != "https://service.test" || !response.Available || response.CheckedAt.AsTime() != checkedAt {
		t.Fatalf("unexpected response: %+v", response)
	}
}
