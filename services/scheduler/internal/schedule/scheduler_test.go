package schedule

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/KRUTONIK/web-service-monitoring/services/scheduler/internal/serviceconfig"
)

type stubPublisher struct {
	services []serviceconfig.Service
	err      error
}

func (stub *stubPublisher) PublishJobs(_ context.Context, services []serviceconfig.Service) error {
	stub.services = services
	return stub.err
}

func TestScheduleReturnsPublisherError(t *testing.T) {
	publisher := &stubPublisher{err: errors.New("broker unavailable")}
	err := New(serviceconfig.NewState(), publisher).Schedule(context.Background())
	if err == nil || !strings.Contains(err.Error(), "publish scheduled jobs") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSchedulePublishesEnabledServices(t *testing.T) {
	state := serviceconfig.NewState()
	state.Replace(serviceconfig.Snapshot{Services: []serviceconfig.Service{
		{ID: "enabled", URL: "https://enabled.test", Enabled: true, Version: 1},
		{ID: "disabled", URL: "https://disabled.test", Enabled: false, Version: 1},
	}})
	publisher := &stubPublisher{}
	if err := New(state, publisher).Schedule(context.Background()); err != nil {
		t.Fatalf("schedule checks: %v", err)
	}
	if len(publisher.services) != 1 || publisher.services[0].ID != "enabled" {
		t.Fatalf("unexpected scheduled services: %+v", publisher.services)
	}
}
