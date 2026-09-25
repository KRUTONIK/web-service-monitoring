package monitoring

import (
	"errors"
	"time"
)

var ErrNotFound = errors.New("check result not found")

type Result struct {
	ServiceURL     string    `json:"service_url"`
	CheckedAt      time.Time `json:"checked_at"`
	Available      bool      `json:"available"`
	StatusCode     int       `json:"status_code"`
	ResponseTimeMS int64     `json:"response_time_ms"`
	Error          string    `json:"error,omitempty"`
}
