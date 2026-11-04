package grpcapi

import (
	"context"
	"errors"
	"time"

	metricsv1 "github.com/KRUTONIK/web-service-monitoring/contracts/gen/go/monitoring/metrics/v1"
	"github.com/KRUTONIK/web-service-monitoring/services/metrics/internal/metric"
	"github.com/KRUTONIK/web-service-monitoring/services/metrics/internal/storage"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type resultReader interface {
	Latest(context.Context, string) (metric.Result, error)
	History(context.Context, string, time.Time, int, int, time.Duration) ([]metric.Result, error)
}

func (server *Server) GetCheckHistory(ctx context.Context, request *metricsv1.GetCheckHistoryRequest) (*metricsv1.GetCheckHistoryResponse, error) {
	if request.GetFrom() == nil || !request.GetFrom().IsValid() || request.GetServiceId() == "" || request.GetLimit() < 1 || request.GetOffset() < 0 {
		return nil, status.Error(codes.InvalidArgument, "invalid history request")
	}
	aggregateWindow := time.Duration(request.GetAggregateWindowSeconds()) * time.Second
	results, err := server.reader.History(ctx, request.GetServiceId(), request.GetFrom().AsTime(), int(request.GetLimit()), int(request.GetOffset()), aggregateWindow)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to read monitoring data")
	}
	checks := make([]*metricsv1.CheckResult, 0, len(results))
	for _, result := range results {
		checks = append(checks, &metricsv1.CheckResult{ServiceId: result.ServiceID, ServiceUrl: result.ServiceURL, CheckedAt: timestamppb.New(result.CheckedAt), Available: result.Available, StatusCode: int32(result.StatusCode), ResponseTimeMs: result.ResponseTimeMS, Error: result.Error})
	}
	return &metricsv1.GetCheckHistoryResponse{Checks: checks}, nil
}

type Server struct {
	metricsv1.UnimplementedMetricsServiceServer
	reader resultReader
}

func New(reader resultReader) *Server {
	return &Server{reader: reader}
}

func (server *Server) GetLatestCheck(ctx context.Context, request *metricsv1.GetLatestCheckRequest) (*metricsv1.GetLatestCheckResponse, error) {
	result, err := server.reader.Latest(ctx, request.GetServiceId())
	if errors.Is(err, storage.ErrNotFound) {
		return nil, status.Error(codes.NotFound, "check result not found")
	}
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to read monitoring data")
	}
	return &metricsv1.GetLatestCheckResponse{
		ServiceId: result.ServiceID, ServiceUrl: result.ServiceURL, CheckedAt: timestamppb.New(result.CheckedAt), Available: result.Available,
		StatusCode: int32(result.StatusCode), ResponseTimeMs: result.ResponseTimeMS, Error: result.Error,
	}, nil
}
