package schedule

import (
	"context"
	"testing"

	"github.com/KRUTONIK/web-service-monitoring/services/scheduler/internal/serviceconfig"
)

type stubPublisher struct{ services []serviceconfig.Service }

func (stub *stubPublisher) PublishJobs(_ context.Context, services []serviceconfig.Service) error {
	stub.services = services
	return nil
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
