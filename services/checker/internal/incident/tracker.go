package incident

import (
	"fmt"

	contracts "github.com/KRUTONIK/web-service-monitoring/contracts/incidents"
	"github.com/KRUTONIK/web-service-monitoring/services/checker/internal/check"
)

// Transition returns an event only for a confirmed availability transition.
func Transition(previous check.Result, hasPrevious bool, current check.Result) *contracts.Event {
	if !hasPrevious || previous.Available == current.Available {
		return nil
	}
	eventType := contracts.Resolved
	if !current.Available {
		eventType = contracts.Started
	}
	return &contracts.Event{ID: fmt.Sprintf("%s:%s", current.JobID, eventType), Type: eventType, ServiceID: current.ServiceID, OccurredAt: current.CheckedAt}
}
