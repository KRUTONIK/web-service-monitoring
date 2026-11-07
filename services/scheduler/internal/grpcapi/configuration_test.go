package grpcapi

import (
	"context"
	"errors"
	"testing"
	"time"

	configurationv1 "github.com/KRUTONIK/web-service-monitoring/contracts/gen/go/monitoring/configuration/v1"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type fakeConfigurationServiceClient struct {
	response *configurationv1.GetSnapshotResponse
	err      error
}

func (fake *fakeConfigurationServiceClient) GetSnapshot(context.Context, *configurationv1.GetSnapshotRequest, ...grpc.CallOption) (*configurationv1.GetSnapshotResponse, error) {
	return fake.response, fake.err
}

func TestSnapshotConvertsConfiguration(t *testing.T) {
	updatedAt := time.Date(2026, time.November, 5, 12, 0, 0, 0, time.UTC)
	client := &ConfigurationClient{client: &fakeConfigurationServiceClient{response: &configurationv1.GetSnapshotResponse{Services: []*configurationv1.Service{{
		Id: "service-1", Url: "https://service.test", Enabled: true, IntervalSeconds: 30, TimeoutSeconds: 5, Version: 4, UpdatedAt: timestamppb.New(updatedAt),
	}}}}}
	snapshot, err := client.Snapshot(context.Background())
	if err != nil || len(snapshot.Services) != 1 {
		t.Fatalf("snapshot = %+v, error = %v", snapshot, err)
	}
	service := snapshot.Services[0]
	if service.ID != "service-1" || service.URL != "https://service.test" || !service.Enabled || service.IntervalSeconds != 30 || service.TimeoutSeconds != 5 || service.Version != 4 || !service.UpdatedAt.Equal(updatedAt) {
		t.Fatalf("service = %+v", service)
	}
}

func TestSnapshotReturnsGRPCError(t *testing.T) {
	client := &ConfigurationClient{client: &fakeConfigurationServiceClient{err: errors.New("API unavailable")}}
	if _, err := client.Snapshot(context.Background()); err == nil {
		t.Fatal("expected snapshot error")
	}
}
