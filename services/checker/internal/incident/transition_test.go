package incident

import (
	"testing"
	"time"

	contracts "github.com/KRUTONIK/web-service-monitoring/contracts/incidents"
	"github.com/KRUTONIK/web-service-monitoring/services/checker/internal/check"
)

func TestTransitionEmitsOnlyAvailabilityChanges(t *testing.T) {
	checkedAt := time.Date(2026, time.November, 3, 12, 0, 0, 0, time.UTC)
	previous := check.Result{ServiceID: "service-1", Available: true}
	failed := check.Result{JobID: "job-failed", ServiceID: "service-1", Available: false, CheckedAt: checkedAt}

	firstFailure := Transition(previous, false, failed)
	if firstFailure == nil || firstFailure.Type != contracts.Started || firstFailure.ID != "job-failed:incident.started" {
		t.Fatalf("first failed result event = %#v", firstFailure)
	}
	started := Transition(previous, true, failed)
	if started == nil || started.Type != contracts.Started || started.ID != "job-failed:incident.started" || !started.OccurredAt.Equal(checkedAt) {
		t.Fatalf("started event = %#v", started)
	}
	if event := Transition(failed, true, failed); event != nil {
		t.Fatalf("unchanged result produced event: %#v", event)
	}
	recovered := check.Result{JobID: "job-recovered", ServiceID: "service-1", Available: true, CheckedAt: checkedAt.Add(time.Minute)}
	resolved := Transition(failed, true, recovered)
	if resolved == nil || resolved.Type != contracts.Resolved || resolved.ID != "job-recovered:incident.resolved" {
		t.Fatalf("resolved event = %#v", resolved)
	}
}
