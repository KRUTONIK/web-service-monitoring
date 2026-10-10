package check

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"
)

var (
	ErrMissingServiceID  = errors.New("service ID is required")
	ErrMissingHost       = errors.New("service URL host is required")
	ErrUnsupportedScheme = errors.New("service URL scheme must be http or https")
)

type Result struct {
	ServiceID      string    `json:"service_id"`
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

func (checker *Checker) Run(ctx context.Context, serviceID string, serviceURL string) Result {
	result := Result{
		ServiceID:  serviceID,
		ServiceURL: serviceURL,
		CheckedAt:  time.Now().UTC(),
	}
	if serviceID == "" {
		result.Error = ErrMissingServiceID.Error()
		return result
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, serviceURL, nil)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	if request.URL.Host == "" {
		result.Error = ErrMissingHost.Error()
		return result
	}
	if request.URL.Scheme != "http" && request.URL.Scheme != "https" {
		result.Error = fmt.Errorf("%w: %s", ErrUnsupportedScheme, request.URL.Scheme).Error()
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
