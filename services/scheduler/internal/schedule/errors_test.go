package schedule

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/KRUTONIK/web-service-monitoring/services/scheduler/internal/serviceconfig"
)

type failingPublisher struct{ err error }

func (p failingPublisher) PublishJobs(context.Context, []serviceconfig.Service) error {
	return p.err
}

func TestSchedulePropagatesPublishError(t *testing.T) {
	expected := errors.New("broker unavailable")
	err := New(serviceconfig.NewState(), failingPublisher{err: expected}).Schedule(context.Background())
	if !errors.Is(err, expected) {
		t.Fatalf("expected wrapped publishing error, got %v", err)
	}
	if !strings.Contains(err.Error(), "publish scheduled jobs") {
		t.Fatalf("missing operation context: %v", err)
	}
}
