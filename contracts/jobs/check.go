package jobs

import (
	"crypto/sha256"
	"fmt"
	"time"
)

// CheckJob is a durable RabbitMQ message consumed by Checker workers.
type CheckJob struct {
	JobID       string    `json:"job_id"`
	ServiceID   string    `json:"service_id"`
	ServiceURL  string    `json:"service_url"`
	ScheduledAt time.Time `json:"scheduled_at"`
}

func NewCheckJob(serviceID string, serviceURL string, scheduledAt time.Time) CheckJob {
	scheduledAt = scheduledAt.UTC()
	return CheckJob{
		JobID:       ID(serviceID, serviceURL, scheduledAt),
		ServiceID:   serviceID,
		ServiceURL:  serviceURL,
		ScheduledAt: scheduledAt,
	}
}

// ID is stable for one service scheduled at one exact instant.
func ID(serviceID string, serviceURL string, scheduledAt time.Time) string {
	value := fmt.Sprintf("%s\x00%s\x00%d", serviceID, serviceURL, scheduledAt.UTC().UnixNano())
	sum := sha256.Sum256([]byte(value))
	return fmt.Sprintf("%x", sum[:])
}
