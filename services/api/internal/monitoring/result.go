package monitoring

import (
	"errors"
	"time"
)

var ErrNotFound = errors.New("check result not found")

type Result struct {
	ServiceID      string    `json:"service_id"`
	ServiceURL     string    `json:"service_url"`
	CheckedAt      time.Time `json:"checked_at"`
	Available      bool      `json:"available"`
	StatusCode     int       `json:"status_code"`
	ResponseTimeMS int64     `json:"response_time_ms"`
	Error          string    `json:"error,omitempty"`
}

type History struct {
	Checks []Result `json:"checks"`
	Limit  int      `json:"limit"`
	Offset int      `json:"offset"`
}

type Summary struct {
	TotalChecks       int64   `json:"total_checks"`
	SuccessfulChecks  int64   `json:"successful_checks"`
	SuccessRate       float64 `json:"success_rate"`
	AverageResponseMS float64 `json:"average_response_ms,omitempty"`
	P95ResponseMS     int64   `json:"p95_response_ms,omitempty"`
	IncidentCount     int     `json:"incident_count"`
	DowntimeSeconds   int64   `json:"downtime_seconds"`
}
