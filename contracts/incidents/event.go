package incidents

import "time"

const (
	Started  = "incident.started"
	Resolved = "incident.resolved"
)

// Event — неизменяемое сообщение о переходе доступности сервиса.
type Event struct {
	ID             string    `json:"id"`
	Type           string    `json:"type"`
	ServiceID      string    `json:"service_id"`
	OccurredAt     time.Time `json:"occurred_at"`
	StatusCode     int       `json:"status_code,omitempty"`
	ResponseTimeMS int64     `json:"response_time_ms,omitempty"`
	Error          string    `json:"error,omitempty"`
}
