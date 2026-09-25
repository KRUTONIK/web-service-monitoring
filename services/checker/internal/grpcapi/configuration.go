package grpcapi

import (
	"context"
	"fmt"

	configurationv1 "github.com/KRUTONIK/web-service-monitoring/contracts/gen/go/monitoring/configuration/v1"
	"github.com/KRUTONIK/web-service-monitoring/services/checker/internal/serviceconfig"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type ConfigurationClient struct {
	connection *grpc.ClientConn
	client     configurationv1.ConfigurationServiceClient
}

func OpenConfigurationClient(address string) (*ConfigurationClient, error) {
	connection, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("create API Service connection: %w", err)
	}
	return &ConfigurationClient{connection: connection, client: configurationv1.NewConfigurationServiceClient(connection)}, nil
}

func (client *ConfigurationClient) Close() error {
	return client.connection.Close()
}

func (client *ConfigurationClient) Snapshot(ctx context.Context) (serviceconfig.Snapshot, error) {
	response, err := client.client.GetSnapshot(ctx, &configurationv1.GetSnapshotRequest{})
	if err != nil {
		return serviceconfig.Snapshot{}, fmt.Errorf("request configuration snapshot: %w", err)
	}
	snapshot := serviceconfig.Snapshot{Services: make([]serviceconfig.Service, 0, len(response.Services))}
	for _, service := range response.Services {
		snapshot.Services = append(snapshot.Services, serviceconfig.Service{
			ID: service.Id, URL: service.Url, Enabled: service.Enabled, Version: service.Version,
			UpdatedAt: service.UpdatedAt.AsTime(),
		})
	}
	return snapshot, nil
}
