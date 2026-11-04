package schedule

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/KRUTONIK/web-service-monitoring/services/scheduler/internal/serviceconfig"
)

type stubPublisher struct {
	services []serviceconfig.Service
	err      error
	calls    int
}

func TestScheduleRunsTenSecondServiceOnOneSecondTicks(t *testing.T) {
	state := serviceconfig.NewState()
	state.Replace(serviceconfig.Snapshot{Services: []serviceconfig.Service{{ID: "service-1", URL: "https://service.test", Enabled: true, IntervalSeconds: 10}}})
	publisher := &stubPublisher{}
	scheduler := New(state, publisher)
	now := time.Date(2026, time.November, 2, 12, 0, 0, 0, time.UTC)
	for second := 0; second <= 20; second++ {
		if err := scheduler.scheduleAt(context.Background(), now.Add(time.Duration(second)*time.Second)); err != nil {
			t.Fatalf("schedule at %d seconds: %v", second, err)
		}
	}
	if publisher.calls != 3 {
		t.Fatalf("expected checks at 0, 10 and 20 seconds, got %d publications", publisher.calls)
	}
}

func TestScheduleRespectsIndividualIntervals(t *testing.T) {
	state := serviceconfig.NewState()
	state.Replace(serviceconfig.Snapshot{Services: []serviceconfig.Service{
		{ID: "fast", URL: "https://fast.test", Enabled: true, IntervalSeconds: 10, TimeoutSeconds: 2, Version: 1},
		{ID: "slow", URL: "https://slow.test", Enabled: true, IntervalSeconds: 30, TimeoutSeconds: 2, Version: 1},
	}})
	publisher := &stubPublisher{}
	scheduler := New(state, publisher)
	now := time.Date(2026, time.November, 2, 12, 0, 0, 0, time.UTC)
	if err := scheduler.scheduleAt(context.Background(), now); err != nil {
		t.Fatalf("schedule initial checks: %v", err)
	}
	if len(publisher.services) != 2 {
		t.Fatalf("initial services: %+v", publisher.services)
	}
	if err := scheduler.scheduleAt(context.Background(), now.Add(10*time.Second)); err != nil {
		t.Fatalf("schedule fast service: %v", err)
	}
	if len(publisher.services) != 1 || publisher.services[0].ID != "fast" {
		t.Fatalf("services after 10 seconds: %+v", publisher.services)
	}
	if err := scheduler.scheduleAt(context.Background(), now.Add(30*time.Second)); err != nil {
		t.Fatalf("schedule both services: %v", err)
	}
	if len(publisher.services) != 2 {
		t.Fatalf("services after 30 seconds: %+v", publisher.services)
	}
}

func TestScheduleAppliesChangedIntervalWithoutRestart(t *testing.T) {
	state := serviceconfig.NewState()
	state.Replace(serviceconfig.Snapshot{Services: []serviceconfig.Service{{ID: "service-1", URL: "https://service.test", Enabled: true, IntervalSeconds: 60, Version: 1}}})
	publisher := &stubPublisher{}
	scheduler := New(state, publisher)
	now := time.Date(2026, time.November, 2, 12, 0, 0, 0, time.UTC)
	if err := scheduler.scheduleAt(context.Background(), now); err != nil {
		t.Fatalf("schedule initial check: %v", err)
	}
	state.Apply(serviceconfig.Service{ID: "service-1", URL: "https://service.test", Enabled: true, IntervalSeconds: 10, Version: 2})
	if err := scheduler.scheduleAt(context.Background(), now.Add(10*time.Second)); err != nil {
		t.Fatalf("schedule updated interval: %v", err)
	}
	if len(publisher.services) != 1 || publisher.services[0].IntervalSeconds != 10 {
		t.Fatalf("updated service was not scheduled: %+v", publisher.services)
	}
}

func (stub *stubPublisher) PublishJobs(_ context.Context, services []serviceconfig.Service) error {
	stub.calls++
	stub.services = services
	return stub.err
}

func TestScheduleReturnsPublisherError(t *testing.T) {
	publisher := &stubPublisher{err: errors.New("broker unavailable")}
	state := serviceconfig.NewState()
	state.Replace(serviceconfig.Snapshot{Services: []serviceconfig.Service{{ID: "service-1", URL: "https://service.test", Enabled: true, IntervalSeconds: 10}}})
	err := New(state, publisher).Schedule(context.Background())
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

func TestScheduleSkipsServiceDisabledByDeletion(t *testing.T) {
	state := serviceconfig.NewState()
	state.Replace(serviceconfig.Snapshot{Services: []serviceconfig.Service{{ID: "service-1", URL: "https://service.test", Enabled: true, Version: 1}}})
	state.Apply(serviceconfig.Service{ID: "service-1", URL: "https://service.test", Enabled: false, Version: 2})
	publisher := &stubPublisher{}
	if err := New(state, publisher).Schedule(context.Background()); err != nil {
		t.Fatalf("schedule checks: %v", err)
	}
	if len(publisher.services) != 0 {
		t.Fatalf("deleted service was scheduled: %+v", publisher.services)
	}
}
