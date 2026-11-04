package messaging

import (
	"testing"
	"time"

	contracts "github.com/KRUTONIK/web-service-monitoring/contracts/incidents"
)

func TestValidIncidentEvent(t *testing.T) {
	now := time.Now().UTC()
	valid := contracts.Event{ID: "event-1", ServiceID: "service-1", Type: contracts.Started, OccurredAt: now}
	if !validIncidentEvent(valid) {
		t.Fatal("valid event was rejected")
	}

	for _, event := range []contracts.Event{
		{ID: "event", ServiceID: "service", Type: "unknown", OccurredAt: now},
		{ID: "event", ServiceID: "service", Type: contracts.Started},
		{ID: "", ServiceID: "service", Type: contracts.Started, OccurredAt: now},
	} {
		if validIncidentEvent(event) {
			t.Fatalf("invalid event accepted: %#v", event)
		}
	}
}
