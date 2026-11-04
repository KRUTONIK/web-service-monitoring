package schedule

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/KRUTONIK/web-service-monitoring/services/scheduler/internal/serviceconfig"
)

type jobPublisher interface {
	PublishJobs(context.Context, []serviceconfig.Service) error
}

type Scheduler struct {
	state         *serviceconfig.State
	publisher     jobPublisher
	mutex         sync.Mutex
	lastScheduled map[string]time.Time
}

func New(state *serviceconfig.State, publisher jobPublisher) *Scheduler {
	return &Scheduler{state: state, publisher: publisher, lastScheduled: make(map[string]time.Time)}
}

func (scheduler *Scheduler) Schedule(ctx context.Context) error {
	return scheduler.scheduleAt(ctx, time.Now().UTC())
}

func (scheduler *Scheduler) scheduleAt(ctx context.Context, now time.Time) error {
	scheduler.mutex.Lock()
	defer scheduler.mutex.Unlock()

	due := make([]serviceconfig.Service, 0)
	for _, service := range scheduler.state.EnabledServices() {
		interval := time.Duration(service.IntervalSeconds) * time.Second
		if interval <= 0 {
			interval = time.Minute
		}
		if last, exists := scheduler.lastScheduled[service.ID]; !exists || now.Sub(last) >= interval {
			due = append(due, service)
		}
	}
	if len(due) == 0 {
		return nil
	}
	if err := scheduler.publisher.PublishJobs(ctx, due); err != nil {
		return fmt.Errorf("publish scheduled jobs: %w", err)
	}
	for _, service := range due {
		scheduler.lastScheduled[service.ID] = now
	}
	return nil
}
