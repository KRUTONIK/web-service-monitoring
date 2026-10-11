package jobs

import (
	"testing"
	"time"
)

func TestNewCheckJobUsesStableID(t *testing.T) {
	scheduledAt := time.Date(2026, time.October, 11, 12, 0, 0, 0, time.FixedZone("UTC+4", 4*60*60))
	first := NewCheckJob("service-1", "https://service.test", scheduledAt)
	second := NewCheckJob("service-1", "https://service.test", scheduledAt.UTC())
	if first.JobID == "" || first.JobID != second.JobID {
		t.Fatalf("job IDs must be stable: %q and %q", first.JobID, second.JobID)
	}
	if !first.ScheduledAt.Equal(scheduledAt.UTC()) {
		t.Fatalf("scheduled time = %s, want %s", first.ScheduledAt, scheduledAt.UTC())
	}
}

func TestNewCheckJobChangesIDForAnotherSchedule(t *testing.T) {
	first := NewCheckJob("service-1", "https://service.test", time.Unix(1, 0))
	second := NewCheckJob("service-1", "https://service.test", time.Unix(2, 0))
	if first.JobID == second.JobID {
		t.Fatal("different scheduled checks must have different IDs")
	}
}
