package grpcapi

import (
	"context"

	configurationv1 "github.com/KRUTONIK/web-service-monitoring/contracts/gen/go/configuration/v1"
	"github.com/KRUTONIK/web-service-monitoring/services/api/internal/serviceconfig"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type configurationRepository interface {
	List(context.Context) ([]serviceconfig.Service, error)
}

type ConfigurationServer struct {
	configurationv1.UnimplementedConfigurationServiceServer
	repository configurationRepository
}

func NewConfigurationServer(repository configurationRepository) *ConfigurationServer {
	return &ConfigurationServer{repository: repository}
}

func (server *ConfigurationServer) GetSnapshot(ctx context.Context, _ *configurationv1.GetSnapshotRequest) (*configurationv1.GetSnapshotResponse, error) {
	services, err := server.repository.List(ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to read configuration")
	}

	response := &configurationv1.GetSnapshotResponse{Services: make([]*configurationv1.Service, 0, len(services))}
	for _, service := range services {
		response.Services = append(response.Services, &configurationv1.Service{
			Id: service.ID, Url: service.URL, Enabled: service.Enabled, Version: service.Version,
			UpdatedAt: timestamppb.New(service.UpdatedAt),
		})
	}
	return response, nil
}
