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
	"google.golang.org/protobuf/types/known/timestamppb"
)

type stubReader struct {
	result           metric.Result
	latestMany       []metric.Result
	summary          metric.Summary
	history          []metric.Result
	requestedService string
	from             time.Time
	limit, offset    int
	aggregate        time.Duration
	err              error
}

func (stub *stubReader) Latest(_ context.Context, serviceID string) (metric.Result, error) {
	stub.requestedService = serviceID
	return stub.result, stub.err
}

func (stub *stubReader) LatestMany(_ context.Context, _ []string) ([]metric.Result, error) {
	if stub.latestMany != nil {
		return stub.latestMany, stub.err
	}
	if stub.result.ServiceID == "" {
		return nil, stub.err
	}
	return []metric.Result{stub.result}, stub.err
}

func (stub *stubReader) History(_ context.Context, serviceID string, from time.Time, limit, offset int, aggregate time.Duration) ([]metric.Result, error) {
	stub.requestedService, stub.from, stub.limit, stub.offset, stub.aggregate = serviceID, from, limit, offset, aggregate
	return stub.history, stub.err
}

func (stub *stubReader) Summary(_ context.Context, _ string, _ time.Time) (metric.Summary, error) {
	return stub.summary, stub.err
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

func TestGetCheckHistory(t *testing.T) {
	from := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	reader := &stubReader{history: []metric.Result{{ServiceID: "service-1", CheckedAt: from.Add(time.Hour), Available: true, StatusCode: 200}}}
	response, err := New(reader).GetCheckHistory(context.Background(), &metricsv1.GetCheckHistoryRequest{ServiceId: "service-1", From: timestamppb.New(from), Limit: 100, AggregateWindowSeconds: 300})
	if err != nil || len(response.Checks) != 1 || reader.requestedService != "service-1" || !reader.from.Equal(from) || reader.limit != 100 || reader.aggregate != 5*time.Minute {
		t.Fatalf("unexpected response=%+v err=%v reader=%+v", response, err, reader)
	}
}

func TestGetCheckHistoryRejectsInvalidRequest(t *testing.T) {
	_, err := New(&stubReader{}).GetCheckHistory(context.Background(), &metricsv1.GetCheckHistoryRequest{ServiceId: "service-1"})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("unexpected status code: %s", status.Code(err))
	}
}

func TestGetLatestChecks(t *testing.T) {
	checkedAt := time.Date(2026, time.November, 5, 12, 0, 0, 0, time.UTC)
	reader := &stubReader{latestMany: []metric.Result{
		{ServiceID: "one", ServiceURL: "https://one.test", CheckedAt: checkedAt, Available: true, StatusCode: 299, ResponseTimeMS: 12},
		{ServiceID: "two", ServiceURL: "https://two.test", CheckedAt: checkedAt.Add(time.Second), Available: false, StatusCode: 503, Error: "unavailable"},
	}}
	response, err := New(reader).GetLatestChecks(context.Background(), &metricsv1.GetLatestChecksRequest{ServiceIds: []string{"one", "two"}})
	if err != nil || len(response.Checks) != 2 {
		t.Fatalf("response = %+v, error = %v", response, err)
	}
	first, second := response.Checks[0], response.Checks[1]
	if first.ServiceId != "one" || first.StatusCode != 299 || first.ResponseTimeMs != 12 || !first.CheckedAt.AsTime().Equal(checkedAt) || second.Error != "unavailable" || second.Available {
		t.Fatalf("checks = %+v", response.Checks)
	}
}

func TestGetLatestChecksEmptyAndError(t *testing.T) {
	server := New(&stubReader{err: errors.New("InfluxDB unavailable")})
	response, err := server.GetLatestChecks(context.Background(), &metricsv1.GetLatestChecksRequest{})
	if err != nil || len(response.Checks) != 0 {
		t.Fatalf("empty response = %+v, error = %v", response, err)
	}
	_, err = server.GetLatestChecks(context.Background(), &metricsv1.GetLatestChecksRequest{ServiceIds: []string{"one"}})
	if status.Code(err) != codes.Internal {
		t.Fatalf("error status = %s", status.Code(err))
	}
}

func TestGetCheckSummary(t *testing.T) {
	from := time.Date(2026, time.November, 1, 0, 0, 0, 0, time.UTC)
	reader := &stubReader{summary: metric.Summary{TotalChecks: 10, SuccessfulChecks: 8, HasResponseMetrics: true, AverageResponseMS: 25.5, P95ResponseMS: 80}}
	response, err := New(reader).GetCheckSummary(context.Background(), &metricsv1.GetCheckSummaryRequest{ServiceId: "service-1", From: timestamppb.New(from)})
	if err != nil || response.TotalChecks != 10 || response.SuccessfulChecks != 8 || !response.HasResponseMetrics || response.AverageResponseMs != 25.5 || response.P95ResponseMs != 80 {
		t.Fatalf("response = %+v, error = %v", response, err)
	}
	_, err = New(reader).GetCheckSummary(context.Background(), &metricsv1.GetCheckSummaryRequest{ServiceId: "service-1"})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("invalid summary status = %s", status.Code(err))
	}
	reader.err = errors.New("InfluxDB unavailable")
	_, err = New(reader).GetCheckSummary(context.Background(), &metricsv1.GetCheckSummaryRequest{ServiceId: "service-1", From: timestamppb.New(from)})
	if status.Code(err) != codes.Internal {
		t.Fatalf("summary error status = %s", status.Code(err))
	}
}
