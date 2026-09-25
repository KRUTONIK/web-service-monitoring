package grpcapi

import (
	"context"
	"errors"

	metricsv1 "github.com/KRUTONIK/web-service-monitoring/contracts/gen/go/monitoring/metrics/v1"
	"github.com/KRUTONIK/web-service-monitoring/services/metrics/internal/metric"
	"github.com/KRUTONIK/web-service-monitoring/services/metrics/internal/storage"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type resultReader interface {
	Latest(context.Context) (metric.Result, error)
}

type Server struct {
	metricsv1.UnimplementedMetricsServiceServer
	reader resultReader
}

func New(reader resultReader) *Server {
	return &Server{reader: reader}
}

func (server *Server) GetLatestCheck(ctx context.Context, _ *metricsv1.GetLatestCheckRequest) (*metricsv1.GetLatestCheckResponse, error) {
	result, err := server.reader.Latest(ctx)
	if errors.Is(err, storage.ErrNotFound) {
		return nil, status.Error(codes.NotFound, "check result not found")
	}
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to read monitoring data")
	}
	return &metricsv1.GetLatestCheckResponse{
		ServiceUrl: result.ServiceURL, CheckedAt: timestamppb.New(result.CheckedAt), Available: result.Available,
		StatusCode: int32(result.StatusCode), ResponseTimeMs: result.ResponseTimeMS, Error: result.Error,
	}, nil
}
