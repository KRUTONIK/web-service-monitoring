package grpcapi

import (
	"context"
	"testing"
	"time"

	configurationv1 "github.com/KRUTONIK/web-service-monitoring/contracts/gen/go/configuration/v1"
	"github.com/KRUTONIK/web-service-monitoring/services/api/internal/serviceconfig"
)

type stubRepository struct{ services []serviceconfig.Service }

func (stub stubRepository) List(context.Context) ([]serviceconfig.Service, error) {
	return stub.services, nil
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
