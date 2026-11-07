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
	nextScheduled map[string]time.Time
	intervals     map[string]time.Duration
}

func New(state *serviceconfig.State, publisher jobPublisher) *Scheduler {
	return &Scheduler{
		state:         state,
		publisher:     publisher,
		nextScheduled: make(map[string]time.Time),
		intervals:     make(map[string]time.Duration),
	}
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
		if previous, exists := scheduler.intervals[service.ID]; exists && previous != interval {
			scheduler.nextScheduled[service.ID] = now
		}
		scheduler.intervals[service.ID] = interval

		next, exists := scheduler.nextScheduled[service.ID]
		if !exists {
			next = now
		}
		if now.Before(next) {
			continue
		}
		if missed := now.Sub(next) / interval; missed > 0 {
			next = next.Add(missed * interval)
		}
		service.ScheduledAt = next
		due = append(due, service)
	}
	if len(due) == 0 {
		return nil
	}
	if err := scheduler.publisher.PublishJobs(ctx, due); err != nil {
		return fmt.Errorf("publish scheduled jobs: %w", err)
	}
	for _, service := range due {
		scheduler.nextScheduled[service.ID] = service.ScheduledAt.Add(scheduler.intervals[service.ID])
	}
	return nil
}
