package grpcapi

import (
	"context"
	"errors"
	"testing"
	"time"

	metricsv1 "github.com/KRUTONIK/web-service-monitoring/contracts/gen/go/monitoring/metrics/v1"
	"github.com/KRUTONIK/web-service-monitoring/services/api/internal/monitoring"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type fakeMetricsServiceClient struct {
	latest     *metricsv1.GetLatestCheckResponse
	latestMany *metricsv1.GetLatestChecksResponse
	history    *metricsv1.GetCheckHistoryResponse
	summary    *metricsv1.GetCheckSummaryResponse
	err        error
}

func (fake *fakeMetricsServiceClient) GetLatestCheck(context.Context, *metricsv1.GetLatestCheckRequest, ...grpc.CallOption) (*metricsv1.GetLatestCheckResponse, error) {
	return fake.latest, fake.err
}
func (fake *fakeMetricsServiceClient) GetLatestChecks(context.Context, *metricsv1.GetLatestChecksRequest, ...grpc.CallOption) (*metricsv1.GetLatestChecksResponse, error) {
	return fake.latestMany, fake.err
}
func (fake *fakeMetricsServiceClient) GetCheckHistory(context.Context, *metricsv1.GetCheckHistoryRequest, ...grpc.CallOption) (*metricsv1.GetCheckHistoryResponse, error) {
	return fake.history, fake.err
}
func (fake *fakeMetricsServiceClient) GetCheckSummary(context.Context, *metricsv1.GetCheckSummaryRequest, ...grpc.CallOption) (*metricsv1.GetCheckSummaryResponse, error) {
	return fake.summary, fake.err
}

func TestMetricsClientConvertsResponses(t *testing.T) {
	at := time.Date(2026, time.November, 5, 12, 0, 0, 0, time.UTC)
	check := &metricsv1.CheckResult{ServiceId: "service-1", ServiceUrl: "https://service.test", CheckedAt: timestamppb.New(at), Available: true, StatusCode: 201, ResponseTimeMs: 42, Error: ""}
	fake := &fakeMetricsServiceClient{
		latest:     &metricsv1.GetLatestCheckResponse{ServiceId: check.ServiceId, ServiceUrl: check.ServiceUrl, CheckedAt: check.CheckedAt, Available: check.Available, StatusCode: check.StatusCode, ResponseTimeMs: check.ResponseTimeMs},
		latestMany: &metricsv1.GetLatestChecksResponse{Checks: []*metricsv1.CheckResult{check}},
		history:    &metricsv1.GetCheckHistoryResponse{Checks: []*metricsv1.CheckResult{check}},
		summary:    &metricsv1.GetCheckSummaryResponse{TotalChecks: 3, SuccessfulChecks: 2, HasResponseMetrics: true, AverageResponseMs: 25, P95ResponseMs: 40},
	}
	client := &MetricsClient{client: fake, timeout: time.Second}
	latest, err := client.Latest(context.Background(), "service-1")
	if err != nil || latest.ServiceID != "service-1" || !latest.CheckedAt.Equal(at) || latest.StatusCode != 201 || latest.ResponseTimeMS != 42 {
		t.Fatalf("latest = %+v, error = %v", latest, err)
	}
	many, err := client.LatestMany(context.Background(), []string{"service-1"})
	if err != nil || len(many) != 1 || many[0].ServiceURL != "https://service.test" {
		t.Fatalf("latest many = %+v, error = %v", many, err)
	}
	history, err := client.History(context.Background(), "service-1", at.Add(-time.Hour), 10, 2, 5*time.Minute)
	if err != nil || len(history.Checks) != 1 || history.Limit != 10 || history.Offset != 2 {
		t.Fatalf("history = %+v, error = %v", history, err)
	}
	summary, err := client.Summary(context.Background(), "service-1", at.Add(-time.Hour))
	if err != nil || summary.TotalChecks != 3 || summary.SuccessfulChecks != 2 || summary.P95ResponseMS != 40 {
		t.Fatalf("summary = %+v, error = %v", summary, err)
	}
}

func TestMetricsClientMapsNotFoundAndErrors(t *testing.T) {
	client := &MetricsClient{client: &fakeMetricsServiceClient{err: status.Error(codes.NotFound, "missing")}, timeout: time.Second}
	if _, err := client.Latest(context.Background(), "missing"); !errors.Is(err, monitoring.ErrNotFound) {
		t.Fatalf("latest error = %v", err)
	}
	client.client = &fakeMetricsServiceClient{err: errors.New("unavailable")}
	if _, err := client.LatestMany(context.Background(), []string{"service-1"}); err == nil {
		t.Fatal("expected latest-many error")
	}
	if _, err := client.History(context.Background(), "service-1", time.Now(), 1, 0, 0); err == nil {
		t.Fatal("expected history error")
	}
	if _, err := client.Summary(context.Background(), "service-1", time.Now()); err == nil {
		t.Fatal("expected summary error")
	}
}
