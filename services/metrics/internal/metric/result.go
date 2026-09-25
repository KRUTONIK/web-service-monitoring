package metric

import "time"

type Result struct {
	ServiceURL     string
	CheckedAt      time.Time
	Available      bool
	StatusCode     int
	ResponseTimeMS int64
	Error          string
}
