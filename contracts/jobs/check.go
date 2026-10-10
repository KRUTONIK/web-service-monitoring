package jobs

import "time"

// CheckJob is a durable RabbitMQ message consumed by Checker workers.
type CheckJob struct {
	ServiceID   string    `json:"service_id"`
	ServiceURL  string    `json:"service_url"`
	ScheduledAt time.Time `json:"scheduled_at"`
}
