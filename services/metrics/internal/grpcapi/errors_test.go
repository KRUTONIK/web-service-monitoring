package grpcapi

import (
 "context"
 "errors"
 "testing"

 metricsv1 "github.com/KRUTONIK/web-service-monitoring/contracts/gen/go/monitoring/metrics/v1"
 "github.com/KRUTONIK/web-service-monitoring/services/metrics/internal/metric"
 "github.com/KRUTONIK/web-service-monitoring/services/metrics/internal/storage"
 "google.golang.org/grpc/codes"
 "google.golang.org/grpc/status"
)

type errorReader struct { err error }

func (r errorReader) Latest(context.Context, string) (metric.Result, error) {
 return metric.Result{}, r.err
}

func TestGetLatestCheckErrors(t *testing.T) {
 cases := []struct {
  name string
  err error
  code codes.Code
 }{
  {"not found", storage.ErrNotFound, codes.NotFound},
  {"storage failure", errors.New("storage unavailable"), codes.Internal},
 }
 for _, tc := range cases {
  t.Run(tc.name, func(t *testing.T) {
   _, err := New(errorReader{err: tc.err}).GetLatestCheck(context.Background(), &metricsv1.GetLatestCheckRequest{})
   if status.Code(err) != tc.code {
    t.Fatalf("expected gRPC code %v, got %v", tc.code, status.Code(err))
   }
  })
 }
}
