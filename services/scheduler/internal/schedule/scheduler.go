package schedule

import (
	"context"
	"fmt"

	"github.com/KRUTONIK/web-service-monitoring/services/scheduler/internal/serviceconfig"
)

type jobPublisher interface {
	PublishJobs(context.Context, []serviceconfig.Service) error
}

type Scheduler struct {
	state     *serviceconfig.State
	publisher jobPublisher
}

func New(state *serviceconfig.State, publisher jobPublisher) *Scheduler {
	return &Scheduler{state: state, publisher: publisher}
}

func (scheduler *Scheduler) Schedule(ctx context.Context) error {
	if err := scheduler.publisher.PublishJobs(ctx, scheduler.state.EnabledServices()); err != nil {
		return fmt.Errorf("publish scheduled jobs: %w", err)
	}
	return nil
}
