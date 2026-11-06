package jobs

import (
	"testing"
	"time"
)

func TestNewCheckJobUsesStableID(t *testing.T) {
	scheduledAt := time.Date(2026, time.October, 11, 12, 0, 0, 0, time.FixedZone("UTC+4", 4*60*60))
	first := NewCheckJob("service-1", "https://service.test", 5, scheduledAt)
	second := NewCheckJob("service-1", "https://service.test", 5, scheduledAt.UTC())
	if first.JobID == "" || first.JobID != second.JobID {
		t.Fatalf("job IDs must be stable: %q and %q", first.JobID, second.JobID)
	}
	if !first.ScheduledAt.Equal(scheduledAt.UTC()) {
		t.Fatalf("scheduled time = %s, want %s", first.ScheduledAt, scheduledAt.UTC())
	}
}

func TestNewCheckJobChangesIDForAnotherSchedule(t *testing.T) {
	first := NewCheckJob("service-1", "https://service.test", 5, time.Unix(1, 0))
	second := NewCheckJob("service-1", "https://service.test", 5, time.Unix(2, 0))
	if first.JobID == second.JobID {
		t.Fatal("different scheduled checks must have different IDs")
	}
}

func TestIDChangesOnlyWhenItsIdentityInputsChange(t *testing.T) {
	at := time.Date(2026, time.November, 5, 12, 0, 0, 0, time.UTC)
	base := ID("service-1", "https://service.test", at)
	for _, test := range []struct {
		name string
		id   string
		url  string
		at   time.Time
	}{
		{name: "different service", id: "service-2", url: "https://service.test", at: at},
		{name: "different URL", id: "service-1", url: "https://other.test", at: at},
		{name: "different time", id: "service-1", url: "https://service.test", at: at.Add(time.Nanosecond)},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := ID(test.id, test.url, test.at); got == base {
				t.Fatal("changed job identity input produced the same ID")
			}
		})
	}
}
