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
	return metric.Summary{}, stub.err
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
