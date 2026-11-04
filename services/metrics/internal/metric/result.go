package metric

import "time"

type Result struct {
	ServiceID      string
	ServiceURL     string
	CheckedAt      time.Time
	Available      bool
	StatusCode     int
	ResponseTimeMS int64
	Error          string
}

type Summary struct {
	TotalChecks, SuccessfulChecks, P95ResponseMS int64
	HasResponseMetrics                           bool
	AverageResponseMS                            float64
}
