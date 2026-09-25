package check

import (
	"context"
	"net/http"
	"time"
)

type Result struct {
	ServiceURL     string    `json:"service_url"`
	CheckedAt      time.Time `json:"checked_at"`
	Available      bool      `json:"available"`
	StatusCode     int       `json:"status_code"`
	ResponseTimeMS int64     `json:"response_time_ms"`
	Error          string    `json:"error,omitempty"`
}

type Checker struct {
	client *http.Client
}

func New(client *http.Client) *Checker {
	return &Checker{client: client}
}

func (checker *Checker) Run(ctx context.Context, serviceURL string) Result {
	result := Result{
		ServiceURL: serviceURL,
		CheckedAt:  time.Now().UTC(),
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, serviceURL, nil)
	if err != nil {
		result.Error = err.Error()
		return result
	}

	startedAt := time.Now()
	response, err := checker.client.Do(request)
	result.ResponseTimeMS = time.Since(startedAt).Milliseconds()
	if err != nil {
		result.Error = err.Error()
		return result
	}
	defer func() { _ = response.Body.Close() }()

	result.StatusCode = response.StatusCode
	result.Available = response.StatusCode >= http.StatusOK && response.StatusCode < http.StatusMultipleChoices

	return result
}
