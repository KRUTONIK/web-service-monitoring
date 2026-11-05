package notifications

import "time"

// Event is the immutable payload delivered to the notification service.
type Event struct {
	ID             string    `json:"id"`
	Type           string    `json:"type"`
	ServiceID      string    `json:"service_id"`
	ServiceName    string    `json:"service_name"`
	ServiceURL     string    `json:"service_url"`
	Recipient      string    `json:"recipient"`
	OccurredAt     time.Time `json:"occurred_at"`
	StartedAt      time.Time `json:"started_at"`
	StatusCode     int       `json:"status_code,omitempty"`
	ResponseTimeMS int64     `json:"response_time_ms,omitempty"`
	Error          string    `json:"error,omitempty"`
	WebURL         string    `json:"web_url,omitempty"`
}
