package grpcapi

import (
	"context"
	"errors"
	"testing"
	"time"

	configurationv1 "github.com/KRUTONIK/web-service-monitoring/contracts/gen/go/monitoring/configuration/v1"
	"github.com/KRUTONIK/web-service-monitoring/services/api/internal/serviceconfig"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type stubRepository struct {
	services []serviceconfig.Service
	err      error
}

func (stub stubRepository) List(context.Context) ([]serviceconfig.Service, error) {
	return stub.services, stub.err
}

func TestGetSnapshotReturnsInternalError(t *testing.T) {
	server := NewConfigurationServer(stubRepository{err: errors.New("database unavailable")})
	_, err := server.GetSnapshot(context.Background(), &configurationv1.GetSnapshotRequest{})
	if status.Code(err) != codes.Internal {
		t.Fatalf("unexpected status code: %s", status.Code(err))
	}
}

func TestGetSnapshot(t *testing.T) {
	updatedAt := time.Date(2026, 9, 25, 10, 30, 0, 0, time.UTC)
	server := NewConfigurationServer(stubRepository{services: []serviceconfig.Service{{
		ID: "service-1", URL: "https://service.test", Enabled: true, Version: 3, UpdatedAt: updatedAt,
	}}})

	response, err := server.GetSnapshot(context.Background(), &configurationv1.GetSnapshotRequest{})
	if err != nil {
		t.Fatalf("get snapshot: %v", err)
	}
	if len(response.Services) != 1 || response.Services[0].Id != "service-1" || response.Services[0].Version != 3 {
		t.Fatalf("unexpected response: %+v", response)
	}
}
